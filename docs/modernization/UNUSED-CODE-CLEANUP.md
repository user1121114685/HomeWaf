# 孤立代码清理记录

日期：2026-11-16。

## 范围与判定

本次清理 Go/Vue 的不可达文件、函数、类型和局部声明，不调整页面结构和产品功能。删除 64 个源码文件（Go 25、前端 39），移除 212 个 Go 函数/方法（包含随整个文件删除的函数），以及前端无调用的 API 封装、旧示例数据、编辑器适配、导出声明、导入和局部变量。

不能把“当前平台没有调用”“没有菜单”或“前端没有引用某个 API”当作删除依据。分析使用：

- `golang.org/x/tools/cmd/deadcode`（`v0.52.1-0.20261106145633-d407941a339a`）的调用图，包含所有主程序、工具、内置插件主程序及测试入口。分别检查 Windows/amd64、Linux/amd64、Linux/arm64、Darwin/amd64、Darwin/arm64。
- `staticcheck v0.8.2 -checks=U1000` 检查内部声明；用 Go AST 核对全部受版本管理的 Go 文件，包括特殊构建标签文件，再检查注册、接口、反射与代码生成引用。
- Vue 官方 SFC 编译器、Babel AST 和原生 TypeScript 的 `--noUnusedLocals`。模板引用、路由懒加载、全局组件/图标、状态模块、类型声明、重导出、测试和构建脚本均纳入分析。
- 生产路由注册清单，以及 `audit-backend-apis.mjs`、`audit-ui-capabilities.mjs` 的前后比较。

新增只读工具 [audit-unused-frontend.mjs](../../scripts/audit-unused-frontend.mjs)。它从 HTML、入口、配置和测试脚本遍历引用，输出候选清单，**不会删除文件**。遇到生产源码中的未解析动态导入、glob/目录加载或解析失败时，标记需要复核并使 `--check` 失败，不将潜在目标当成孤立文件。报告写入忽略目录 `artifacts/unused-code/frontend.json`。

```sh
node scripts/audit-unused-frontend.mjs --check
npm run typecheck --prefix web
node web/node_modules/typescript/bin/tsc --project web/.cache/typecheck/tsconfig.json --noEmit --noUnusedLocals
```

本次复查结果：312 个前端源码文件，文件和导出候选均为 0，解析错误为 0。测试脚本中的 `import(new URL(...))` 和生成语言包的 data URL 被单独记录；相关源码已通过字符串路径及应用入口覆盖。结果只说明该保守分析未发现新的候选，不是对任意外部动态调用的形式化证明。

## 被清理的实现与保留的归属

| 清理对象 | 当前能力归属 / 保留依据 |
| --- | --- |
| 旧 CC 页面、阈值/命中/应急子页面及条件编辑组件 | `pipeline/ServicePolicyPage.vue`、`ServiceTaskEditor.vue`、`CCWorkbench.vue`；当前使用的 `anticc/component/baniplist` 保留 |
| 旧 IP/URL 黑白名单页面 | 服务策略中的 `AccessRestrictions.vue`、`RuleCollection.vue`；实际使用的 IP 查询、处置与添加 API 保留 |
| 旧访问配置、账号、会话页面 | `ServiceIdentityPanel.vue` 与服务策略编辑器；账号/会话操作仍按端口隔离 |
| 旧规则列表/编辑页、规则生成器、旧 CodeMirror 适配 | `RuleAssistant.vue`、服务规则集合、当前代码编辑器；构建、编译、AI 辅助、保存重开保留 |
| 旧敏感词、隐私规则、脱敏 URL 页面 | 服务策略中的敏感内容、响应脱敏与路径规则编辑能力 |
| 前端模板演示数据、旧空布局、旧图表模拟数据、旧对话框补丁 | 均无入口引用；当前布局、真实统计图表、Element Plus 控件与全局注册保留 |
| 旧通知渠道适配和未使用的企业微信消息工具 | `wafnotify/channels/` 的现有渠道发送链路保留；仍有测试调用的文字截断工具保留 |
| `xdaemon`、无消费者的雪花 ID 初始化、空占位类型和未用工具方法 | 当前 supervisor、服务入口、UUID/ID 实现保留 |
| 从未注册的旧插件管理 router/API 及仅被其使用的模型 | 生产路由无变化；签名进程插件、协议、SDK、加载器与 pipeline 插件能力保留；未执行数据库表删除 |
| 其它无调用服务方法和前端接口封装 | 注册中的后端 API 继续保留；移除客户端封装不代表移除服务器能力 |

路由迁移已通过可用历史提交 `1341e17c`、`390b1a5b` 和 `2c2e4868` 核对。仓库规则提到的 `52078dfc164a3cb60637f903ba9b630dc64d5f45` 不在当前 Git 对象库中，不能声称已与该提交逐项比对。

## 隐式使用与保留项

- `init`、空白导入、`go:embed`、数据库驱动/序列化注册、反射规则方法、接口实现和生成资源按使用中的代码处理。
- `wafSystenService.Start/Stop` 属于系统服务接口，保留。`splitByIPVersion`、`ipItemIsV6`、`globalLogPaths` 在 Linux 使用，不能按 Windows 的 U1000 输出删除。
- Windows/Darwin/Linux 文件逐个平台核对；只移除各适用平台均没有调用的辅助方法，保留实际的平台实现。
- `crossdb` / `iptagperf` 测试中引用的服务方法保留；这些方法不能按默认标签的 deadcode 结果删除。
- 插件 SDK、协议边界与本地第三方源码副本的导出接口未按应用调用图批量裁剪，第三方许可声明保留。
- 仍可访问的旧来源编辑器、迁移入口、封禁列表、证书凭据编辑、日志和统计模块保留。

## 验证证据

| 检查 | 结果 |
| --- | --- |
| `npm run typecheck --prefix web` | 通过 |
| 原生 TypeScript `--noUnusedLocals` | 通过 |
| `npm test --prefix web` | 通过；导航/日志回归与 27 项应用流程检查 |
| `npm run build --prefix web` | 通过；97 个生成 JS 分块语法检查；仍有构建体积提示 |
| `node scripts/build.mjs --skip-install` | 通过；生成前后端与授权工具，嵌入本地前端产物 |
| `node scripts/verify.mjs` | 通过；全包编译及仓库规定的鉴权、持久化、更新、通知、pipeline 等隔离回归 |
| `go test -tags=crossdb,iptagperf ./service/waf_service ./wafqueue -run '^$'` | 通过编译；没有运行外部 MySQL/PostgreSQL 集成测试 |
| 五个发布组合的 deadcode 分析 | 均完成；包含测试入口，剩余结果按上述隐式调用/特殊标签/接口边界分类保留 |
| 五个发布组合的 `CGO_ENABLED=0 go build ./...` | 全部通过 |
| 清理涉及模块的额外 Go 回归 | 防火墙、通知文字处理、流量检测、IP 定位、HostGuard、OWASP、隧道及插件管理等测试通过；无测试包也完成编译 |
| 生产路由前后比较 | 全部 599 条路由及认证/角色元数据逐项一致，其中 422 条具有角色限制 |
| `verify-console-browser.mjs` | 432 次路由/尺寸/主题组合，未发现路由不匹配、页面异常或横向溢出；登录、导航、主题与键盘搜索检查通过 |
| `verify-service-policy-browser.mjs` | 14 个防护入口；CC/IP/URL 保存重开、失败保留草稿、服务隔离与取消通过 |
| `verify-api-ui-integration-browser.mjs` | 8 条完整任务流程、48 个尺寸/主题状态、12 次隔离写入；错误列表为空 |
| `verify-secondary-console-browser.mjs` | API 目录、处置确认、应用回滚、隧道、HostGuard、OWASP 与旧来源编辑器的 7 类流程通过 |

浏览器运行使用隔离模拟响应，不连接真实防护实例；不能据此宣称验证了外部通知、证书签发、AI 服务或真实上游可达性。后端运行时与持久化保证来自对应隔离测试。桌面、平板及手机深色截图已抽查；未进行页面视觉重设计。

**清理前已存在的限制：** `go test -tags=crossdb ./waftask -run '^$'` 无法编译，旧测试引用不存在的 `wafdb.RunLogDBMigrations` 和 `wafdb.RunStatsDBMigrations`。本次清理前后的报错相同，未删除这些测试或伪报通过。默认构建及规定的验证命令不受影响。

静态 API/UI 审计现在为 599 条注册路由、456 条静态可达引用、6 条客户端需复核、137 条无静态引用。后两类不等于服务端死代码；路由注册与外部 API 调用仍是有效入口。
