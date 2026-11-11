package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPortPersistenceRejectsMissingAccountAndRollsBackBind(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "port.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.Hosts{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLog, oldPorts := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports
	global.GWAF_LOCAL_DB = db
	global.GWAF_LOCAL_LOG_DB = nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	t.Cleanup(func() { global.GWAF_LOCAL_DB = oldDB; global.GWAF_LOCAL_LOG_DB = oldLog; wafpipeline.Ports = oldPorts })
	live := &wafpipeline.Runtime{Forward: func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, w http.ResponseWriter) {
		w.WriteHeader(204)
	}}
	s := &WafPipelineService{Prepare: live.Prepare, OnChange: func(old, next *wafpipeline.Snapshot) error {
		if next != nil && next.Config.ListenEnabled {
			return errors.New("bind fixture failure")
		}
		return nil
	}}
	c := wafpipeline.PortConfig{Schema: 1, Port: 45123, Name: "fixture", Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.Enabled = true
	if _, err := s.Save(PortSaveRequest{Config: c}); err == nil {
		t.Fatal("enabled empty gate accepted")
	}
	c.Policy.Gate.Enabled = false
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", c.Port).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	plain, decryptErr := wafsec.DecryptBound(raw, "port_config/secrets_json")
	if decryptErr != nil || len(plain) == 0 || string(plain) == raw {
		t.Fatal("port key failed authenticated field decryption")
	}
	if _, wrongScopeErr := wafsec.DecryptBound(raw, "other_table/secrets_json"); wrongScopeErr == nil {
		t.Fatal("port key was not bound to its field")
	}
	c.ListenEnabled = true
	if _, err := s.Save(PortSaveRequest{Config: c}); err == nil {
		t.Fatal("failed bind accepted")
	}
	stored, err := s.Get(c.Port)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ListenEnabled || stored.Revision != 1 {
		t.Fatal("failed activation changed snapshot")
	}
	var row model.PortConfig
	if err = db.Where("port = ?", c.Port).First(&row).Error; err != nil || row.Revision != 1 {
		t.Fatal("failed activation changed persisted revision")
	}
	snapshot, ok := wafpipeline.Ports.Get(c.Port)
	if !ok || snapshot.Compiled == nil || snapshot.Compiled.Pipeline == nil {
		t.Fatal("rollback lost executable policy")
	}
	req := httptest.NewRequest("GET", "http://example.test/", nil)
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(c.Port)}))
	response := httptest.NewRecorder()
	live.Serve(snapshot, response, req)
	if response.Code != 204 {
		t.Fatalf("rolled-back policy no longer serves: %d", response.Code)
	}
	// One corrupt persisted port must not prevent a healthy port from loading.
	c.Port++
	c.Revision = 0
	c.ListenEnabled = false
	if _, err := s.Save(PortSaveRequest{Config: c}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.PortConfig{}).Where("port = ?", c.Port).Update("config_json", "invalid-json").Error; err != nil {
		t.Fatal(err)
	}
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	if err := s.Load(); err == nil {
		t.Fatal("corrupt port not reported")
	}
	if _, ok := wafpipeline.Ports.Get(c.Port - 1); !ok {
		t.Fatal("healthy port not recovered")
	}
	if _, ok := wafpipeline.Ports.Get(c.Port); ok {
		t.Fatal("corrupt port published")
	}
	if s.Issues()[c.Port] == "" {
		t.Fatal("missing per-port issue")
	}
	configs, err := s.List()
	if err != nil || len(configs) != 1 || configs[0].Port != c.Port-1 {
		t.Fatal("corrupt row broke management listing")
	}
}
