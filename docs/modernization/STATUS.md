# HomeWaf 改造与验证结果

日期：2026-11-09。代码整改与本机必要验证已完成。结果仅覆盖下述范围；商业主体、签约、生产密钥和监管认证不由代码代替。

## 已实施

- [x] 保留后端、前端与复制组件的来源、原始协议及修改声明。
- [x] 主模块 HomeWaf、入口 cmd/homewaf、前端包 homewaf-web、服务与发行物命名统一。
- [x] Vue 3 / Element Plus / Pinia 迁移及 Go、Node、Vite、TypeScript 和实际依赖闭包升级。
- [x] 替换旧插件执行框架、字体/图片/地图、历史前端二进制和重复静态脚本。
- [x] 移除 CSS 内嵌 TencentSansW7；验证码使用原版 LXGW WenKai OFL 字体；网页使用系统字体。
- [x] Ed25519 离线授权、实例绑定、期限检查、会话隔离导入与签名插件执行。
- [x] 新加密 SQLite 引擎及旧数据兼容；MIT MySQL 适配；维护中的 YAML v4 和 GeoIP2 v2。
- [x] 默认发行服务隔离、环境报告 opt-in、本地帮助和授权页面。
- [x] 许可证、NOTICE、SBOM、升级说明、源码构建和候选发行流程。

## 已通过的必要验证

| 检查 | 结果 |
|---|---|
| Go 全部生产/测试包编译 | `go test ./... -run '^$'` 通过；不执行危险的外部集成/种子测试 |
| 迁移相关回归 | `node scripts/verify.mjs` 通过；覆盖签名、SQLite、MySQL 适配、WebSocket、模板释放、规则、HTTP/3、授权导入、更新和 AI 模型 |
| 数据库兼容 | 旧 go-wxsqlite3 生成的合成加密库可读取；错误密钥拒绝；新建库加密及重开；现有 wafdb/dialect/partition 回归通过 |
| 授权 API | 有效签名预览不激活、跨会话确认拒绝、确认后持久化到应用目录、伪造替换拒绝，均通过 |
| 插件进程 | 真实签名子进程握手、认证 gRPC、IP 过滤和篡改 manifest 拒绝通过；测试密钥仅临时使用 |
| 前端构建 | 锁文件安装、Vue SFC/TypeScript 7 检查、Vite 生产构建及全部 101 个输出 JS chunk 语法检查通过 |
| 既有前端纯逻辑检查 | `npm test --prefix web`：27 通过、0 失败；使用 Node 26 直接加载实际 TypeScript 源码 |
| 最终 Windows 二进制浏览器验证 | 登录/首次改密、仪表盘、网站表格、导航、分组新增、授权页、上传伪造许可并拒绝；页面异常和失败请求均为 0 |
| 验证码 | 本地真实字体图片生成、点击标记、CAP widget 0.1.61 + WASM 0.0.8 + Go 服务完成解题/令牌验证；不访问外部资源 |
| 漏洞扫描 | govulncheck v1.9.0：No vulnerabilities found；npm audit：0 报告项。不是所有 C/WASM 代码或未知漏洞的安全证明 |
| 发布配置 | GoReleaser v2.20.0 `check` 通过；默认禁止发布，workflow 仅构建候选包 |
| 提交范围 | 不纳入测试实例数据、日志、生成二进制、node_modules、缓存或私钥；旧兼容数据库 fixture 为合成数据 |

## 明确的例外与边界

- Go 版本核验覆盖 Windows amd64、Linux amd64/arm64、macOS arm64 的实际运行依赖闭包，无可用版本升级遗漏。仅存在于第三方未使用包/测试的模块图条目不冒充应用运行依赖。
- 最新官方 CAP WASM 的 Rust 构建依赖按上游锁文件记录；未声称擅自重编译后仍保持 ABI。可选 WASM 工具回退的 @emnapi 2.0.0-beta.2 也保留上游匹配版本，不能把稳定标签 1.11.3 当成同代替换。详见版本审计 JSON。
- 小型稳定算法/工具库可能仍以早年的提交作为上游最新版本。没有通过改版本字符串、伪造发布号或删除协议来宣称升级。
- 自定义类型检查使用官方 Vue SFC 编译器与 TypeScript 7 原生编译器，保留原项目非严格策略；不是 vue-tsc 对全部模板表达式的完整类型证明。已执行实际浏览器检查补充验证。
- 当前机器没有 Docker 或隔离 MySQL 服务，未执行容器运行、真实 MySQL 服务联调及 Linux/macOS 原生运行；已配置跨平台 CI，不能写成这些环境已通过。
- SQLite 上游在 MinGW 下仍有 FORCEINLINE 宏重定义编译警告，Windows 编译与兼容测试通过；原生许可证和代码保留。
- 云授权、生产签名私钥、实际销售主体、价格与付费功能清单未被虚构。生产发布前的业务与法律事项见 COMPLIANCE.md。

原始协议文本和复制的 SQLite 头文件按原样保留；.gitattributes 仅豁免其空白格式检查，不豁免应用代码。
