package waf_service

import (
	"HomeWaf/cache"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/utils"
	"HomeWaf/wafsec"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyAuthenticationHashConversionPortBindingAndRevocation(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "auth.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.HttpAuthBase{}, &model.HttpAuthSession{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldCache := global.GWAF_LOCAL_DB, global.GCACHE_WAFCACHE
	global.GWAF_LOCAL_DB = db
	global.GCACHE_WAFCACHE = cache.InitWafCache()
	defer func() { global.GWAF_LOCAL_DB = oldDB; global.GCACHE_WAFCACHE = oldCache }()
	password := "legacy-password-fixture-1"
	row := model.HttpAuthBase{BaseOrm: baseorm.BaseOrm{Id: "old-account"}, HostCode: "same-host", UserName: "alice", Password: password}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	accounts, err := WafHttpAuthBaseServiceApp.LoadForHost(row.HostCode)
	if err != nil || len(accounts) != 1 || !accounts[0].PasswordHashed || !utils.BcryptVerify(accounts[0].Password, password) {
		t.Fatalf("one-way conversion failed: %v", err)
	}
	public, _ := json.Marshal(accounts)
	if strings.Contains(string(public), password) || strings.Contains(string(public), accounts[0].Password) {
		t.Fatal("account API exposes password proof")
	}
	host := model.Hosts{Code: row.HostCode}
	cfg := model.DecodeHttpAuthConfig("")
	token, _, err := WafHttpAuthSessionServiceApp.CreateCustomSession(host, row.UserName, "198.51.100.18", "fixture", cfg, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := WafHttpAuthSessionServiceApp.ValidateCustom(host.Code, token, "198.51.100.18", cfg, 5001); ok {
		t.Fatal("cross-port session accepted")
	}
	if _, ok := WafHttpAuthSessionServiceApp.ValidateCustom(host.Code, token, "198.51.100.18", cfg, 5000); !ok {
		t.Fatal("wrong-port probe invalidated original session")
	}
	if BasicSessionCode(host.Code, "alice", "198.51.100.18", 5000) == BasicSessionCode(host.Code, "alice", "198.51.100.18", 5001) {
		t.Fatal("Basic sessions share identity")
	}
	if err := WafHttpAuthBaseServiceApp.ModifyApi(request.WafHttpAuthBaseEditReq{Id: row.Id, HostCode: row.HostCode, UserName: row.UserName, Password: "changed-password-fixture-2"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := WafHttpAuthSessionServiceApp.ValidateCustom(host.Code, token, "198.51.100.18", cfg, 5000); ok {
		t.Fatal("changed password retained existing session")
	}
}
