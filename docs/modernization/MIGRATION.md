# HomeWaf 构建、迁移与发布

## 工具链与构建

Go 1.27.3、Node 26.13.0；CGO 需要本机 C 编译器。运行 `node scripts/build.mjs`，得到 artifacts 下的 HomeWaf 二进制和离线签名工具。Windows 本次验证使用 MSYS2 GCC；机器上旧的交叉路径 as.exe 无法加载，已通过进程级 `CC="gcc -BC:/msys64/mingw64/bin/"` 选择可用工具，没有修改系统安装。

前端 `npm ci` 的根 postinstall 仅执行已入库的编译器兼容补丁。patch-dependencies.mjs 限定版本，保留原版权：entities 9 的 HtmlEntityDecoder、Node 26 ESM 默认导出、estree-walker 导出条件，以及 Vite 对 Rolldown 1.3 预加载标记的完整替换。构建后对所有 JS chunk 做语法验证。同步 source-map 改用修复后的 source-map-js；lodash-unified 改用同 API 的 lodash-es 命名空间。不要使用 --force 或 --legacy-peer-deps 绕过安装错误。

验证入口为 `node scripts/verify.mjs`；它编译全部包，只执行迁移相关的必要回归。外部数据库、消息系统和真实证书签发测试不能指向生产环境。可选测试需自行提供隔离实例。跨平台 CI 已配置，但本地没有执行 Docker 或 Linux/macOS 原生运行验证。

## 从旧发行版迁移

1. 停止旧服务；备份整个安装目录，特别是 conf、data、ssl、plugins 和 registration_data.bin。记录旧版本、数据库类型、配置和自定义挑战页/规则。
2. 使用新目录预演迁移，复制备份进行验证；确认后再切换服务。不要让两个版本同时写同一数据库或监听相同端口。
3. 新服务名是 HomeWafService，二进制为 HomeWaf64.exe / HomeWafLinux64 / HomeWafLinuxArm64；显式安装和启动新服务。旧服务由操作员停用/卸载，本脚本不自动删除。
4. 现有 tenant ID（例如 SamWafCom）、数据库字段、锁文件、认证 Cookie、swt2 报文、挑战 URL 和 HTML 占位符属于兼容协议，仍可保留原字符串；它们不是产品包名。直接改值可能造成租户数据不可见、锁失效或已有页面无法登录。
5. 旧 AES128CBC 数据库读取已通过合成旧版库验证。仍应先在备份上测试实际数据。数据库结构升级后的回退应恢复完整备份，不能假定旧程序能读新库。
6. 资源释放遵守 lock.txt、自定义内容指纹和 .new 文件机制；新规则/字体/挑战页需要管理员检查差异，不能强行覆盖自定义文件。新版 CRS 更严格，先观察再启用阻断。
7. 旧无签名/伪授权文件不能升级为有效商业授权，需按新实例 ID 重签；旧插件必须针对新协议重新编译并签名。删除旧前端浏览器缓存后重新登录。

## 发行方端点

默认没有 HomeWaf 更新、公告、机器人数据或公网 IP 服务地址。不要把用户请求发送给原作者，也不要假设 HomeWaf 已拥有某个域名。

| 变量 | 用途 |
|---|---|
| HOMEWAF_UPDATE_URL | 发行方 HTTPS 清单根地址；也供公告使用 |
| HOMEWAF_BETA_UPDATE_URL | 发行方 HTTPS release 元数据端点；下载资产必须有 SHA256 digest |
| HOMEWAF_BOT_DATA_URL | 有权使用的机器人 IP 数据根地址 |
| HOMEWAF_PUBLIC_IP_URL | 显式启用的纯文本公网 IP 查询地址 |
| HOMEWAF_DEV=1 | 隔离开发目录与回环 HTTP 调试；不得用作生产默认设置 |

更新请求默认仅携带软件版本；`update_env_report` 默认 false，开启后增加 OS/架构/运行形态，不增加永久机器码。HTTPS 端点不允许嵌入账号密码。后台正常启用的证书申请、DNS、通知、AI API 仍会向相应服务商发送该功能所需数据，应在部署方隐私策略中说明。

## 商业授权与插件

离线签名工具：

```text
homewaf-license keygen --key-id publisher-2026 --key /secure/offline/private.pem --out public-keys.json
homewaf-license sign --key-id publisher-2026 --key /secure/offline/private.pem --claims claims.json --out license.json
homewaf-license sign-plugin --key-id publisher-2026 --key /secure/offline/private.pem --binary plugin --out plugin.sig.json
```

在受控发行构建中用 Go linker `-X` 给 `HomeWaf/internal/licensing.TrustedKeysJSON` 和 `HomeWaf/plugin/process.TrustedKeysJSON` 设置对应公钥 JSON。私钥只保留在离线签发环境，不放进源码、容器、CI 公开变量或交付包。Windows 的 0600 模式不能代替 NTFS ACL，应由发行方用独立目录/账号限制签名私钥访问。

claims 的 version=2、product=homewaf、edition=commercial；还需填写 license_id、subject、instance_id、features，以及 UTC issued_at、not_before、expires_at。实例 ID 通过后台授权页读取；显示名称变动不改变实例 ID。确认导入重新验证签名与期限。无可信公钥时安全拒绝，不接受旧 RSA 注册文件或伪造许可。

已实现离线许可和特性检查接口。未来云授权协议、订单系统、价格和付费功能清单尚未定义，不应宣传为已上线。

## 交付物

每份发行包/镜像包含 LICENSE、NOTICE、THIRD_PARTY_NOTICES.md、LICENSES 和本目录资料；前端提供本地帮助与许可入口。安装脚本接受显式 HTTPS 下载地址或本地 tar.gz，必须给出发行方 SHA256；不提供默认下载源，不自动安装服务。GitHub 发布流程仅生成可审阅的候选包，不自动向旧账号、镜像仓库或服务上传。

可选构建代理的 Kerberos 协商未被原构建使用，也缺少独立许可文本，已替换为项目内明确拒绝该认证方式的策略模块；HTTPS/Basic 代理与本地构建不受影响。

授权文件固定写入应用安装/开发目录，不依赖 Windows 服务管理器的工作目录。

使用 `node scripts/build.mjs` 时，可将 `HOMEWAF_LICENSE_PUBLIC_KEYS`、`HOMEWAF_PLUGIN_PUBLIC_KEYS` 分别设置为公钥 JSON 文件路径。构建脚本校验键 ID 和 32 字节 Ed25519 公钥，仅嵌入公钥；不读取私钥。GoReleaser 默认 disable 发布，发行方须先明确自己的仓库与发布授权再调整。
