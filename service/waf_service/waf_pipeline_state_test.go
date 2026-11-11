package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"net/netip"
	"path/filepath"
	"testing"
)

func TestPipelineStateSQLiteRestoreAndManagement(t *testing.T) {
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "states.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.PipelineState{}, &model.PipelineDailyMetrics{}, &model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.Hosts{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogs, oldRegistry, oldService := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineServiceApp
	global.GWAF_LOCAL_DB = db
	global.GWAF_LOCAL_LOG_DB = nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	live := &wafpipeline.Runtime{}
	WafPipelineServiceApp = &WafPipelineService{Prepare: live.Prepare}
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB = oldDB
		global.GWAF_LOCAL_LOG_DB = oldLogs
		wafpipeline.Ports = oldRegistry
		WafPipelineServiceApp = oldService
	})
	policy := wafpipeline.DefaultPolicy()
	policy.Honeypot.Enabled = true
	policy.HitPolicy.Lockdown.Enabled = true
	config := wafpipeline.PortConfig{Schema: 1, Port: 45126, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: policy}
	if _, err := WafPipelineServiceApp.Save(PortSaveRequest{Config: config}); err != nil {
		t.Fatal(err)
	}
	s := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	port := wafpipeline.PortID(config.Port)
	hit := func(ip, category string, severity wafpipeline.Severity, req string) {
		s.Punisher.Submit(wafpipeline.HitEvent{PortID: port, ClientIP: netip.MustParseAddr(ip), Category: category, Severity: severity, ReqID: req})
		s.Punisher.Drain(256)
	}
	hit("198.51.100.7", "rce", wafpipeline.Critical, "critical")
	for i := 0; i < 3; i++ {
		hit("198.51.100.8", "weak_pwd", wafpipeline.Low, fmt.Sprint("weak", i))
	}
	for i := 1; i <= 5; i++ {
		hit(fmt.Sprintf("198.51.100.%d", 20+i), "sqli", wafpipeline.High, fmt.Sprint("high", i))
	}
	if err := s.Punisher.Flush(); err != nil {
		t.Fatal(err)
	}
	loaded := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if loaded.Punisher.BanReason("port:45127", netip.MustParseAddr("198.51.100.7")) == "" || !loaded.Punisher.InHoneypot(port, netip.MustParseAddr("198.51.100.8")) || !loaded.Punisher.Locked(port) || loaded.Punisher.Locked("port:45127") {
		t.Fatal("persisted states or scope lost")
	}
	var banID string
	for _, r := range loaded.Punisher.Records(port) {
		if r.Kind == "ban" {
			banID = r.ID
		}
	}
	if err := loaded.Manage(config.Port, banID, "permanent", "test-operator"); err != nil {
		t.Fatal(err)
	}
	for _, r := range loaded.Punisher.Records(port) {
		if r.ID == banID && r.ExpiresAt != 0 {
			t.Fatal("permanent operation ignored")
		}
	}
	if err := loaded.Manage(config.Port, banID, "whitelist", "test-operator"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := wafpipeline.Ports.Get(config.Port)
	if !ok || !snapshot.Compiled.Trusted.Contains(net.ParseIP("198.51.100.7"), 0) {
		t.Fatal("whitelist was not published")
	}
	if loaded.Punisher.BanReason(port, netip.MustParseAddr("198.51.100.7")) != "" {
		t.Fatal("whitelist retained the ban")
	}
	restarted := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := restarted.Load(); err != nil {
		t.Fatal(err)
	}
	if restarted.Punisher.BanReason(port, netip.MustParseAddr("198.51.100.7")) != "" {
		t.Fatal("release resurrected after restart")
	}
	snapshot.Compiled.Metrics.AddDaily(12, 3, 7)
	if err := loaded.PersistMetrics(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.LoadMetrics(); err != nil {
		t.Fatal(err)
	}
	stats := restarted.MetricsForPort(config.Port).Stats()
	if stats["today_requests"] != 12 || stats["today_denied"] != 3 || stats["today_hits"] != 7 {
		t.Fatalf("daily counters lost on restart: %v", stats)
	}
}

func TestPipelineStatePersistenceWritesOnlyChangedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "delta.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	if err = db.AutoMigrate(&model.PipelineState{}); err != nil {
		t.Fatal(err)
	}
	old := global.GWAF_LOCAL_DB
	global.GWAF_LOCAL_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_DB = old })
	s := &WafPipelineStateService{}
	a := wafpipeline.PunishmentRecord{ID: "a", PortID: "port:5000", Kind: "ban"}
	b := a
	b.ID = "b"
	changes := func() int64 {
		var n int64
		if err := db.Raw("SELECT total_changes()").Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	if err = s.persist([]wafpipeline.PunishmentRecord{a}); err != nil {
		t.Fatal(err)
	}
	before := changes()
	if err = s.persist([]wafpipeline.PunishmentRecord{a, b}); err != nil {
		t.Fatal(err)
	}
	if changes()-before != 1 {
		t.Fatal("adding one state rewrote unchanged rows")
	}
	before = changes()
	if err = s.persist([]wafpipeline.PunishmentRecord{b}); err != nil {
		t.Fatal(err)
	}
	if changes()-before != 1 {
		t.Fatal("removing one state rewrote remaining rows")
	}
	var rows []model.PipelineState
	if err = db.Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].ID != "b" {
		t.Fatalf("persisted state incorrect: %+v %v", rows, err)
	}
}

func TestPipelineMetricsChurnKeepsLivePortAndBoundsHelpers(t *testing.T) {
	old := wafpipeline.Ports
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](2)
	t.Cleanup(func() { wafpipeline.Ports = old })
	s := &WafPipelineStateService{}
	first := s.MetricsForPort(1)
	first.AddDaily(5, 1, 2)
	publish := func(port uint16, m *wafpipeline.PortMetrics) {
		t.Helper()
		if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) {
			return &wafpipeline.Snapshot{Compiled: &wafpipeline.CompiledPolicy{Metrics: m}}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	publish(1, first)
	for port := uint16(10); port < 5010; port++ {
		m := s.MetricsForPort(port)
		publish(port, m)
		wafpipeline.Ports.Remove(port)
	}
	if len(s.metrics) > 2 || s.MetricsForPort(1) != first || first.Daily().Requests != 5 {
		t.Fatal("port churn lost live metrics or exceeded capacity")
	}
	s.metricsSaved = map[uint16]wafpipeline.DailyMetrics{1: first.Daily(), 999: {Day: 1}}
	if err := s.PersistMetrics(); err != nil {
		t.Fatal(err)
	}
	if len(s.metricsSaved) != 1 {
		t.Fatal("deleted port retained in persistence cache")
	}
}
