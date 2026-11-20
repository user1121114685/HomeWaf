package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/wafsec"
	"HomeWaf/internal/protection/wafpipeline"
	sqlite "HomeWaf/internal/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"strings"
	"testing"
)

func nativePortFixture(t *testing.T) (*WafPipelineService, *gorm.DB) {
	t.Helper()
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "ports.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { conn.Close() })
	if err = db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.IPGroupItem{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLog, oldPorts, oldGate := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	WafPipelineGateServiceApp = &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp = oldDB, oldLog, oldPorts, oldGate
	})
	s := &WafPipelineService{Prepare: (&wafpipeline.Runtime{}).Prepare, OnChange: func(old, next *wafpipeline.Snapshot) error { return nil }}
	return s, db
}

func TestPortPublishFailureRollsBackDatabaseAndInitialAccount(t *testing.T) {
	s, db := nativePortFixture(t)
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](1)
	s.Prepare = func(snap *wafpipeline.Snapshot) error {
		if err := (&wafpipeline.Runtime{}).Prepare(snap); err != nil {
			return err
		}
		return wafpipeline.Ports.Publish(5400, func() (*wafpipeline.Snapshot, error) { return &wafpipeline.Snapshot{}, nil })
	}
	c := wafpipeline.PortConfig{Schema: 1, Port: 5301, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: newServicePolicyFixture()}
	c.Policy.Gate.Enabled = true
	if _, err := s.Save(PortSaveRequest{Config: c, Username: "owner", Password: "correct-home-password"}); err == nil {
		t.Fatal("registry quota failure accepted")
	}
	var ports, users int64
	db.Model(&model.PortConfig{}).Where("port = ?", 5301).Count(&ports)
	db.Model(&model.GateUser{}).Where("port_id = ?", wafpipeline.PortID(5301)).Count(&users)
	if ports != 0 || users != 0 {
		t.Fatalf("failed publication left persisted rows ports=%d users=%d", ports, users)
	}
}

func TestBusinessHeadersAreSealedAndUpdatesDoNotMutateSnapshots(t *testing.T) {
	s, db := nativePortFixture(t)
	secret := "fixture-upstream-private-token"
	config := wafpipeline.PortConfig{Schema: 1, Port: 5300, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: newServicePolicyFixture()}
	config.HeaderProfile = "origin"
	profile := wafpipeline.BusinessHeaders{Request: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: "Bearer " + secret}}}
	c, err := s.Save(PortSaveRequest{Config: config, BusinessHeaders: map[string]wafpipeline.BusinessHeaders{"origin": profile}})
	if err != nil {
		t.Fatal(err)
	}
	var stored, encrypted string
	if err := db.Raw("SELECT config_json FROM port_config WHERE port = ?", 5300).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", 5300).Scan(&encrypted).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) || strings.Contains(encrypted, secret) {
		t.Fatal("business credential stored in plaintext")
	}
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	old, ok := wafpipeline.Ports.Get(5300)
	if !ok {
		t.Fatal("missing saved snapshot")
	}
	changed := map[string]wafpipeline.BusinessHeaders{"origin": {Request: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: "Bearer replacement-fixture"}}}}
	if _, err = s.Save(PortSaveRequest{Config: c, BusinessHeaders: changed}); err != nil {
		t.Fatal(err)
	}
	if old.Secrets.BusinessHeaders["origin"].Request[0].HeaderValue != "Bearer "+secret {
		t.Fatal("updating secret headers mutated published old snapshot")
	}
	copy, err := s.Copy(5300, 5350, false)
	if err != nil {
		t.Fatal(err)
	}
	if copy.HeaderProfile != "" {
		t.Fatal("copy retained secret header reference")
	}
	for _, route := range copy.Routes {
		if route.HeaderProfile != "" {
			t.Fatal("route copy retained credentials")
		}
	}
}
