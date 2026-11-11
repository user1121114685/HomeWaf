package waf_service

import (
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
)

func legacyMigrationFixture(t *testing.T) (*WafPipelineService, *gorm.DB) {
	t.Helper()
	if !wafsec.DataKeyReady() {
		if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "migrate.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { conn.Close() })
	if err = db.AutoMigrate(&model.Hosts{}, &model.PortConfig{}, &model.PipelinePortMigration{}, &model.GateUser{}, &model.GateSession{}, &model.HttpAuthBase{}, &model.IPAllowList{}, &model.IPBlockList{}, &model.AntiCC{}, &model.IPGroupItem{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldLog, oldPorts, oldGate := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, nil
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	WafPipelineGateServiceApp = &WafPipelineGateService{Store: wafpipeline.NewGateStore()}
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, WafPipelineGateServiceApp = oldDB, oldLog, oldPorts, oldGate
		wafpipeline.SetLegacyPortOwner(5300, false)
	})
	s := &WafPipelineService{Prepare: (&wafpipeline.Runtime{}).Prepare, OnChange: func(old, next *wafpipeline.Snapshot) error { return nil }}
	for _, h := range []model.Hosts{{BaseOrm: pipelineORM(), Code: "nas", Host: "nas.test", Port: 5300, Remote_host: "http://nas-origin.test", Remote_ip: "127.0.0.1", Remote_port: 8080, GUARD_STATUS: 1, IsEnableHttpAuthBase: 1}, {BaseOrm: pipelineORM(), Code: "camera", Host: "camera.test", Port: 5300, Remote_host: "http://camera-origin.test", Remote_ip: "127.0.0.2", Remote_port: 8081, GUARD_STATUS: 1}} {
		if err = db.Create(&h).Error; err != nil {
			t.Fatal(err)
		}
	}
	hash, err := utils.BcryptHash("correct-home-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&model.HttpAuthBase{BaseOrm: pipelineORM(), HostCode: "nas", UserName: "owner", Password: hash, PasswordHashed: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&model.IPAllowList{BaseOrm: pipelineORM(), HostCode: "nas", Ip: "192.168.5.0/24", IsEnable: 1}).Error; err != nil {
		t.Fatal(err)
	}
	return s, db
}

func TestLegacyPortMigrationCopiesRealConfigurationAndPreservesSources(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	var before []model.Hosts
	db.Order("code").Find(&before)
	beforeJSON, _ := json.Marshal(before)
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blockers) > 0 || len(p.Config.Routes) != 2 || len(p.Config.Policy.IPGroup.Entries) != 1 || len(p.AccountNames) != 1 || !p.Config.Policy.Gate.Enabled {
		t.Fatalf("incomplete conversion: %+v", p)
	}
	public, _ := json.Marshal(p)
	if strings.Contains(string(public), "correct-home-password") || strings.Contains(string(public), "$2a$") || strings.Contains(string(public), "private_key") {
		t.Fatal("preview exposed secret material")
	}
	var count int64
	db.Model(&model.PortConfig{}).Count(&count)
	if count != 0 {
		t.Fatal("preview wrote port config")
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenEnabled || len(c.Routes) != 2 || wafpipeline.OwnsLegacyPort(5300) {
		t.Fatal("import started or took over listener")
	}
	var stored model.PortConfig
	if err = db.First(&stored, "port = ?", 5300).Error; err != nil {
		t.Fatal(err)
	}
	decoded, err := decodePort(stored)
	if err != nil || decoded.Secrets.GateKey == "" {
		t.Fatal("new secrets/config not persisted", err)
	}
	var accounts []model.GateUser
	db.Where("port_id = ?", "port:5300").Find(&accounts)
	if len(accounts) != 1 || accounts[0].PasswordHash == "" {
		t.Fatal("legacy account was not migrated")
	}
	var after []model.Hosts
	db.Order("code").Find(&after)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatal("migration rewrote original host data")
	}
	if _, err = s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err == nil {
		t.Fatal("repeat import overwrote configured port")
	}
}

func TestLegacyMigrationRejectsStalePreviewAndRollsBackFailedActivation(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&model.Hosts{}).Where("code = ?", "nas").Update("remote_port", 9090).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err == nil {
		t.Fatal("stale source preview accepted")
	}
	p, err = s.PreviewLegacyPort(5300, "nas")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	s.OnChange = func(old, next *wafpipeline.Snapshot) error {
		if !wafpipeline.OwnsLegacyPort(5300) {
			t.Error("legacy source was not fenced before native activation")
		}
		return errors.New("fixture bind failure")
	}
	c.ListenEnabled = true
	if _, err = s.Save(PortSaveRequest{Config: c, Operator: "tester"}); err == nil {
		t.Fatal("failed activation accepted")
	}
	if wafpipeline.OwnsLegacyPort(5300) {
		t.Fatal("failed activation kept ownership")
	}
	var marker model.PipelinePortMigration
	db.First(&marker, "port = ?", 5300)
	if marker.State != "imported" {
		t.Fatal("failed activation kept active marker")
	}
	c, err = s.Get(5300)
	if err != nil || c.ListenEnabled || c.Revision != 1 {
		t.Fatal("failed activation changed saved policy", err)
	}
	s.OnChange = func(old, next *wafpipeline.Snapshot) error { return nil }
	c.ListenEnabled = true
	c, err = s.Save(PortSaveRequest{Config: c, Operator: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if !wafpipeline.OwnsLegacyPort(5300) {
		t.Fatal("successful activation did not retain ownership")
	}
	c.ListenEnabled = false
	c, err = s.Save(PortSaveRequest{Config: c, Operator: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(5300, "tester"); err != nil {
		t.Fatal(err)
	}
	wafpipeline.SetLegacyPortOwner(5300, false)
	if err = s.LoadLegacyPortOwnership(); err != nil {
		t.Fatal(err)
	}
	if !wafpipeline.OwnsLegacyPort(5300) {
		t.Fatal("deleting native config would resurrect old listener on restart")
	}
}

func TestPortPublishFailureRollsBackDatabaseAndInitialAccount(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](1)
	s.Prepare = func(snap *wafpipeline.Snapshot) error {
		if err := (&wafpipeline.Runtime{}).Prepare(snap); err != nil {
			return err
		}
		return wafpipeline.Ports.Publish(5400, func() (*wafpipeline.Snapshot, error) { return &wafpipeline.Snapshot{}, nil })
	}
	c := wafpipeline.PortConfig{Schema: 1, Port: 5301, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.Enabled = true
	if _, err := s.Save(PortSaveRequest{Config: c, Username: "owner", Password: "correct-home-password"}); err == nil {
		t.Fatal("registry quota failure accepted")
	}
	var ports, users int64
	db.Model(&model.PortConfig{}).Where("port = ?", 5301).Count(&ports)
	db.Model(&model.GateUser{}).Where("port_id = ?", wafpipeline.PortID(5301)).Count(&users)
	if ports != 0 || users != 0 {
		t.Fatalf("failed publication left persisted rows ports=%d users=%d", ports, users)
	}
}

func TestFailedPortSaveNeverRollsBackNewerRevision(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	s.OnChange = func(old, next *wafpipeline.Snapshot) error {
		newer := &WafPipelineService{Prepare: (&wafpipeline.Runtime{}).Prepare, OnChange: func(old, next *wafpipeline.Snapshot) error { return nil }}
		config := wafpipeline.CloneConfig(next.Config)
		config.Name = "newer-request"
		if _, err := newer.Save(PortSaveRequest{Config: config}); err != nil {
			t.Fatal(err)
		}
		return errors.New("older listener update failed")
	}
	c.Name = "older-request"
	if _, err = s.Save(PortSaveRequest{Config: c}); err == nil {
		t.Fatal("failed save accepted")
	}
	var row model.PortConfig
	if err = db.First(&row, "port = ?", 5300).Error; err != nil {
		t.Fatal(err)
	}
	stored, err := decodePort(row)
	if err != nil || stored.Config.Name != "newer-request" || stored.Config.Revision != 3 {
		t.Fatal("newer database revision overwritten", err)
	}
	live, ok := wafpipeline.Ports.Get(5300)
	if !ok || live.Config.Name != "newer-request" || live.Config.Revision != 3 {
		t.Fatal("newer live revision overwritten")
	}
}

func TestLegacyMigrationCopiesExecutableCustomRulesAndURLLists(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.Rules{}, &model.URLAllowList{}, &model.URLBlockList{}, &model.IPGroup{}); err != nil {
		t.Fatal(err)
	}
	rule := model.Rules{BaseOrm: pipelineORM(), HostCode: "camera", RuleCode: "family-custom", RuleName: "FamilyRule", RuleStatus: 1, RuleContent: `rule RFamily "" salience 10 { when RF.IPInGroup(MF.SRC_IP,"family") && MF.GetHeaderValue("X-Family-Test") == "block" then RF.Deny(); }`}
	for _, value := range []any{&rule, &model.IPGroup{BaseOrm: pipelineORM(), GroupCode: "group-family", GroupName: "family"}, &model.IPGroupItem{BaseOrm: pipelineORM(), GroupCode: "group-family", Ip: "198.51.100.0/24"}, &model.URLBlockList{BaseOrm: pipelineORM(), HostCode: "camera", CompareType: "等于", Url: "/secret", IsEnable: 1}, &model.URLAllowList{BaseOrm: pipelineORM(), HostCode: "camera", CompareType: "前缀匹配", Url: "/notes", IsEnable: 1}} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal("custom migration failed", err, p.Blockers)
	}
	if len(p.Config.Policy.RuleSet.Scripts) != 1 || len(p.Config.Policy.RuleSet.URLs) != 2 || len(p.Config.ScriptGroups["family"]) != 1 {
		t.Fatal("custom policy not copied")
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "camera", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Rules{}).Where("rule_code = ?", rule.RuleCode).Update("rule_content", `rule RFamily "" { when true then RF.AllowAll(); }`).Error; err != nil {
		t.Fatal(err)
	}
	runtime := &wafpipeline.Runtime{}
	for _, path := range []string{"/secret", "/ordinary"} {
		req := httptest.NewRequest("GET", "http://camera.test"+path, nil)
		req.Header.Set("X-Family-Test", "block")
		result, err := runtime.Explain(&wafpipeline.Snapshot{Config: c}, req, netip.MustParseAddr("198.51.100.25"))
		if err != nil || result.Verdict.Action != wafpipeline.Deny {
			t.Fatal("saved custom policy lost executable behavior", err, result)
		}
	}
}

func TestLegacyMigrationCopiesPathRoutesAndStaticBusiness(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.HostPathRule{}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, rule := range []model.HostPathRule{
		{BaseOrm: pipelineORM(), HostCode: "camera", RuleName: "api", Path: "/api/", MatchType: 1, TargetType: 1, Priority: 20, StripPrefix: 1, RemoteScheme: "https", RemoteHost: "api-origin.test", RemoteIP: "127.0.0.3", RemotePort: 8443},
		{BaseOrm: pipelineORM(), HostCode: "camera", RuleName: "static", Path: "/files/", MatchType: 1, TargetType: 2, Priority: 10, StripPrefix: 1, StaticRoot: root, SpaFallback: 1},
		{BaseOrm: pipelineORM(), HostCode: "camera", RuleName: "redirect", Path: "/old", MatchType: 2, TargetType: 3, Priority: 5, RedirectURL: "/new", RedirectCode: 308},
	} {
		if err := db.Create(&rule).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal("business migration blocked", err, p.Blockers)
	}
	if len(p.Config.Routes) != 5 {
		t.Fatalf("path business missing: %d", len(p.Config.Routes))
	}
	seen := map[string]wafpipeline.UpstreamRoute{}
	for _, route := range p.Config.Routes {
		seen[route.PathPrefix] = route
	}
	if !seen["/api/"].StripPrefix || seen["/api/"].Upstream != "https://api-origin.test:8443" || seen["/files/"].Static == nil || seen["/files/"].Static.Settings.StaticSitePath != root || !seen["/files/"].Static.SPAFallback || seen["/old"].RedirectCode != 308 {
		t.Fatal("business route settings lost")
	}
	if err := db.Model(&model.HostPathRule{}).Where("host_code = ? AND path = ?", "camera", "/old").Update("redirect_url", "/changed").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "camera", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err == nil {
		t.Fatal("path change did not invalidate migration preview")
	}
}

func TestLegacyBusinessHeadersAreSealedAndUpdatesDoNotMutateSnapshots(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	secret := "fixture-upstream-private-token"
	profile := model.CustomHeadersConfig{IsEnableCustomHeaders: 1, Headers: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: "Bearer " + secret}}}
	raw, _ := json.Marshal(profile)
	if err := db.Model(&model.Hosts{}).Where("code = ?", "nas").Update("custom_headers_json", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	public, _ := json.Marshal(p)
	if strings.Contains(string(public), secret) {
		t.Fatal("preview leaked upstream credential")
	}
	c, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	var stored, encrypted string
	if err := db.Raw("SELECT config_json FROM port_config WHERE port = ?", 5300).Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", 5300).Scan(&encrypted).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) || strings.Contains(encrypted, secret) {
		t.Fatal("business credential stored in plaintext")
	}
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	old, ok := wafpipeline.Ports.Get(5300)
	if !ok {
		t.Fatal("missing saved snapshot")
	}
	changed := map[string]wafpipeline.BusinessHeaders{"legacy-nas": {Request: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: "Bearer replacement-fixture"}}}}
	if _, err = s.Save(PortSaveRequest{Config: c, BusinessHeaders: changed}); err != nil {
		t.Fatal(err)
	}
	if old.Secrets.BusinessHeaders["legacy-nas"].Request[0].HeaderValue != "Bearer "+secret {
		t.Fatal("updating secret headers mutated published old snapshot")
	}
	copy, err := s.Copy(5300, 5350, false)
	if err != nil {
		t.Fatal(err)
	}
	if copy.HeaderProfile != "" {
		t.Fatal("copy retained secret header reference")
	}
	for _, route := range copy.Routes {
		if route.HeaderProfile != "" {
			t.Fatal("route copy retained credentials")
		}
	}
}

func TestLegacyMigrationCopiesWeightedBusinessBackends(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.LoadBalance{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Updates(map[string]any{"is_enable_load_balance": 1, "load_balance_stage": 2}).Error; err != nil {
		t.Fatal(err)
	}
	for _, backend := range []model.LoadBalance{{BaseOrm: pipelineORM(), HostCode: "camera", Remote_ip: "127.0.0.3", Remote_port: 8100, Weight: 3}, {BaseOrm: pipelineORM(), HostCode: "camera", Remote_ip: "127.0.0.4", Remote_port: 8200, Weight: 1}} {
		if err := db.Create(&backend).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	found := false
	for _, route := range p.Config.Routes {
		if len(route.Backends) > 0 {
			found = true
			if route.Balance != "ip_hash" || len(route.Backends) != 2 {
				t.Fatal("load balancing config not migrated")
			}
			if route.Backends[0].DialIP != "127.0.0.3" && route.Backends[1].DialIP != "127.0.0.3" {
				t.Fatal("fixed target IP lost")
			}
		}
	}
	if !found {
		t.Fatal("missing business backends")
	}
}

func TestLegacyMigrationCopiesCacheRulesWithoutDiskData(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.CacheRule{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Update("cache_json", `{"is_enable_cache":1,"cache_location":"file","cache_dir":"do-not-read-fixture"}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CacheRule{BaseOrm: pipelineORM(), HostCode: "camera", RuleType: 1, RuleContent: ".jpg;.png", ParamType: 2, CacheTime: 60, Priority: 10, RequestMethod: "GET;HEAD"}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	found := false
	for _, route := range p.Config.Routes {
		if route.Cache != nil {
			found = true
			if len(route.Cache.Rules) != 1 || route.Cache.Rules[0].CacheTime != 60 {
				t.Fatal("cache rules not copied")
			}
		}
	}
	if !found {
		t.Fatal("missing cache policy")
	}
	public, _ := json.Marshal(p.Config)
	if strings.Contains(string(public), "do-not-read-fixture") {
		t.Fatal("old disk cache path retained as active store")
	}
}

func TestLegacyEditorCannotClaimNativePortsOrEditHandedOffSource(t *testing.T) {
	s, _ := legacyMigrationFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 5350, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.Enabled = false
	if _, err := s.Save(PortSaveRequest{Config: c}); err != nil {
		t.Fatal(err)
	}
	if err := guardNativePortOwnership(model.Hosts{Port: 5350}, nil); err == nil {
		t.Fatal("legacy site could occupy native configured port")
	}
	wafpipeline.SetLegacyPortOwner(5300, true)
	source := model.Hosts{Port: 5300}
	if err := guardNativePortOwnership(source, &source); err == nil {
		t.Fatal("fully handed-off source remained an active editor")
	}
	source.BindMorePort = "5301"
	if err := guardNativePortOwnership(source, &source); err != nil {
		t.Fatal("partial migration blocked remaining legacy port", err)
	}
	newBinding := source
	newBinding.BindMorePort = "5301,5350"
	if err := guardNativePortOwnership(newBinding, &source); err == nil {
		t.Fatal("legacy update added a native listener")
	}
}

func TestMigrationSnapshotsLocalAIModeAndDetectsGlobalSourceChanges(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	oldEnable, oldMode := global.CurrentSettings().GCONFIG_AI_ENABLE, global.CurrentSettings().GCONFIG_AI_MODE
	defer func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_AI_ENABLE, cfg.GCONFIG_AI_MODE = oldEnable, oldMode })
	}()
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_AI_ENABLE, cfg.GCONFIG_AI_MODE = 1, "block" })
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Update("defense_json", `{"ai":1}`).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || p.Config.Policy.RuleSet.AI != wafpipeline.ModeBlock {
		t.Fatal("AI policy not copied", err)
	}
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_AI_MODE = "observe" })
	if _, err = s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "camera", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err == nil {
		t.Fatal("global source policy changed without invalidating preview")
	}
}
