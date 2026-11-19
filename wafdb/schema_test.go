package wafdb

import (
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStorageFormatRejectsUnmarkedDatabases(t *testing.T) {
	for name, init := range map[string]func(*gorm.DB) error{"core": InitializeCoreSchema, "log": InitializeLogSchema, "stats": InitializeStatsSchema} {
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "existing.db")))
			if err != nil {
				t.Fatal(err)
			}
			conn, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := db.Exec("CREATE TABLE operator_data (value INTEGER NOT NULL)").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("INSERT INTO operator_data VALUES (17)").Error; err != nil {
				t.Fatal(err)
			}
			if err := init(db); err == nil {
				t.Fatal("accepted an existing database")
			}
			tables, err := db.Migrator().GetTables()
			if err != nil || len(tables) != 1 || tables[0] != "operator_data" {
				t.Fatalf("schema changed: %v %v", tables, err)
			}
			var value int
			if err := db.Raw("SELECT value FROM operator_data").Scan(&value).Error; err != nil || value != 17 {
				t.Fatal("existing data changed", err)
			}
		})
	}
}

func TestStorageFormatRejectsInvalidMarkers(t *testing.T) {
	for name, ddl := range map[string]string{
		"missing version":     "CREATE TABLE " + storageFormatTable + " (format INTEGER NOT NULL)",
		"unsupported version": "CREATE TABLE " + storageFormatTable + " (version INTEGER PRIMARY KEY); INSERT INTO " + storageFormatTable + " (version) VALUES (2)",
		"duplicate versions":  "CREATE TABLE " + storageFormatTable + " (version INTEGER NOT NULL); INSERT INTO " + storageFormatTable + " (version) VALUES (3), (3)",
	} {
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "invalid.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			conn, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := db.Exec(ddl).Error; err != nil {
				t.Fatal(err)
			}
			if err := ensureStorageFormat(db); err == nil {
				t.Fatal("accepted an invalid format marker")
			}
		})
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
	if !db.Migrator().HasTable(&model.PortConfig{}) {
		t.Fatal("missing native port configuration")
	}
	if db.Migrator().HasTable("migrations") {
		t.Fatal("fresh installation replayed historical migrations")
	}
	for _, table := range []string{"dashboard_share", "log_export_task"} {
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
	if err := db.Model(&model.Task{}).Count(&count).Error; err != nil || count != 25 {
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
