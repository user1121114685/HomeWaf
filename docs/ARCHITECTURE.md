# HomeWaf 架构

HomeWaf 在同一个 Go 模块中构建管理端和流量防护程序。管理员通过控制台或管理 API 修改配置。受保护服务的访问者连接各业务端口，由当前生效的端口策略处理请求。两条路径的身份与权限各自独立。

## 目录边界

根目录保留命令、控制台、插件接口及示例、嵌入资源、部署配置和维护工具。后端实现位于 Go 的 `internal/` 边界，按运行职责组织。

| 目录 | 处理的问题 |
| --- | --- |
| [instance](../internal/instance) | 实例配置、命令行、启动、进程接管、运行指标和升级。 |
| [management](../internal/management) | 管理 HTTP 和 MCP 入口，以及这些入口的鉴权、路由和请求处理。 |
| [services](../internal/services) | 管理操作对应的业务流程，包含服务配置、证书、受管应用和隧道。 |
| [protection](../internal/protection) | 流量监听、规则匹配、反向代理、主机保护和防火墙集成。 |
| [model](../internal/model) 与 [storage](../internal/storage) | 共享数据结构、数据库和缓存，以及日志归档与导出。 |
| [jobs](../internal/jobs) | 后台任务、事件队列和通知投递。 |
| [platform](../internal/platform) | 加密、网络、日志及其他跨业务使用的基础能力。 |

## 从哪里改

| 任务 | 代码入口 | 职责 |
| --- | --- | --- |
| 启动、接管和停止实例 | [cmd/homewaf](../cmd/homewaf)、[supervisor](../internal/instance/supervisor) | 主程序选择运行方式，监督进程协调工作进程的就绪、接管和退出。 |
| 新增管理操作 | [internal/management](../internal/management) | 服务端组装 Gin。router 注册路径和角色，middleware 校验访问权限，api 绑定请求并调用业务服务。mcp 通过管理路由分派工具调用。 |
| 修改端口配置 | [端口配置服务](../internal/services/waf_service) | 业务服务校验配置、保留密钥、协调写库与运行时应用。数据库记录和表映射在 [model](../internal/model)，数据库初始化与方言在 [wafdb](../internal/storage/wafdb)。 |
| 修改业务请求的处理 | [wafpipeline](../internal/protection/wafpipeline)、[wafenginecore](../internal/protection/wafenginecore) | pipeline 持有每个端口的策略快照、监听器和规则执行顺序。代理转发、认证与访问日志等集成由工作进程装配。 |
| 修改请求检测 | [SQL 注入与 XSS](../internal/protection/wafpipeline/detectors_injection.go)、[扫描器识别](../internal/protection/wafpipeline/detectors_scanner.go)、[命令检测](../internal/protection/wafpipeline/rce_signatures.go) | 检测器在端口管线中处理已归一化的请求。SQL 注入和 XSS 使用 go.mod 固定版本的上游 libinjection。 |
| 修改证书、通知或定时工作 | [资源服务](../internal/services/waf_service)、[waftask](../internal/jobs/waftask)、[wafnotify](../internal/jobs/wafnotify) | 资源服务持有配置，任务负责调度，通知通道负责协议、消息格式和发送结果。端口配置引用证书资源。 |
| 修改控制台或嵌入资源 | [web/src](../web/src)、[public](../public) | Vue 路由、页面、API 客户端和共享控件分别在 router、pages、apis 和 ui。构建将前端产物复制到 public/dist，由 Go 嵌入。 |

## 一次端口保存

以 `POST /api/v1/pipeline/ports/save` 为例，控制台和允许执行此操作的管理客户端共用同一个写入入口。

1. 管理路由要求安全管理员权限。处理器绑定输入、记录操作人，并在 MCP 分派时检查工具允许写入的范围。[路由](../internal/management/router/waf_pipeline.go) · [处理器](../internal/management/api/waf_pipeline.go)
2. 端口服务校验输入、依赖和预期版本，合并保留的凭据，并准备新的运行时快照。[端口保存](../internal/services/waf_service/port_config_save.go)
3. 数据库事务写入策略、受保护的秘密字段，以及新服务所需的初始账号。更新按端口和旧版本匹配。自动处置操作在同一事务中检查运行权。[提交与回滚](../internal/services/waf_service/port_config_commit.go) · [数据模型](../internal/model/port_pipeline.go)
4. 服务发布快照，按需调用工作进程装配的监听器。发布或监听失败时，它用失败版本作为条件恢复数据，避免覆盖较新请求。运行中的请求通过端口注册表获取策略。[启动装配](../cmd/homewaf/pipeline.go) · [快照注册表](../internal/protection/wafpipeline/registry.go)

保存成功、监听器已启动和外部流量可达是三个可以分别观测的状态。改动这条路径时，应同时核对版本冲突、监听失败恢复和凭据保留。

## 对外边界

- 管理 HTTP 路径和请求格式是客户端契约。管理实现使用 Go 的 `internal` 可见性，外部工具通过受授权的管理接口访问。控制台管理员、管理 API 密钥和业务端口的访客账号分别校验。[管理服务](../internal/management/localserver.go)
- 实例级插件管理器在启动时读取 [conf/plugins.yml](../conf/plugins.yml)。每端口的签名插件策略属于服务配置，在控制台的「我的服务 → 配置服务 → 检测规则 → 攻击与登录检测」编辑。端口管线使用[进程池](../plugin/manager/port_pipeline.go)应用该服务的策略，启动时校验本机插件的签名。[示例插件](../plugins/builtin/simple_ip_blocker)
- 构建、开发热重载和发布分别从 [build.mjs](../scripts/build.mjs)、[dev.mjs](../scripts/dev.mjs) 和[发行工作流](../.github/workflows/goreleaser.yml)进入。正式安装器为 [install_homewaf.sh](../install_homewaf.sh) 和 [install_homewaf.ps1](../install_homewaf.ps1)。
