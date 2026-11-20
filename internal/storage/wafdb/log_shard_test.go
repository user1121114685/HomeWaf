// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/enums"
	"HomeWaf/internal/platform/common/uuid"

	sqlite "HomeWaf/internal/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openShardTestDBs(t *testing.T) (core, logDB *gorm.DB) {
	t.Helper()
	core = openTenantTestDB(t, "sqlite")
	if err := core.AutoMigrate(&model.ShareDb{}); err != nil {
		t.Fatalf("建 share_dbs 失败: %v", err)
	}
	logDB = openTenantTestDB(t, "sqlite")
	if err := InitializeLogSchema(logDB); err != nil {
		t.Fatalf("建日志表失败: %v", err)
	}
	return core, logDB
}

func countShard(t *testing.T, core *gorm.DB, name string) int64 {
	t.Helper()
	var n int64
	if err := core.Model(&model.ShareDb{}).Where("file_name = ?", name).Count(&n).Error; err != nil {
		t.Fatalf("统计 %s 失败: %v", name, err)
	}
	return n
}

// 已有归档分片、却没有实时记录时也要补上：只在整表为空时才补的话，这种库会一直缺这一行，
// 归档列表里就没有任何一项被标为 is_current。
func TestEnsureLiveShardRecordWithArchivesOnly(t *testing.T) {
	core, logDB := openShardTestDBs(t)
	for _, name := range []string{"local_log_202606.db", "local_log_202606_02.db"} {
		if err := core.Create(&model.ShareDb{BaseOrm: baseorm.BaseOrm{Id: uuid.GenUUID()}, DbLogicType: "log", FileName: name}).Error; err != nil {
			t.Fatalf("写归档记录失败: %v", err)
		}
	}

	ensureLiveShardRecord(core, logDB, enums.DB_LOG)

	if got := countShard(t, core, enums.DB_LOG); got != 1 {
		t.Fatalf("实时记录条数 = %d，应为 1", got)
	}
}

func TestEnsureLiveShardRecordIdempotent(t *testing.T) {
	core, logDB := openShardTestDBs(t)

	ensureLiveShardRecord(core, logDB, enums.DB_LOG)
	ensureLiveShardRecord(core, logDB, enums.DB_LOG)

	if got := countShard(t, core, enums.DB_LOG); got != 1 {
		t.Fatalf("重复调用后实时记录条数 = %d，应为 1", got)
	}
}

// 补登用的名字必须和归档列表判 is_current 用的是同一个。
func TestLiveLogNameSQLite(t *testing.T) {
	if got := LiveLogName(); got != enums.DB_LOG {
		t.Fatalf("SQLite 下 LiveLogName() = %q，应为 %q", got, enums.DB_LOG)
	}
}

func makeArchive(t *testing.T, name string, accessRows int) string {
	t.Helper()
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("data", name)
	dsn := path
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("建归档文件失败: %v", err)
	}
	if err := InitializeLogSchema(db); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	for i := 0; i < accessRows; i++ {
		row := &model.AccessLog{LogNarrow: model.LogNarrow{ReqUUID: fmt.Sprintf("%s_a%d", name, i),
			TenantId: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, UNIX_ADD_TIME: int64(i + 1)}}
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("写 access_log 失败: %v", err)
		}
	}
	if s, e := db.DB(); e == nil {
		_ = s.Close()
	}
	return path
}

func TestResolveTierTablesReadOnlyArchive(t *testing.T) {
	const name = "local_log_202601.db"
	path := makeArchive(t, name, 3)
	t.Cleanup(func() { closeShardDB(name) })
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tier := ResolveTierTables(name)
	if tier.Err != nil {
		t.Fatalf("打开归档失败: %v", tier.Err)
	}
	if tier.Access != model.AccessLogTableName || tier.Event != model.SecurityEventTableName || tier.Payload != model.EventPayloadTableName || tier.DB.Migrator().HasTable("web_logs") {
		t.Fatalf("unexpected archive schema: %+v", tier)
	}
	var n int64
	if err := tier.DB.Table(tier.Access).Count(&n).Error; err != nil || n != 3 {
		t.Fatalf("应读到 3 行，实际 %d (%v)", n, err)
	}
	if tier.Empty {
		t.Fatal("有数据的归档不该标成空")
	}

	// 只读：连接层拒写，文件大小与修改时间不变
	if err := tier.DB.Exec("DELETE FROM access_log").Error; err == nil {
		t.Fatal("归档连接不该能写")
	}
	closeShardDB(name)
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("归档文件被改动：%d/%v → %d/%v", before.Size(), before.ModTime(), after.Size(), after.ModTime())
	}
	if _, err := os.Stat(path + "-wal"); err == nil {
		if fi, _ := os.Stat(path + "-wal"); fi.Size() > 0 {
			t.Fatal("只读打开不该留下有内容的 WAL")
		}
	}
}

func TestResolveTierTablesEmptyArchive(t *testing.T) {
	const name = "local_log_202601_03.db"
	makeArchive(t, name, 0)
	t.Cleanup(func() { closeShardDB(name) })

	tier := ResolveTierTables(name)
	if tier.Err != nil || !tier.Empty {
		t.Fatalf("空归档应可打开且标为空，实际 %+v", tier)
	}
}

// 文件不在时报缺失，且不能在原位置建出一个空库，也不能回落实时库
func TestResolveTierTablesMissingArchive(t *testing.T) {
	const name = "local_log_202601_04.db"
	tier := ResolveTierTables(name)
	if !errors.Is(tier.Err, ErrShardMissing) {
		t.Fatalf("应报 ErrShardMissing，实际 %v", tier.Err)
	}
	if tier.DB != nil {
		t.Fatal("缺失的分片不该返回任何连接")
	}
	if _, err := os.Stat(filepath.Join("data", name)); !os.IsNotExist(err) {
		t.Fatal("打开缺失分片时在原位置建出了文件")
	}
	if !ShardFileMissing(name) {
		t.Fatal("ShardFileMissing 应为 true")
	}
}

func TestResolveTierTablesRejectsBadNames(t *testing.T) {
	for _, name := range []string{"../local.db", "local_log_1.db/../../x.db", "local.db", "local_log_abc.db", "local_log_20260101000001.db", "local_log_202613.db", "local_log_202601_01.db"} {
		if tier := ResolveTierTables(name); tier.Err == nil {
			t.Fatalf("%q 不该被当成归档打开", name)
		}
	}
}

// 淘汰出缓存的连接在宽限期内仍可用；显式关闭时连同宽限期里的一起关掉
func TestWorkerCloseReleasesActiveAndRetiredPools(t *testing.T) {
	oldCore, oldLog, oldStats := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB = oldCore, oldLog, oldStats
	})
	core := openTenantTestDB(t, "sqlite")
	live := openTenantTestDB(t, "sqlite")
	stats := openTenantTestDB(t, "sqlite")
	active := openTenantTestDB(t, "sqlite")
	retired := openTenantTestDB(t, "sqlite")
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB = core, live, stats

	// Use an isolated cache for a worker that has reopened a retired shard.
	shardMu.Lock()
	oldActive, oldUse, oldRetired := shardDBs, shardLastUse, shardRetired
	shardDBs = map[string]*gorm.DB{}
	shardLastUse = map[string]time.Time{}
	shardRetired = map[string][]*gorm.DB{}
	shardMu.Unlock()
	t.Cleanup(func() {
		shardMu.Lock()
		shardDBs, shardLastUse, shardRetired = oldActive, oldUse, oldRetired
		shardMu.Unlock()
	})
	const name = "worker-close.db"
	putShardDB(name, retired)
	shardMu.Lock()
	retireLocked(name)
	shardMu.Unlock()
	putShardDB(name, active)
	Close()
	if cached := getShardDB(name); cached != nil {
		t.Fatal("closed shard is still available to readers")
	}
	for label, db := range map[string]*gorm.DB{"core": core, "log": live, "stats": stats, "active": active, "retired": retired} {
		pool, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		if pool.Ping() == nil {
			t.Errorf("%s pool remained open after shutdown", label)
		}
	}
}

func TestShardRetireKeepsConnectionUsable(t *testing.T) {
	names := make([]string, 0, shardCacheMax+1)
	for i := 0; i <= shardCacheMax; i++ {
		name := fmt.Sprintf("local_log_202602_%02d.db", i+2)
		makeArchive(t, name, 1)
		names = append(names, name)
	}
	t.Cleanup(func() {
		for _, n := range names {
			closeShardDB(n)
		}
	})
	first := ResolveTierTables(names[0])
	if first.Err != nil {
		t.Fatal(first.Err)
	}
	for _, n := range names[1:] {
		if tier := ResolveTierTables(n); tier.Err != nil {
			t.Fatal(tier.Err)
		}
	}
	if getShardDB(names[0]) != nil {
		t.Fatal("超过上限后最早的连接应被移出缓存")
	}
	var n int64
	if err := first.DB.Table(model.AccessLogTableName).Count(&n).Error; err != nil {
		t.Fatalf("宽限期内被淘汰的连接应仍可用: %v", err)
	}
	closeShardDB(names[0])
	if err := first.DB.Table(model.AccessLogTableName).Count(&n).Error; err == nil {
		t.Fatal("显式关闭后宽限期里的连接也应已关")
	}
}

func TestShardHasIndex(t *testing.T) {
	const name = "local_log_202601_05.db"
	makeArchive(t, name, 1)
	t.Cleanup(func() { closeShardDB(name) })
	tier := ResolveTierTables(name)
	if tier.Err != nil {
		t.Fatal(tier.Err)
	}
	if ShardHasIndex(tier.DB, name, model.AccessLogTableName, "idx_no_such_index") {
		t.Fatal("不存在的索引应返回 false")
	}
	if !ShardHasIndex(tier.DB, name, model.AccessLogTableName, "idx_al_time") {
		t.Fatal("结构体上声明的索引应存在")
	}
}
