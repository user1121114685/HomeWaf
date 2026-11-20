// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waftask

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
)

// TaskLogQpsClean  清空LOG QPS
func TaskLogQpsClean() {
	innerLogName := "TaskLogQpsClean"
	if utils.CheckDebugEnvInfo() {
		zlog.Debug(innerLogName, "准备进行TaskLogQpsClean")
	}

	// 更新实时QPS计算 (基于差分计算)
	global.UpdateRealtimeQPS()
}
