package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafdb"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"crypto/sha256"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestV201BusinessConfigurationMigrationRehearsal(t *testing.T) {
	source := os.Getenv("HOMEWAF_V201_DB")
	if source == "" {
		t.Skip("run scripts/verify-pipeline-migration.ps1 for the actual v2.0.1 fixture")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	original := sha256.Sum256(data)
	dest := filepath.Join(t.TempDir(), "business-upgrade.db")
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
	if db.Migrator().HasTable(&model.PortConfig{}) {
		t.Fatal("source already contains pipeline schema")
	}
	if err = wafdb.InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	if !wafsec.DataKeyReady() {
		if err = wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	oldDB, oldLog, oldPorts, oldGate := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	WafPipelineGateServiceApp = &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	defer func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp = oldDB, oldLog, oldPorts, oldGate
		wafpipeline.SetLegacyPortOwner(48080, false)
	}()
	var before model.Hosts
	if err = db.First(&before, "id = ?", "rehearsal-host").Error; err != nil {
		t.Fatal(err)
	}
	s := &WafPipelineService{Prepare: (&wafpipeline.Runtime{}).Prepare}
	p, err := s.PreviewLegacyPort(48080, "legacy-port")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blockers) > 0 {
		t.Fatalf("v2.0.1 conversion blocked: %v", p.Blockers)
	}
	if len(p.Config.Routes) != 1 || !reflect.DeepEqual(p.Config.Routes[0].Hosts, []string{"family.test", "album.test"}) || len(p.Config.Policy.IPGroup.Entries) != 1 || p.Config.Policy.IPGroup.Entries[0].Value != "192.168.20.0/24" {
		t.Fatal("actual old business policy was not copied")
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 48080, PolicyHost: "legacy-port", PreviewToken: p.PreviewToken, AcceptChanges: true}, "migration-rehearsal")
	if err != nil {
		t.Fatal(err)
	}
	var saved model.PortConfig
	if err = db.First(&saved, "port = ?", 48080).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodePort(saved)
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenEnabled || snapshot.Config.ListenEnabled || snapshot.Secrets.GateKey == "" || wafpipeline.OwnsLegacyPort(48080) {
		t.Fatal("import enabled listener or omitted encrypted secrets")
	}
	if err = wafdb.InitializeCoreSchema(db); err != nil {
		t.Fatal(err)
	}
	if err = s.Load(); err != nil {
		t.Fatal(err)
	}
	loaded, ok := wafpipeline.Ports.Get(48080)
	if !ok || !reflect.DeepEqual(loaded.Config, c) {
		t.Fatal("imported business config did not survive reload")
	}
	var after model.Hosts
	db.First(&after, "id = ?", "rehearsal-host")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("source host configuration was altered")
	}
	untouched, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(untouched) != original {
		t.Fatal("original v2.0.1 database changed")
	}
	t.Log("actual v2.0.1 host, alias, upstream and IP policy migrated; sealed secrets persisted; reload preserved native config; old source unchanged; listener remains disabled")
}
