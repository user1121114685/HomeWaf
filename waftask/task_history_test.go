// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waftask

import (
	"HomeWaf/global"
	"HomeWaf/wafdb"
	"HomeWaf/wafdb/dialect"
	"gorm.io/gorm"
	"testing"
)

func TestTaskDeleteHistoryInfo(t *testing.T) {
	// Never open ../data: it may contain a developer's live or legacy database.
	currentPath := t.TempDir()
	t.Chdir(currentPath)
	t.Setenv("SamWafIDE", "1")
	previousDialect := func() (d dialect.DBDialect) {
		defer func() { _ = recover() }()
		return dialect.Get()
	}()
	dialect.Register(&dialect.SQLiteDialect{})
	oldCore, oldLog, oldStats := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB = nil, nil, nil
	t.Cleanup(func() {
		for _, db := range []*gorm.DB{global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB} {
			if db != nil {
				if raw, err := db.DB(); err == nil {
					_ = raw.Close()
				}
			}
		}
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB = oldCore, oldLog, oldStats
		dialect.Register(previousDialect)
	})
	for _, init := range []func(string) (bool, error){wafdb.InitCoreDb, wafdb.InitLogDb, wafdb.InitStatsDb} {
		if _, err := init(currentPath); err != nil {
			t.Fatal(err)
		}
	}
	TaskDeleteHistoryInfo()
	for _, db := range []*gorm.DB{global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GWAF_LOCAL_STATS_DB} {
		var integrity string
		if err := db.Raw("PRAGMA integrity_check").Scan(&integrity).Error; err != nil || integrity != "ok" {
			t.Fatalf("cleanup damaged a database: %q, %v", integrity, err)
		}
	}
}
