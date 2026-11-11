package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGateDurableRevocationAndRestart(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "gate.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.GateRuntime{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLog, oldService, oldGate := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, WafPipelineServiceApp, WafPipelineGateServiceApp
	oldPorts := wafpipeline.Ports
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	global.GWAF_LOCAL_DB = db
	global.GWAF_LOCAL_LOG_DB = nil
	WafPipelineServiceApp = &WafPipelineService{}
	s := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	WafPipelineGateServiceApp = s
	t.Cleanup(func() {
		wafpipeline.Ports = oldPorts
		global.GWAF_LOCAL_DB = oldDB
		global.GWAF_LOCAL_LOG_DB = oldLog
		WafPipelineServiceApp = oldService
		WafPipelineGateServiceApp = oldGate
	})
	cfg := wafpipeline.PortConfig{Schema: 1, Port: 45124, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	cfg.Policy.Gate.Enabled = true
	raw, _ := json.Marshal(cfg)
	secret, _ := pipelineSecret()
	secretRaw, _ := json.Marshal(wafpipeline.PortSecrets{GateKey: secret})
	if err = db.Create(&model.PortConfig{BaseOrm: pipelineORM(), Port: int(cfg.Port), ConfigJSON: string(raw), SecretsJSON: string(secretRaw), Revision: 1}).Error; err != nil {
		t.Fatal(err)
	}
	hash, _ := utils.BcryptHash("long-safe-pass-42")
	user := model.GateUser{BaseOrm: pipelineORM(), PortID: wafpipeline.PortID(cfg.Port), Username: "owner", PasswordHash: hash, AuthVersion: "v1", Enabled: true}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.Load(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	session := wafpipeline.GateSessionRecord{ID: "session-1", PortID: user.PortID, Username: "owner", TokenHash: "hash-only", Version: "v1", CreatedAt: now, LastSeen: now, ExpiresAt: now + 86400}
	if err = s.Store.Add(session, 0); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	if err = restarted.Load(); err != nil {
		t.Fatal(err)
	}
	if len(restarted.Store.Sessions(user.PortID)) != 1 {
		t.Fatal("clean restart lost session")
	}
	if err = restarted.Kick(cfg.Port, session.ID, "test"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.GateSession{}).Count(&count)
	if count != 0 {
		t.Fatal("kick not durable")
	}
	if err = restarted.Store.Add(session, 0); err != nil {
		t.Fatal(err)
	}
	if err = restarted.Store.Flush(); err != nil {
		t.Fatal(err)
	}
	crash := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	if err = crash.Load(); err != nil {
		t.Fatal(err)
	}
	if len(crash.Store.Sessions(user.PortID)) != 0 {
		t.Fatal("unclean restart resurrected browser sessions")
	}
	if err = crash.Delete(cfg.Port, "owner", "test"); err == nil {
		t.Fatal("deleted last gate account")
	}
	if err = crash.Save(GateUserSaveRequest{Port: cfg.Port, Username: "owner", Password: "password123", Enabled: true}, "test"); err == nil {
		t.Fatal("accepted weak password")
	}
	if err = crash.Store.Add(session, 0); err != nil {
		t.Fatal(err)
	}
	if err = crash.Save(GateUserSaveRequest{Port: cfg.Port, Username: "owner", Password: "different-strong-password", Enabled: true}, "test"); err != nil {
		t.Fatal(err)
	}
	if len(crash.Store.Sessions(user.PortID)) != 0 {
		t.Fatal("password change did not revoke")
	}
	token, err := WafPipelineServiceApp.RotateServiceToken(cfg.Port, false, "test", 1)
	if err != nil || token == "" {
		t.Fatalf("rotate: %v", err)
	}
	snapshot, ok := wafpipeline.Ports.Get(cfg.Port)
	if !ok || snapshot.Config.Revision != 2 || len(snapshot.Secrets.ServiceTokenHashes) != 1 {
		t.Fatal("rotation did not publish the new revision and secret")
	}
	if _, err := WafPipelineServiceApp.RotateServiceToken(cfg.Port, true, "test", 1); err == nil {
		t.Fatal("stale draft revoked the current token")
	}
	after, _ := wafpipeline.Ports.Get(cfg.Port)
	if after != snapshot {
		t.Fatal("stale rotation replaced the snapshot")
	}
	var stored string
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", cfg.Port).Scan(&stored).Error; err != nil || strings.Contains(stored, token) || strings.Contains(stored, snapshot.Secrets.ServiceTokenHashes[0]) {
		t.Fatal("token or its hash escaped the encrypted envelope", err)
	}
	if _, err := WafPipelineServiceApp.RotateServiceToken(cfg.Port, true, "test", 2); err != nil {
		t.Fatal(err)
	}
	after, _ = wafpipeline.Ports.Get(cfg.Port)
	if after.Config.Revision != 3 || len(after.Secrets.ServiceTokenHashes) != 0 {
		t.Fatal("current revision could not revoke the token")
	}
}
