# 家庭网络申请 HTTPS 证书

在 `/waf-host/wafsslOrderlist` 新建证书申请，默认使用 **DNS 验证（DNS-01）**。
不需要公网 IP，不需要开放、监听或转发公网入站 80/443。证书可用于 HTTPS 8443
等自定义端口，也可用于局域网服务。设备需要能向证书机构和 DNS API 发起出站
请求，并进行 DNS 查询；证书签发不会自动建立外网访问或路由器端口映射。

1. 选择关联网站/服务和证书机构。
2. 选择实际托管域名 DNS 解析的服务商，不一定是域名购买平台。
3. 选择密钥分组，按该服务商的字段说明配置凭据。只需填写选定认证方式需要的
   字段；同义别名、备选认证方式和可选参数不用全部填写。
4. 填写邮箱和域名。多域名用逗号或换行分隔，例如 `example.com,*.example.com`。
5. 保留本地 DNS 传播检查并提交。自动续期沿用订单的服务商及密钥分组。

IP 地址不能使用 DNS-01；HTTP-01 需要证书机构能通过公网 TCP 80 访问挑战文件，
不能用 8080/8443 替代。缺少公网入站端口时，请使用域名证书。

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
