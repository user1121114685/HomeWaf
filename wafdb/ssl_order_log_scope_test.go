package wafdb

import (
	"path/filepath"
	"testing"

	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSSLOrderLogMigrationAndProductionTenantScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "scope.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"a", "b"} {
		if err := db.Create(&model.SslOrder{BaseOrm: baseorm.BaseOrm{Id: owner, Tenant_ID: "tenant-" + owner, USER_CODE: "user-" + owner}, ApplyStatus: "fail"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`CREATE TABLE ssl_order_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, order_id TEXT, created_at datetime, level TEXT, message TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO ssl_order_logs (order_id, level, message) VALUES ('a','info','first'),('b','info','second'),('orphan','info','orphan')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := initializeSSLOrderLogScope(db); err != nil {
		t.Fatal(err)
	}
	oldTenant, oldUser := global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	t.Cleanup(func() { global.GWAF_TENANT_ID, global.GWAF_USER_CODE = oldTenant, oldUser })
	if err := RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"a", "b"} {
		global.GWAF_TENANT_ID, global.GWAF_USER_CODE = "tenant-"+owner, "user-"+owner
		var rows []model.SslOrderLog
		if err := db.Order("id").Find(&rows).Error; err != nil {
			t.Fatalf("production tenant query failed: %v", err)
		}
		if len(rows) != 1 || rows[0].OrderID != owner {
			t.Fatalf("wrong migrated ownership for %s: %+v", owner, rows)
		}
	}
	var orphan struct {
		Tenant string
		User   string
	}
	if err := db.Raw("SELECT coalesce(tenant_id,'') AS tenant, coalesce(user_code,'') AS user FROM ssl_order_logs WHERE order_id='orphan'").Scan(&orphan).Error; err != nil {
		t.Fatal(err)
	}
	if orphan.Tenant != "" || orphan.User != "" {
		t.Fatal("orphan reassigned to active tenant")
	}
}
