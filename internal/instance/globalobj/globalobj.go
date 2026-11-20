// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package globalobj

import (
	"HomeWaf/internal/jobs/waftask"
	"HomeWaf/internal/services/wafappengine"
	"HomeWaf/internal/services/waftunnelengine"
	"HomeWaf/plugin/manager"
)

var (
	/***
	本地对象映射关系
	*/
	GWAF_RUNTIME_OBJ_TUNNEL_ENGINE     *waftunnelengine.WafTunnelEngine //当前tunnel引擎对象
	GWAF_RUNTIME_OBJ_WAF_TaskRegistry  *waftask.TaskRegistry            // 任务执行器
	GWAF_RUNTIME_OBJ_WAF_TaskScheduler *waftask.TaskScheduler           // 任务计划
	GWAF_RUNTIME_OBJ_PLUGIN_MANAGER    *manager.PluginManager           // 插件管理器
	GWAF_RUNTIME_OBJ_APP_ENGINE        *wafappengine.WafAppEngine       // 应用管理引擎
)
