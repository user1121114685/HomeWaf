package waftask

import (
	"path/filepath"
	"testing"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/spec"
	"HomeWaf/wafdb"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafsec"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Exercise the scheduled job, persistence, tenant callback and queue without
// starting an issuer or making DNS/CA requests. No expiry-monitor rows exist.
func TestSSLOrderScheduledRenewalNeedsNoExpiryMonitor(t *testing.T) {
	previousDialect := func() (d dialect.DBDialect) { defer func() { _ = recover() }(); return dialect.Get() }()
	dialect.Register(&dialect.SQLiteDialect{})
	t.Cleanup(func() { dialect.Register(previousDialect) })
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "scheduled.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.Hosts{}, &model.SslOrder{}, &model.SslOrderLog{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldQueue := global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder
	oldTenant, oldUser := global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	oldDays := global.CurrentSettings().GCONFIG_RECORD_SSLOrder_EXPIRE_DAY
	global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = db, make(chan spec.ChanSslOrder, 10)
	global.GWAF_TENANT_ID, global.GWAF_USER_CODE = "renew-tenant", "renew-user"
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.GCONFIG_RECORD_SSLOrder_EXPIRE_DAY = 30 })
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = oldDB, oldQueue
		global.GWAF_TENANT_ID, global.GWAF_USER_CODE = oldTenant, oldUser
		global.UpdateSettings(func(s *global.RuntimeSettings) { s.GCONFIG_RECORD_SSLOrder_EXPIRE_DAY = oldDays })
	})
	if err := wafdb.RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		days    int
		enabled bool
	}{{"due", 15, true}, {"disabled", 15, false}, {"fresh", 60, true}} {
		base := baseorm.BaseOrm{Id: tc.name, Tenant_ID: global.GWAF_TENANT_ID, USER_CODE: global.GWAF_USER_CODE, CREATE_TIME: customtype.JsonTime(time.Now()), UPDATE_TIME: customtype.JsonTime(time.Now())}
		host := model.Hosts{BaseOrm: base, Code: tc.name, Host: tc.name + ".example.com", Port: 8443, Ssl: 1, BindSslId: "cert-" + tc.name}
		if err := db.Create(&host).Error; err != nil {
			t.Fatal(err)
		}
		order := model.SslOrder{BaseOrm: base, HostCode: tc.name, ApplyDomain: host.Host, ApplyStatus: "success", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyEmail: "owner@example.com", PrivateGroupName: "household", AutoRenew: &tc.enabled, ResultCertificate: []byte("fixture-cert"), ResultPrivateKey: []byte("fixture-key"), ResultValidTo: customtype.JsonTime(time.Now().Add(time.Duration(tc.days) * 24 * time.Hour))}
		if err := db.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
	}
	SSLOrderReload()
	SSLOrderReload()
	var pending []model.SslOrder
	if err := db.Where("apply_status = ?", "submitted").Find(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].RenewFrom != "due" || !pending[0].AutoRenewEnabled() || pending[0].PrivateGroupName != "household" {
		t.Fatal("scheduled renewal ignored preference, due date or saved credentials")
	}
	if len(global.GWAF_CHAN_SSLOrder) != 1 {
		t.Fatal("scheduled check duplicated issuance")
	}
	if db.Migrator().HasTable(&model.SslExpire{}) {
		t.Fatal("renewal unexpectedly depends on expiry-monitor configuration")
	}
}
