package wafdb

import (
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestFreshSchemaAndRestartPreserveUserSettings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "fresh.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	initialize := func() {
		t.Helper()
		if err := InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
		if err := InitializeTasks(db); err != nil {
			t.Fatal(err)
		}
	}
	initialize()
	if db.Migrator().HasTable("migrations") {
		t.Fatal("fresh installation replayed historical migrations")
	}
	for _, table := range []string{"hosts", "anti_cc_rules", "access_account", "dashboard_share", "log_export_task"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("missing current table %s", table)
		}
	}
	var task model.Task
	if err := db.First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&task).Update("task_value", 123).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.DataRetentionPolicy{}).Where("table_name = ?", "stats_ip_days").Updates(map[string]any{"retain_days": 7, "clean_enabled": 1}).Error; err != nil {
		t.Fatal(err)
	}
	initialize()
	var count int64
	if err := db.Model(&model.Task{}).Count(&count).Error; err != nil || count != 29 {
		t.Fatalf("default tasks duplicated or missing: %d, %v", count, err)
	}
	if err := db.First(&task, "id = ?", task.Id).Error; err != nil || task.TaskValue != 123 {
		t.Fatalf("restart overwrote task settings: %+v, %v", task, err)
	}
	var policy model.DataRetentionPolicy
	if err := db.Where("table_name = ?", "stats_ip_days").First(&policy).Error; err != nil || policy.RetainDays != 7 || policy.CleanEnabled != 1 {
		t.Fatalf("restart overwrote retention policy: %+v, %v", policy, err)
	}
}
