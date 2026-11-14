package waf_service

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestHoneypotDropSQLiteRestoreArchiveQueryAndNotification(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "honey-controls.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.PipelineState{}, &model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogDB, oldQueue, oldMessages := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GQEQUE_LOG_DB, global.GQEQUE_MESSAGE_DB
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, db
	global.GQEQUE_LOG_DB = nil
	global.GQEQUE_MESSAGE_DB = queue.NewQueue()
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GQEQUE_LOG_DB, global.GQEQUE_MESSAGE_DB = oldDB, oldLogDB, oldQueue, oldMessages
	})
	now := time.Now().Unix()
	rows := []wafpipeline.PunishmentRecord{
		{ID: "drop|port:5001|198.51.100.7/32", Kind: "drop", Scope: "port", PortID: "port:5001", Prefix: "198.51.100.7/32", Reason: "honeypot_hit_threshold", ExpiresAt: now + 10800},
		{ID: "honeypot_lockdown|port:5000|", Kind: "honeypot_lockdown", Scope: "port", PortID: "port:5000", Reason: "honeypot_distinct_sources", ExpiresAt: now + 10800, CooldownUntil: now + 10860},
	}
	s := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := s.persist(rows); err != nil {
		t.Fatal(err)
	}
	restored := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := restored.Load(); err != nil {
		t.Fatal(err)
	}
	if len(restored.Punisher.Records("")) != 2 {
		t.Fatal("drop records lost across sqlite restore")
	}
	logs := []model.PipelineLog{
		{ID: "honey-drop", PortID: "port:5000", Kind: "request", Verdict: "drop", Reason: "honeypot:drop:honeypot_distinct_sources", TimeMillis: 5},
		{ID: "other-drop", PortID: "port:5000", Kind: "request", Verdict: "drop", Reason: "access_disabled", TimeMillis: 9},
		{ID: "state", PortID: "port:5000", Kind: "state", Verdict: "drop", Reason: "honeypot:drop:state", TimeMillis: 9},
		{ID: "other-port", PortID: "port:5001", Kind: "request", Verdict: "drop", Reason: "honeypot:drop:honeypot_hit_threshold", TimeMillis: 9},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	got, err := restored.HoneypotLogs(5000)
	if err != nil || len(got) != 1 || got[0].ID != "honey-drop" {
		t.Fatal("honeypot drops missing from recent archive", got, err)
	}
	s.recordChange(wafpipeline.PunishmentChange{Record: rows[1], Event: "honeypot_lockdown_started"})
	if global.GQEQUE_MESSAGE_DB.Size() != 1 {
		t.Fatal("distributed drop did not enter existing notification queue")
	}
	if err := restored.Manage(5000, rows[1].ID, "release", "test-operator"); err != nil {
		t.Fatal(err)
	}
	again := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	for _, r := range again.Punisher.Records("") {
		if r.Kind == "honeypot_lockdown" {
			t.Fatal("released lock revived from sqlite")
		}
	}
}
