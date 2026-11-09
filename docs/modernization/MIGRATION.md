# HomeWaf 构建、迁移与发布

## 工具链与构建

工具链版本以 `go.mod`、`web/package.json` 和 CI 配置为准：Go 1.27.3、Node 26.13.0；CGO 需要本机 C 编译器。Windows 可使用 MSYS2 GCC，并通过 `CC` 指定编译器。

在仓库根目录运行 `node scripts/build.mjs`，得到 `artifacts/` 下的 HomeWaf 二进制和离线签名工具。常用选项：

| 命令 | 用途 |
| --- | --- |
| `node scripts/build.mjs` | 按锁文件安装依赖，检查并构建前后端 |
| `node scripts/build.mjs --web-only` | 仅构建并嵌入前端及许可声明 |
| `node scripts/build.mjs --skip-install` | 已按当前锁文件安装依赖时跳过安装 |
| `node scripts/verify.mjs` | 编译全部 Go 包并执行选定回归 |
| `npm test --prefix web` | 执行前端逻辑测试 |

前端 `npm ci` 的根 postinstall 仅执行已入库的编译器兼容补丁。patch-dependencies.mjs 限定版本，保留原版权：entities 9 的 HtmlEntityDecoder、Node 26 ESM 默认导出、estree-walker 导出条件，以及 Vite 对 Rolldown 1.3 预加载标记的完整替换。构建后对所有 JS chunk 做语法验证。同步 source-map 改用修复后的 source-map-js；lodash-unified 改用同 API 的 lodash-es 命名空间。不要使用 --force 或 --legacy-peer-deps 绕过安装错误。

验证入口为 `node scripts/verify.mjs`；它编译全部包，只执行选定回归。外部数据库、消息系统和真实证书签发测试需自行提供隔离实例。平台支持和测试结果应以对应提交的 CI 记录及部署环境实测为准。

## 从旧发行版迁移

1. 停止旧服务；备份整个安装目录，特别是 conf、data、ssl、plugins 和 registration_data.bin。记录旧版本、数据库类型、配置和自定义挑战页/规则。
2. 使用新目录预演迁移，复制备份进行验证；确认后再切换服务。不要让两个版本同时写同一数据库或监听相同端口。
3. 新服务名是 HomeWafService，二进制为 HomeWaf64.exe / HomeWafLinux64 / HomeWafLinuxArm64；显式安装和启动新服务。旧服务由操作员停用/卸载，本脚本不自动删除。
4. 现有 tenant ID（例如 SamWafCom）、数据库字段、锁文件、认证 Cookie、swt2 报文、挑战 URL 和 HTML 占位符属于兼容协议，仍可保留原字符串；它们不是产品包名。直接改值可能造成租户数据不可见、锁失效或已有页面无法登录。
5. 旧 AES128CBC 数据库读取已通过合成旧版库验证。仍应先在备份上测试实际数据。数据库结构升级后的回退应恢复完整备份，不能假定旧程序能读新库。
6. 资源释放遵守 lock.txt、自定义内容指纹和 .new 文件机制；新规则/字体/挑战页需要管理员检查差异，不能强行覆盖自定义文件。新版 CRS 更严格，先观察再启用阻断。
7. 旧无签名/伪授权文件不能升级为有效商业授权，需按新实例 ID 重签；旧插件必须针对新协议重新编译并签名。删除旧前端浏览器缓存后重新登录。

## 发行方端点

软件更新默认检查 `user1121114685/HomeWaf` 仓库的 GitHub 正式 Release。仓库尚无正式 Release 时显示“尚未发布正式版本”；网络错误、限流和缺少平台安装包分别报错。默认没有公告、机器人数据或公网 IP 服务地址，不会把这些请求发送到 GitHub release API 或原作者服务。

IP 数据库独立于软件更新：未配置发行方数据镜像时，`/sys/IPLocation` 直接从 ip2region 官方 GitHub 仓库检查开源 XDB 数据。仅请求公开仓库信息，不附带实例标识；检查缓存 5 分钟，下载固定到提交并校验文件大小、Git blob 内容标识及 XDB 协议。下载或重载失败保留／恢复原文件。配置了发行方源时仍兼容原有 `ipdb-dataset/latest.json` 协议。

IPv4、IPv6 的字段格式默认统一为 `opensource`（国家、省份、城市、运营商，可选国家代码），不随发行版本改变。手动选择的 legacy/full/standard/compact 格式仍可用于对应的自备数据库；下载官方开源文件后对应协议自动切换到开源版解析。语言由数据库内容决定，不能根据字段格式推断为纯中文或纯英文。

| 变量 | 用途 |
|---|---|
| HOMEWAF_UPDATE_URL | 发行方 HTTPS 清单根地址；也供公告使用 |
| HOMEWAF_RELEASE_URL | 正式 release 元数据端点；默认 `https://api.github.com/repos/user1121114685/HomeWaf/releases/latest`。未设置清单根地址时使用；显式设为空可禁用此源 |
| HOMEWAF_BETA_UPDATE_URL | 发行方 HTTPS release 元数据端点；下载资产必须有 SHA256 digest |
| HOMEWAF_BOT_DATA_URL | 有权使用的机器人 IP 数据根地址 |
| HOMEWAF_PUBLIC_IP_URL | 显式启用的纯文本公网 IP 查询地址 |
| HOMEWAF_DEV=1 | 隔离开发目录与回环 HTTP 调试；不得用作生产默认设置 |

GitHub release 请求只读取公开元数据，不附加版本或实例标识。自建清单更新请求默认仅携带软件版本；`update_env_report` 默认 false，开启后增加 OS/架构/运行形态，不增加永久机器码。HTTPS 端点不允许嵌入账号密码。后台正常启用的证书申请、DNS、通知、AI API 仍会向相应服务商发送该功能所需数据，应在部署方隐私策略中说明。

发行时推送 `v主版本.次版本.修订号` 标签，`HomeWaf release packages` 工作流构建 Windows x86_64、Linux x86_64/arm64 安装包和校验清单，并在本仓库创建草稿 Release。审核草稿资产后点击 Publish release，客户端才能检测到该版本；草稿、debug 和预发布版本不会进入正式更新渠道。手动运行非标签工作流仍只生成候选构建产物。更新器按平台与版本精确匹配 `HomeWaf_Windows_x86_64_2.0.1.zip` 或 `HomeWaf_Linux_x86_64_2.0.1.tar.gz` 等名称，下载完成后验证 GitHub 资产元数据中的 SHA256 digest，验证成功才解压替换。

## 平滑升级与滚动重启

默认运行模式由 Supervisor 管理业务 Worker。在线升级先下载并校验新二进制，再启动新 Worker；新 Worker 完成配置、策略与任务初始化，并确认自己的业务监听器和管理端监听器进入接收循环后，才通知旧 Worker 停止接收新连接、排空在途请求。不能用共享端口“能连通”代替新进程就绪检查，因为连接可能由旧进程应答。

新 Worker 启动或就绪失败时保留旧 Worker；真正的版本更新会尝试恢复升级前备份，并把交接失败反馈给管理端。仅重启当前版本时不会恢复旧版本备份。旧 Worker 退出前不会再次接受在线更新，避免并发覆盖候选二进制。

手动平滑重启（不下载新版本）：先确认当前实例及其 Supervisor 正在运行，再进入该实例的安装目录执行：

```powershell
.\HomeWaf64.exe rolling-restart
```

Linux x86_64 对应运行 `./HomeWafLinux64 rolling-restart`，Linux arm64 运行 `./HomeWafLinuxArm64 rolling-restart`。如果程序已改名，使用实际文件名。命令读取本实例的 `data/supervisor.state` 连接 Supervisor；返回表示已提交交接请求，完成情况查看 Supervisor 日志。`restart` 是服务管理器的停止／启动操作；管理端“重启 WAF 引擎”则是重载引擎配置，与进程滚动重启用途不同。

`conf/config.yml` 的 `drain_time_out` 指定旧业务连接的排空期限，默认 30 秒。HTTP 在途请求及 WebSocket 等已升级连接在旧 Worker 内继续处理，主动结束后退出；超过期限才关闭剩余连接。连接与进程内状态不会迁移到新进程，管理端 WebSocket 可能自动重连。HTTP/3 使用已有 QUIC 排空流程，不保证存量 QUIC 会话跨进程无感迁移。应用托管、隧道和定时任务等独占资源在旧 Worker 退出后接管，可能有短暂停顿。涉及不向后兼容的数据库迁移仍须安排维护窗口。

本地回归：`go test ./wafnet ./supervisor` 包含真实子进程切换、持续 HTTP 请求、慢请求、升级连接和候选启动失败场景。完整在线升级测试可用两个版本递增的本地二进制运行 `node scripts/verify-graceful-update.mjs <旧程序> <新程序> <新版本号>`；脚本使用临时目录、回环更新源和关闭挑战拦截的临时测试站点，只验证传输连续性，不使用生产数据。

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

每份发行包/镜像包含 `LICENSE`、`NOTICE`、`THIRD_PARTY_NOTICES.md`、`LICENSES/THIRD_PARTY.txt`，以及本目录的 `MIGRATION.md` 和 `provenance.json`；前端提供本地帮助与许可入口。

`LICENSES/THIRD_PARTY.txt` 将相同的许可文本合并保存，保留各组件的来源索引和原始版权声明。复制到仓库内的第三方源码仍保留原位置的许可证。`provenance.json` 集中记录内嵌资源和本地适配代码的来源、版本及校验值，不是运行配置。

Go/npm 依赖版本由 `go.mod`、`go.sum`、`web/package.json` 和 `web/package-lock.json` 维护。升级依赖时同步更新第三方清单、许可文本及相关来源记录。版本审计、SBOM、测试结果等临时报告放入已忽略的 `artifacts/` 或 `.cache/`，无需作为源码快照提交。

安装脚本接受显式 HTTPS 下载地址或本地 tar.gz，必须给出发行方 SHA256；不提供默认下载源，不自动安装服务。GitHub 发布流程仅生成可审阅的候选包，不自动向旧账号、镜像仓库或服务上传。

可选构建代理的 Kerberos 协商未被原构建使用，也缺少独立许可文本，已替换为项目内明确拒绝该认证方式的策略模块；HTTPS/Basic 代理与本地构建不受影响。

授权文件固定写入应用安装/开发目录，不依赖 Windows 服务管理器的工作目录。

使用 `node scripts/build.mjs` 时，可将 `HOMEWAF_LICENSE_PUBLIC_KEYS`、`HOMEWAF_PLUGIN_PUBLIC_KEYS` 分别设置为公钥 JSON 文件路径。构建脚本校验键 ID 和 32 字节 Ed25519 公钥，仅嵌入公钥；不读取私钥。GoReleaser 的发布目标固定为 `user1121114685/HomeWaf`，创建草稿后由发行方审核并公开发布。
