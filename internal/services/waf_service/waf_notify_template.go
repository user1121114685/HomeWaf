// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/zlog"
	"bytes"
	"errors"
	"fmt"
	"html"
	"strings"
	"text/template"
	"time"
)

/*
*
通知消息模板引擎（issue #822）

三条硬约束：

 1. 模板渲染失败绝不能让告警消失 —— 一律降级回内置默认格式，只在日志里标记 custom_fallback。
 2. 模板变量（Url / RuleInfo / UserAgent / Message）是攻击者可控输入，
    直接塞进 Markdown / 邮件 HTML 会破坏排版甚至带出可执行内容，所以按渠道逐个转义 + 截断。
 3. 只用标准库 text/template，不引入任何模板/表达式依赖，
    funcMap 是白名单，不暴露任何 IO 能力。
*/

const (
	notifyVarMaxLen      = 512         // 单个变量最大长度
	notifyRenderMaxBytes = 64 * 1024   // 渲染输出上限
	notifyRenderTimeout  = time.Second // 渲染超时
	notifyContentMaxLen  = 4096        // 最终正文长度上限
	notifyTitleMaxLen    = 200         // 最终标题长度上限
	notifyTruncatedMark  = "...(内容过长已截断)"
)

// notifyTemplateFuncs 模板可用函数白名单
var notifyTemplateFuncs = template.FuncMap{
	"default": func(def, s string) string {
		if strings.TrimSpace(s) == "" {
			return def
		}
		return s
	},
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"truncate": func(n int, s string) string {
		if n <= 0 || len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"contains": strings.Contains,
}

// RenderNotifyMessage 渲染一条通知的标题与正文
//
// 返回 templateUsed 标明本次用的是内置默认还是自定义模板（渲染失败会是 custom_fallback），
// 管理端据此在订阅格子上提示"模板渲染失败"。
func RenderNotifyMessage(sub model.NotifySubscription, channelType string, ev NotifyEvent) (title, content, templateUsed string) {
	title, content, templateUsed = ev.DefaultTitle, ev.DefaultContent, model.TemplateUsedDefault
	if channelType == "bark" {
		content = PlainNotifyContent(content)
	}

	titleTpl := strings.TrimSpace(sub.TitleTemplate)
	contentTpl := strings.TrimSpace(sub.ContentTemplate)
	if titleTpl == "" && contentTpl == "" {
		return title, truncateText(content, notifyContentMaxLen), templateUsed
	}

	vars := prepareTemplateVars(channelType, ev)
	templateUsed = model.TemplateUsedCustom

	if titleTpl != "" {
		if rendered, err := renderTemplateSafely("title", titleTpl, vars); err == nil {
			title = rendered
		} else {
			zlog.Warn("通知标题模板渲染失败，已降级为默认格式", "subscription", sub.Id, "error", err.Error())
			templateUsed = model.TemplateUsedFallback
		}
	}
	if contentTpl != "" {
		if rendered, err := renderTemplateSafely("content", contentTpl, vars); err == nil {
			content = rendered
		} else {
			zlog.Warn("通知正文模板渲染失败，已降级为默认格式", "subscription", sub.Id, "error", err.Error())
			templateUsed = model.TemplateUsedFallback
		}
	}

	return truncateText(title, notifyTitleMaxLen), truncateText(content, notifyContentMaxLen), templateUsed
}

// PlainNotifyContent strips only built-in field labels, preserving their values.
func PlainNotifyContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if label, value, ok := strings.Cut(line, ":** "); ok && strings.HasPrefix(label, "**") {
			lines[i] = strings.TrimPrefix(label, "**") + "：" + value
		}
	}
	return strings.Join(lines, "\n")
}

// prepareTemplateVars 变量转义 + 截断，得到可安全塞进模板的数据
func prepareTemplateVars(channelType string, ev NotifyEvent) map[string]string {
	out := make(map[string]string, len(ev.Vars))
	for k, v := range ev.Vars {
		out[k] = escapeForChannel(channelType, truncateText(v, notifyVarMaxLen))
	}
	return out
}

// escapeForChannel 按渠道转义变量
//
// 邮件最终会以 HTML 呈现，必须做实体转义；IM 渠道走 Markdown，转义会破坏排版的元字符。
func escapeForChannel(channelType, s string) string {
	if s == "" {
		return s
	}
	switch channelType {
	case "email":
		return html.EscapeString(s)
	case "webhook":
		// 通用 Webhook 的对端格式未知（可能是 JSON 字段、纯文本、也可能是别的），
		// 在这里做 Markdown 转义只会把反斜杠原样发给对方；真正的转义按 Content-Type
		// 在 wafnotify/webhook 渲染报文时做。
		return s
	case "bark", "telegram", "gotify", "socket", "wechatwork_app":
		// These providers receive plain text or perform their own format-specific
		// escaping. Markdown escaping here would appear literally (or be escaped
		// a second time by Telegram).
		return s
	default:
		return escapeMarkdown(s)
	}
}

// escapeMarkdown 转义 Markdown 结构字符，防止攻击 payload 把消息排版打乱
//
// 只处理会改变结构的几个字符；不做全量转义，否则正常的 URL 会变得难以阅读。
var markdownEscaper = strings.NewReplacer(
	"`", "'",
	"*", "\\*",
	"_", "\\_",
	"[", "\\[",
	"]", "\\]",
	"\r", " ",
)

func escapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

// truncateText 按字节截断，并保证不切断 UTF-8 字符
func truncateText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !isUTF8Start(s[cut]) {
		cut--
	}
	return s[:cut] + notifyTruncatedMark
}

func isUTF8Start(b byte) bool {
	return b&0xC0 != 0x80
}

// renderTemplateSafely 带超时与输出上限的模板渲染
func renderTemplateSafely(name, tpl string, vars map[string]string) (string, error) {
	tmpl, err := template.New(name).Funcs(notifyTemplateFuncs).Option("missingkey=zero").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("模板语法错误: %w", err)
	}

	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: fmt.Errorf("模板渲染 panic: %v", r)}
			}
		}()
		var buf bytes.Buffer
		lw := &limitedWriter{buf: &buf, limit: notifyRenderMaxBytes}
		if execErr := tmpl.Execute(lw, vars); execErr != nil {
			ch <- result{err: execErr}
			return
		}
		ch <- result{out: buf.String()}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return "", r.err
		}
		if strings.TrimSpace(r.out) == "" {
			return "", errors.New("模板渲染结果为空")
		}
		return r.out, nil
	case <-time.After(notifyRenderTimeout):
		return "", errors.New("模板渲染超时")
	}
}

// limitedWriter 输出超过上限即报错，防止模板产出巨量内容
type limitedWriter struct {
	buf     *bytes.Buffer
	limit   int
	written int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		return 0, errors.New("模板渲染输出超出上限")
	}
	w.written += len(p)
	return w.buf.Write(p)
}

// ========== 变量说明表（前端"点击插入变量"用） ==========

// NotifyTemplateVar 模板变量说明
type NotifyTemplateVar struct {
	Name    string `json:"name"`    // 变量名，如 Domain
	Desc    string `json:"desc"`    // 中文说明
	Example string `json:"example"` // 示例值
}

var notifyCommonVars = []NotifyTemplateVar{
	{Name: "Time", Desc: "事件时间", Example: "2026-08-05 10:00:00"},
	{Name: "ServerName", Desc: "本机名称", Example: "HomeWaf"},
	{Name: "MessageTypeName", Desc: "消息类型名称", Example: "攻击信息"},
	{Name: "Severity", Desc: "严重级别", Example: "critical"},
}

var notifyTypeVars = map[string][]NotifyTemplateVar{
	model.MSG_TYPE_RULE_TRIGGER: {
		{Name: "RequestID", Desc: "请求编号", Example: "31924a4e9db54ab3a4b384b67ed2b13d"},
		{Name: "Outcome", Desc: "实际处理结果", Example: "请求被拒绝（HTTP 403）"},
		{Name: "OperaType", Desc: "操作类型", Example: "命中保护规则"},
		{Name: "Server", Desc: "服务器", Example: "samwaf-01"},
		{Name: "Domain", Desc: "域名", Example: "www.example.com"},
		{Name: "RuleInfo", Desc: "规则信息", Example: "SQL注入检测"},
		{Name: "Ip", Desc: "来源IP", Example: "1.2.3.4"},
	},
	model.MSG_TYPE_ATTACK_INFO: {
		{Name: "AttackType", Desc: "攻击类型", Example: "SQL注入"},
		{Name: "Url", Desc: "攻击URL", Example: "https://www.example.com/api?id=1"},
		{Name: "Ip", Desc: "攻击IP", Example: "1.2.3.4"},
	},
	model.MSG_TYPE_IP_BAN: {
		{Name: "Permanent", Desc: "是否永久封禁（true/false）", Example: "false"},
		{Name: "Scope", Desc: "封禁生效范围", Example: "port:15000"},
		{Name: "DurationSeconds", Desc: "实际封禁时长（秒）", Example: "1800"},
		{Name: "Ip", Desc: "被封禁IP", Example: "1.2.3.4"},
		{Name: "Reason", Desc: "封禁原因", Example: "CC攻击"},
		{Name: "Duration", Desc: "封禁时长(分钟)", Example: "30"},
		{Name: "Remaining", Desc: "剩余时间", Example: "29分30秒"},
	},
	model.MSG_TYPE_SSL_EXPIRE: {
		{Name: "Endpoint", Desc: "外部证书检查的域名及端口", Example: "www.example.com:8443"},
		{Name: "Hint", Desc: "处理提示（如有）", Example: "请检查续期与部署状态"},
		{Name: "Server", Desc: "来源实例", Example: "HomeWaf"},
		{Name: "Domain", Desc: "域名", Example: "www.example.com"},
		{Name: "ExpireTime", Desc: "过期时间", Example: "2026-09-01 00:00:00"},
		{Name: "DaysLeft", Desc: "剩余天数", Example: "7"},
	},
	model.MSG_TYPE_USER_LOGIN: {
		{Name: "Location", Desc: "本次登录归属地", Example: "内网"},
		{Name: "Username", Desc: "用户名", Example: "admin"},
		{Name: "Ip", Desc: "登录IP", Example: "192.168.1.10"},
	},
	model.MSG_TYPE_MANAGE_LOGIN_ABNORMAL: {
		{Name: "Location", Desc: "本次登录归属地", Example: "内网"},
		{Name: "Username", Desc: "用户名", Example: "admin"},
		{Name: "Ip", Desc: "本次登录IP", Example: "192.0.2.1"},
		{Name: "LastIp", Desc: "上次登录IP", Example: "192.168.1.10"},
		{Name: "LastLocation", Desc: "上次登录归属地", Example: "内网IP"},
		{Name: "LastTime", Desc: "上次登录时间", Example: "2026-08-05 09:12:30"},
	},
	model.MSG_TYPE_WEEKLY_REPORT: {
		{Name: "TotalRequests", Desc: "总请求数", Example: "120000"},
		{Name: "BlockedRequests", Desc: "拦截请求数", Example: "356"},
		{Name: "BlockRate", Desc: "拦截率", Example: "0.30%"},
		{Name: "WeekRange", Desc: "统计周期", Example: "2026-07-29 ~ 2026-08-04"},
	},
	model.MSG_TYPE_SYSTEM_ERROR: {
		{Name: "RequestID", Desc: "关联请求编号（请求诊断事件）", Example: "31924a4e9db54ab3a4b384b67ed2b13d"},
		{Name: "ErrorType", Desc: "错误类型", Example: "数据库连接失败"},
		{Name: "ErrorMsg", Desc: "错误信息", Example: "connection refused"},
	},
	model.MSG_TYPE_OPERATION_NOTICE: {
		{Name: "OperaType", Desc: "操作类型", Example: "新增网站"},
		{Name: "Server", Desc: "服务器", Example: "samwaf-01"},
		{Name: "OperaCnt", Desc: "操作内容", Example: "www.example.com"},
	},
	model.MSG_TYPE_SOFTWARE_UPDATE: {
		{Name: "CurrentVersion", Desc: "当前版本", Example: "v2.0.1"},
		{Name: "NewVersion", Desc: "可用版本", Example: "v2.0.2"},
	},
	model.MSG_TYPE_ACCESS_LOGIN: {
		{Name: "EventName", Desc: "事件名称", Example: "访问认证登录成功"},
		{Name: "AccountName", Desc: "访问账号", Example: "zhangsan"},
		{Name: "Host", Desc: "访问域名", Example: "oa.example.com"},
		{Name: "Url", Desc: "访问地址", Example: "https://oa.example.com/"},
		{Name: "Ip", Desc: "来源IP", Example: "1.2.3.4"},
		{Name: "Location", Desc: "IP归属地", Example: "浙江杭州"},
		{Name: "Message", Desc: "事件详情", Example: "登录成功"},
	},
}

// GetNotifyTemplateVars 取某消息类型可用的模板变量（公共变量 + 类型专属变量）
func GetNotifyTemplateVars(messageType string) []NotifyTemplateVar {
	vars := make([]NotifyTemplateVar, 0, 12)
	vars = append(vars, notifyCommonVars...)
	if messageType == model.MSG_TYPE_ACCESS_ABNORMAL {
		messageType = model.MSG_TYPE_ACCESS_LOGIN // 两者变量集合一致
	}
	if typeVars, ok := notifyTypeVars[messageType]; ok {
		vars = append(vars, typeVars...)
	}
	if messageType == model.MSG_TYPE_SSL_EXPIRE || messageType == model.MSG_TYPE_OPERATION_NOTICE {
		vars = append(vars, NotifyTemplateVar{Name: "CertificateID", Desc: "证书生命周期事件的证书编号"},
			NotifyTemplateVar{Name: "OrderID", Desc: "证书事件的关联订单（如有）"},
			NotifyTemplateVar{Name: "CertificateEvent", Desc: "证书生命周期事件类型"})
	}
	return vars
}

// ========== 样例事件（预览 / 测试发送 / 干跑 用） ==========

// SampleNotifyEvent 按消息类型构造一条样例事件
//
// 预览和测试都必须用真实的渲染链路，否则"预览没问题、真发出去是乱的"就毫无意义。
func SampleNotifyEvent(messageType string) NotifyEvent {
	vars := map[string]string{}
	for _, v := range GetNotifyTemplateVars(messageType) {
		vars[v.Name] = v.Example
	}
	now := time.Now().Format(time.RFC3339)
	base := innerbean.BaseMessageInfo{OperaType: vars["OperaType"], Server: vars["ServerName"]}
	var msg any
	switch messageType {
	case model.MSG_TYPE_RULE_TRIGGER:
		msg = innerbean.RuleMessageInfo{BaseMessageInfo: base, Domain: vars["Domain"], RuleInfo: vars["RuleInfo"], Ip: vars["Ip"], RequestID: vars["RequestID"], Outcome: vars["Outcome"], Time: now}
	case model.MSG_TYPE_OPERATION_NOTICE:
		msg = innerbean.OperatorMessageInfo{BaseMessageInfo: base, OperaCnt: vars["OperaCnt"], Time: now}
	case model.MSG_TYPE_SOFTWARE_UPDATE:
		return SoftwareUpdateEvent(vars["CurrentVersion"], vars["NewVersion"])
	case model.MSG_TYPE_USER_LOGIN, model.MSG_TYPE_MANAGE_LOGIN_ABNORMAL:
		msg = innerbean.UserLoginMessageInfo{BaseMessageInfo: base, Username: vars["Username"], Ip: vars["Ip"], Location: vars["Location"], Time: now, Abnormal: messageType == model.MSG_TYPE_MANAGE_LOGIN_ABNORMAL, LastIp: vars["LastIp"], LastLocation: vars["LastLocation"], LastTime: vars["LastTime"]}
	case model.MSG_TYPE_ATTACK_INFO:
		msg = innerbean.AttackInfoMessageInfo{BaseMessageInfo: base, AttackType: vars["AttackType"], Url: vars["Url"], Ip: vars["Ip"], Time: now}
	case model.MSG_TYPE_WEEKLY_REPORT:
		msg = innerbean.WeeklyReportMessageInfo{BaseMessageInfo: base, TotalRequests: 120000, BlockedRequests: 356, WeekRange: vars["WeekRange"]}
	case model.MSG_TYPE_SSL_EXPIRE:
		msg = innerbean.SSLExpireMessageInfo{BaseMessageInfo: base, Domain: vars["Domain"], ExpireTime: vars["ExpireTime"], DaysLeft: 7, Hint: vars["Hint"], Endpoint: vars["Endpoint"]}
	case model.MSG_TYPE_SYSTEM_ERROR:
		msg = innerbean.SystemErrorMessageInfo{BaseMessageInfo: base, ErrorType: vars["ErrorType"], ErrorMsg: vars["ErrorMsg"], Time: now}
	case model.MSG_TYPE_IP_BAN:
		msg = innerbean.IPBanMessageInfo{BaseMessageInfo: base, Ip: vars["Ip"], Reason: vars["Reason"], Time: now, Scope: vars["Scope"], Duration: 30, RemainingSeconds: 1770}
	case model.MSG_TYPE_ACCESS_LOGIN, model.MSG_TYPE_ACCESS_ABNORMAL:
		abnormal := messageType == model.MSG_TYPE_ACCESS_ABNORMAL
		event := model.AccessEventLoginOK
		if abnormal {
			event = model.AccessEventBadReturnTo
		}
		msg = innerbean.AccessMessageInfo{BaseMessageInfo: base, Event: event, EventName: model.AccessEventName(event), AccountName: vars["AccountName"], Host: vars["Host"], Url: vars["Url"], Ip: vars["Ip"], Location: vars["Location"], Message: vars["Message"], Time: now, Abnormal: abnormal}
	default:
		return NotifyEvent{}
	}
	event := WafNotifySenderServiceApp.BuildNotifyEvent(msg)
	event.Vars["ServerName"] = base.Server
	return event
}
