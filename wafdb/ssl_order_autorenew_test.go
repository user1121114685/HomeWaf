package wafdb

import (
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSSLOrderAutoRenewMigrationDefaultsExistingOrdersAndPreservesOptOut(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "renew.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&model.SslOrder{}, "AutoRenew"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ssl_orders (id, apply_status) VALUES ('existing-success', 'success')").Error; err != nil {
		t.Fatal(err)
	}
	if err := InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	var order model.SslOrder
	if err := db.First(&order, "id = ?", "existing-success").Error; err != nil {
		t.Fatal(err)
	}
	if order.AutoRenew == nil || !order.AutoRenewEnabled() {
		t.Fatal("existing certificate was not enabled by default")
	}
	if err := db.Model(&order).Update("auto_renew", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&order, "id = ?", order.Id).Error; err != nil {
		t.Fatal(err)
	}
	if order.AutoRenewEnabled() {
		t.Fatal("subsequent startup overwrote explicit opt-out")
	}
}
