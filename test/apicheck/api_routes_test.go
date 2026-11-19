// Isolated route registration and unauthenticated-response integration checks.
package apicheck

import (
	"HomeWaf/api"
	"HomeWaf/global"
	"HomeWaf/wafmangeweb"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// 配置入口：修改这里即可
// ─────────────────────────────────────────────────────────────────────────────

func fixtureURL(t *testing.T) string {
	t.Helper()
	oldRef, oldRelease := api.GinEngineRef, global.GWAF_RELEASE
	global.GWAF_RELEASE = "false"
	t.Cleanup(func() { api.GinEngineRef = oldRef; global.GWAF_RELEASE = oldRelease })
	r := gin.New()
	r.Use(gin.Recovery())
	_ = r.SetTrustedProxies(nil)
	(&wafmangeweb.WafWebManager{}).RegisterRoutes(r)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	return server.URL
}

// ─────────────────────────────────────────────────────────────────────────────
// 路由清单（与 @Router 注解保持一致）
// ─────────────────────────────────────────────────────────────────────────────

type route struct {
	method string
	path   string
	desc   string
}

var registeredRoutes = []route{
	{"GET", "/api/v1/pipeline/ports/index", "受保护服务列表"},
	{"GET", "/api/v1/pipeline/ports/detail", "服务详情"},
	{"POST", "/api/v1/pipeline/ports/draft", "新服务草稿"},
	{"POST", "/api/v1/pipeline/ports/check", "校验服务配置"},
	{"POST", "/api/v1/pipeline/ports/save", "保存服务策略"},
	{"POST", "/api/v1/pipeline/ports/access", "修改访问状态"},
	{"POST", "/api/v1/pipeline/ports/delete", "删除服务"},
	{"GET", "/api/v1/pipeline/state", "服务处罚记录"},
	{"POST", "/api/v1/pipeline/state/manage", "管理服务处罚"},
	{"GET", "/api/v1/pipeline/cc/hits", "CC 规则命中"},
	{"POST", "/api/v1/pipeline/cc/recommend", "推荐 CC 阈值"},
	{"POST", "/api/v1/wafhost/rule/format", "格式化规则"},
	{"POST", "/api/v1/wafhost/rule/test", "校验规则"},

	// ── IP 失败封禁 ────────────────────────────────────────────────

	// ── 防火墙 IP 封禁 ─────────────────────────────────────────────
	{"POST", "/api/v1/firewall/ipblock/add", "新增防火墙IP封禁"},
	{"POST", "/api/v1/firewall/ipblock/list", "获取防火墙IP封禁列表"},
	{"GET", "/api/v1/firewall/ipblock/del", "删除防火墙IP封禁"},
	{"POST", "/api/v1/firewall/ipblock/edit", "编辑防火墙IP封禁"},

	// ── 一键修改 ───────────────────────────────────────────────────
	{"GET", "/api/v1/wafhost/onekeymod/detail", "获取一键修改详情"},
	{"POST", "/api/v1/wafhost/onekeymod/list", "获取一键修改列表"},
	{"GET", "/api/v1/wafhost/onekeymod/del", "删除一键修改"},
	{"POST", "/api/v1/wafhost/onekeymod/doModify", "执行一键修改"},
	{"GET", "/api/v1/wafhost/onekeymod/restore", "还原一键修改"},

	// ── 任务管理 ───────────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/task/add", "新增任务"},
	{"GET", "/api/v1/wafhost/task/detail", "获取任务详情"},
	{"POST", "/api/v1/wafhost/task/list", "获取任务列表"},
	{"GET", "/api/v1/wafhost/task/del", "删除任务"},
	{"POST", "/api/v1/wafhost/task/edit", "编辑任务"},
	{"GET", "/api/v1/wafhost/task/manual_exec", "手动执行任务"},
	{"GET", "/api/v1/wafhost/task/log", "获取任务日志"},
	{"GET", "/api/v1/wafhost/task/log/clear", "清空任务日志"},

	// ── 数据保留策略 ───────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/dataretention/list", "获取数据保留策略列表"},
	{"GET", "/api/v1/wafhost/dataretention/detail", "获取数据保留策略详情"},
	{"POST", "/api/v1/wafhost/dataretention/edit", "编辑数据保留策略"},

	// ── 私有信息 ───────────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/privateinfo/add", "新增私有信息"},
	{"GET", "/api/v1/wafhost/privateinfo/detail", "获取私有信息详情"},
	{"POST", "/api/v1/wafhost/privateinfo/list", "获取私有信息列表"},
	{"GET", "/api/v1/wafhost/privateinfo/del", "删除私有信息"},
	{"POST", "/api/v1/wafhost/privateinfo/edit", "编辑私有信息"},

	// ── 私有分组 ───────────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/privategroup/add", "新增私有分组"},
	{"GET", "/api/v1/wafhost/privategroup/detail", "获取私有分组详情"},
	{"POST", "/api/v1/wafhost/privategroup/list", "获取私有分组列表"},
	{"POST", "/api/v1/wafhost/privategroup/listbybelongcloud", "按云获取私有分组列表"},
	{"GET", "/api/v1/wafhost/privategroup/del", "删除私有分组"},
	{"POST", "/api/v1/wafhost/privategroup/edit", "编辑私有分组"},

	// ── SSL 证书 ───────────────────────────────────────────────────
	{"POST", "/api/v1/sslconfig/add", "新增SSL证书"},
	{"GET", "/api/v1/sslconfig/detail", "获取SSL证书详情"},
	{"POST", "/api/v1/sslconfig/list", "获取SSL证书列表"},
	{"GET", "/api/v1/sslconfig/del", "删除SSL证书"},
	{"POST", "/api/v1/sslconfig/edit", "编辑SSL证书"},

	// ── SSL 订单 ───────────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/sslorder/add", "新增SSL订单"},
	{"GET", "/api/v1/wafhost/sslorder/detail", "获取SSL订单详情"},
	{"POST", "/api/v1/wafhost/sslorder/list", "获取SSL订单列表"},
	{"GET", "/api/v1/wafhost/sslorder/del", "删除SSL订单"},
	{"POST", "/api/v1/wafhost/sslorder/edit", "编辑SSL订单"},

	// ── SSL 到期监控 ───────────────────────────────────────────────
	{"POST", "/api/v1/wafhost/sslexpire/add", "新增SSL到期监控"},
	{"GET", "/api/v1/wafhost/sslexpire/detail", "获取SSL到期监控详情"},
	{"POST", "/api/v1/wafhost/sslexpire/list", "获取SSL到期监控列表"},
	{"GET", "/api/v1/wafhost/sslexpire/del", "删除SSL到期监控"},
	{"POST", "/api/v1/wafhost/sslexpire/edit", "编辑SSL到期监控"},
	{"GET", "/api/v1/wafhost/sslexpire/nowcheck", "立即检测SSL到期"},
	{"GET", "/api/v1/wafhost/sslexpire/sync_host", "从站点同步SSL监控"},

	// ── 隧道 ───────────────────────────────────────────────────────
	{"POST", "/api/v1/tunnel/tunnel/add", "新增隧道"},
	{"GET", "/api/v1/tunnel/tunnel/detail", "获取隧道详情"},
	{"POST", "/api/v1/tunnel/tunnel/list", "获取隧道列表"},
	{"GET", "/api/v1/tunnel/tunnel/del", "删除隧道"},
	{"POST", "/api/v1/tunnel/tunnel/edit", "编辑隧道"},
	{"GET", "/api/v1/tunnel/tunnel/connections", "获取隧道连接数"},

	// ── 通知渠道 ───────────────────────────────────────────────────
	{"POST", "/api/v1/notify/channel/add", "新增通知渠道"},
	{"GET", "/api/v1/notify/channel/detail", "获取通知渠道详情"},
	{"POST", "/api/v1/notify/channel/list", "获取通知渠道列表"},
	{"GET", "/api/v1/notify/channel/del", "删除通知渠道"},
	{"POST", "/api/v1/notify/channel/edit", "编辑通知渠道"},

	// ── 通知订阅 ───────────────────────────────────────────────────
	{"POST", "/api/v1/notify/subscription/add", "新增通知订阅"},
	{"POST", "/api/v1/notify/subscription/list", "获取通知订阅列表"},
	{"GET", "/api/v1/notify/subscription/del", "删除通知订阅"},

	// ── 通知日志 ───────────────────────────────────────────────────
	{"POST", "/api/v1/notify/log/list", "获取通知日志列表"},
	{"GET", "/api/v1/notify/log/detail", "获取通知日志详情"},
	{"GET", "/api/v1/notify/log/del", "删除通知日志"},

	// ── VIP 配置 ───────────────────────────────────────────────────
	{"POST", "/api/v1/vipconfig/updateIpWhitelist", "更新管理IP白名单"},
	{"GET", "/api/v1/vipconfig/getIpWhitelist", "获取管理IP白名单"},
	{"POST", "/api/v1/vipconfig/updateSslEnable", "更新SSL启用状态"},
	{"GET", "/api/v1/vipconfig/getSslStatus", "获取SSL启用状态"},
	{"GET", "/api/v1/vipconfig/getSecurityEntry", "获取安全入口"},
	{"POST", "/api/v1/vipconfig/updateSecurityEntry", "更新安全入口"},
	{"GET", "/api/v1/vipconfig/getNoticeTitle", "获取通知标题前缀"},
	{"POST", "/api/v1/vipconfig/updateNoticeTitle", "更新通知标题前缀"},

	// ── 系统配置 ───────────────────────────────────────────────────
	{"POST", "/api/v1/systemconfig/list", "获取系统配置列表"},
	{"POST", "/api/v1/systemconfig/edit", "更新系统配置"},
	{"POST", "/api/v1/systemconfig/editByItem", "通过item更新系统配置"},

	// ── 系统监控 ───────────────────────────────────────────────────
	{"GET", "/api/v1/monitor/system_info", "获取系统监控信息"},

	// ── 统计数据 ───────────────────────────────────────────────────
	{"GET", "/api/v1/wafstatsumday", "获取今日访问统计"},
	{"GET", "/api/v1/wafstatsumdayrange", "按日期范围统计访问量"},
	{"GET", "/api/v1/wafstatsumdaytopiprange", "统计攻击IP排行"},
	{"GET", "/api/v1/statsysinfo", "获取首页系统基本信息"},
	{"GET", "/api/v1/statrumtimesysinfo", "获取运行时系统基本信息"},
	{"GET", "/api/v1/wafstatsiteoverview", "站点综合概览统计"},
	{"GET", "/api/v1/wafstatsitedetail", "站点详情趋势查询"},

	// ── 系统信息 ───────────────────────────────────────────────────
	{"GET", "/api/v1/sysinfo/version", "获取版本信息"},
	{"GET", "/api/v1/sysinfo/runtimeinfo", "获取系统运行环境信息"},

	// ── 系统日志 ───────────────────────────────────────────────────
	{"GET", "/api/v1/sys_log/list", "获取系统日志列表"},
	{"GET", "/api/v1/sys_log/detail", "获取系统日志详情"},

	// ── 攻击日志 ───────────────────────────────────────────────────
	{"POST", "/api/v1/waflog/attack/list", "获取攻击日志列表"},
	{"GET", "/api/v1/waflog/attack/detail", "获取攻击日志详情"},

	// ── 日志文件 ───────────────────────────────────────────────────
	{"GET", "/api/v1/logfilewrite/preview", "预览日志文件"},
	{"GET", "/api/v1/logfilewrite/currentfile", "获取当前日志文件"},
	{"GET", "/api/v1/logfilewrite/backupfiles", "获取备份日志文件列表"},
	{"POST", "/api/v1/logfilewrite/clear", "清空日志文件"},
	{"GET", "/api/v1/logfilewrite/variables", "获取日志变量"},

	// ── WAF 引擎 ───────────────────────────────────────────────────
	{"POST", "/api/v1/pipeline/reload", "重新加载受保护服务"},

	// ── OWASP 规则管理 ─────────────────────────────────────────────
	{"GET", "/api/v1/owasp/rules", "获取OWASP规则列表"},
	{"GET", "/api/v1/owasp/rule_detail", "获取OWASP规则详情"},
	{"POST", "/api/v1/owasp/rule_toggle", "启用/禁用OWASP规则"},
	{"POST", "/api/v1/owasp/rule_override", "改写OWASP规则内容"},
	{"POST", "/api/v1/owasp/rule_reset", "还原OWASP规则为上游版本"},
	{"GET", "/api/v1/owasp/audit_log", "获取OWASP规则变更审计日志"},
	{"GET", "/api/v1/owasp/crs_vars", "获取所有自定义CRS事务变量"},
	{"POST", "/api/v1/owasp/crs_var", "设置单个CRS事务变量"},
	{"DELETE", "/api/v1/owasp/crs_var", "删除单个CRS事务变量"},
	{"GET", "/api/v1/owasp/base_config", "获取OWASP基线配置"},
	{"POST", "/api/v1/owasp/base_config", "更新OWASP基线配置"},
	{"GET", "/api/v1/owasp/tuning", "获取OWASP调参配置"},
	{"POST", "/api/v1/owasp/tuning", "更新OWASP调参配置"},
	{"POST", "/api/v1/owasp/reload", "手动触发WAF热重载"},
	{"GET", "/api/v1/owasp/files", "获取OWASP规则文件列表"},
	{"GET", "/api/v1/owasp/file_content", "获取OWASP规则文件原始内容"},
	{"POST", "/api/v1/owasp/test/dry_run", "OWASP沙盒测试"},
	{"GET", "/api/v1/owasp/update/check", "检查OWASP规则库是否有新版本"},
	{"POST", "/api/v1/owasp/update/apply", "执行OWASP规则库升级"},
	{"GET", "/api/v1/owasp/usage/doc", "获取OWASP规则管理使用说明"},
	{"GET", "/api/v1/owasp/hit_stats", "获取OWASP规则命中统计"},
	{"POST", "/api/v1/owasp/hit_stats/reset", "清空OWASP规则命中统计"},
}

// ─────────────────────────────────────────────────────────────────────────────

var httpClient = &http.Client{Timeout: 10 * time.Second}

// sendRequest 发起请求，GET/POST 均带 X-Token，POST body 为空 JSON
func sendRequest(method, url string) (*http.Response, error) {
	var body *bytes.Reader
	if method == "POST" {
		body = bytes.NewReader([]byte("{}"))
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	// No credentials: mutating handlers must stop at the real authentication boundary.
	req.Header.Set("Content-Type", "application/json")
	return httpClient.Do(req)
}

// checkJSON 判断 body 是否为合法 JSON（允许对象或数组）
func checkJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return false
	}
	var v json.RawMessage
	return json.Unmarshal(data, &v) == nil
}

// TestAllRoutes 逐条验证 @Router 中的路由是否注册可达（非 404）且响应为合法 JSON
func TestAllRoutes(t *testing.T) {
	baseURL := fixtureURL(t)

	type result struct {
		route
		status      int
		err         string
		notFound    bool // 路由不存在（404）
		notJSON     bool // Content-Type 不含 application/json
		invalidJSON bool // body 无法解析为 JSON
		passed      bool
	}

	results := make([]result, 0, len(registeredRoutes))
	failCount := 0

	for _, r := range registeredRoutes {
		url := baseURL + r.path
		resp, err := sendRequest(r.method, url)

		res := result{route: r}
		if err != nil {
			res.err = err.Error()
			res.passed = false
		} else {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			res.status = resp.StatusCode
			res.notFound = resp.StatusCode == http.StatusNotFound

			ct := resp.Header.Get("Content-Type")
			res.notJSON = !strings.Contains(ct, "application/json")
			res.invalidJSON = !checkJSON(bodyBytes)

			res.passed = !res.notFound && !res.notJSON && !res.invalidJSON
		}
		results = append(results, res)
		if !res.passed {
			failCount++
		}
	}

	// ── 打印汇总 ──────────────────────────────────────────────────
	t.Log(strings.Repeat("─", 90))
	t.Logf("%-6s %-50s %-6s %-16s %s", "方法", "路径", "状态", "问题", "描述")
	t.Log(strings.Repeat("─", 90))
	for _, res := range results {
		statusStr := fmt.Sprintf("%d", res.status)
		if res.err != "" {
			statusStr = "ERR"
		}
		mark := "✓"
		issue := "-"
		if !res.passed {
			mark = "✗"
			problems := []string{}
			if res.err != "" {
				problems = append(problems, "连接失败")
			}
			if res.notFound {
				problems = append(problems, "404")
			}
			if res.notJSON {
				problems = append(problems, "非JSON类型")
			}
			if res.invalidJSON {
				problems = append(problems, "JSON解析失败")
			}
			issue = strings.Join(problems, ",")
		}
		t.Logf("%s %-6s %-50s %-6s %-16s %s", mark, res.method, res.path, statusStr, issue, res.desc)
	}
	t.Log(strings.Repeat("─", 90))
	t.Logf("合计 %d 条，通过 %d 条，失败 %d 条", len(results), len(results)-failCount, failCount)

	if failCount > 0 {
		t.Fail()
	}
}

// TestSingleRoute 调试单条路由时使用（修改下面的 method/path 后运行）
func TestSingleRoute(t *testing.T) {
	baseURL := fixtureURL(t)

	method := "GET"
	path := "/api/v1/sysinfo/version"

	url := baseURL + path
	resp, err := sendRequest(method, url)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	t.Logf("%s %s → HTTP %d  Content-Type: %s", method, url, resp.StatusCode, ct)
	t.Logf("响应体: %s", string(bodyBytes))

	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("路由不存在 (404)")
	}
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 不是 application/json，实际: %s", ct)
	}
	if !checkJSON(bodyBytes) {
		t.Errorf("响应体不是合法 JSON")
	}
}
