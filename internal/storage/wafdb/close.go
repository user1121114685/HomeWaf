// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafdb

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/platform/common/zlog"

	"gorm.io/gorm"
)

// Close releases the worker's database pools after traffic and tasks have drained.
func Close() {
	closeConnection("core", global.GWAF_LOCAL_DB)
	closeConnection("log", global.GWAF_LOCAL_LOG_DB)
	closeConnection("stats", global.GWAF_LOCAL_STATS_DB)
	closeAllShardDBs()
}

func closeConnection(name string, db *gorm.DB) {
	if db == nil {
		return
	}
	pool, err := db.DB()
	if err == nil {
		err = pool.Close()
	}
	if err != nil {
		zlog.Error("关闭数据库连接失败", "database", name, "error", err.Error())
	}
}
