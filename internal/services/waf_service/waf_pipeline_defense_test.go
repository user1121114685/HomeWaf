package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/wafsec"
	"HomeWaf/internal/protection/wafpipeline"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/internal/storage/wafdb"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newServicePolicyFixture() wafpipeline.Policy {
	p := wafpipeline.DefaultPolicy()
	d := wafpipeline.DefaultDefense()
	d.Enabled = true
	p.Defense = &d
	return p
}

// Older stored configurations remain loadable, but may no longer be created
// through the service onboarding save path.
func seedLegacyPortFixture(t *testing.T, s *WafPipelineService, db *gorm.DB, c wafpipeline.PortConfig) wafpipeline.PortConfig {
	t.Helper()
	c.Revision = 1
	c.HostCode = "pipeline-" + fmt.Sprint(c.Port)
	key, err := pipelineSecret()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := json.Marshal(wafpipeline.PortSecrets{GateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.PortConfig{BaseOrm: pipelineORM(), Port: int(c.Port), Revision: 1, ConfigJSON: string(raw), SecretsJSON: string(secrets)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewServiceRequiresEnabledDefenseBeforePersistence(t *testing.T) {
	s, db, _ := defenseServiceFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 45131, Protocol: "http", Upstream: "http://127.0.0.1:1", Policy: wafpipeline.DefaultPolicy()}
	for _, kind := range []string{"missing", "disabled", "invalid-mode", "invalid-threshold", "unready-honeypot", "proxied-quiet"} {
		t.Run(kind, func(t *testing.T) {
			candidate := wafpipeline.CloneConfig(c)
			if kind != "missing" {
				d := wafpipeline.DefaultDefense()
				d.Enabled = kind != "disabled"
				if kind == "invalid-mode" {
					d.Mode = ""
				}
				if kind == "invalid-threshold" {
					d.Count = 0
				}
				if kind == "unready-honeypot" {
					d.Mode = wafpipeline.DefenseHoney
				}
				if kind == "proxied-quiet" {
					candidate.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
				}
				candidate.Policy.Defense = &d
			}
			// Forging a revision must not bypass creation validation.
			candidate.Revision = 8
			if _, err := s.ports().Save(PortSaveRequest{Config: candidate}); err == nil {
				t.Fatal("unconfigured service was accepted")
			}
			var count int64
			if err := db.Model(&model.PortConfig{}).Where("port = ?", c.Port).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("invalid onboarding persisted: %d %v", count, err)
			}
			if _, exists := wafpipeline.Ports.Get(c.Port); exists {
				t.Fatal("invalid onboarding published runtime")
			}
		})
	}
	c.Policy = newServicePolicyFixture()
	c.Policy.Defense.Mode = wafpipeline.DefenseClose
	c.Policy.Defense.Count = 2
	saved, err := s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ListenEnabled {
		t.Fatal("new service started listening")
	}
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	if err := s.ports().Load(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.ports().Get(c.Port)
	if err != nil || reloaded.Policy.Defense == nil || *reloaded.Policy.Defense != *c.Policy.Defense {
		t.Fatalf("onboarding policy did not reload: %+v %v", reloaded.Policy.Defense, err)
	}
	reloaded.ListenEnabled = true
	reloaded, err = s.ports().Save(PortSaveRequest{Config: reloaded})
	if err != nil {
		t.Fatal(err)
	}
	active := triggerDefense(t, s, reloaded)
	if active.Policy.Access.Enabled {
		t.Fatal("configured onboarding defense did not close access")
	}
}

func TestDefenseConfigurationDoesNotRequireMigrationConfirmation(t *testing.T) {
	s, db, _ := defenseServiceFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 45132, Protocol: "http", Upstream: "http://127.0.0.1:1", Policy: wafpipeline.DefaultPolicy()}
	c = seedLegacyPortFixture(t, s.ports(), db, c)
	d := wafpipeline.DefaultDefense()
	d.Enabled = true
	c.Policy.Defense = &d
	saved, err := s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Policy.Defense == nil {
		t.Fatal("defense policy was not persisted")
	}
	saved.Policy.Defense = nil
	if _, err := s.ports().Save(PortSaveRequest{Config: saved}); err == nil {
		t.Fatalf("defense removal was accepted: %v", err)
	}
}

func defenseServiceFixture(t *testing.T) (*WafPipelineDefenseService, *gorm.DB, *time.Time) {
	t.Helper()
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "defense.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.Hosts{}, &model.PipelineSuccessSource{}, &model.PipelineHoneypot{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLogs, oldPorts, oldService, oldDefense := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineServiceApp, WafPipelineDefenseServiceApp
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, nil
	if err := wafdb.RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	now := time.Now()
	e := wafpipeline.NewDefenseEngine()
	e.Now = func() time.Time { return now }
	live := &wafpipeline.Runtime{Defense: e}
	service := &WafPipelineService{Prepare: live.Prepare, OnChange: func(old, next *wafpipeline.Snapshot) error { return nil }}
	s := &WafPipelineDefenseService{Engine: e, Ports: service}
	WafPipelineServiceApp, WafPipelineDefenseServiceApp = service, s
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = oldDB, oldLogs
		wafpipeline.Ports = oldPorts
		WafPipelineServiceApp, WafPipelineDefenseServiceApp = oldService, oldDefense
		wafpipeline.PublishSharedHoneypot(nil)
	})
	if err := s.LoadHistory(); err != nil {
		t.Fatal(err)
	}
	return s, db, &now
}

func saveDefenseService(t *testing.T, s *WafPipelineDefenseService, port uint16, mode string) wafpipeline.PortConfig {
	t.Helper()
	p := wafpipeline.DefaultPolicy()
	d := wafpipeline.DefaultDefense()
	d.Enabled = true
	d.Mode = mode
	d.Count = 2
	p.Defense = &d
	c, err := s.ports().Save(PortSaveRequest{Config: wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: "http://127.0.0.1:1", Policy: p}})
	if err != nil {
		t.Fatal(err)
	}
	c.ListenEnabled = true
	c, err = s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func triggerDefense(t *testing.T, s *WafPipelineDefenseService, c wafpipeline.PortConfig) wafpipeline.PortConfig {
	t.Helper()
	for _, ip := range []string{"198.51.100.1", "203.0.113.1"} {
		ctx := wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://fixture/", nil), wafpipeline.PortID(c.Port), netip.MustParseAddr(ip), wafpipeline.DefaultViewLimits())
		s.Engine.Observe(&wafpipeline.Snapshot{Config: c}, ctx, wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, Reason: "blacklist:custom"})
	}
	s.Tick()
	current, err := s.ports().Get(c.Port)
	if err != nil {
		t.Fatal(err)
	}
	if current.DefenseState == nil {
		t.Fatal("defense state did not persist")
	}
	return current
}

func TestDefenseAutomaticCloseRestoreAndManualOwnership(t *testing.T) {
	s, _, now := defenseServiceFixture(t)
	a := saveDefenseService(t, s, 45120, wafpipeline.DefenseClose)
	b := saveDefenseService(t, s, 45121, wafpipeline.DefenseQuiet)
	a = triggerDefense(t, s, a)
	if a.Policy.Access.Enabled || a.DefenseState.ExpiresAt != now.Unix()+10800 {
		t.Fatal("service was not closed for three hours")
	}
	other, _ := s.ports().Get(b.Port)
	if !other.Policy.Access.Enabled || other.Revision != b.Revision {
		t.Fatal("another port changed")
	}
	// A new engine sees the persisted closure; an unrelated edit preserves its owner.
	restarted := &WafPipelineDefenseService{Engine: wafpipeline.NewDefenseEngine(), Ports: s.ports()}
	restarted.Engine.Now = s.Engine.Now
	a.Name = "renamed while closed"
	saved, err := s.ports().Save(PortSaveRequest{Config: a})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(3 * time.Hour)
	restarted.Tick()
	a, _ = s.ports().Get(a.Port)
	if !a.Policy.Access.Enabled || a.DefenseState != nil || a.Name != saved.Name {
		t.Fatal("restart recovery overwrote config or failed to reopen")
	}
	a = triggerDefense(t, s, a)
	_, err = s.Access(DefenseAccessRequest{Port: a.Port, Revision: a.Revision, Action: "close"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(4 * time.Hour)
	s.Tick()
	a, _ = s.ports().Get(a.Port)
	if a.Policy.Access.Enabled || a.DefenseState != nil {
		t.Fatal("automatic timer overrode manual close")
	}
}

func TestDefenseVersionConflictForgeryAndEmptyHistory(t *testing.T) {
	s, _, _ := defenseServiceFixture(t)
	c := saveDefenseService(t, s, 45122, wafpipeline.DefenseRestrict)
	c.DefenseState = &wafpipeline.DefenseState{ID: "forged", Mode: wafpipeline.DefenseClose, AccessWasEnabled: true}
	saved, err := s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefenseState != nil {
		t.Fatal("client forged runtime state")
	}
	active := triggerDefense(t, s, saved)
	if active.DefenseState.Reason != "no_success_history" || active.Policy.Access.Enabled {
		t.Fatal("empty history did not use access switch")
	}
	_, err = s.Access(DefenseAccessRequest{Port: active.Port, Revision: saved.Revision, StateID: active.DefenseState.ID, Action: "release"}, "operator")
	if err == nil {
		t.Fatal("stale write reopened the service")
	}
	_, err = s.Access(DefenseAccessRequest{Port: active.Port, Revision: active.Revision, StateID: "another-event", Action: "release"}, "operator")
	if err == nil {
		t.Fatal("wrong incident released")
	}
}

func TestDefenseHistoryAndSharedHoneypotPersistIndependently(t *testing.T) {
	s, _, _ := defenseServiceFixture(t)
	c := saveDefenseService(t, s, 45123, wafpipeline.DefenseRestrict)
	ctx := wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://fixture/", nil), wafpipeline.PortID(c.Port), netip.MustParseAddr("198.51.100.2"), wafpipeline.DefaultViewLimits())
	s.Engine.Observe(&wafpipeline.Snapshot{Config: c}, ctx, wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 200, Reason: "upstream_response"})
	if err := s.PersistHistory(); err != nil {
		t.Fatal(err)
	}
	restored := &WafPipelineDefenseService{Engine: wafpipeline.NewDefenseEngine(), Ports: s.ports()}
	if err := restored.LoadHistory(); err != nil {
		t.Fatal(err)
	}
	if restored.Engine.Status(&wafpipeline.Snapshot{Config: c}).SuccessPrefixes != 1 {
		t.Fatal("history did not survive restart")
	}
	honey := &WafPipelineHoneypotService{}
	shared, err := honey.Get()
	if err != nil || shared.Configured {
		t.Fatal(shared, err)
	}
	shared.Config.Lures = []wafpipeline.HoneypotLure{{ID: "panel", Title: "缁熶竴鎻愮ず", Message: "鍥哄畾鏂囨湰", Weight: 1}}
	saved, err := honey.Save(shared, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := honey.Save(shared, "stale"); err == nil {
		t.Fatal("stale shared configuration overwrote data")
	}
	if err := honey.Load(); err != nil {
		t.Fatal(err)
	}
	c.Policy.Defense.Mode = wafpipeline.DefenseHoney
	c, err = s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	effective, ready := wafpipeline.HoneypotPolicy(&wafpipeline.Snapshot{Config: c})
	if !ready || !wafpipeline.SharedHoneypotReady() || effective.Lures[0].Title != saved.Config.Lures[0].Title {
		t.Fatal("shared policy was not consumed")
	}
}

func TestDefenseCloseReachesLiveListenerAndRecovers(t *testing.T) {
	s, _, now := defenseServiceFixture(t)
	mgr := &wafpipeline.ListenerManager{}
	live := &wafpipeline.Runtime{Defense: s.Engine, Forward: func(_ *wafpipeline.Snapshot, c *wafpipeline.RequestContext, w http.ResponseWriter) {
		c.Values["response_verdict"] = wafpipeline.Verdict{Action: wafpipeline.Allow, Stage: 5, Status: 204, Reason: "upstream_response"}
		w.WriteHeader(204)
	}, RuleSteps: func(_ *wafpipeline.Snapshot) []wafpipeline.Step {
		return []wafpipeline.Step{{Number: 3, Name: "fixture", Inspect: func(c *wafpipeline.RequestContext) (wafpipeline.Verdict, error) {
			if c.Request.URL.Path == "/blocked" {
				c.Hit("scanner", "fixture", wafpipeline.High, false, "test denial")
				return wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, Reason: "fixture"}, nil
			}
			return wafpipeline.Verdict{Action: wafpipeline.Continue}, nil
		}}}
	}}
	mgr.SetHandler(live.Serve)
	s.ports().Prepare = live.Prepare
	s.ports().OnChange = mgr.Apply
	t.Cleanup(func() { mgr.StopAll() })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	c := saveDefenseService(t, s, port, wafpipeline.DefenseClose)
	client := &http.Client{Timeout: 3 * time.Second}
	t.Cleanup(func() { client.CloseIdleConnections() })
	get := func(path string) int {
		t.Helper()
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if get("/blocked") != 403 || get("/blocked") != 403 {
		t.Fatal("baseline rejection failed")
	}
	s.Tick()
	if get("/") != 503 {
		t.Fatal("persisted closure did not reach the actual listener")
	}
	closed, _ := s.ports().Get(c.Port)
	if closed.Policy.Access.Enabled || closed.DefenseState == nil {
		t.Fatal("closure was not persisted")
	}
	*now = now.Add(3*time.Hour + time.Second)
	s.Tick()
	if get("/") != 204 {
		t.Fatal("timer did not restore actual access")
	}
}

func TestDefenseWorkerOwnershipAndDeletedServiceHistory(t *testing.T) {
	s, db, _ := defenseServiceFixture(t)
	if err := db.AutoMigrate(&model.GateRuntime{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GateRuntime{ID: 1, Owner: "new-worker"}).Error; err != nil {
		t.Fatal(err)
	}
	c := saveDefenseService(t, s, 45128, wafpipeline.DefenseClose)
	s.Owner = "old-worker"
	for _, ip := range []string{"192.0.2.1", "192.0.2.2"} {
		ctx := wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://fixture/", nil), wafpipeline.PortID(c.Port), netip.MustParseAddr(ip), wafpipeline.DefaultViewLimits())
		s.Engine.Observe(&wafpipeline.Snapshot{Config: c}, ctx, wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, Reason: "blacklist:custom"})
	}
	if err := s.Activate(s.Engine.Pending()[0]); err == nil {
		t.Fatal("old worker committed after ownership handoff")
	}
	current, _ := s.ports().Get(c.Port)
	if !current.Policy.Access.Enabled {
		t.Fatal("old worker closed the service")
	}
	s.Owner = "new-worker"
	ctx := wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://fixture/", nil), wafpipeline.PortID(c.Port), netip.MustParseAddr("198.51.100.1"), wafpipeline.DefaultViewLimits())
	s.Engine.Observe(&wafpipeline.Snapshot{Config: c}, ctx, wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 200, Reason: "upstream_response"})
	if err := s.PersistHistory(); err != nil {
		t.Fatal(err)
	}
	c.ListenEnabled = false
	c, err := s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ports().Delete(c.Port, "operator"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.PipelineSuccessSource{}).Where("port = ?", c.Port).Count(&count)
	if count != 0 || s.Engine.Status(&wafpipeline.Snapshot{Config: c}).SuccessPrefixes != 0 {
		t.Fatal("deleted service history leaked into reusable port identity")
	}
}

func TestDefensePolicyEditsPreserveAutomaticReopen(t *testing.T) {
	s, _, now := defenseServiceFixture(t)
	c := triggerDefense(t, s, saveDefenseService(t, s, 45129, wafpipeline.DefenseClose))
	id, deadline := c.DefenseState.ID, c.DefenseState.ExpiresAt
	c.Policy.Defense.Count, c.Policy.Defense.DurationSeconds = 20, 6*3600
	c.Policy.Defense.Mode = wafpipeline.DefenseQuiet
	c, err := s.ports().Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if c.Policy.Access.Enabled || c.DefenseState == nil || c.DefenseState.ID != id || c.DefenseState.ExpiresAt != deadline {
		t.Fatal("editing the policy lost or extended the automatic reopen")
	}
	*now = time.Unix(deadline+1, 0)
	s.Tick()
	c, _ = s.ports().Get(c.Port)
	if !c.Policy.Access.Enabled || c.DefenseState != nil || c.Policy.Defense.Mode != wafpipeline.DefenseQuiet {
		t.Fatal("new policy did not resume after the original closure")
	}
}

func TestDefenseTemplateOwnership(t *testing.T) {
	s, _, _ := defenseServiceFixture(t)
	c := saveDefenseService(t, s, 45130, wafpipeline.DefenseQuiet)
	template := wafpipeline.Template{Schema: 1, ID: "user-defense-test", Name: "Defense template", Version: 1, Policy: wafpipeline.DefaultPolicy()}
	updated, err := wafpipeline.ApplyTemplate(c, template, []string{"hit_policy", "defense"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Policy.Defense == nil {
		t.Fatal("template lost defense policy")
	}
	if _, err := s.ports().Save(PortSaveRequest{Config: updated}); err != nil {
		t.Fatal(err)
	}
}
