package wafdb

import (
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestV201SchemaMigrationRehearsal(t *testing.T) {
	source := os.Getenv("HOMEWAF_V201_DB")
	if source == "" {
		t.Skip("run scripts/verify-pipeline-migration.ps1 to generate the real v2.0.1 fixture")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	original := sha256.Sum256(data)
	dest := filepath.Join(t.TempDir(), "upgrade.db")
	if err = os.WriteFile(dest, data, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(dest)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var before model.Hosts
	var allowBefore model.IPAllowList
	if err = db.Where("id = ?", "rehearsal-host").First(&before).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Where("id = ?", "rehearsal-allow").First(&allowBefore).Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("port_config") || db.Migrator().HasColumn(&model.HttpAuthBase{}, "PasswordHashed") {
		t.Fatal("source is not the pre-pipeline schema")
	}
	for i := 0; i < 2; i++ {
		if err = InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var after model.Hosts
	var allowAfter model.IPAllowList
	if err = db.Where("id = ?", "rehearsal-host").First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Where("id = ?", "rehearsal-allow").First(&allowAfter).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(allowBefore, allowAfter) {
		t.Fatal("migration changed legacy host or IP policy")
	}
	for _, table := range []string{"port_config", "config_template", "gate_user", "gate_session", "gate_runtime", "pipeline_state", "pipeline_daily_metrics", "pipeline_port_migration"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("missing new table %s", table)
		}
	}
	for _, change := range []struct {
		model  any
		column string
	}{{&model.HttpAuthBase{}, "PasswordHashed"}, {&model.HttpAuthSession{}, "Port"}, {&model.GateUser{}, "AuthVersion"}, {&model.GateUser{}, "TOTPSecret"}, {&model.GateSession{}, "AuthVersion"}, {&model.GateRuntime{}, "Owner"}} {
		if !db.Migrator().HasColumn(change.model, change.column) {
			t.Errorf("missing additive column %s", change.column)
		}
	}
	var count int64
	if err = db.Model(&model.PortConfig{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("migration silently created or enabled new listeners")
	}
	untouched, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(untouched) != original {
		t.Fatal("source database was changed")
	}
	t.Log("actual v2.0.1 schema upgraded twice; host/IP preserved, additive columns/tables present, source untouched")
}

func TestGateRuntimeOwnerAdditiveMigrationPreservesShutdownMarker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "gate-old.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Exec("CREATE TABLE gate_runtime (id integer PRIMARY KEY, clean_shutdown numeric)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO gate_runtime (id, clean_shutdown) VALUES (1, 1)").Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&model.GateRuntime{}, "Owner") {
		t.Fatal("fixture already contains owner column")
	}
	for i := 0; i < 2; i++ {
		if err := InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
		if !db.Migrator().HasColumn(&model.GateRuntime{}, "Owner") {
			t.Fatal("owner column was not added")
		}
		var marker model.GateRuntime
		if err := db.First(&marker, 1).Error; err != nil {
			t.Fatal(err)
		}
		if !marker.CleanShutdown || marker.Owner != "" {
			t.Fatalf("migration changed existing shutdown marker: %+v", marker)
		}
	}
	if err := db.Model(&model.GateRuntime{}).Where("id = ?", 1).Update("owner", "migration-test-owner").Error; err != nil {
		t.Fatal(err)
	}
	if err := InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	var marker model.GateRuntime
	if err := db.First(&marker, 1).Error; err != nil {
		t.Fatal(err)
	}
	if marker.Owner != "migration-test-owner" || !marker.CleanShutdown {
		t.Fatalf("repeat initialization overwrote the persisted owner: %+v", marker)
	}
}
