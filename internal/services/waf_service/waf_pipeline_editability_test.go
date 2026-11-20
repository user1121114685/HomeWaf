package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/wafsec"
	"HomeWaf/internal/protection/wafpipeline"
	sqlite "HomeWaf/internal/sqlitedialect"
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPortPolicyRepeatedEditsPersistWithoutRecreationOrSecretLoss(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "editable.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.Hosts{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogs, oldPorts := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	t.Cleanup(func() { global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports = oldDB, oldLogs, oldPorts })
	runtime := &wafpipeline.Runtime{}
	service := &WafPipelineService{Prepare: runtime.Prepare}
	config := wafpipeline.PortConfig{Schema: 1, Port: 45129, Name: "editable", Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	config.Policy.Gate.Enabled = false
	config = seedLegacyPortFixture(t, service, db, config)
	initial := wafpipeline.CloneConfig(config)
	snapshot, _ := wafpipeline.Ports.Get(config.Port)
	gateKey := snapshot.Secrets.GateKey
	for version := 2; version <= 4; version++ {
		config.Name = fmt.Sprintf("edited-%d", version)
		config.Policy.Defense.Enabled = true
		config.Policy.Defense.Mode = wafpipeline.DefenseQuiet
		config.Policy.Defense.Count = version * 10
		config.Policy.Gate.Title = fmt.Sprintf("Login %d", version)
		config.Policy.RuleSet.QPS.PerIP = float64(40 + version)
		config.Policy.Network.MaxProxyHops = version + 8
		config.Policy.IPGroup.Entries = []wafpipeline.IPEntry{{ID: "source", Type: "allow", Trust: "allow_only", Value: "198.51.100.7"}}
		config, err = service.Save(PortSaveRequest{Config: config})
		if err != nil {
			t.Fatal(err)
		}
		if config.Revision != int64(version) {
			t.Fatalf("unexpected edit revision: %d", config.Revision)
		}
		wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
		loaded := &WafPipelineService{Prepare: runtime.Prepare}
		if err := loaded.Load(); err != nil {
			t.Fatal(err)
		}
		config, err = loaded.Get(config.Port)
		if err != nil {
			t.Fatal(err)
		}
		if config.Name != fmt.Sprintf("edited-%d", version) || !config.Policy.Defense.Enabled || config.Policy.Defense.Mode != wafpipeline.DefenseQuiet || config.Policy.Defense.Count != version*10 || config.Policy.Gate.Title != fmt.Sprintf("Login %d", version) || config.Policy.RuleSet.QPS.PerIP != float64(40+version) || config.Policy.Network.MaxProxyHops != version+8 || len(config.Policy.IPGroup.Entries) != 1 {
			t.Fatal("saved configuration did not survive reload", config)
		}
		snapshot, _ = wafpipeline.Ports.Get(config.Port)
		if snapshot.Secrets.GateKey != gateKey || config.ListenEnabled {
			t.Fatal("editing recreated credentials or activated the listener")
		}
		service = loaded
	}
	if _, err := service.Save(PortSaveRequest{Config: initial}); err == nil {
		t.Fatal("stale editor overwrote newer settings")
	}
	current, err := service.Get(config.Port)
	if err != nil || current.Revision != config.Revision || current.Name != config.Name {
		t.Fatal("conflict changed the saved policy", err)
	}
}
