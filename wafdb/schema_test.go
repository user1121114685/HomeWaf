package wafdb

import (
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSSLOrderAdditiveSchemaPreservesPendingDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy-orders.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.Exec("CREATE TABLE ssl_orders (id text PRIMARY KEY, host_code text, apply_status text, tenant_id text, user_code text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_orders (id, host_code, apply_status) VALUES ('a','nas','submitted'), ('b','nas','submitted')").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("ssl_orders").Where("apply_status = ?", "submitted").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("migration changed pending history: %d %v", count, err)
	}
	for _, field := range []string{"RequestID", "ActiveKey", "SubmissionHash", "RenewFrom", "Progress"} {
		if !db.Migrator().HasColumn(&model.SslOrder{}, field) {
			t.Errorf("missing %s", field)
		}
	}
	for _, index := range []string{"uni_ssl_order_request", "uni_ssl_order_active"} {
		if !db.Migrator().HasIndex(&model.SslOrder{}, index) {
			t.Errorf("missing %s", index)
		}
	}
}

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
