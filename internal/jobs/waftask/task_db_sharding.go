// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waftask

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/services/waf_service"
	"HomeWaf/internal/storage/wafdb"
	"HomeWaf/internal/storage/wafdb/dialect"
	"HomeWaf/internal/storage/wafdb/partition"
	"context"
	"fmt"
	"os"
	"time"
)

// livePeriodKey 返回实时库里最早一条日志所属的周期键。
//
// 「实时库现在装的是哪个周期」这件事不另存状态，直接问数据本身：
// 最早一条落在上一个周期，就说明周期边界已经过了、该切了。这样重启、停机几天、
// 手工删表都不会让状态和事实脱节（存一个全局变量反而要处理这些不一致）。
// 表不存在或没有数据时返回 false —— 没数据就没什么可切的。
func livePeriodKey() (string, bool) {
	db := global.GWAF_LOCAL_LOG_DB
	if db == nil {
		return "", false
	}
	var oldest *int64
	if err := db.Model(&model.AccessLog{}).Select("MIN(unix_add_time)").Scan(&oldest).Error; err != nil {
		zlog.Debug("TaskDBSharding", "取最早日志时间失败", err.Error())
		return "", false
	}
	if oldest == nil || *oldest <= 0 {
		return "", false
	}
	// UNIX_ADD_TIME 是毫秒
	return partition.KeyOf(time.UnixMilli(*oldest)), true
}

// nextPeriodSeq 返回某个周期下一个可用的序号：正常是 1（一个周期一个分区），
// 已经有同周期分区时才往上加——那是「同周期内体积超限被迫再切」的异常兜底。
func nextPeriodSeq(periodKey string) int {
	if global.GWAF_LOCAL_DB == nil {
		return 1
	}
	var cnt int64
	if err := global.GWAF_LOCAL_DB.Model(&model.ShareDb{}).
		Where("db_logic_type = ? and period_key = ?", "log", periodKey).Count(&cnt).Error; err != nil {
		zlog.Debug("TaskDBSharding", "统计同周期分区数失败", err.Error())
		return 1
	}
	return int(cnt) + 1
}

// 检测库是否切换。
//
// 切分口径（M3 E2）：**按时间周期切**，一个月一个分区。
// 行数与体积超限降级为异常兜底——同一个周期内真被写爆了才提前切，分区名带序号
// （access_log_202609_02）。这样「一段时间落在哪些分区上」是算出来的而不是猜的，
// 过期也变成丢整个分区（见 CleanExpiredArchiveShard）。
func TaskShareDbInfo() {
	innerLogName := "TaskDBSharding"
	zlog.Debug(innerLogName, "检测是否需要进行分库")

	if global.GDATA_CURRENT_CHANGE {
		//如果正在切换库 跳过
		zlog.Debug(innerLogName, "切库状态")
		return
	}

	if global.GWAF_LOCAL_DB == nil || global.GWAF_LOCAL_LOG_DB == nil {
		zlog.Debug(innerLogName, "数据库没有初始化完成呢")
		return
	}

	// 周期边界优先：实时库里最早一条日志落在上一个周期，就把这一段整体切成那个周期的分区。
	// 放在体积判断之前——常态就该按周期切，体积只是兜底。
	curKey := partition.KeyOf(time.Now())
	if oldKey, ok := livePeriodKey(); ok && oldKey != curKey {
		doLogShardCut(innerLogName, fmt.Sprintf("跨周期切分（%s → %s）", oldKey, curKey), oldKey)
		return
	}

	var total int64
	if err := global.GWAF_LOCAL_LOG_DB.Model(&model.AccessLog{}).Count(&total).Error; err != nil {
		zlog.Error(innerLogName, "统计访问日志失败", err)
		return
	}

	//获取当前数据库文件大小
	currentDir := utils.GetCurrentDir()
	oldDBFilename := "local_log.db"
	dbFilePath := currentDir + "/data/" + oldDBFilename

	needSharding := false
	var shardingReason string

	// 检查记录数量是否超过限制
	if total > global.CurrentSettings().GDATA_SHARE_DB_SIZE {
		needSharding = true
		shardingReason = fmt.Sprintf("记录数量(%d)超过限制(%d)", total, global.CurrentSettings().GDATA_SHARE_DB_SIZE)
	}

	// 检查大小是否超过限制：SQLite 用 .db 文件大小，MySQL/PG 用 access_log 表(数据+索引)大小
	if dialect.Get().IsFileBased() {
		fileInfo, err := os.Stat(dbFilePath)
		if err == nil {
			totalBytes := fileInfo.Size()
			// 把 -wal 大小一并计入：WAL 模式下数据可能暂存在 wal 文件，主库文件显小会导致漏判
			if walInfo, werr := os.Stat(dbFilePath + "-wal"); werr == nil {
				totalBytes += walInfo.Size()
			}
			fileSizeMB := totalBytes / (1024 * 1024) // 转换为MB
			if fileSizeMB > global.CurrentSettings().GDATA_SHARE_DB_FILE_SIZE {
				needSharding = true
				shardingReason = fmt.Sprintf("文件大小(%dMB,含WAL)超过限制(%dMB)", fileSizeMB, global.CurrentSettings().GDATA_SHARE_DB_FILE_SIZE)
			}
		} else {
			zlog.Error(innerLogName, "获取数据库文件大小失败:", err)
		}
	} else {
		tableSizeMB, err := dialect.Get().TableSizeMB(global.GWAF_LOCAL_LOG_DB, model.AccessLogTableName)
		if err == nil {
			if tableSizeMB > global.CurrentSettings().GDATA_SHARE_DB_FILE_SIZE {
				needSharding = true
				shardingReason = fmt.Sprintf("表大小(%dMB)超过限制(%dMB)", tableSizeMB, global.CurrentSettings().GDATA_SHARE_DB_FILE_SIZE)
			}
		} else {
			zlog.Error(innerLogName, "获取"+model.AccessLogTableName+"表大小失败:", err)
		}
	}

	if needSharding {
		// 同周期内被写爆了：仍然按当前周期命名，序号从 02 起，读侧与过期判断照样认得出周期
		doLogShardCut(innerLogName, shardingReason+"（同周期内兜底切分）", curKey)
	}
}

// doLogShardCut rotates the current tiers into a monthly archive.
func doLogShardCut(innerLogName, reason, periodKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	release, err := wafdb.AcquireLogWrite(ctx)
	if err != nil {
		zlog.Debug(innerLogName, "日志来源繁忙，本轮延后切分")
		return
	}
	defer release()
	global.GDATA_CURRENT_CHANGE = true
	defer func() { global.GDATA_CURRENT_CHANGE = false }()
	zlog.Info(innerLogName, "开始分库，原因:", reason)

	var total int64
	if err := global.GWAF_LOCAL_LOG_DB.Model(&model.AccessLog{}).Count(&total).Error; err != nil {
		zlog.Error(innerLogName, "统计待归档访问日志失败", err)
		return
	}
	seq := nextPeriodSeq(periodKey)
	archiveName := partition.FileNameSeq(periodKey, seq)
	if !dialect.Get().IsFileBased() {
		archiveName = partition.TableNameSeq(model.AccessLogTableName, periodKey, seq)
	}

	// 起止时间用**这批数据自己的**最早/最晚时间，而不是「上一个分片的结束时间 → 现在」。
	// 过期回收按 EndTime 判（见 CleanExpiredArchiveShard），拿真实边界才不会把还在保留期内的
	// 数据算成过期；停机几天再启动、或同周期内兜底切分时，这个差别很要紧。
	startTime := customtype.JsonTime(time.Now())
	endTime := customtype.JsonTime(time.Now())
	if lo, hi, ok := cutDataRange(model.AccessLogTableName); ok {
		startTime, endTime = customtype.JsonTime(lo), customtype.JsonTime(hi)
	} else {
		var lastedDb model.ShareDb
		if err := global.GWAF_LOCAL_DB.Limit(1).Order("create_time desc").Find(&lastedDb).Error; err == nil {
			startTime = lastedDb.EndTime
		}
	}
	sharDbBean := model.ShareDb{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		DbLogicType: "log",
		StartTime:   startTime,
		EndTime:     endTime,
		FileName:    archiveName,
		Cnt:         total,
		PeriodKey:   periodKey,
	}

	zlog.Info(innerLogName, "正在切库中...")
	if dialect.Get().IsFileBased() {
		// SQLite：先 checkpoint 再关闭、重命名文件和创建实时库
		currentDir := utils.GetCurrentDir()
		oldPath := currentDir + "/data/local_log.db"
		newPath := currentDir + "/data/" + archiveName

		if _, err := os.Lstat(newPath); !os.IsNotExist(err) {
			zlog.Error(innerLogName, "归档目标已存在或无法访问", "path", newPath, "error", err)
			return
		}
		// Drain all committed pages before renaming the database. The last close
		// removes SQLite's WAL and SHM, which must never be reused by the new live DB.
		var checkpoint struct{ Busy, Log, Checkpointed int }
		if err := global.GWAF_LOCAL_LOG_DB.Raw("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&checkpoint).Error; err != nil || checkpoint.Busy != 0 {
			zlog.Warn(innerLogName, "日志库正在使用，推迟分片", "error", err)
			return
		}
		sqlDB, err := global.GWAF_LOCAL_LOG_DB.DB()
		if err != nil {
			zlog.Error(innerLogName, "无法取得日志库连接", err)
			return
		}
		if err := sqlDB.Close(); err != nil {
			zlog.Error(innerLogName, "关闭日志库失败", err)
			return
		}
		global.GWAF_LOCAL_LOG_DB = nil
		if err := os.Rename(oldPath, newPath); err != nil {
			zlog.Error(innerLogName, "重命名日志库失败", err)
			wafdb.InitLogDb("")
			return
		}
		wafdb.InitLogDb("")
	} else {
		suffix := wafdb.ArchiveTableSuffix(archiveName)
		if err := dialect.Get().RotateLogTables(global.GWAF_LOCAL_LOG_DB, waf_service.ShardTierBases, suffix); err != nil {
			zlog.Error(innerLogName, "日志分表失败", "error", err)
			return
		}
		zlog.Info(innerLogName, "分表完成，归档表:", archiveName)
	}
	if err := global.GWAF_LOCAL_DB.Create(&sharDbBean).Error; err != nil {
		zlog.Error(innerLogName, "登记日志分区失败", "name", archiveName, "error", err)
	}
	// 分区构成变了，归档计数缓存跟着作废
	waf_service.InvalidateShardCounts()
	zlog.Info(innerLogName, "切库完成...")
}

// cutDataRange 返回待切表里数据的真实时间边界 [最早, 最晚]。
// 表空或查不到返回 false，调用方回落到旧口径。
func cutDataRange(table string) (time.Time, time.Time, bool) {
	db := global.GWAF_LOCAL_LOG_DB
	if db == nil || !dialect.Get().TableExists(db, table) {
		return time.Time{}, time.Time{}, false
	}
	var row struct {
		Lo *int64
		Hi *int64
	}
	if err := db.Table(table).Select("MIN(unix_add_time) as lo, MAX(unix_add_time) as hi").Scan(&row).Error; err != nil {
		zlog.Debug("TaskDBSharding", "取数据时间边界失败", err.Error())
		return time.Time{}, time.Time{}, false
	}
	if row.Lo == nil || row.Hi == nil || *row.Lo <= 0 || *row.Hi <= 0 {
		return time.Time{}, time.Time{}, false
	}
	// UNIX_ADD_TIME 是毫秒
	return time.UnixMilli(*row.Lo), time.UnixMilli(*row.Hi), true
}
