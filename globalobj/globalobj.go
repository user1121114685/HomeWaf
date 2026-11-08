// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package globalobj

import (
	"HomeWaf/plugin/manager"
	"HomeWaf/wafappengine"
	"HomeWaf/wafenginecore"
	"HomeWaf/waftask"
	"HomeWaf/waftunnelengine"
)

var (
	/***
	本地对象映射关系
	*/
	GWAF_RUNTIME_OBJ_WAF_ENGINE        *wafenginecore.WafEngine         //当前引擎对象
	GWAF_RUNTIME_OBJ_TUNNEL_ENGINE     *waftunnelengine.WafTunnelEngine //当前tunnel引擎对象
	GWAF_RUNTIME_OBJ_WAF_TaskRegistry  *waftask.TaskRegistry            // 任务执行器
	GWAF_RUNTIME_OBJ_WAF_TaskScheduler *waftask.TaskScheduler           // 任务计划
	GWAF_RUNTIME_OBJ_PLUGIN_MANAGER    *manager.PluginManager           // 插件管理器
	GWAF_RUNTIME_OBJ_APP_ENGINE        *wafappengine.WafAppEngine       // 应用管理引擎
)
