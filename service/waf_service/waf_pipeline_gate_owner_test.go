package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

func TestOlderGateWorkerCannotOverwriteNewOwnerOrReviveLogout(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "owners.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.GateRuntime{}); err != nil {
		t.Fatal(err)
	}
	oldDB := global.GWAF_LOCAL_DB
	global.GWAF_LOCAL_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_DB = oldDB })
	if err := db.Create(&model.PortConfig{BaseOrm: pipelineORM(), Port: 5020}).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.BcryptHash("owner-test-strong-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GateUser{BaseOrm: pipelineORM(), PortID: "port:5020", Username: "owner", PasswordHash: hash, AuthVersion: "v1", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	a := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	if err := a.Load(); err != nil {
		t.Fatal(err)
	}
	b := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	if err := b.Load(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	current := wafpipeline.GateSessionRecord{ID: "current-session", PortID: "port:5020", Username: "owner", TokenHash: "current-token-hash", Version: "v1", CreatedAt: now, LastSeen: now, ExpiresAt: now + 3600}
	if err := b.Store.Add(current, 0); err != nil {
		t.Fatal(err)
	}
	if err := b.Store.Flush(); err != nil {
		t.Fatal(err)
	}
	stale := current
	stale.ID = "old-worker-session"
	stale.TokenHash = "old-worker-token-hash"
	if err := a.Store.Add(stale, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.Flush(); err == nil {
		t.Fatal("superseded worker overwrote current sessions")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	var marker model.GateRuntime
	if err := db.First(&marker, 1).Error; err != nil {
		t.Fatal(err)
	}
	if marker.CleanShutdown {
		t.Fatal("old close marked an active newer worker clean")
	}
	var persisted []model.GateSession
	if err := db.Find(&persisted).Error; err != nil || len(persisted) != 1 || persisted[0].Id != current.ID {
		t.Fatal("old snapshot replaced current session", err)
	}
	if !b.Store.Revoke(current.PortID, current.ID) {
		t.Fatal("current session not revoked in memory")
	}
	// Simulate a crash in the asynchronous logout window: no B.Flush or Close.
	c := &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if len(c.Store.Sessions(current.PortID)) != 0 {
		t.Fatal("crash resurrected a logged-out session")
	}
}
