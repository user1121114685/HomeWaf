package waf_service

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSMLoginLockNotification(t *testing.T) {
	oldMessages, oldLogs := global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB
	global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB = queue.NewQueue(), nil
	t.Cleanup(func() { global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB = oldMessages, oldLogs })
	s := &WafPipelineStateService{}
	record := wafpipeline.PunishmentRecord{Kind: "synology_login_lockdown", PortID: "port:5001", Hits: 10, CreatedAt: 1000, ExpiresAt: 11800}
	s.recordChange(wafpipeline.PunishmentChange{Record: record, Event: "synology_login_lockdown_started"})
	s.recordChange(wafpipeline.PunishmentChange{Record: record, Event: "released"})
	data, ok := global.GQEQUE_MESSAGE_DB.Dequeue()
	msg, typed := data.(innerbean.SystemErrorMessageInfo)
	if !ok || !typed || msg.Server != record.PortID || !strings.Contains(msg.ErrorMsg, "10 次") || !strings.Contains(msg.ErrorMsg, "恢复时间") {
		t.Fatalf("unexpected lock notification: %+v", data)
	}
	if _, ok := global.GQEQUE_MESSAGE_DB.Dequeue(); ok {
		t.Fatal("duplicate lock notification")
	}
}

func TestDSMLoginLockServiceLifecycle(t *testing.T) {
	defense, db, _ := defenseServiceFixture(t)
	if err := db.AutoMigrate(&model.PipelineState{}); err != nil {
		t.Fatal(err)
	}
	oldState, oldMessages := WafPipelineStateServiceApp, global.GQEQUE_MESSAGE_DB
	state := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	WafPipelineStateServiceApp, global.GQEQUE_MESSAGE_DB = state, queue.NewQueue()
	t.Cleanup(func() { WafPipelineStateServiceApp, global.GQEQUE_MESSAGE_DB = oldState, oldMessages })
	if err := state.Load(); err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":false,"error":{"code":400}}`)
	}))
	defer up.Close()
	proxy := &wafpipeline.Forwarder{}
	live := &wafpipeline.Runtime{State: state.Punisher, ForwardPrepare: proxy.Prepare, Forward: proxy.Serve}
	service := defense.ports()
	service.Prepare = live.Prepare
	const port = 43417
	setMode := func(cfg *wafpipeline.PortConfig, mode wafpipeline.Mode) {
		for i := range cfg.Policy.RuleSet.Rules {
			r := &cfg.Policy.RuleSet.Rules[i]
			if r.Category == "synology_login_failure" {
				r.Mode = mode
				r.LoginFailures.Count = 2
			}
		}
	}
	base := wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: up.URL, Policy: newServicePolicyFixture()}
	setMode(&base, wafpipeline.ModeBlock)
	save := func(c wafpipeline.PortConfig) wafpipeline.PortConfig {
		t.Helper()
		saved, err := service.Save(PortSaveRequest{Config: c})
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	login := func(s *wafpipeline.Snapshot, want int) {
		t.Helper()
		if s == nil {
			s, _ = wafpipeline.Ports.Get(port)
		}
		req := httptest.NewRequest("GET", "http://nas.test/webapi/auth.cgi?api=SYNO.API.Auth&method=login", nil)
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: port}))
		w := httptest.NewRecorder()
		live.Serve(s, w, req)
		if w.Code != want {
			t.Fatalf("expected DSM status %d, got %d", want, w.Code)
		}
	}
	checkStored := func(want int) {
		t.Helper()
		fromDB := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
		err := fromDB.Load()
		stored := fromDB.Punisher.Records(wafpipeline.PortID(port))
		if err != nil || len(stored) != want {
			current, _ := wafpipeline.Ports.Get(port)
			t.Fatalf("expected %d persisted DSM locks, got %v, in-memory=%v current-block=%v: %v", want, stored, state.Punisher.Records(wafpipeline.PortID(port)), current.Config.Policy.RuleSet.LocksDSMLogins(), err)
		}
	}
	cfg := save(base)
	login(nil, 200)
	cfg.Name = "Renamed NAS"
	cfg = save(cfg)
	login(nil, 200)
	login(nil, 429)
	checkStored(1)
	setMode(&cfg, wafpipeline.ModeObserve)
	cfg = save(cfg)
	checkStored(0)
	login(nil, 200)
	setMode(&cfg, wafpipeline.ModeBlock)
	save(cfg)
	login(nil, 200)
	login(nil, 200)
	login(nil, 429)
	old, _ := wafpipeline.Ports.Get(port)
	if err := service.Delete(port, "test"); err != nil {
		t.Fatal(err)
	}
	checkStored(0)
	login(old, 200) // A late credential failure after deletion.
	save(base)
	login(old, 200) // The port now belongs to a different NAS service.
	login(nil, 200)
	checkStored(0)
	login(nil, 200)
	checkStored(1)
}

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
	config := wafpipeline.PortConfig{Schema: 1, Port: 45126, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: policy}
	config = seedLegacyPortFixture(t, WafPipelineServiceApp, db, config)
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
	if err := s.Punisher.Flush(); err != nil {
		t.Fatal(err)
	}
	loaded := &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if loaded.Punisher.BanReason(port, netip.MustParseAddr("198.51.100.7")) == "" || loaded.Punisher.BanReason("port:45127", netip.MustParseAddr("198.51.100.7")) != "" {
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
