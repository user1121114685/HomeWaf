package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPipelineHoneypotLogsLatestTwentyAndPortIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "honey.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	old := global.GWAF_LOCAL_LOG_DB
	global.GWAF_LOCAL_LOG_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_LOG_DB = old })
	rows := []model.PipelineLog{}
	for i := range 25 {
		rows = append(rows, model.PipelineLog{ID: fmt.Sprintf("h%02d", i), PortID: "port:5000", Kind: "request", Verdict: "honeypot", TimeMillis: int64(i)})
	}
	// All of this noise is newer than the honeypot requests. Filtering after
	// an ordinary 100-row query would lose every honeypot event.
	for i := range 120 {
		rows = append(rows, model.PipelineLog{ID: fmt.Sprintf("n%03d", i), PortID: "port:5000", Kind: "request", Verdict: "allow", TimeMillis: int64(100 + i)})
	}
	rows = append(rows,
		model.PipelineLog{ID: "other-port", PortID: "port:5001", Kind: "request", Verdict: "honeypot", TimeMillis: 999},
		model.PipelineLog{ID: "state", PortID: "port:5000", Kind: "state", Verdict: "honeypot", TimeMillis: 999},
	)
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	s := &WafPipelineStateService{}
	got, err := s.HoneypotLogs(5000)
	if err != nil || len(got) != 20 {
		t.Fatalf("latest twenty failed: %d %v", len(got), err)
	}
	for i, row := range got {
		if row.ID != fmt.Sprintf("h%02d", 24-i) {
			t.Fatalf("wrong order, kind or port at %d: %+v", i, row)
		}
	}
	all, err := s.Logs(5000)
	if err != nil || len(all) != 100 || all[0].ID != "state" {
		t.Fatalf("existing log behavior changed: %d %v", len(all), err)
	}
	global.GWAF_LOCAL_LOG_DB = nil
	if _, err := s.HoneypotLogs(5000); err == nil {
		t.Fatal("missing log database silently accepted")
	}
}
