// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafdb

import (
	"HomeWaf/common/uuid"
	"HomeWaf/common/zlog"
	"HomeWaf/customtype"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/utils"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafdb/partition"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// LiveLogName returns the live log identifier for the selected database driver.
func LiveLogName() string {
	if dialect.Get().IsFileBased() {
		return enums.DB_LOG // "local_log.db"
	}
	return model.AccessLogTableName
}

// ensureLiveShardRecord 确保 share_dbs 里有一条实时分片记录（file_name = liveName），没有才补。
// 按这一行在不在判断，不看表里总数：已有历史分片时总数不为 0，实时记录缺了也永远补不回来。
func ensureLiveShardRecord(coreDB, logDB *gorm.DB, liveName string) {
	var liveCount int64
	if err := coreDB.Model(&model.ShareDb{}).Where("file_name = ?", liveName).Count(&liveCount).Error; err != nil {
		zlog.Error("查询实时分片记录失败", "file_name", liveName, "error", err.Error())
		return
	}
	if liveCount > 0 {
		return
	}
	var logTotal int64
	logDB.Model(&model.AccessLog{}).Count(&logTotal)
	now := customtype.JsonTime(time.Now())
	coreDB.Create(&model.ShareDb{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: now,
			UPDATE_TIME: now,
		},
		DbLogicType: "log",
		StartTime:   now,
		EndTime:     now,
		FileName:    liveName,
		Cnt:         logTotal,
	})
}

// ErrShardMissing 归档分区的存储已不在：SQLite 文件不存在，或 MySQL/PG 下该分区的表一张都没有。
var ErrShardMissing = errors.New("归档分区的存储已不存在")

// ArchiveTableSuffix returns the suffix of a current SQL archive identifier.
func ArchiveTableSuffix(name string) string {
	suffix, ok := strings.CutPrefix(name, model.AccessLogTableName+"_")
	if !ok || dialect.ValidatePartitionPair(model.AccessLogTableName, name) != nil {
		return ""
	}
	if _, ok := partition.SuffixKey(suffix); !ok {
		return ""
	}
	return suffix
}

func isLiveIdent(name string) bool {
	return name == "" || name == LiveLogName()
}

func IsLogArchive(name string) bool {
	if dialect.Get().IsFileBased() {
		if !strings.HasPrefix(name, "local_log_") || !strings.HasSuffix(name, ".db") {
			return false
		}
		_, ok := partition.KeyFromName(name)
		return ok
	}
	return ArchiveTableSuffix(name) != ""
}

// ShardFilePath SQLite 归档文件的完整路径；名字不是合法的归档文件名时返回错误。
func ShardFilePath(name string) (string, error) {
	if !dialect.Get().IsFileBased() || !IsLogArchive(name) {
		return "", fmt.Errorf("不是合法的归档文件名: %q", name)
	}
	return utils.GetCurrentDir() + "/data/" + name, nil
}

// ShardFileMissing SQLite 归档文件是否已不在（只看文件，不打开）。名字不合法也按缺失算。
func ShardFileMissing(name string) bool {
	path, err := ShardFilePath(name)
	if err != nil {
		return true
	}
	_, err = os.Stat(path)
	return os.IsNotExist(err)
}

// TierTables 一个分片上各层实际要读的表名；空字符串 = 这个分片没有那一层。
type TierTables struct {
	DB      *gorm.DB
	Access  string // access_log / access_log_<ts>
	Event   string // security_event / security_event_<ts>
	Payload string // event_payload / event_payload_<ts>
	Empty   bool   // 归档里一行数据都没有
	Err     error  // 分片打不开；errors.Is(Err, ErrShardMissing) 表示存储已不在
}

// shardTierNames 选层结果，按分片缓存。归档只读，内容不再变；
// 分区被删或按层过期时由 InvalidateShardTierCache / forgetShardTiers 作废。
type shardTierNames struct {
	Access, Event, Payload string
	Empty                  bool
}

var shardTierCache sync.Map

func forgetShardTiers(name string) { shardTierCache.Delete(name) }

// InvalidateShardTierCache 分区构成变化后清空选层与索引缓存。
func InvalidateShardTierCache() {
	shardTierCache.Range(func(k, _ any) bool { shardTierCache.Delete(k); return true })
	shardIndexCache.Range(func(k, _ any) bool { shardIndexCache.Delete(k); return true })
}

// ResolveTierTables returns the existing tiers, including archives whose access rows have expired.
func ResolveTierTables(name string) TierTables {
	db := global.GWAF_LOCAL_LOG_DB
	if isLiveIdent(name) {
		if db == nil {
			return TierTables{Err: errors.New("日志库未就绪")}
		}
		return TierTables{DB: db, Access: model.AccessLogTableName, Event: model.SecurityEventTableName, Payload: model.EventPayloadTableName}
	}

	var suffix string
	if dialect.Get().IsFileBased() {
		if err := InitManaulLogDb("", name); err != nil {
			return TierTables{Err: err}
		}
		db = getShardDB(name)
	} else {
		key := ArchiveTableSuffix(name)
		if key == "" {
			return TierTables{Err: fmt.Errorf("认不出的归档分区标识: %q", name)}
		}
		suffix = "_" + key
	}
	if db == nil {
		return TierTables{Err: fmt.Errorf("日志分区 %s 连接不可用", name)}
	}

	var tiers shardTierNames
	if v, ok := shardTierCache.Load(name); ok {
		tiers = v.(shardTierNames)
	} else {
		tables, err := dialect.Get().ListTables(db)
		if err != nil {
			return TierTables{Err: err}
		}
		for _, table := range tables {
			switch table {
			case model.AccessLogTableName + suffix:
				tiers.Access = table
			case model.SecurityEventTableName + suffix:
				tiers.Event = table
			case model.EventPayloadTableName + suffix:
				tiers.Payload = table
			}
		}
		if tiers.Access == "" && tiers.Event == "" && tiers.Payload == "" {
			return TierTables{Err: fmt.Errorf("%w: %s", ErrShardMissing, name)}
		}
		// Empty registered archives are reported separately from missing storage.
		tiers.Empty = true
		for _, table := range []string{tiers.Access, tiers.Event, tiers.Payload} {
			if table == "" {
				continue
			}
			var one []int
			result := db.Table(table).Select("1").Limit(1).Find(&one)
			if result.Error != nil {
				return TierTables{Err: result.Error}
			}
			if len(one) > 0 {
				tiers.Empty = false
				break
			}
		}
		shardTierCache.Store(name, tiers)
	}
	return TierTables{DB: db, Access: tiers.Access, Event: tiers.Event, Payload: tiers.Payload, Empty: tiers.Empty}
}

// shardIndexCache 分片上某个索引在不在，按 分片|表|索引 缓存
var shardIndexCache sync.Map

// ShardHasIndex 分片上有没有这个索引。强制索引前必须先问：历史文件不再跑迁移，
// 更早切出来的分片可能没有后来才加的索引，强制一个不存在的索引整条 SQL 会报错。
func ShardHasIndex(db *gorm.DB, shard, table, index string) bool {
	if db == nil {
		return false
	}
	if isLiveIdent(shard) {
		shard = "live"
	}
	key := shard + "|" + table + "|" + index
	if v, ok := shardIndexCache.Load(key); ok {
		return v.(bool)
	}
	has := db.Migrator().HasIndex(table, index)
	shardIndexCache.Store(key, has)
	return has
}
