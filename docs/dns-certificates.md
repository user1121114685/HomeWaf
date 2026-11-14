# 家庭网络申请 HTTPS 证书

在 `/waf-host/wafsslOrderlist` 新建证书申请，默认使用 **DNS 验证（DNS-01）**。
不需要公网 IP，不需要开放、监听或转发公网入站 80/443。证书可用于 HTTPS 8443
等自定义端口，也可用于局域网服务。设备需要能向证书机构和 DNS API 发起出站
请求，并进行 DNS 查询；证书签发不会自动建立外网访问或路由器端口映射。

1. 在“关联服务”中选择“服务接入”里已添加的服务，再选择证书机构。下拉框同时支持原有网站；全局规则项不作为服务显示。没有服务时会提示先去添加，读取失败时可重试。
2. 选择实际托管域名 DNS 解析的服务商，不一定是域名购买平台。
3. 选择密钥分组，按该服务商的字段说明配置凭据。只需填写选定认证方式需要的
   字段；同义别名、备选认证方式和可选参数不用全部填写。
4. 填写邮箱和域名。多域名用逗号或换行分隔，例如 `example.com,*.example.com`。
5. “启用自动续期”默认勾选，适用于所有证书机构、DNS 服务商和验证方式。保留本地 DNS 传播检查并提交；如取消勾选，签发后需手动续期。

自动续期直接依据成功订单和服务的证书绑定执行，不需要在 `/waf-host/wafsslExpirelist` 添加记录。该页面仅负责到期检测与提醒。默认计划每天 02:13 检查，域名证书默认在到期前 30 天进入续期窗口，使用系统中已保存的计划和阈值配置。续期、失败重试沿用原申请的续期选择、验证方式、服务商和密钥分组。升级前已有订单默认开启，用户明确取消的设置在后续启动时不会被覆盖。

申请列表直接展示域名、关联服务、申请状态、证书有效期和自动续期状态，支持按域名、状态筛选。点击域名或“日志”查看完整申请配置与过程日志；失败订单显示“重试”，成功订单保留“手动续期”，删除操作收在“更多”中。窄屏改为逐条排列，无需横向拖动寻找操作。

原生端口服务签发后，证书保存到证书夹并绑定到所选服务。已有 HTTPS 服务的新握手
使用更新后的证书；HTTP 服务先保存证书，监听协议、启停状态和防护规则保持不变，
可随后在服务设置中切换 HTTPS。原生服务也参与自动续期；手动替换服务 PEM 会解除
自动绑定，防止自动续期覆盖手动证书。

IP 地址不能使用 DNS-01；HTTP-01 需要证书机构能通过公网 TCP 80 访问挑战文件，
不能用 8080/8443 替代。缺少公网入站端口时，请使用域名证书。

## ZeroSSL 账户认证

选择 ZeroSSL 时，申请页立即读取认证状态。尚未配置或状态读取失败时会显示原因，
并暂时禁用申请按钮；不必等到提交订单才发现缺少配置。

点击申请平台下方的“配置 ZeroSSL 认证”，或在“维护平台 → 编辑 zerossl”中打开
同一配置入口。CA 名称、接口地址、备注与账户认证分别保存。
申请页、CA 编辑页和“系统配置”编辑的是同一组认证参数，无需在各处重复填写。
认证弹窗中的“前往 ZeroSSL 获取 API Access Key”会在新标签页打开
`https://app.zerossl.com/developer`，填写凭据的原窗口保持打开。

- **API Access Key**：对应系统配置 `zerossl_access_key`；申请时自动获取 EAB。
  更换此 Key 会清除原账户的 EAB 缓存，避免混用不同账户。
- **直接配置 EAB**：同时填写 `zerossl_eab_kid` 和 `zerossl_eab_hmac_key`，两项一起保存。

这组凭据属于证书机构，与 DNS 服务商 Token / dns.he.net DDNS Key 不同。当前实例
共用一组配置，由系统管理员保存。密钥加密落库、不回显；“已配置”仅表示字段齐全，
实际有效性在申请时由 ZeroSSL 校验。手工申请、手工续期和后台续期使用同一套检查。
选择 Let’s Encrypt 不会要求这组 ZeroSSL 凭据。

出现“DNS 解析失败”或“连接超时”时，按 `docs/outbound-network.md` 检查运行机器的
DNS 和代理；公网证书机构不需要逐个加入内网放行清单。

## dns.he.net / Hurricane Electric

1. 登录 dns.he.net 并打开域名管理面板。
2. 新建 TXT 记录。申请 `example.com` 或 `*.example.com` 时，主机名为
   `_acme-challenge`，完整记录为 `_acme-challenge.example.com`，内容先填 `init`。
3. 勾选 **Enable entry for dynamic dns**，保存后点击记录旁的刷新/密钥图标，
   生成并复制 DDNS Key（Token）。此处不用网站登录密码。
4. 在 HomeWaf 选择 **Hurricane Electric DNS · hurricane**，配置
   `HURRICANE_TOKENS`，在专用表单填 `example.com` 和复制的 DDNS Key。
5. 根域名与泛域名共用一行。申请 `nas.example.com` 时需另建
   `_acme-challenge.nas.example.com` 的动态 TXT 记录，并在表单增加该域名的 Key。

内部保存格式为 `example.com:token1,nas.example.com:token2`。域名不带 `*.` 或
`_acme-challenge.`，同一域名不要重复。不要删除预建的 TXT 记录，也不要撤销其
动态更新权限；lego 会在申请、清理和续期时更新这条记录的值。根域名和泛域名
验证按服务商的顺序间隔执行，防止覆盖彼此的验证值。

如使用 CNAME 委托，DDNS Key 必须对应实际写入的目标 TXT 记录，参照页面中的
本版本官方说明配置。

## 其他认证方式

- Cloudflare：优先使用 API Token，给目标区域授予 `Zone / DNS / Edit` 和
  `Zone / Zone / Read`，填写 `CF_DNS_API_TOKEN`。可按说明拆分 DNS/Zone Token，
  或使用邮箱和 Global API Key；不要混用不需要的认证字段。
- 阿里云、腾讯云、华为云、百度云：使用拥有目标 DNS 区域管理权限的云 API
  凭据。腾讯云与旧版 DNSPod 为不同选项。临时凭据需填写相应会话 Token。
- Duck DNS：填写账户 Token，使用已注册的完整 Duck DNS 子域名。
- Route 53、Azure、Google Cloud 等：页面列出所需参数、别名、高级选项以及
  当前 lego 版本的示例和认证说明。支持服务商提供的服务账户、凭据文件路径、
  SDK 配置或实例身份方式；文件需位于 HomeWaf 服务器并可被服务账户读取。
- acme-dns：适合原 DNS 平台没有自动更新 API 的情况。先注册 acme-dns 并按
  文档配置 `_acme-challenge` 的 CNAME 委托。
- exec：高级用户可指定服务端已安装的外部 DNS 更新程序。

服务商目录及字段从 `go.mod` 锁定的 lego v4.35.2 源码生成，含全部 195 个注册
条目及别名。其中 194 个可用于自动流程；`manual` 是要求终端输入的手动求解器，
页面明确禁用，避免后台申请或续期卡在等待输入。

凭据按服务商/分组加密保存，状态接口只返回字段名。每个 DNS 订单使用独立的
环境变量作用域，申请和清理完成后恢复原环境；多个 DNS 订单会串行执行。
不要依赖全局环境变量提供组内凭据，请在分组中保存对应参数；服务器上的 SDK
配置文件/实例身份仍按服务商机制使用。修改密钥时空值保留原值；HE 多域名密钥
选择更新时，需要完整重填该分组的域名及 Key。

## 查看申请日志与重试

证书 DNS 查询使用指定的递归解析器 `223.5.5.5:53`，单次 DNS 网络查询超时为 5 秒，不再回退到 Google DNS。传播检查的轮询间隔和总超时由服务商/lego 控制，不再额外嵌套十次、每次 30 秒的等待。此设置仅用于证书验证，不修改操作系统 DNS；CA 仍会执行域名所有权验证。

在“证书申请记录”的订单操作列点击“日志”，可以查看当前阶段、最近心跳、失败原因以及 CA/DNS 验证过程。等待中或申请中的日志每 3 秒刷新；每笔订单保留最近 500 条，认证参数和密钥会脱敏。启用逐单日志之前的订单，会从现存 `logs/log.log` 中按域名与时间筛选历史内容，可能不完整。

失败或中断的订单可在列表点击“重试”，或在日志弹窗点击“重试申请”。服务、域名、CA、DNS 服务商、密钥分组及传播检查选项从原记录读取，不用重新填写。重试会新建订单并保留原失败记录，自动打开新订单日志；重复点击或响应丢失后的重试不会重复建单。进行中的订单不能重试。

如果提示超过 10 分钟没有心跳，先确认服务正常运行，再点击重试。`dev.bat` 仅在后端编译输入的内容发生变化时重建，同内容保存、时间戳更新和无关目录事件不会重载。触发文件、时间和接管后的进程 PID 记录在 `.cache/dev/watch-events.log`。开发热重载使用的 `rolling-restart` 会暂停领取新证书任务，等待已有任务完成后再交接进程；等待超时会放弃此次重载，不强制切断申请。强制结束进程、系统关机不受此保护。

日志显示 TXT 写入成功后仍需等待传播和 CA 校验，不代表已经签发。HE 各权威服务器短时间返回不同 TXT 值时，系统按服务商的轮询间隔继续检查；不要把记录手动改回旧订单的验证值。日志弹窗只有正文一个滚动区域，底部刷新、重试和关闭按钮保持可见。

新增管理接口：`GET /api/v1/wafhost/sslorder/logs?id=...`、`POST /api/v1/wafhost/sslorder/retry`，后者提交 `id` 和稳定的 `request_id`。两者与证书管理写操作一样受系统管理员权限保护。

## 开发验证

```text
go run ./scripts/generate-dns-providers -check
go test ./internal/dnsproviders ./utils/ssl ./service/waf_service ./api -run "^Test(DNS|Hurricane)" -count=1
node scripts/verify.mjs
npm run typecheck --prefix web
npm test --prefix web
npm run build --prefix web
node web/scripts/verify-dns-orders-browser.mjs
```

浏览器测试默认访问开发前端 `http://127.0.0.1:3001`，可通过
`CONSOLE_TEST_ORIGIN` 修改。所有 API 数据均被测试夹具拦截，不使用真实凭据、
不更新公网 DNS，也不提交真实 CA 订单；截图写入忽略的 `artifacts/dns-orders-review`。
