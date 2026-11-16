# 后端能力与前端接入增量审计

本次依据生产路由注册、组件调用链和后端运行时核对。完整路由列表见 [自动生成清单](backend-api-routes.md)，产品合同和验证边界见 [接入记录](API-UI-INTEGRATION.md)。此处不是所有历史路由的完整能力清单。

| 能力 | 现在的 API → 可达控件 | 持久化/运行时 |
| --- | --- | --- |
| OWASP 文件 | `owasp/files`、`owasp/file_content` → 规则页文件预览 | 只读 rooted 文件句柄，2 MiB 上限 |
| 基线 | `owasp/base_config` → 调参页高级基线 | 基线文件→热重载；用户覆盖优先 |
| 扩展配置 | `systemconfig/add`、`systemconfig/del` → 高级维护 | 原 `SystemConfig` 表；程序项保护标记；扩展消费者自行读取 |
| IP 校验 | `wafhost/ipgroup/validate` → 条目表单 | 校验后沿用原新增/编辑服务与运行时重建 |
| 通知定位 | `NotifyLog.subscription_id` → 日志详情链接 | 既有订阅配置抽屉及 API，不创建替代记录 |
| CC 估算/命中 | `pipeline/cc/recommend`、`pipeline/cc/hits` → 服务 CC 页 | 当前服务访问日志、pipeline 日志，明确范围与预算 |
| 临时挑战/例外 | `pipeline/overview`、`pipeline/ports/save` → CC 表单与各服务概览 | 原生 `ruleset.challenge`，版本化保存到对应监听策略 |
| 规则辅助 | 旧 `rule/format`、`aigen`、`aiprompt`（不持久化）→ 构建器 | `pipeline/explain` 再验证→`pipeline/ports/save`，不写旧 Rules 表 |
| 批量会话撤销 | `pipeline/gate/sessions/revoke` → 账号/服务范围动作 | `GateStore` 即时撤销+持久化；旧令牌实际失效 |

迁移后的旧 CC、访问会话、规则 CRUD 接口仍可无前端引用；它们不应重新接收原生 `pipeline-*` 身份。SSE、上传、WebSocket、动态路由和函数别名仍可能被静态审计列为疑点。新增功能并非以“每个接口必须有一个按钮”为目标。

API 接入时的基线：599 条注册路由，456 条静态可达引用，74 条客户端需复核，69 条无静态引用。通过路由鉴权拒绝测试，不代表每个接口所有业务分支或所有返回字段均已覆盖。

2026-11-16 [孤立代码清理](UNUSED-CODE-CLEANUP.md) 后重跑两个审计脚本：599 条注册路由、456 条静态可达引用、6 条客户端需复核、137 条无静态引用。599 条路由及认证/角色元数据与清理前逐项一致；变化来自删除无调用的前端封装，不是移除已注册后端能力。隐式入口、动态请求和外部 API 消费继续按使用中的代码保留。
