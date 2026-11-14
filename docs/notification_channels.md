# 通知渠道与重要消息转发

HomeWaf 直接调用目标平台的推送接口，无需安装或运行 SmsForwarder。
渠道范围和配置字段参考 `pppscn/SmsForwarder` 的 `a142f9270624e96b15ecfae53035581354fb6434`
（2026-11-01），由 HomeWaf 独立实现发送协议。

## 配置步骤

1. 打开 **通知管理 → 通知渠道 → 添加渠道**，选择平台并填写配置。
2. 保存后点击 **测试**，确认目标平台收到消息。测试使用当前保存的配置。
3. 打开 **通知订阅**，在需要的消息类型下添加对应渠道并启用订阅。
4. 按订阅配置冷却时间、去重、聚合、静默时间、筛选条件和消息模板。
5. 在 **通知日志** 查看实际发送结果、失败原因及频控抑制记录。

创建渠道本身不会自动订阅全部事件。建议先订阅系统错误、IP 封禁、证书到期、
管理端登录来源变化和统一访问认证异常；规则触发可配合冷却或聚合，避免攻击期间刷屏。
普通登录和运行汇总按需启用。

## 14 类直接发送渠道

| 渠道 | 主要配置 |
| --- | --- |
| 钉钉群机器人 | Webhook、签名密钥、文本/Markdown、@所有人、手机号、用户 ID |
| 钉钉企业内机器人 | AppKey、AppSecret、接收用户 ID、sampleText/sampleMarkdown；AgentID 保留参考配置入口，新版单聊接口实际使用 AppKey 作为 robotCode |
| 企业微信群机器人 | Webhook、文本/Markdown；文本支持 @所有人、用户 ID、手机号 |
| 企业微信应用消息 | 企业 ID、AgentID、应用 Secret、成员/部门/标签、自定义 API 地址 |
| 飞书群机器人 | Webhook、签名密钥、文本/交互卡片、@所有人/指定用户、自定义卡片 |
| 飞书企业应用 | App ID、App Secret、接收 ID 及其类型、文本/交互卡片、自定义卡片 |
| 邮箱 | 原有 SMTP、用户名/密码、发件人、默认收件人、SSL/STARTTLS；仍支持订阅级收件人 |
| Bark | 完整推送地址、分组、图标、声音、角标、链接、通知级别、持续提醒、复制内容、可选 AES 加密 |
| 自定义 Webhook | 原有 URL、HTTP 方法、请求头、Content-Type、报文模板和预设 |
| Telegram 机器人 | Bot Token、Chat ID、话题 ID、GET/POST、TEXT/HTML/MarkdownV2、自定义 Bot API 地址 |
| Server酱 | SendKey、推送通道、抄送 OpenID；支持 SCT 和 sctp 格式 |
| PushPlus | 服务网站、Token、群组、模板、推送渠道、Webhook 编码、回调、有效期 |
| Gotify | 完整 `/message?token=...` 推送地址、优先级 |
| Socket | MQTT/TCP/UDP、主机/端口、消息模板、签名、应答关键字、字符编码；MQTT 支持账号密码、发布/接收主题、tcp/ssl/ws/wss、Client ID、QoS、保留消息 |

短信 SIM 转发与 Android URL Scheme 不是服务端推送平台，不纳入本次渠道范围。
手机端事件监控、来电/短信接收、App 唤起等能力也不属于 HomeWaf 通知渠道。

标题与正文模板继续在订阅中配置。飞书自定义卡片 JSON 的字符串值可使用
`{{MSG_TITLE}}`、`{{MSG_CONTENT}}`，替换时自动 JSON 转义。
Socket 报文支持 `[title]`、`[content]`、`[msg]`、`[timestamp]`、`[sign]`；
合法 JSON 模板中的值按 JSON 转义，其余模板按 URL 编码。
Telegram HTML/MarkdownV2 会对事件内容转义，避免攻击请求内容破坏消息格式。

## 已核对的消息触发来源

以下 9 类已经存在实际产生事件的代码，可以配置自动转发：

| 订阅类型 | 实际消息来源 | 代码位置 |
| --- | --- | --- |
| `rule_trigger` 规则触发 | 已接入的保护规则命中、错误处理及防篡改事件；不是每一条访问日志 | `wafenginecore/error_process.go`、`wafenginecore/tamper.go` |
| `ip_ban` IP 封禁 | AntiCC 自动封禁、主机防爆破封禁 | `wafenginecore/anticc_ban.go`、`wafhostguard/action.go` |
| `ssl_expire` SSL 证书到期 | 站点证书与管理端证书的到期提醒 | `waftask/task_ssl.go`、`waftask/task_manager_cert.go` |
| `system_error` 系统错误 | 已接入的管理账号锁定、引擎问题、端口防护/诊断事件；不是所有错误日志 | `api/waf_login.go`、`wafenginecore/wafengine.go`、`service/waf_service/waf_pipeline_state.go` |
| `operation_notice` 操作通知 | 定时运行汇总、证书申请/续期结果；不是所有管理操作审计 | `waftask/task_notify.go`、`waftask/task_manager_cert.go`、`wafenginecore/sslorder.go` |
| `user_login` 用户登录 | 管理端普通登录成功 | `api/waf_login.go` |
| `manage_login_abnormal` 管理端登录来源变化 | 登录来源变化时的单独告警 | `api/waf_login.go`、`service/waf_service/waf_notify_sender.go` |
| `access_login` 统一访问认证登录成功 | 统一认证成功事件 | `service/waf_service/waf_security_audit_service.go` |
| `access_abnormal` 统一访问认证异常告警 | 认证锁定、票据重放、异常回跳地址、网站密码访问锁定 | `model/security_audit_log.go`、`service/waf_service/waf_security_audit_service.go` |

两个原有订阅类型尚未接入自动生产事件，页面保留并明确标注：

- `attack_info` 攻击信息：具备数据结构、模板和队列处理，但没有自动入队调用。
  当前实际攻击拦截通知应订阅 **规则触发**。
- `weekly_report` 周报：具备数据结构、模板和队列处理，但没有周报生成/定时入队任务。
  已有定时运行汇总属于 **操作通知**，不能等同于周报。

升级结果、日志导出完成、实时操作结果和系统统计目前走管理端 WebSocket，
未作为独立外部通知订阅类型。本次不自动扩大事件范围，也不自动开启任何订阅。

## 配置与兼容说明

- 原有 6 类渠道配置继续可用；新增渠道和旧机器人扩展字段通过现有 `config_json` 存储，无需新增数据库列。
- 新渠道密钥、设备推送地址等不会在列表/详情中回显；编辑留空保留原值，显式清空使用原有哨兵语义。
  切换渠道类型不会继承上一种平台的凭据。数据库仍使用现有敏感字段加密机制。
- 渠道测试和订阅实际发送共用发送入口；订阅测试仍使用既有模板、日志逻辑。
- HTTP(S) 推送沿用系统代理及出站安全策略；本次没有新增每渠道独立代理配置。
  内网 Gotify、Bark、自建 API 和 Socket 目标需要运维在 `config.yml` 中配置
  `security.outbound_allowed_hosts`。MQTT/TCP/UDP 直接连接目标，并在连接时检查真实 IP。
- 所有新增发送器设置超时、限制响应大小，并检查平台返回的业务成功标记。
  HTTP 重定向会被拒绝，避免发送凭据随跳转泄漏；TLS 证书正常验证。
- SMTP 加密邮件、Android 证书库等手机特有高级选项没有移植；邮箱与 Webhook 保持现有配置方式。

## 验证

```powershell
go test ./wafnotify/... ./service/waf_service ./api ./utils ./wafqueue ./router
cd web
npm run typecheck
node --test scripts/verify-notify-providers.test.mjs
npm run build
node scripts/verify-notify-browser.mjs
```

协议测试使用模拟平台和本地 TCP/UDP/MQTT 服务，覆盖业务错误、密钥保持/清空、
加密保存、签名、报文转义和出站限制。浏览器测试运行真实构建产物，所有 API 请求
由内存夹具处理，不会修改真实配置或发送真实通知。真实账号的权限、平台配额和实际
收信仍需部署后填写有效凭据，通过页面“测试”验证。
