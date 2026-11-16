# 后端 API 路由核查清单

由 `node scripts/audit-backend-apis.mjs` 从生产 `RegisterRoutes` 生成，包含开启调试时的路由及测试应急入口。没有启动监听端口或调用业务写操作。

总计 614 条；登录令牌/API Key 535 条，仅登录令牌 73 条，公开 5 条，下载票据 1 条。

“鉴权拒绝”表示已实际请求验证缺失令牌、过期令牌、非法角色均被拦截，不表示该接口全部业务分支均已测试。“角色限制”来自真实中间件链；细分角色和 API Key 的权限语义见审计报告。参数绑定列仅为源码索引，不等同完整输入校验。

| 方法 | 路径 | 认证 | 角色限制 | 鉴权拒绝 | 参数入口 | 业务处理器 |
| --- | --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/account/add` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAccountApi.AddApi](../../api/waf_account.go#L32) |
| POST | `/api/v1/account/changemypwd` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafAccountApi.ChangeMyPasswordApi](../../api/waf_account.go#L133) |
| GET | `/api/v1/account/del` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAccountApi.DelAccountApi](../../api/waf_account.go#L90) |
| GET | `/api/v1/account/detail` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAccountApi.GetDetailApi](../../api/waf_account.go#L61) |
| POST | `/api/v1/account/edit` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAccountApi.ModifyAccountApi](../../api/waf_account.go#L114) |
| POST | `/api/v1/account/list` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAccountApi.GetListApi](../../api/waf_account.go#L73) |
| POST | `/api/v1/account/resetotp` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAccountApi.ResetAccountOTPApi](../../api/waf_account.go#L209) |
| POST | `/api/v1/account/resetpwd` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAccountApi.ResetAccountPwdApi](../../api/waf_account.go#L189) |
| GET | `/api/v1/account_log/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccountLogApi.GetDetailApi](../../api/waf_account_log.go#L14) |
| GET | `/api/v1/account_log/list` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccountLogApi.GetListApi](../../api/waf_account_log.go#L24) |
| POST | `/api/v1/ai/dashboard` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.GetAIDashboardApi](../../api/waf_ai_api.go#L157) |
| POST | `/api/v1/ai/export` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.ExportTrainDataApi](../../api/waf_ai_api.go#L251) |
| POST | `/api/v1/ai/label/batch_mark` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.BatchMarkLabelApi](../../api/waf_ai_api.go#L206) |
| POST | `/api/v1/ai/label/batch_unmark` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.BatchUnmarkLabelApi](../../api/waf_ai_api.go#L221) |
| POST | `/api/v1/ai/label/by_uuids` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.LabelByUuidsApi](../../api/waf_ai_api.go#L192) |
| POST | `/api/v1/ai/label/list` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.LabelListApi](../../api/waf_ai_api.go#L199) |
| POST | `/api/v1/ai/label/mark` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.MarkLabelApi](../../api/waf_ai_api.go#L164) |
| POST | `/api/v1/ai/label/unmark` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAIApi.UnmarkLabelApi](../../api/waf_ai_api.go#L178) |
| POST | `/api/v1/ai/model/reload` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAIApi.ReloadAIModelApi](../../api/waf_ai_api.go#L130) |
| POST | `/api/v1/ai/model/unload` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAIApi.UnloadAIModelApi](../../api/waf_ai_api.go#L149) |
| POST | `/api/v1/ai/model/upload` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAIApi.UploadAIModelApi](../../api/waf_ai_api.go#L66) |
| GET | `/api/v1/ai/status` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAIApi.GetAIStatusApi](../../api/waf_ai_api.go#L50) |
| GET | `/api/v1/analysis/actor/list` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafAnalysisApi.AnalysisActorListApi](../../api/waf_analysis.go#L41) |
| GET | `/api/v1/analysis/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafAnalysisApi.AnalysisDetailApi](../../api/waf_analysis.go#L61) |
| GET | `/api/v1/analysis/path/list` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafAnalysisApi.AnalysisPathListApi](../../api/waf_analysis.go#L51) |
| GET | `/api/v1/analysis/spider` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafAnalysisApi.AnalysisSpiderRangeApi](../../api/waf_analysis.go#L28) |
| GET | `/api/v1/analysis/wafanalysisdaycountryrange` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafAnalysisApi.StatAnalysisDayCountryRangeApi](../../api/waf_analysis.go#L15) |
| POST | `/api/v1/application/app/add` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.AddApi](../../api/waf_app_api.go#L51) |
| GET | `/api/v1/application/app/backups` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.GetBackupsApi](../../api/waf_app_api.go#L478) |
| POST | `/api/v1/application/app/changelogs` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.GetChangeLogsApi](../../api/waf_app_api.go#L508) |
| POST | `/api/v1/application/app/clearlogs` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.ClearLogsApi](../../api/waf_app_api.go#L302) |
| GET | `/api/v1/application/app/del` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.DelApi](../../api/waf_app_api.go#L224) |
| GET | `/api/v1/application/app/detail` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.GetDetailApi](../../api/waf_app_api.go#L167) |
| POST | `/api/v1/application/app/directory_preview` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.DirectoryPreviewApi](../../api/waf_app_api.go#L37) |
| POST | `/api/v1/application/app/edit` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.ModifyApi](../../api/waf_app_api.go#L177) |
| GET | `/api/v1/application/app/feature_status` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.GetFeatureStatusApi](../../api/waf_app_api.go#L27) |
| POST | `/api/v1/application/app/list` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafAppApi.GetListApi](../../api/waf_app_api.go#L100) |
| GET | `/api/v1/application/app/logs` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.GetLogsApi](../../api/waf_app_api.go#L288) |
| GET | `/api/v1/application/app/network` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.GetNetStatsApi](../../api/waf_app_api.go#L493) |
| GET | `/api/v1/application/app/restart` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.RestartApi](../../api/waf_app_api.go#L260) |
| GET | `/api/v1/application/app/rollback` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.RollbackApi](../../api/waf_app_api.go#L425) |
| GET | `/api/v1/application/app/start` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.StartApi](../../api/waf_app_api.go#L250) |
| GET | `/api/v1/application/app/status` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafAppApi.GetStatusApi](../../api/waf_app_api.go#L265) |
| GET | `/api/v1/application/app/stop` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.StopApi](../../api/waf_app_api.go#L255) |
| POST | `/api/v1/application/app/upgrade` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.UpgradeApi](../../api/waf_app_api.go#L366) |
| POST | `/api/v1/application/app/upload` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.UploadFileApi](../../api/waf_app_api.go#L329) |
| GET | `/api/v1/application/app/verifypwd` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafAppApi.VerifyPasswordApi](../../api/waf_app_api.go#L32) |
| POST | `/api/v1/batch_task/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBatchTaskApi.AddBatchTaskApi](../../api/waf_batchtask.go#L19) |
| GET | `/api/v1/batch_task/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBatchTaskApi.DelBatchTaskApi](../../api/waf_batchtask.go#L65) |
| GET | `/api/v1/batch_task/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBatchTaskApi.GetBatchTaskDetailApi](../../api/waf_batchtask.go#L36) |
| POST | `/api/v1/batch_task/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBatchTaskApi.ModifyBatchTaskApi](../../api/waf_batchtask.go#L83) |
| POST | `/api/v1/batch_task/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBatchTaskApi.GetBatchTaskListApi](../../api/waf_batchtask.go#L48) |
| GET | `/api/v1/batch_task/manual` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBatchTaskApi.ManualBatchTaskApi](../../api/waf_batchtask.go#L100) |
| POST | `/api/v1/cdnip/provider/autofetch` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCDNIPApi.SetAutoFetchApi](../../api/waf_cdn_ip_api.go#L55) |
| POST | `/api/v1/cdnip/provider/credential/clear` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCDNIPApi.ClearCredentialApi](../../api/waf_cdn_ip_api.go#L83) |
| POST | `/api/v1/cdnip/provider/credential` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCDNIPApi.SetCredentialApi](../../api/waf_cdn_ip_api.go#L69) |
| GET | `/api/v1/cdnip/provider/info` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafCDNIPApi.GetInfoApi](../../api/waf_cdn_ip_api.go#L24) |
| GET | `/api/v1/cdnip/provider/list` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafCDNIPApi.GetListApi](../../api/waf_cdn_ip_api.go#L18) |
| POST | `/api/v1/cdnip/provider/ranges` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafCDNIPApi.RangesApi](../../api/waf_cdn_ip_api.go#L39) |
| POST | `/api/v1/cdnip/provider/refresh` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCDNIPApi.RefreshApi](../../api/waf_cdn_ip_api.go#L97) |
| GET | `/api/v1/certificates/bind-targets` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslConfigApi.LifecycleBindingTargets](../../api/waf_certificate_lifecycle.go#L120) |
| POST | `/api/v1/certificates/bind` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.LifecycleBind](../../api/waf_certificate_lifecycle.go#L128) |
| GET | `/api/v1/certificates/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafSslConfigApi.LifecycleDetail](../../api/waf_certificate_lifecycle.go#L28) |
| POST | `/api/v1/certificates/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafSslConfigApi.LifecycleList](../../api/waf_certificate_lifecycle.go#L15) |
| POST | `/api/v1/certificates/notifications/retry` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.LifecycleNotificationRetry](../../api/waf_certificate_lifecycle.go#L87) |
| GET | `/api/v1/certificates/notifications` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafSslConfigApi.LifecycleNotifications](../../api/waf_certificate_lifecycle.go#L70) |
| POST | `/api/v1/certificates/policy` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.LifecyclePolicy](../../api/waf_certificate_lifecycle.go#L41) |
| POST | `/api/v1/certificates/renew` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.LifecycleRenew](../../api/waf_certificate_lifecycle.go#L54) |
| GET | `/api/v1/certificates/settings` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslConfigApi.LifecycleSettings](../../api/waf_certificate_lifecycle.go#L101) |
| POST | `/api/v1/dashboard/shares/revoke` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafDashboardScreenApi.RevokeShare](../../api/waf_dashboard_screen.go#L99) |
| GET | `/api/v1/dashboard/shares` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafDashboardScreenApi.Shares](../../api/waf_dashboard_screen.go#L74) |
| POST | `/api/v1/dashboard/shares` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafDashboardScreenApi.CreateShare](../../api/waf_dashboard_screen.go#L82) |
| GET | `/api/v1/dashboard/snapshot` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafDashboardScreenApi.Snapshot](../../api/waf_dashboard_screen.go#L21) |
| POST | `/api/v1/dashboard/target` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafDashboardScreenApi.SaveTarget](../../api/waf_dashboard_screen.go#L30) |
| POST | `/api/v1/diagnostic/cpuprofile/start` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafDiagnosticApi.StartCpuProfileApi](../../api/waf_diagnostic.go#L51) |
| GET | `/api/v1/diagnostic/cpuprofile/status` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafDiagnosticApi.GetCpuProfileStatusApi](../../api/waf_diagnostic.go#L66) |
| GET | `/api/v1/diagnostic/package` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafDiagnosticApi.DownloadPackageApi](../../api/waf_diagnostic.go#L77) |
| GET | `/api/v1/diagnostic/snapshot` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafDiagnosticApi.GetSnapshotApi](../../api/waf_diagnostic.go#L29) |
| GET | `/api/v1/diagnostic/trend` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafDiagnosticApi.GetTrendApi](../../api/waf_diagnostic.go#L40) |
| GET | `/api/v1/export` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafCommonApi.ExportExcelApi](../../api/waf_common.go#L26) |
| GET | `/api/v1/file/data_files` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafFileApi.GetDataFilesApi](../../api/waf_file.go#L64) |
| GET | `/api/v1/file/delete_by_id` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafFileApi.DeleteFileByIdApi](../../api/waf_file.go#L151) |
| POST | `/api/v1/firewall/ipblock/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.AddApi](../../api/waf_firewall_ip_block_api.go#L31) |
| POST | `/api/v1/firewall/ipblock/batch/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.BatchAddApi](../../api/waf_firewall_ip_block_api.go#L187) |
| POST | `/api/v1/firewall/ipblock/batch/del` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.BatchDelApi](../../api/waf_firewall_ip_block_api.go#L170) |
| GET | `/api/v1/firewall/ipblock/capability` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafFirewallIPBlockApi.GetCapabilityApi](../../api/waf_firewall_ip_block_api.go#L290) |
| POST | `/api/v1/firewall/ipblock/clear/expired` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafFirewallIPBlockApi.ClearExpiredApi](../../api/waf_firewall_ip_block_api.go#L267) |
| GET | `/api/v1/firewall/ipblock/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafFirewallIPBlockApi.DelApi](../../api/waf_firewall_ip_block_api.go#L115) |
| GET | `/api/v1/firewall/ipblock/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafFirewallIPBlockApi.GetDetailApi](../../api/waf_firewall_ip_block_api.go#L61) |
| POST | `/api/v1/firewall/ipblock/disable` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.DisableApi](../../api/waf_firewall_ip_block_api.go#L229) |
| POST | `/api/v1/firewall/ipblock/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.ModifyApi](../../api/waf_firewall_ip_block_api.go#L143) |
| POST | `/api/v1/firewall/ipblock/enable` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.EnableApi](../../api/waf_firewall_ip_block_api.go#L212) |
| POST | `/api/v1/firewall/ipblock/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.GetListApi](../../api/waf_firewall_ip_block_api.go#L83) |
| GET | `/api/v1/firewall/ipblock/statistics` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafFirewallIPBlockApi.GetStatisticsApi](../../api/waf_firewall_ip_block_api.go#L277) |
| POST | `/api/v1/firewall/ipblock/sync` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafFirewallIPBlockApi.SyncApi](../../api/waf_firewall_ip_block_api.go#L246) |
| POST | `/api/v1/gpt/chat` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafGPTApi.ChatApi](../../api/waf_gpt.go#L416) |
| POST | `/api/v1/gpt/config/save` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafGPTApi.SaveGptConfigApi](../../api/waf_gpt.go#L276) |
| GET | `/api/v1/gpt/config` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafGPTApi.GetGptConfigApi](../../api/waf_gpt.go#L266) |
| GET | `/api/v1/heartbeat` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafCommonApi.HeartbeatApi](../../api/waf_common.go#L163) |
| POST | `/api/v1/hostconn/block` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostConnApi.BlockApi](../../api/waf_host_conn_api.go#L86) |
| POST | `/api/v1/hostconn/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostConnApi.GetListApi](../../api/waf_host_conn_api.go#L25) |
| GET | `/api/v1/hostconn/refresh` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostConnApi.RefreshApi](../../api/waf_host_conn_api.go#L71) |
| GET | `/api/v1/hostconn/summary` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostConnApi.GetSummaryApi](../../api/waf_host_conn_api.go#L54) |
| POST | `/api/v1/hostguard/ban/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostGuardApi.GetBanListApi](../../api/waf_host_guard_api.go#L114) |
| POST | `/api/v1/hostguard/ban/manual` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.ManualBanApi](../../api/waf_host_guard_api.go#L196) |
| POST | `/api/v1/hostguard/ban/permanent` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.PromoteBanApi](../../api/waf_host_guard_api.go#L170) |
| POST | `/api/v1/hostguard/ban/release` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.ReleaseBanApi](../../api/waf_host_guard_api.go#L144) |
| POST | `/api/v1/hostguard/event/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostGuardApi.GetEventListApi](../../api/waf_host_guard_api.go#L84) |
| GET | `/api/v1/hostguard/ladder/list` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostGuardApi.GetLadderApi](../../api/waf_host_guard_api.go#L307) |
| POST | `/api/v1/hostguard/ladder/save` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.SaveLadderApi](../../api/waf_host_guard_api.go#L326) |
| POST | `/api/v1/hostguard/offender/del` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.DelOffenderApi](../../api/waf_host_guard_api.go#L283) |
| POST | `/api/v1/hostguard/offender/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostGuardApi.GetOffenderListApi](../../api/waf_host_guard_api.go#L226) |
| POST | `/api/v1/hostguard/offender/reset` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.ResetOffenderApi](../../api/waf_host_guard_api.go#L257) |
| GET | `/api/v1/hostguard/statistics` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostGuardApi.GetStatisticsApi](../../api/waf_host_guard_api.go#L71) |
| GET | `/api/v1/hostguard/status` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostGuardApi.GetStatusApi](../../api/waf_host_guard_api.go#L60) |
| POST | `/api/v1/hostguard/whitelist/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGuardApi.AddWhitelistApi](../../api/waf_host_guard_api.go#L377) |
| POST | `/api/v1/hostguard/whitelist/test` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostGuardApi.TestWhitelistApi](../../api/waf_host_guard_api.go#L349) |
| POST | `/api/v1/import` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafCommonApi.ImportExcelApi](../../api/waf_common.go#L100) |
| POST | `/api/v1/iplocation/config/save` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPLocationApi.SaveIPDBConfigApi](../../api/waf_iplocation.go#L202) |
| GET | `/api/v1/iplocation/config` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.GetIPDBConfigApi](../../api/waf_iplocation.go#L191) |
| POST | `/api/v1/iplocation/reload` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.ReloadIPDBApi](../../api/waf_iplocation.go#L418) |
| GET | `/api/v1/iplocation/status` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.GetIPDBStatusApi](../../api/waf_iplocation.go#L122) |
| POST | `/api/v1/iplocation/test` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafIPLocationApi.TestIPLookupApi](../../api/waf_iplocation.go#L514) |
| POST | `/api/v1/iplocation/upgrade/apply` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPLocationApi.ApplyIPDBUpgradeApi](../../api/waf_iplocation.go#L455) |
| POST | `/api/v1/iplocation/upgrade/cancel` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.CancelIPDBUpgradeApi](../../api/waf_iplocation.go#L505) |
| GET | `/api/v1/iplocation/upgrade/check` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.CheckIPDBUpgradeApi](../../api/waf_iplocation.go#L436) |
| GET | `/api/v1/iplocation/upgrade/progress` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.GetIPDBUpgradeProgressApi](../../api/waf_iplocation.go#L498) |
| POST | `/api/v1/iplocation/upload` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafIPLocationApi.UploadIPDBFileApi](../../api/waf_iplocation.go#L264) |
| POST | `/api/v1/license/checklicense` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafLicenseApi.CheckLicense](../../api/waf_license.go#L52) |
| POST | `/api/v1/license/confirm` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafLicenseApi.ConfirmApi](../../api/waf_license.go#L89) |
| GET | `/api/v1/license/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafLicenseApi.GetDetailApi](../../api/waf_license.go#L48) |
| GET | `/api/v1/logfilewrite/backupfiles` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogFileWriteApi.GetBackupFilesApi](../../api/waf_logfilewrite.go#L91) |
| POST | `/api/v1/logfilewrite/clear` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafLogFileWriteApi.ClearLogFileApi](../../api/waf_logfilewrite.go#L116) |
| GET | `/api/v1/logfilewrite/currentfile` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogFileWriteApi.GetCurrentFileInfoApi](../../api/waf_logfilewrite.go#L66) |
| GET | `/api/v1/logfilewrite/preview` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogFileWriteApi.GetPreviewApi](../../api/waf_logfilewrite.go#L26) |
| GET | `/api/v1/logfilewrite/variables` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogFileWriteApi.GetTemplateVariablesApi](../../api/waf_logfilewrite.go#L141) |
| GET | `/api/v1/login_history/list` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafLoginHistoryApi.GetListApi](../../api/waf_login_history.go#L15) |
| POST | `/api/v1/logout` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafLoginApi.LoginOutApi](../../api/waf_login.go#L283) |
| GET | `/api/v1/monitor/system_info` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSystemMonitorApi.GetSystemMonitorApi](../../api/waf_system_monitor.go#L24) |
| POST | `/api/v1/notify/channel/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifyChannelApi.AddApi](../../api/waf_notify_channel.go#L40) |
| GET | `/api/v1/notify/channel/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafNotifyChannelApi.DelApi](../../api/waf_notify_channel.go#L139) |
| GET | `/api/v1/notify/channel/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafNotifyChannelApi.GetDetailApi](../../api/waf_notify_channel.go#L88) |
| POST | `/api/v1/notify/channel/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifyChannelApi.ModifyApi](../../api/waf_notify_channel.go#L166) |
| POST | `/api/v1/notify/channel/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafNotifyChannelApi.GetListApi](../../api/waf_notify_channel.go#L110) |
| POST | `/api/v1/notify/channel/test` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifyChannelApi.TestApi](../../api/waf_notify_channel.go#L203) |
| GET | `/api/v1/notify/channel/types` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafNotifyChannelApi.GetTypesApi](../../api/waf_notify_channel.go#L219) |
| POST | `/api/v1/notify/globalthrottle/update` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.UpdateGlobalThrottleApi](../../api/waf_notify_subscription_config.go#L489) |
| GET | `/api/v1/notify/globalthrottle` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafNotifySubscriptionApi.GetGlobalThrottleApi](../../api/waf_notify_subscription_config.go#L456) |
| GET | `/api/v1/notify/log/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafNotifyLogApi.DelApi](../../api/waf_notify_log.go#L75) |
| GET | `/api/v1/notify/log/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafNotifyLogApi.GetDetailApi](../../api/waf_notify_log.go#L54) |
| POST | `/api/v1/notify/log/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafNotifyLogApi.GetListApi](../../api/waf_notify_log.go#L28) |
| POST | `/api/v1/notify/subscription/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.AddApi](../../api/waf_notify_subscription.go#L28) |
| POST | `/api/v1/notify/subscription/batchconfig` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.BatchConfigApi](../../api/waf_notify_subscription_config.go#L205) |
| POST | `/api/v1/notify/subscription/config` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.SaveConfigApi](../../api/waf_notify_subscription_config.go#L156) |
| GET | `/api/v1/notify/subscription/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafNotifySubscriptionApi.DelApi](../../api/waf_notify_subscription.go#L98) |
| GET | `/api/v1/notify/subscription/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafNotifySubscriptionApi.GetDetailApi](../../api/waf_notify_subscription.go#L51) |
| POST | `/api/v1/notify/subscription/dryrun` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.DryRunApi](../../api/waf_notify_subscription_config.go#L384) |
| POST | `/api/v1/notify/subscription/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.ModifyApi](../../api/waf_notify_subscription.go#L116) |
| POST | `/api/v1/notify/subscription/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.GetListApi](../../api/waf_notify_subscription.go#L72) |
| POST | `/api/v1/notify/subscription/preview` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.PreviewApi](../../api/waf_notify_subscription_config.go#L303) |
| GET | `/api/v1/notify/subscription/templatevars` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafNotifySubscriptionApi.TemplateVarsApi](../../api/waf_notify_subscription_config.go#L429) |
| POST | `/api/v1/notify/subscription/test` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafNotifySubscriptionApi.TestApi](../../api/waf_notify_subscription_config.go#L344) |
| GET | `/api/v1/oplatform/doc/api` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafOPlatformDocApi.GetDocApi](../../api/waf_oplatform_doc.go#L593) |
| GET | `/api/v1/oplatform/doc/status` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafOPlatformDocApi.GetDocStatusApi](../../api/waf_oplatform_doc.go#L661) |
| POST | `/api/v1/oplatform/key/add` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOPlatformKeyApi.AddApi](../../api/waf_oplatform_key.go#L14) |
| GET | `/api/v1/oplatform/key/del` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafOPlatformKeyApi.DelApi](../../api/waf_oplatform_key.go#L47) |
| GET | `/api/v1/oplatform/key/detail` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafOPlatformKeyApi.GetDetailApi](../../api/waf_oplatform_key.go#L61) |
| POST | `/api/v1/oplatform/key/edit` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOPlatformKeyApi.ModifyApi](../../api/waf_oplatform_key.go#L33) |
| POST | `/api/v1/oplatform/key/list` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOPlatformKeyApi.GetListApi](../../api/waf_oplatform_key.go#L72) |
| POST | `/api/v1/oplatform/key/resetSecret` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOPlatformKeyApi.ResetSecretApi](../../api/waf_oplatform_key.go#L98) |
| GET | `/api/v1/oplatform/log/del` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafOPlatformLogApi.DelApi](../../api/waf_oplatform_log.go#L51) |
| GET | `/api/v1/oplatform/log/detail` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafOPlatformLogApi.GetDetailApi](../../api/waf_oplatform_log.go#L14) |
| POST | `/api/v1/oplatform/log/list` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOPlatformLogApi.GetListApi](../../api/waf_oplatform_log.go#L25) |
| GET | `/api/v1/owasp/audit_log` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.AuditLogApi](../../api/waf_owasp.go#L354) |
| GET | `/api/v1/owasp/base_config` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.BaseConfigGetApi](../../api/waf_owasp.go#L461) |
| POST | `/api/v1/owasp/base_config` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.BaseConfigSetApi](../../api/waf_owasp.go#L484) |
| DELETE | `/api/v1/owasp/crs_var` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.CRSVarDeleteApi](../../api/waf_owasp.go#L431) |
| POST | `/api/v1/owasp/crs_var` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.CRSVarSetApi](../../api/waf_owasp.go#L399) |
| GET | `/api/v1/owasp/crs_vars` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.CRSVarsGetApi](../../api/waf_owasp.go#L376) |
| GET | `/api/v1/owasp/file_content` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.FileContentApi](../../api/waf_owasp.go#L735) |
| GET | `/api/v1/owasp/files` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.FilesListApi](../../api/waf_owasp.go#L703) |
| POST | `/api/v1/owasp/hit_stats/reset` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.HitStatsResetApi](../../api/waf_owasp.go#L928) |
| GET | `/api/v1/owasp/hit_stats` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.HitStatsApi](../../api/waf_owasp.go#L902) |
| POST | `/api/v1/owasp/reload` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.ReloadApi](../../api/waf_owasp.go#L681) |
| GET | `/api/v1/owasp/rule_detail` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.RuleDetailApi](../../api/waf_owasp.go#L175) |
| POST | `/api/v1/owasp/rule_override` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.RuleOverrideApi](../../api/waf_owasp.go#L271) |
| POST | `/api/v1/owasp/rule_reset` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.RuleResetApi](../../api/waf_owasp.go#L324) |
| POST | `/api/v1/owasp/rule_toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.RuleToggleApi](../../api/waf_owasp.go#L233) |
| GET | `/api/v1/owasp/rules` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafOwaspApi.RulesListApi](../../api/waf_owasp.go#L51) |
| POST | `/api/v1/owasp/test/dry_run` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.TestDryRunApi](../../api/waf_owasp.go#L763) |
| GET | `/api/v1/owasp/tuning` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.TuningGetApi](../../api/waf_owasp.go#L549) |
| POST | `/api/v1/owasp/tuning` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOwaspApi.TuningSetApi](../../api/waf_owasp.go#L572) |
| POST | `/api/v1/owasp/update/apply` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.UpdateApplyApi](../../api/waf_owasp.go#L861) |
| GET | `/api/v1/owasp/update/check` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.UpdateCheckApi](../../api/waf_owasp.go#L839) |
| GET | `/api/v1/owasp/usage/doc` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafOwaspApi.UsageDocApi](../../api/waf_owasp.go#L886) |
| GET | `/api/v1/pipeline/cc/hits` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineCCHits](../../api/waf_pipeline_cc.go#L39) |
| POST | `/api/v1/pipeline/cc/recommend` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.RecommendPipelineCC](../../api/waf_pipeline_cc.go#L18) |
| GET | `/api/v1/pipeline/defense/capabilities` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineDefenseCapabilities](../../api/waf_pipeline_defense.go#L24) |
| GET | `/api/v1/pipeline/defense` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineDefense](../../api/waf_pipeline_defense.go#L11) |
| POST | `/api/v1/pipeline/explain` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.ExplainPort](../../api/waf_pipeline.go#L312) |
| POST | `/api/v1/pipeline/feed/save` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SaveFeed](../../api/waf_pipeline.go#L417) |
| POST | `/api/v1/pipeline/feed/sync` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SyncFeed](../../api/waf_pipeline.go#L440) |
| GET | `/api/v1/pipeline/feed` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.FeedConfig](../../api/waf_pipeline.go#L391) |
| POST | `/api/v1/pipeline/gate/sessions/kick` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.KickGateSession](../../api/waf_pipeline_gate.go#L125) |
| POST | `/api/v1/pipeline/gate/sessions/revoke` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.RevokeGateSessions](../../api/waf_pipeline_gate.go#L146) |
| GET | `/api/v1/pipeline/gate/sessions` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.GateSessions](../../api/waf_pipeline_gate.go#L105) |
| POST | `/api/v1/pipeline/gate/token/rotate` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.RotateGateToken](../../api/waf_pipeline_gate.go#L171) |
| POST | `/api/v1/pipeline/gate/users/delete` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.DeleteGateUser](../../api/waf_pipeline_gate.go#L81) |
| POST | `/api/v1/pipeline/gate/users/save` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SaveGateUser](../../api/waf_pipeline_gate.go#L57) |
| GET | `/api/v1/pipeline/gate/users` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.GateUsers](../../api/waf_pipeline_gate.go#L32) |
| GET | `/api/v1/pipeline/honeypot/logs` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SharedPipelineHoneypotLogs](../../api/waf_pipeline_defense.go#L77) |
| POST | `/api/v1/pipeline/honeypot/save` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SaveSharedPipelineHoneypot](../../api/waf_pipeline_defense.go#L64) |
| GET | `/api/v1/pipeline/honeypot` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SharedPipelineHoneypot](../../api/waf_pipeline_defense.go#L39) |
| GET | `/api/v1/pipeline/issues` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineIssues](../../api/waf_pipeline.go#L138) |
| GET | `/api/v1/pipeline/logs` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineLogs](../../api/waf_pipeline.go#L274) |
| POST | `/api/v1/pipeline/migration/import` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.ImportLegacyPipelinePort](../../api/waf_pipeline_migration.go#L67) |
| GET | `/api/v1/pipeline/migration/legacy` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.LegacyPipelinePorts](../../api/waf_pipeline_migration.go#L23) |
| POST | `/api/v1/pipeline/migration/preview` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PreviewLegacyPipelinePort](../../api/waf_pipeline_migration.go#L43) |
| GET | `/api/v1/pipeline/overview` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineOverview](../../api/waf_pipeline.go#L33) |
| POST | `/api/v1/pipeline/ports/access` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineAccess](../../api/waf_pipeline_defense.go#L27) |
| POST | `/api/v1/pipeline/ports/check` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.CheckPipelinePort](../../api/waf_pipeline.go#L90) |
| POST | `/api/v1/pipeline/ports/copy` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.CopyPort](../../api/waf_pipeline.go#L666) |
| POST | `/api/v1/pipeline/ports/delete` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.DeletePort](../../api/waf_pipeline.go#L512) |
| POST | `/api/v1/pipeline/ports/save` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SavePort](../../api/waf_pipeline.go#L486) |
| GET | `/api/v1/pipeline/ports` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.ListPorts](../../api/waf_pipeline.go#L465) |
| GET | `/api/v1/pipeline/regions` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineRegions](../../api/waf_pipeline.go#L372) |
| POST | `/api/v1/pipeline/state/manage` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.ManagePipelineState](../../api/waf_pipeline.go#L245) |
| GET | `/api/v1/pipeline/state` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineState](../../api/waf_pipeline.go#L210) |
| GET | `/api/v1/pipeline/stats` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PipelineStats](../../api/waf_pipeline.go#L154) |
| POST | `/api/v1/pipeline/templates/delete` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.DeleteTemplate](../../api/waf_pipeline.go#L581) |
| GET | `/api/v1/pipeline/templates/export` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.ExportTemplate](../../api/waf_pipeline.go#L635) |
| POST | `/api/v1/pipeline/templates/import-preview` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PreviewTemplateImport](../../api/waf_pipeline.go#L114) |
| POST | `/api/v1/pipeline/templates/preview` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.PreviewTemplate](../../api/waf_pipeline.go#L605) |
| POST | `/api/v1/pipeline/templates/save` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.SaveTemplate](../../api/waf_pipeline.go#L556) |
| GET | `/api/v1/pipeline/templates` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.Templates](../../api/waf_pipeline.go#L535) |
| POST | `/api/v1/pipeline/upstream/test` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafPipelineApi.TestUpstream](../../api/waf_pipeline.go#L691) |
| GET | `/api/v1/public/dashboard-share` | 公开 | 无 | 单独验证 | 自定义/无请求体 | [WafDashboardScreenApi.PublicShare](../../api/waf_dashboard_screen.go#L116) |
| POST | `/api/v1/public/login` | 公开 | 无 | 单独验证 | JSON | [WafLoginApi.LoginApi](../../api/waf_login.go#L29) |
| POST | `/api/v1/public/seckey` | 公开 | 无 | 单独验证 | JSON | [WafSecKeyApi.SecKeyApi](../../api/waf_seckey.go#L31) |
| GET | `/api/v1/public/uiconfig` | 公开 | 无 | 单独验证 | 自定义/无请求体 | [WafVpConfigApi.GetAdminTitlesApi](../../api/waf_admin_titles_api.go#L38) |
| GET | `/api/v1/resetWAF` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafEngineApi.ResetWaf](../../api/waf_engine.go#L22) |
| POST | `/api/v1/sql_query/execute` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafSqlQueryApi.ExecuteQueryApi](../../api/waf_sql_query.go#L17) |
| GET | `/api/v1/sql_query/queryable` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafSqlQueryApi.GetQueryableSchemaApi](../../api/waf_sql_query.go#L60) |
| GET | `/api/v1/sql_query/table_info` | 仅登录令牌 | 有 | 通过 × 3 | 参数绑定 | [WafSqlQueryApi.GetTableInfoApi](../../api/waf_sql_query.go#L45) |
| POST | `/api/v1/sslconfig/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.AddSslConfigApi](../../api/waf_ssl.go#L32) |
| GET | `/api/v1/sslconfig/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSslConfigApi.DelSslConfigApi](../../api/waf_ssl.go#L111) |
| GET | `/api/v1/sslconfig/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafSslConfigApi.GetSslConfigDetailApi](../../api/waf_ssl.go#L60) |
| POST | `/api/v1/sslconfig/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslConfigApi.ModifySslConfigApi](../../api/waf_ssl.go#L138) |
| POST | `/api/v1/sslconfig/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafSslConfigApi.GetSslConfigListApi](../../api/waf_ssl.go#L81) |
| GET | `/api/v1/statrumtimesysinfo` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafStatApi.StatRumtimeSysinfoApi](../../api/waf_stat.go#L42) |
| GET | `/api/v1/statsysinfo` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafStatApi.StatSysinfoApi](../../api/waf_stat.go#L25) |
| GET | `/api/v1/sys_log/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSysLogApi.GetDetailApi](../../api/waf_sys_log.go#L24) |
| GET | `/api/v1/sys_log/list` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSysLogApi.GetListApi](../../api/waf_sys_log.go#L45) |
| GET | `/api/v1/sysinfo/announcement` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.GetAnnouncementApi](../../api/waf_sys_info.go#L64) |
| GET | `/api/v1/sysinfo/cancelupdate` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.CancelUpdateApi](../../api/waf_sys_info.go#L419) |
| GET | `/api/v1/sysinfo/checkversion` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.CheckVersionApi](../../api/waf_sys_info.go#L165) |
| GET | `/api/v1/sysinfo/rollback` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.RollbackApi](../../api/waf_sys_info.go#L258) |
| GET | `/api/v1/sysinfo/rollbacklist` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.RollbackListApi](../../api/waf_sys_info.go#L247) |
| GET | `/api/v1/sysinfo/runtimeinfo` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.SysRuntimeInfoApi](../../api/waf_sys_info.go#L130) |
| GET | `/api/v1/sysinfo/systemparams` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.SystemParamsApi](../../api/waf_sys_info.go#L221) |
| GET | `/api/v1/sysinfo/update` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.UpdateApi](../../api/waf_sys_info.go#L281) |
| GET | `/api/v1/sysinfo/updateprogress` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.UpdateProgressApi](../../api/waf_sys_info.go#L407) |
| GET | `/api/v1/sysinfo/version` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSysInfoApi.SysVersionApi](../../api/waf_sys_info.go#L113) |
| POST | `/api/v1/systemconfig/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSystemConfigApi.AddApi](../../api/waf_system_config.go#L25) |
| GET | `/api/v1/systemconfig/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSystemConfigApi.DelApi](../../api/waf_system_config.go#L99) |
| GET | `/api/v1/systemconfig/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSystemConfigApi.GetDetailApi](../../api/waf_system_config.go#L59) |
| POST | `/api/v1/systemconfig/editByItem` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSystemConfigApi.ModifyByItemApi](../../api/waf_system_config.go#L161) |
| POST | `/api/v1/systemconfig/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSystemConfigApi.ModifyApi](../../api/waf_system_config.go#L127) |
| GET | `/api/v1/systemconfig/getdetailByItem` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSystemConfigApi.GetDetailByItemApi](../../api/waf_system_config.go#L192) |
| POST | `/api/v1/systemconfig/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSystemConfigApi.GetListApi](../../api/waf_system_config.go#L81) |
| POST | `/api/v1/threatip/channel/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafThreatIPApi.AddApi](../../api/waf_threat_ip_api.go#L26) |
| GET | `/api/v1/threatip/channel/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafThreatIPApi.DelApi](../../api/waf_threat_ip_api.go#L70) |
| GET | `/api/v1/threatip/channel/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafThreatIPApi.GetDetailApi](../../api/waf_threat_ip_api.go#L120) |
| POST | `/api/v1/threatip/channel/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafThreatIPApi.ModifyApi](../../api/waf_threat_ip_api.go#L49) |
| POST | `/api/v1/threatip/channel/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafThreatIPApi.GetListApi](../../api/waf_threat_ip_api.go#L93) |
| POST | `/api/v1/threatip/channel/sync` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafThreatIPApi.SyncApi](../../api/waf_threat_ip_api.go#L140) |
| POST | `/api/v1/threatip/exclude/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafThreatIPExcludeApi.AddApi](../../api/waf_threat_ip_exclude_api.go#L38) |
| POST | `/api/v1/threatip/exclude/audit` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafThreatIPExcludeApi.GetAuditListApi](../../api/waf_threat_ip_exclude_api.go#L154) |
| GET | `/api/v1/threatip/exclude/builtin` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafThreatIPExcludeApi.EffectiveRulesApi](../../api/waf_threat_ip_exclude_api.go#L149) |
| GET | `/api/v1/threatip/exclude/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafThreatIPExcludeApi.DelApi](../../api/waf_threat_ip_exclude_api.go#L110) |
| POST | `/api/v1/threatip/exclude/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafThreatIPExcludeApi.ModifyApi](../../api/waf_threat_ip_exclude_api.go#L87) |
| POST | `/api/v1/threatip/exclude/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafThreatIPExcludeApi.GetListApi](../../api/waf_threat_ip_exclude_api.go#L125) |
| POST | `/api/v1/threatip/exclude/preview` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafThreatIPExcludeApi.PreviewApi](../../api/waf_threat_ip_exclude_api.go#L72) |
| POST | `/api/v1/threatip/landed/ips` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafThreatIPApi.LandedIPsApi](../../api/waf_threat_ip_api.go#L165) |
| GET | `/api/v1/threatip/landed/summary` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafThreatIPApi.LandedSummaryApi](../../api/waf_threat_ip_api.go#L154) |
| POST | `/api/v1/tunnel/tunnel/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTunnelApi.AddApi](../../api/waf_tunnel_api.go#L36) |
| GET | `/api/v1/tunnel/tunnel/connections` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafTunnelApi.GetTunnelConnectionsApi](../../api/waf_tunnel_api.go#L232) |
| GET | `/api/v1/tunnel/tunnel/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafTunnelApi.DelApi](../../api/waf_tunnel_api.go#L161) |
| GET | `/api/v1/tunnel/tunnel/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafTunnelApi.GetDetailApi](../../api/waf_tunnel_api.go#L114) |
| POST | `/api/v1/tunnel/tunnel/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTunnelApi.ModifyApi](../../api/waf_tunnel_api.go#L196) |
| POST | `/api/v1/tunnel/tunnel/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafTunnelApi.GetListApi](../../api/waf_tunnel_api.go#L135) |
| GET | `/api/v1/uipreference/get` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafUIPreferenceApi.GetApi](../../api/waf_ui_preference_api.go#L16) |
| POST | `/api/v1/uipreference/save` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafUIPreferenceApi.SaveApi](../../api/waf_ui_preference_api.go#L37) |
| POST | `/api/v1/upgradenotice/ack` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafUpgradeNoticeApi.AckApi](../../api/waf_upgrade_notice_api.go#L75) |
| POST | `/api/v1/upgradenotice/downgradeack` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafUpgradeNoticeApi.DowngradeAckApi](../../api/waf_upgrade_notice_api.go#L144) |
| POST | `/api/v1/upgradenotice/ignore` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafUpgradeNoticeApi.IgnoreApi](../../api/waf_upgrade_notice_api.go#L89) |
| POST | `/api/v1/upgradenotice/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafUpgradeNoticeApi.GetListApi](../../api/waf_upgrade_notice_api.go#L46) |
| POST | `/api/v1/upgradenotice/popupshown` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafUpgradeNoticeApi.PopupShownApi](../../api/waf_upgrade_notice_api.go#L128) |
| POST | `/api/v1/upgradenotice/restore` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafUpgradeNoticeApi.RestoreApi](../../api/waf_upgrade_notice_api.go#L103) |
| GET | `/api/v1/upgradenotice/summary` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafUpgradeNoticeApi.GetSummaryApi](../../api/waf_upgrade_notice_api.go#L26) |
| GET | `/api/v1/vipconfig/cdnProviderRanges` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetCDNProviderRangesApi](../../api/waf_vpconfig_api.go#L214) |
| POST | `/api/v1/vipconfig/clearLocalCert` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.ClearLocalCertApi](../../api/waf_localcert_api.go#L107) |
| POST | `/api/v1/vipconfig/generateLocalCert` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.GenerateLocalCertApi](../../api/waf_localcert_api.go#L27) |
| GET | `/api/v1/vipconfig/getAdminTitles` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetAdminTitlesApi](../../api/waf_admin_titles_api.go#L38) |
| GET | `/api/v1/vipconfig/getCorsAllowOrigins` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetCorsAllowOriginsApi](../../api/waf_vpconfig_api.go#L234) |
| GET | `/api/v1/vipconfig/getDomainWhitelist` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetDomainWhitelistApi](../../api/waf_vpconfig_api.go#L614) |
| GET | `/api/v1/vipconfig/getIpWhitelist` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetIpWhitelistApi](../../api/waf_vpconfig_api.go#L102) |
| GET | `/api/v1/vipconfig/getManageCDNProvider` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetManageCDNProviderApi](../../api/waf_vpconfig_api.go#L186) |
| GET | `/api/v1/vipconfig/getManageTrustedProxies` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetManageTrustedProxiesApi](../../api/waf_vpconfig_api.go#L122) |
| GET | `/api/v1/vipconfig/getNoticeTitle` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetNoticeTitleApi](../../api/waf_vpconfig_api.go#L570) |
| GET | `/api/v1/vipconfig/getSecurityEntry` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetSecurityEntryApi](../../api/waf_vpconfig_api.go#L524) |
| GET | `/api/v1/vipconfig/getSslBindCert` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetSslBindCertApi](../../api/waf_vpconfig_api.go#L766) |
| GET | `/api/v1/vipconfig/getSslForceHttps` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetSslForceHttpsApi](../../api/waf_vpconfig_api.go#L727) |
| GET | `/api/v1/vipconfig/getSslStatus` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetSslStatusApi](../../api/waf_vpconfig_api.go#L303) |
| GET | `/api/v1/vipconfig/localCertStatus` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetLocalCertStatusApi](../../api/waf_localcert_api.go#L123) |
| GET | `/api/v1/vipconfig/manageClientIpProbe` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.GetManageClientIPProbeApi](../../api/waf_vpconfig_api.go#L181) |
| POST | `/api/v1/vipconfig/restartManager` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.RestartManagerApi](../../api/waf_vpconfig_api.go#L674) |
| POST | `/api/v1/vipconfig/rotateLocalCa` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.RotateLocalCaApi](../../api/waf_localcert_api.go#L71) |
| POST | `/api/v1/vipconfig/updateAdminTitles` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafVpConfigApi.UpdateAdminTitlesApi](../../api/waf_admin_titles_api.go#L45) |
| POST | `/api/v1/vipconfig/updateCorsAllowOrigins` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateCorsAllowOriginsApi](../../api/waf_vpconfig_api.go#L243) |
| POST | `/api/v1/vipconfig/updateDomainWhitelist` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateDomainWhitelistApi](../../api/waf_vpconfig_api.go#L631) |
| POST | `/api/v1/vipconfig/updateIpWhitelist` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateIpWhitelistApi](../../api/waf_vpconfig_api.go#L43) |
| POST | `/api/v1/vipconfig/updateManageCDNProvider` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateManageCDNProviderApi](../../api/waf_vpconfig_api.go#L191) |
| POST | `/api/v1/vipconfig/updateManageTrustedProxies` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateManageTrustedProxiesApi](../../api/waf_vpconfig_api.go#L139) |
| POST | `/api/v1/vipconfig/updateNoticeTitle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateNoticeTitleApi](../../api/waf_vpconfig_api.go#L587) |
| POST | `/api/v1/vipconfig/updateSecurityEntry` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateSecurityEntryApi](../../api/waf_vpconfig_api.go#L542) |
| POST | `/api/v1/vipconfig/updateSslBindCert` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateSslBindCertApi](../../api/waf_vpconfig_api.go#L790) |
| POST | `/api/v1/vipconfig/updateSslEnable` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateSslEnableApi](../../api/waf_vpconfig_api.go#L278) |
| POST | `/api/v1/vipconfig/updateSslForceHttps` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UpdateSslForceHttpsApi](../../api/waf_vpconfig_api.go#L744) |
| POST | `/api/v1/vipconfig/uploadSslCert` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafVpConfigApi.UploadSslCertApi](../../api/waf_vpconfig_api.go#L361) |
| POST | `/api/v1/wafhost/accessaccount/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.AddApi](../../api/waf_access.go#L32) |
| GET | `/api/v1/wafhost/accessaccount/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccessAccountApi.DelApi](../../api/waf_access.go#L150) |
| GET | `/api/v1/wafhost/accessaccount/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccessAccountApi.GetDetailApi](../../api/waf_access.go#L82) |
| POST | `/api/v1/wafhost/accessaccount/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.ModifyApi](../../api/waf_access.go#L106) |
| POST | `/api/v1/wafhost/accessaccount/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.GetListApi](../../api/waf_access.go#L55) |
| POST | `/api/v1/wafhost/accessaccount/otp/bind` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.OtpBindApi](../../api/waf_access.go#L195) |
| GET | `/api/v1/wafhost/accessaccount/otp/init` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccessAccountApi.OtpInitApi](../../api/waf_access.go#L172) |
| POST | `/api/v1/wafhost/accessaccount/otp/unbind` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.OtpUnbindApi](../../api/waf_access.go#L217) |
| POST | `/api/v1/wafhost/accessaccount/resetpwd` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAccountApi.ResetPwdApi](../../api/waf_access.go#L129) |
| POST | `/api/v1/wafhost/accessaudit/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessAuditApi.GetListApi](../../api/waf_access.go#L403) |
| GET | `/api/v1/wafhost/accessconfig/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAccessConfigApi.GetDetailApi](../../api/waf_access.go#L243) |
| GET | `/api/v1/wafhost/accessconfig/hostoptions` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAccessConfigApi.GetCenterHostOptionsApi](../../api/waf_access.go#L255) |
| POST | `/api/v1/wafhost/accessconfig/regenerate_secret` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAccessConfigApi.RegenerateSecretApi](../../api/waf_access.go#L290) |
| POST | `/api/v1/wafhost/accessconfig/save` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessConfigApi.SaveApi](../../api/waf_access.go#L269) |
| GET | `/api/v1/wafhost/accesssession/kick` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAccessSessionApi.KickApi](../../api/waf_access.go#L340) |
| GET | `/api/v1/wafhost/accesssession/kickall` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAccessSessionApi.KickAllApi](../../api/waf_access.go#L384) |
| POST | `/api/v1/wafhost/accesssession/kickbyaccount` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessSessionApi.KickByAccountApi](../../api/waf_access.go#L362) |
| POST | `/api/v1/wafhost/accesssession/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAccessSessionApi.GetListApi](../../api/waf_access.go#L312) |
| POST | `/api/v1/wafhost/anticc/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCApi.AddApi](../../api/waf_anticc.go#L39) |
| GET | `/api/v1/wafhost/anticc/baniplist` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAntiCCApi.GetBanIpListApi](../../api/waf_anticc.go#L127) |
| GET | `/api/v1/wafhost/anticc/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCApi.DelAntiCCApi](../../api/waf_anticc.go#L207) |
| GET | `/api/v1/wafhost/anticc/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCApi.GetDetailApi](../../api/waf_anticc.go#L90) |
| POST | `/api/v1/wafhost/anticc/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCApi.ModifyAntiCCApi](../../api/waf_anticc.go#L237) |
| POST | `/api/v1/wafhost/anticc/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCApi.GetListApi](../../api/waf_anticc.go#L111) |
| POST | `/api/v1/wafhost/anticc/removebanip` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCApi.RemoveCCBanIPApi](../../api/waf_anticc.go#L173) |
| POST | `/api/v1/wafhost/anticcrule/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.AddApi](../../api/waf_anticc_rule.go#L85) |
| GET | `/api/v1/wafhost/anticcrule/captchaoverview` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafAntiCCRuleApi.GetCaptchaOverviewApi](../../api/waf_anticc_rule.go#L227) |
| GET | `/api/v1/wafhost/anticcrule/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCRuleApi.DelApi](../../api/waf_anticc_rule.go#L115) |
| GET | `/api/v1/wafhost/anticcrule/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCRuleApi.GetDetailApi](../../api/waf_anticc_rule.go#L76) |
| POST | `/api/v1/wafhost/anticcrule/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.ModifyApi](../../api/waf_anticc_rule.go#L100) |
| GET | `/api/v1/wafhost/anticcrule/emergency/status` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCRuleApi.GetEmergencyStatusApi](../../api/waf_anticc_rule.go#L343) |
| POST | `/api/v1/wafhost/anticcrule/emergency` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.SetEmergencyApi](../../api/waf_anticc_rule.go#L318) |
| GET | `/api/v1/wafhost/anticcrule/hits` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAntiCCRuleApi.GetHitsApi](../../api/waf_anticc_rule.go#L287) |
| POST | `/api/v1/wafhost/anticcrule/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.GetListApi](../../api/waf_anticc_rule.go#L42) |
| POST | `/api/v1/wafhost/anticcrule/sort` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.SortApi](../../api/waf_anticc_rule.go#L148) |
| POST | `/api/v1/wafhost/anticcrule/threshold/recommend` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.RecommendThresholdApi](../../api/waf_anticc_rule.go#L256) |
| POST | `/api/v1/wafhost/anticcrule/toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAntiCCRuleApi.ToggleApi](../../api/waf_anticc_rule.go#L133) |
| POST | `/api/v1/wafhost/blockingpage/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockingPageApi.AddApi](../../api/waf_blockingpage_api.go#L70) |
| GET | `/api/v1/wafhost/blockingpage/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBlockingPageApi.DelApi](../../api/waf_blockingpage_api.go#L140) |
| GET | `/api/v1/wafhost/blockingpage/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafBlockingPageApi.GetDetailApi](../../api/waf_blockingpage_api.go#L105) |
| POST | `/api/v1/wafhost/blockingpage/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockingPageApi.ModifyApi](../../api/waf_blockingpage_api.go#L163) |
| POST | `/api/v1/wafhost/blockingpage/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafBlockingPageApi.GetListApi](../../api/waf_blockingpage_api.go#L116) |
| GET | `/api/v1/wafhost/blockingpage/unmatchedbuiltin` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafBlockingPageApi.GetUnmatchedBuiltinApi](../../api/waf_blockingpage_api.go#L192) |
| POST | `/api/v1/wafhost/cacherule/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCacheRuleApi.AddApi](../../api/waf_cacherule_api.go#L30) |
| GET | `/api/v1/wafhost/cacherule/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafCacheRuleApi.DelApi](../../api/waf_cacherule_api.go#L112) |
| GET | `/api/v1/wafhost/cacherule/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafCacheRuleApi.GetDetailApi](../../api/waf_cacherule_api.go#L65) |
| POST | `/api/v1/wafhost/cacherule/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCacheRuleApi.ModifyApi](../../api/waf_cacherule_api.go#L145) |
| POST | `/api/v1/wafhost/cacherule/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafCacheRuleApi.GetListApi](../../api/waf_cacherule_api.go#L86) |
| POST | `/api/v1/wafhost/caserverinfo/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCaServerInfoApi.AddApi](../../api/waf_caserverinfo_api.go#L16) |
| GET | `/api/v1/wafhost/caserverinfo/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafCaServerInfoApi.DelApi](../../api/waf_caserverinfo_api.go#L67) |
| GET | `/api/v1/wafhost/caserverinfo/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafCaServerInfoApi.GetDetailApi](../../api/waf_caserverinfo_api.go#L40) |
| POST | `/api/v1/wafhost/caserverinfo/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafCaServerInfoApi.ModifyApi](../../api/waf_caserverinfo_api.go#L84) |
| POST | `/api/v1/wafhost/caserverinfo/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafCaServerInfoApi.GetListApi](../../api/waf_caserverinfo_api.go#L51) |
| GET | `/api/v1/wafhost/dataretention/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafDataRetentionApi.GetDetailApi](../../api/waf_data_retention_api.go#L52) |
| POST | `/api/v1/wafhost/dataretention/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafDataRetentionApi.ModifyApi](../../api/waf_data_retention_api.go#L76) |
| POST | `/api/v1/wafhost/dataretention/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafDataRetentionApi.GetListApi](../../api/waf_data_retention_api.go#L22) |
| POST | `/api/v1/wafhost/host/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.AddApi](../../api/waf_host.go#L181) |
| GET | `/api/v1/wafhost/host/alldomainbyhostcode` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafHostAPi.GetDomainsByHostCodeApi](../../api/waf_host.go#L417) |
| GET | `/api/v1/wafhost/host/allhost` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostAPi.GetAllListApi](../../api/waf_host.go#L387) |
| POST | `/api/v1/wafhost/host/batchcopyconfig` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.BatchCopyConfigApi](../../api/waf_host.go#L248) |
| POST | `/api/v1/wafhost/host/checkports` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostAPi.CheckPortsApi](../../api/waf_host.go#L685) |
| GET | `/api/v1/wafhost/host/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHostAPi.DelHostApi](../../api/waf_host.go#L463) |
| GET | `/api/v1/wafhost/host/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafHostAPi.GetDetailApi](../../api/waf_host.go#L292) |
| POST | `/api/v1/wafhost/host/diagnostic` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.DiagnosticApi](../../api/waf_host_diagnostic.go#L84) |
| POST | `/api/v1/wafhost/host/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.ModifyHostApi](../../api/waf_host.go#L492) |
| GET | `/api/v1/wafhost/host/guardstatus` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHostAPi.ModifyGuardStatusApi](../../api/waf_host.go#L563) |
| POST | `/api/v1/wafhost/host/health/test` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.TestHealthApi](../../api/waf_host_health.go#L85) |
| GET | `/api/v1/wafhost/host/ipsource/probe/clear` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafHostAPi.IPSourceProbeClearApi](../../api/waf_host_ipprobe.go#L92) |
| GET | `/api/v1/wafhost/host/ipsource/probe` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostAPi.IPSourceProbeApi](../../api/waf_host_ipprobe.go#L40) |
| POST | `/api/v1/wafhost/host/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostAPi.GetListApi](../../api/waf_host.go#L318) |
| POST | `/api/v1/wafhost/host/modfiyallstatus` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostAPi.ModifyAllGuardStatusApi](../../api/waf_host.go#L583) |
| GET | `/api/v1/wafhost/host/startstatus` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHostAPi.ModifyStartStatusApi](../../api/waf_host.go#L638) |
| POST | `/api/v1/wafhost/hostgroup/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGroupApi.AddApi](../../api/waf_host_group.go#L25) |
| GET | `/api/v1/wafhost/hostgroup/all` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostGroupApi.GetAllApi](../../api/waf_host_group.go#L140) |
| POST | `/api/v1/wafhost/hostgroup/assign` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGroupApi.AssignApi](../../api/waf_host_group.go#L181) |
| GET | `/api/v1/wafhost/hostgroup/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHostGroupApi.DelApi](../../api/waf_host_group.go#L72) |
| GET | `/api/v1/wafhost/hostgroup/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafHostGroupApi.GetDetailApi](../../api/waf_host_group.go#L98) |
| POST | `/api/v1/wafhost/hostgroup/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGroupApi.ModifyApi](../../api/waf_host_group.go#L49) |
| POST | `/api/v1/wafhost/hostgroup/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostGroupApi.GetListApi](../../api/waf_host_group.go#L117) |
| POST | `/api/v1/wafhost/hostgroup/sort` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostGroupApi.SortApi](../../api/waf_host_group.go#L154) |
| POST | `/api/v1/wafhost/httpauthbase/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHttpAuthBaseApi.AddApi](../../api/waf_httpauthbase_api.go#L20) |
| GET | `/api/v1/wafhost/httpauthbase/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHttpAuthBaseApi.DelApi](../../api/waf_httpauthbase_api.go#L72) |
| GET | `/api/v1/wafhost/httpauthbase/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafHttpAuthBaseApi.GetDetailApi](../../api/waf_httpauthbase_api.go#L45) |
| POST | `/api/v1/wafhost/httpauthbase/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHttpAuthBaseApi.ModifyApi](../../api/waf_httpauthbase_api.go#L91) |
| POST | `/api/v1/wafhost/httpauthbase/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHttpAuthBaseApi.GetListApi](../../api/waf_httpauthbase_api.go#L56) |
| GET | `/api/v1/wafhost/httpauthsession/kick` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHttpAuthSessionApi.KickApi](../../api/waf_httpauthsession_api.go#L56) |
| POST | `/api/v1/wafhost/httpauthsession/kickall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHttpAuthSessionApi.KickAllApi](../../api/waf_httpauthsession_api.go#L102) |
| POST | `/api/v1/wafhost/httpauthsession/kickbyuser` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHttpAuthSessionApi.KickByUserApi](../../api/waf_httpauthsession_api.go#L78) |
| POST | `/api/v1/wafhost/httpauthsession/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHttpAuthSessionApi.GetListApi](../../api/waf_httpauthsession_api.go#L28) |
| GET | `/api/v1/wafhost/ip/lookup` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafIPLookupApi.LookupApi](../../api/waf_ip_lookup.go#L26) |
| POST | `/api/v1/wafhost/ipblock/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.AddApi](../../api/waf_block_ip.go#L31) |
| POST | `/api/v1/wafhost/ipblock/batch/del` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.BatchDelBlockIpApi](../../api/waf_block_ip.go#L236) |
| GET | `/api/v1/wafhost/ipblock/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBlockIpApi.DelBlockIpApi](../../api/waf_block_ip.go#L171) |
| POST | `/api/v1/wafhost/ipblock/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.DelAllBlockIpApi](../../api/waf_block_ip.go#L264) |
| GET | `/api/v1/wafhost/ipblock/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBlockIpApi.GetDetailApi](../../api/waf_block_ip.go#L124) |
| POST | `/api/v1/wafhost/ipblock/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.ModifyBlockIpApi](../../api/waf_block_ip.go#L201) |
| POST | `/api/v1/wafhost/ipblock/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.GetListApi](../../api/waf_block_ip.go#L145) |
| GET | `/api/v1/wafhost/ipblock/recommend-layer` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafBlockIpApi.GetRecommendLayerApi](../../api/waf_block_ip.go#L108) |
| POST | `/api/v1/wafhost/ipblock/toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockIpApi.ToggleApi](../../api/waf_block_ip.go#L304) |
| GET | `/api/v1/wafhost/ipfailure/baniplist` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafIPFailureApi.GetBanIpListApi](../../api/waf_ip_failure.go#L115) |
| GET | `/api/v1/wafhost/ipfailure/config` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPFailureApi.GetConfigApi](../../api/waf_ip_failure.go#L32) |
| POST | `/api/v1/wafhost/ipfailure/config` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPFailureApi.SetConfigApi](../../api/waf_ip_failure.go#L57) |
| POST | `/api/v1/wafhost/ipfailure/removebanip` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPFailureApi.RemoveIPFailureBanIPApi](../../api/waf_ip_failure.go#L184) |
| POST | `/api/v1/wafhost/ipgroup/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupApi.AddApi](../../api/waf_ip_group.go#L27) |
| GET | `/api/v1/wafhost/ipgroup/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPGroupApi.DelApi](../../api/waf_ip_group.go#L152) |
| GET | `/api/v1/wafhost/ipgroup/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPGroupApi.GetDetailApi](../../api/waf_ip_group.go#L89) |
| POST | `/api/v1/wafhost/ipgroup/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupApi.ModifyApi](../../api/waf_ip_group.go#L108) |
| POST | `/api/v1/wafhost/ipgroup/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupApi.GetListApi](../../api/waf_ip_group.go#L54) |
| GET | `/api/v1/wafhost/ipgroup/options` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafIPGroupApi.GetOptionsApi](../../api/waf_ip_group.go#L77) |
| GET | `/api/v1/wafhost/ipgroup/refs` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPGroupApi.GetRefsApi](../../api/waf_ip_group.go#L133) |
| POST | `/api/v1/wafhost/ipgroup/validate` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupApi.ValidateApi](../../api/waf_ip_group.go#L178) |
| POST | `/api/v1/wafhost/ipgroupitem/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.AddApi](../../api/waf_ip_group_item.go#L29) |
| POST | `/api/v1/wafhost/ipgroupitem/batch/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.BatchAddApi](../../api/waf_ip_group_item.go#L53) |
| POST | `/api/v1/wafhost/ipgroupitem/batch/del` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.BatchDelApi](../../api/waf_ip_group_item.go#L168) |
| GET | `/api/v1/wafhost/ipgroupitem/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPGroupItemApi.DelApi](../../api/waf_ip_group_item.go#L144) |
| POST | `/api/v1/wafhost/ipgroupitem/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.DelAllApi](../../api/waf_ip_group_item.go#L195) |
| GET | `/api/v1/wafhost/ipgroupitem/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafIPGroupItemApi.GetDetailApi](../../api/waf_ip_group_item.go#L103) |
| POST | `/api/v1/wafhost/ipgroupitem/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.ModifyApi](../../api/waf_ip_group_item.go#L121) |
| POST | `/api/v1/wafhost/ipgroupitem/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafIPGroupItemApi.GetListApi](../../api/waf_ip_group_item.go#L80) |
| POST | `/api/v1/wafhost/ipwhite/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.AddApi](../../api/waf_allow_ip.go#L31) |
| POST | `/api/v1/wafhost/ipwhite/batchdel` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.BatchDelAllowIpApi](../../api/waf_allow_ip.go#L230) |
| GET | `/api/v1/wafhost/ipwhite/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAllowIpApi.DelAllowIpApi](../../api/waf_allow_ip.go#L125) |
| POST | `/api/v1/wafhost/ipwhite/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.DelAllAllowIpApi](../../api/waf_allow_ip.go#L258) |
| GET | `/api/v1/wafhost/ipwhite/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAllowIpApi.GetDetailApi](../../api/waf_allow_ip.go#L78) |
| POST | `/api/v1/wafhost/ipwhite/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.ModifyAllowIpApi](../../api/waf_allow_ip.go#L155) |
| POST | `/api/v1/wafhost/ipwhite/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.GetListApi](../../api/waf_allow_ip.go#L99) |
| POST | `/api/v1/wafhost/ipwhite/toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowIpApi.ToggleApi](../../api/waf_allow_ip.go#L195) |
| POST | `/api/v1/wafhost/ldpurl/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLdpUrlApi.AddApi](../../api/waf_ldp_url.go#L21) |
| POST | `/api/v1/wafhost/ldpurl/batchdel` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLdpUrlApi.BatchDelLdpUrlApi](../../api/waf_ldp_url.go#L125) |
| GET | `/api/v1/wafhost/ldpurl/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafLdpUrlApi.DelLdpUrlApi](../../api/waf_ldp_url.go#L70) |
| POST | `/api/v1/wafhost/ldpurl/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLdpUrlApi.DelAllLdpUrlApi](../../api/waf_ldp_url.go#L153) |
| GET | `/api/v1/wafhost/ldpurl/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafLdpUrlApi.GetDetailApi](../../api/waf_ldp_url.go#L45) |
| POST | `/api/v1/wafhost/ldpurl/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLdpUrlApi.ModifyLdpUrlApi](../../api/waf_ldp_url.go#L90) |
| POST | `/api/v1/wafhost/ldpurl/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLdpUrlApi.GetListApi](../../api/waf_ldp_url.go#L55) |
| POST | `/api/v1/wafhost/loadbalance/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLoadBalanceApi.AddApi](../../api/waf_loadbalance.go#L29) |
| GET | `/api/v1/wafhost/loadbalance/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafLoadBalanceApi.DelLoadBalanceApi](../../api/waf_loadbalance.go#L111) |
| GET | `/api/v1/wafhost/loadbalance/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafLoadBalanceApi.GetDetailApi](../../api/waf_loadbalance.go#L64) |
| POST | `/api/v1/wafhost/loadbalance/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafLoadBalanceApi.ModifyApi](../../api/waf_loadbalance.go#L141) |
| POST | `/api/v1/wafhost/loadbalance/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLoadBalanceApi.GetListApi](../../api/waf_loadbalance.go#L85) |
| GET | `/api/v1/wafhost/onekeymod/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafOneKeyModApi.DelApi](../../api/waf_onekeymod.go#L74) |
| GET | `/api/v1/wafhost/onekeymod/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafOneKeyModApi.GetDetailApi](../../api/waf_onekeymod.go#L27) |
| POST | `/api/v1/wafhost/onekeymod/doModify` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafOneKeyModApi.DoOneKeyModifyApi](../../api/waf_onekeymod.go#L102) |
| POST | `/api/v1/wafhost/onekeymod/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafOneKeyModApi.GetListApi](../../api/waf_onekeymod.go#L48) |
| POST | `/api/v1/wafhost/onekeymod/parseNginx` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafOneKeyModApi.ParseNginxApi](../../api/waf_onekeymod.go#L128) |
| GET | `/api/v1/wafhost/onekeymod/restore` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafOneKeyModApi.RestoreApi](../../api/waf_onekeymod.go#L159) |
| POST | `/api/v1/wafhost/otp/bind` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOtpApi.BindApi](../../api/waf_otp_api.go#L53) |
| GET | `/api/v1/wafhost/otp/init` | 仅登录令牌 | 有 | 通过 × 3 | 自定义/无请求体 | [WafOtpApi.InitOtpApi](../../api/waf_otp_api.go#L17) |
| POST | `/api/v1/wafhost/otp/unbind` | 仅登录令牌 | 有 | 通过 × 3 | JSON | [WafOtpApi.UnBindApi](../../api/waf_otp_api.go#L93) |
| POST | `/api/v1/wafhost/pathrule/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostPathRuleApi.AddApi](../../api/waf_host_path_rule_api.go#L20) |
| GET | `/api/v1/wafhost/pathrule/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafHostPathRuleApi.DelApi](../../api/waf_host_path_rule_api.go#L66) |
| GET | `/api/v1/wafhost/pathrule/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafHostPathRuleApi.GetDetailApi](../../api/waf_host_path_rule_api.go#L39) |
| POST | `/api/v1/wafhost/pathrule/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafHostPathRuleApi.ModifyApi](../../api/waf_host_path_rule_api.go#L89) |
| POST | `/api/v1/wafhost/pathrule/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafHostPathRuleApi.GetListApi](../../api/waf_host_path_rule_api.go#L50) |
| GET | `/api/v1/wafhost/port/overview` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafHostAPi.GetPortOverviewApi](../../api/waf_host.go#L713) |
| POST | `/api/v1/wafhost/privategroup/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafPrivateGroupApi.AddApi](../../api/waf_privategroup_api.go#L26) |
| GET | `/api/v1/wafhost/privategroup/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafPrivateGroupApi.DelApi](../../api/waf_privategroup_api.go#L133) |
| GET | `/api/v1/wafhost/privategroup/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafPrivateGroupApi.GetDetailApi](../../api/waf_privategroup_api.go#L60) |
| POST | `/api/v1/wafhost/privategroup/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafPrivateGroupApi.ModifyApi](../../api/waf_privategroup_api.go#L160) |
| POST | `/api/v1/wafhost/privategroup/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafPrivateGroupApi.GetListApi](../../api/waf_privategroup_api.go#L81) |
| POST | `/api/v1/wafhost/privategroup/listbybelongcloud` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafPrivateGroupApi.GetListByBelongCloudApi](../../api/waf_privategroup_api.go#L107) |
| POST | `/api/v1/wafhost/privateinfo/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafPrivateInfoApi.AddApi](../../api/waf_privateinfo_api.go#L26) |
| GET | `/api/v1/wafhost/privateinfo/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafPrivateInfoApi.DelApi](../../api/waf_privateinfo_api.go#L119) |
| GET | `/api/v1/wafhost/privateinfo/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafPrivateInfoApi.GetDetailApi](../../api/waf_privateinfo_api.go#L60) |
| POST | `/api/v1/wafhost/privateinfo/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafPrivateInfoApi.ModifyApi](../../api/waf_privateinfo_api.go#L147) |
| POST | `/api/v1/wafhost/privateinfo/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafPrivateInfoApi.GetListApi](../../api/waf_privateinfo_api.go#L87) |
| POST | `/api/v1/wafhost/rule/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.AddApi](../../api/waf_rule.go#L76) |
| POST | `/api/v1/wafhost/rule/aigen` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.AiGenRuleApi](../../api/waf_rule.go#L588) |
| GET | `/api/v1/wafhost/rule/aiprompt` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafRuleAPi.RuleAiPromptApi](../../api/waf_rule.go#L584) |
| POST | `/api/v1/wafhost/rule/batchdel` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.BatchDelRuleApi](../../api/waf_rule.go#L316) |
| GET | `/api/v1/wafhost/rule/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafRuleAPi.DelRuleApi](../../api/waf_rule.go#L209) |
| POST | `/api/v1/wafhost/rule/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.DelAllRuleApi](../../api/waf_rule.go#L344) |
| GET | `/api/v1/wafhost/rule/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafRuleAPi.GetDetailApi](../../api/waf_rule.go#L162) |
| POST | `/api/v1/wafhost/rule/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.ModifyRuleApi](../../api/waf_rule.go#L241) |
| POST | `/api/v1/wafhost/rule/format` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.FormatRuleApi](../../api/waf_rule.go#L375) |
| POST | `/api/v1/wafhost/rule/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.GetListApi](../../api/waf_rule.go#L183) |
| GET | `/api/v1/wafhost/rule/rulestatus` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafRuleAPi.ModifyRuleStatusApi](../../api/waf_rule.go#L433) |
| POST | `/api/v1/wafhost/rule/test` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleAPi.TestRuleApi](../../api/waf_rule.go#L454) |
| GET | `/api/v1/wafhost/ruleban/baniplist` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafRuleBanApi.GetBanIpListApi](../../api/waf_rule_ban.go#L75) |
| GET | `/api/v1/wafhost/ruleban/config` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafRuleBanApi.GetConfigApi](../../api/waf_rule_ban.go#L28) |
| POST | `/api/v1/wafhost/ruleban/config` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleBanApi.SetConfigApi](../../api/waf_rule_ban.go#L48) |
| POST | `/api/v1/wafhost/ruleban/removebanip` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafRuleBanApi.RemoveBanIpApi](../../api/waf_rule_ban.go#L92) |
| POST | `/api/v1/wafhost/sensitive/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSensitiveApi.AddApi](../../api/waf_sensitive.go#L29) |
| POST | `/api/v1/wafhost/sensitive/batch/del` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSensitiveApi.BatchDelSensitiveApi](../../api/waf_sensitive.go#L142) |
| GET | `/api/v1/wafhost/sensitive/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSensitiveApi.DelSensitiveApi](../../api/waf_sensitive.go#L122) |
| POST | `/api/v1/wafhost/sensitive/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSensitiveApi.DelAllSensitiveApi](../../api/waf_sensitive.go#L161) |
| GET | `/api/v1/wafhost/sensitive/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSensitiveApi.GetDetailApi](../../api/waf_sensitive.go#L75) |
| POST | `/api/v1/wafhost/sensitive/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSensitiveApi.ModifySensitiveApi](../../api/waf_sensitive.go#L192) |
| POST | `/api/v1/wafhost/sensitive/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSensitiveApi.GetListApi](../../api/waf_sensitive.go#L96) |
| POST | `/api/v1/wafhost/sslexpire/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslExpireApi.AddApi](../../api/waf_sslexpire_api.go#L28) |
| GET | `/api/v1/wafhost/sslexpire/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSslExpireApi.DelApi](../../api/waf_sslexpire_api.go#L113) |
| GET | `/api/v1/wafhost/sslexpire/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafSslExpireApi.GetDetailApi](../../api/waf_sslexpire_api.go#L62) |
| POST | `/api/v1/wafhost/sslexpire/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslExpireApi.ModifyApi](../../api/waf_sslexpire_api.go#L140) |
| POST | `/api/v1/wafhost/sslexpire/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafSslExpireApi.GetListApi](../../api/waf_sslexpire_api.go#L83) |
| GET | `/api/v1/wafhost/sslexpire/nowcheck` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafSslExpireApi.NowCheckExpireApi](../../api/waf_sslexpire_api.go#L164) |
| GET | `/api/v1/wafhost/sslexpire/sync_host` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafSslExpireApi.SyncHostApi](../../api/waf_sslexpire_api.go#L197) |
| POST | `/api/v1/wafhost/sslorder/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslOrderApi.AddApi](../../api/waf_sslorder.go#L66) |
| GET | `/api/v1/wafhost/sslorder/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSslOrderApi.DelApi](../../api/waf_sslorder.go#L173) |
| GET | `/api/v1/wafhost/sslorder/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafSslOrderApi.GetDetailApi](../../api/waf_sslorder.go#L118) |
| GET | `/api/v1/wafhost/sslorder/dns/credentials` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslOrderApi.DNSCredentialStatus](../../api/waf_dns_provider.go#L26) |
| GET | `/api/v1/wafhost/sslorder/dns/providers` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslOrderApi.DNSProviders](../../api/waf_dns_provider.go#L21) |
| POST | `/api/v1/wafhost/sslorder/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslOrderApi.ModifyApi](../../api/waf_sslorder.go#L201) |
| POST | `/api/v1/wafhost/sslorder/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafSslOrderApi.GetListApi](../../api/waf_sslorder.go#L143) |
| GET | `/api/v1/wafhost/sslorder/logs` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafSslOrderApi.LogsApi](../../api/waf_sslorder.go#L20) |
| POST | `/api/v1/wafhost/sslorder/retry` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslOrderApi.RetryApi](../../api/waf_sslorder.go#L39) |
| GET | `/api/v1/wafhost/sslorder/targets` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslOrderApi.CertificateTargets](../../api/waf_dns_provider.go#L12) |
| GET | `/api/v1/wafhost/sslorder/zerossl/account` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafSslOrderApi.ZeroSSLAccountStatus](../../api/waf_ssl_zerossl.go#L9) |
| POST | `/api/v1/wafhost/sslorder/zerossl/account` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafSslOrderApi.SaveZeroSSLAccount](../../api/waf_ssl_zerossl.go#L13) |
| POST | `/api/v1/wafhost/tamperrule/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTamperRuleApi.AddApi](../../api/waf_tamperrule_api.go#L25) |
| POST | `/api/v1/wafhost/tamperrule/addbatch` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTamperRuleApi.AddBatchApi](../../api/waf_tamperrule_api.go#L176) |
| GET | `/api/v1/wafhost/tamperrule/baseline` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafTamperRuleApi.GetBaselineApi](../../api/waf_tamperrule_api.go#L211) |
| GET | `/api/v1/wafhost/tamperrule/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafTamperRuleApi.DelApi](../../api/waf_tamperrule_api.go#L78) |
| POST | `/api/v1/wafhost/tamperrule/delbatch` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTamperRuleApi.DelBatchApi](../../api/waf_tamperrule_api.go#L194) |
| GET | `/api/v1/wafhost/tamperrule/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafTamperRuleApi.GetDetailApi](../../api/waf_tamperrule_api.go#L49) |
| POST | `/api/v1/wafhost/tamperrule/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTamperRuleApi.ModifyApi](../../api/waf_tamperrule_api.go#L102) |
| POST | `/api/v1/wafhost/tamperrule/extract` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafTamperRuleApi.ExtractUrlsApi](../../api/waf_tamperrule_api.go#L160) |
| POST | `/api/v1/wafhost/tamperrule/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafTamperRuleApi.GetListApi](../../api/waf_tamperrule_api.go#L61) |
| GET | `/api/v1/wafhost/tamperrule/relearn` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafTamperRuleApi.RelearnApi](../../api/waf_tamperrule_api.go#L121) |
| POST | `/api/v1/wafhost/tamperrule/relearnbatch` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTamperRuleApi.RelearnBatchApi](../../api/waf_tamperrule_api.go#L143) |
| POST | `/api/v1/wafhost/task/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTaskApi.AddApi](../../api/waf_task_api.go#L30) |
| GET | `/api/v1/wafhost/task/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafTaskApi.DelApi](../../api/waf_task_api.go#L107) |
| GET | `/api/v1/wafhost/task/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafTaskApi.GetDetailApi](../../api/waf_task_api.go#L60) |
| POST | `/api/v1/wafhost/task/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafTaskApi.ModifyApi](../../api/waf_task_api.go#L134) |
| POST | `/api/v1/wafhost/task/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafTaskApi.GetListApi](../../api/waf_task_api.go#L81) |
| GET | `/api/v1/wafhost/task/log/clear` | 登录令牌/API Key | 有 | 通过 × 3 | 自定义/无请求体 | [WafTaskApi.ClearTaskLogApi](../../api/waf_task_api.go#L243) |
| GET | `/api/v1/wafhost/task/log` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafTaskApi.GetTaskLogApi](../../api/waf_task_api.go#L197) |
| GET | `/api/v1/wafhost/task/manual_exec` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafTaskApi.ManualExecuteApi](../../api/waf_task_api.go#L168) |
| POST | `/api/v1/wafhost/urlblock/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.AddApi](../../api/waf_block_url.go#L31) |
| POST | `/api/v1/wafhost/urlblock/batchdel` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.BatchDelBlockUrlApi](../../api/waf_block_url.go#L215) |
| GET | `/api/v1/wafhost/urlblock/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBlockUrlApi.DelBlockUrlApi](../../api/waf_block_url.go#L117) |
| POST | `/api/v1/wafhost/urlblock/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.DelAllBlockUrlApi](../../api/waf_block_url.go#L243) |
| GET | `/api/v1/wafhost/urlblock/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafBlockUrlApi.GetDetailApi](../../api/waf_block_url.go#L70) |
| POST | `/api/v1/wafhost/urlblock/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.ModifyBlockUrlApi](../../api/waf_block_url.go#L147) |
| POST | `/api/v1/wafhost/urlblock/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.GetListApi](../../api/waf_block_url.go#L91) |
| POST | `/api/v1/wafhost/urlblock/toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafBlockUrlApi.ToggleApi](../../api/waf_block_url.go#L180) |
| POST | `/api/v1/wafhost/urlwhite/add` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.AddApi](../../api/waf_allow_url.go#L31) |
| POST | `/api/v1/wafhost/urlwhite/batchdel` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.BatchDelAllowUrlApi](../../api/waf_allow_url.go#L215) |
| GET | `/api/v1/wafhost/urlwhite/del` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAllowUrlApi.DelAllowUrlApi](../../api/waf_allow_url.go#L117) |
| POST | `/api/v1/wafhost/urlwhite/delall` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.DelAllAllowUrlApi](../../api/waf_allow_url.go#L243) |
| GET | `/api/v1/wafhost/urlwhite/detail` | 登录令牌/API Key | 有 | 通过 × 3 | 参数绑定 | [WafAllowUrlApi.GetDetailApi](../../api/waf_allow_url.go#L70) |
| POST | `/api/v1/wafhost/urlwhite/edit` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.ModifyAllowUrlApi](../../api/waf_allow_url.go#L147) |
| POST | `/api/v1/wafhost/urlwhite/list` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.GetListApi](../../api/waf_allow_url.go#L91) |
| POST | `/api/v1/wafhost/urlwhite/toggle` | 登录令牌/API Key | 有 | 通过 × 3 | JSON | [WafAllowUrlApi.ToggleApi](../../api/waf_allow_url.go#L180) |
| GET | `/api/v1/waflog/attack/alliptag` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogAPi.GetAllIpTagApi](../../api/waf_log.go#L250) |
| GET | `/api/v1/waflog/attack/allsharedb` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogAPi.GetAllShareDbApi](../../api/waf_log.go#L139) |
| POST | `/api/v1/waflog/attack/attackiplist` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.GetAttackIPListApi](../../api/waf_log.go#L228) |
| POST | `/api/v1/waflog/attack/deletetagbyname` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.DeleteTagByNameApi](../../api/waf_log.go#L263) |
| GET | `/api/v1/waflog/attack/detail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafLogAPi.GetDetailApi](../../api/waf_log.go#L51) |
| GET | `/api/v1/waflog/attack/download` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogAPi.DownloadApi](../../api/waf_log.go#L130) |
| GET | `/api/v1/waflog/attack/export` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogAPi.ExportDBApi](../../api/waf_log.go#L126) |
| GET | `/api/v1/waflog/attack/httpcopymask` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafLogAPi.GetHttpCopyMaskApi](../../api/waf_log.go#L186) |
| GET | `/api/v1/waflog/attack/iptagdbstatus` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafLogAPi.GetIPTagDBStatusApi](../../api/waf_log.go#L220) |
| POST | `/api/v1/waflog/attack/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.GetListApi](../../api/waf_log.go#L86) |
| POST | `/api/v1/waflog/attack/shard/del` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.DelShardApi](../../api/waf_log.go#L165) |
| POST | `/api/v1/waflog/attack/watchlist/add` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.AddIPWatchlistApi](../../api/waf_ip_watchlist.go#L13) |
| POST | `/api/v1/waflog/attack/watchlist/del` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.DelIPWatchlistApi](../../api/waf_ip_watchlist.go#L27) |
| POST | `/api/v1/waflog/attack/watchlist/list` | 登录令牌/API Key | 无 | 通过 × 3 | JSON | [WafLogAPi.GetIPWatchlistApi](../../api/waf_ip_watchlist.go#L41) |
| POST | `/api/v1/waflog/exports/:id/cancel` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Cancel-fm` |
| POST | `/api/v1/waflog/exports/:id/download-ticket` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.DownloadTicket-fm` |
| GET | `/api/v1/waflog/exports/:id/download` | 短效下载票据 | 无 | 单独验证 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Download-fm` |
| DELETE | `/api/v1/waflog/exports/:id` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Delete-fm` |
| GET | `/api/v1/waflog/exports/:id` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Get-fm` |
| GET | `/api/v1/waflog/exports/capabilities` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Capabilities-fm` |
| GET | `/api/v1/waflog/exports` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.List-fm` |
| POST | `/api/v1/waflog/exports` | 登录令牌/API Key | 无 | 通过 × 3 | 特殊入口 | `HomeWaf/api.WafLogExportAPI.Create-fm` |
| GET | `/api/v1/wafstatqpstrend` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafStatApi.StatQpsTrendApi](../../api/waf_stat.go#L76) |
| GET | `/api/v1/wafstatsitedetail` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafStatApi.StatSiteDetailApi](../../api/waf_stat.go#L177) |
| GET | `/api/v1/wafstatsiteoverview` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafStatApi.StatSiteOverviewApi](../../api/waf_stat.go#L151) |
| GET | `/api/v1/wafstatsumday` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafStatApi.StatHomeSumDayApi](../../api/waf_stat.go#L55) |
| GET | `/api/v1/wafstatsumdayrange` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafStatApi.StatHomeSumDayRangeApi](../../api/waf_stat.go#L97) |
| GET | `/api/v1/wafstatsumdaytopiprange` | 登录令牌/API Key | 无 | 通过 × 3 | 参数绑定 | [WafStatApi.StatHomeSumDayTopIPRangeApi](../../api/waf_stat.go#L124) |
| GET | `/api/v1/ws` | 登录令牌/API Key | 无 | 通过 × 3 | 自定义/无请求体 | [WafWebSocketApi.WebSocketMessageApi](../../api/waf_websocket.go#L24) |
| GET | `/audit-emergency` | 公开 | 无 | 单独验证 | 特殊入口 | `HomeWaf/wafmangeweb.(*WafWebManager).RegisterRoutes.func1` |
| GET | `/debug/pprof/` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
| GET | `/debug/pprof/allocs` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/block` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/cmdline` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
| GET | `/debug/pprof/goroutine` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/heap` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/mutex` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/profile` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
| GET | `/debug/pprof/symbol` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
| POST | `/debug/pprof/symbol` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
| GET | `/debug/pprof/threadcreate` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapH.func1` |
| GET | `/debug/pprof/trace` | 仅登录令牌 | 有 | 通过 × 3 | 特殊入口 | `github.com/gin-gonic/gin.WrapF.func1` |
