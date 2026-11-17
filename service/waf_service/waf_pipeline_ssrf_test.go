package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNASTemplateSSRFDefaultPersistsAndCanBeReenabled(t *testing.T) {
	defense, db, _ := defenseServiceFixture(t)
	if err := db.AutoMigrate(&model.ConfigTemplate{}); err != nil {
		t.Fatal(err)
	}
	service := defense.ports()
	engine := wafpipeline.DefaultRuleEngine()
	forwarded := 0
	var last wafpipeline.Result
	live := &wafpipeline.Runtime{
		RuleSteps: engine.Steps,
		Forward: func(_ *wafpipeline.Snapshot, _ *wafpipeline.RequestContext, w http.ResponseWriter) {
			forwarded++
			w.WriteHeader(http.StatusNoContent)
		},
		OnResult: func(_ *wafpipeline.Snapshot, _ *wafpipeline.RequestContext, result wafpipeline.Result) { last = result },
	}
	service.Prepare = live.Prepare
	ruleMode := func(config wafpipeline.PortConfig) wafpipeline.Mode {
		t.Helper()
		for _, rule := range config.Policy.RuleSet.Rules {
			if rule.Category == "ssrf" {
				return rule.Mode
			}
		}
		t.Fatal("SSRF rule missing")
		return ""
	}
	for i, id := range []string{"family-nas", "public-site"} {
		tpl, err := service.Template(id)
		if err != nil {
			t.Fatal(err)
		}
		base := wafpipeline.PortConfig{Schema: 1, Port: uint16(45231 + i), Name: id, Protocol: "http", Upstream: "http://127.0.0.1:1", Policy: newServicePolicyFixture()}
		// Exercise the canonical ruleset apply/save path without requiring a login gate.
		config, err := wafpipeline.ApplyTemplate(base, tpl, []string{"ruleset"})
		if err != nil {
			t.Fatal(err)
		}
		wantMode := wafpipeline.ModeOff
		if id == "public-site" {
			wantMode = wafpipeline.ModeObserve
		}
		if ruleMode(config) != wantMode {
			t.Fatalf("%s template default changed", id)
		}
		if _, err = service.Save(PortSaveRequest{Config: config}); err != nil {
			t.Fatal(err)
		}
		var stored model.PortConfig
		if err = db.Where("port = ?", config.Port).First(&stored).Error; err != nil {
			t.Fatal(err)
		}
		var persisted wafpipeline.PortConfig
		if err = json.Unmarshal([]byte(stored.ConfigJSON), &persisted); err != nil {
			t.Fatal(err)
		}
		if ruleMode(persisted) != wantMode || persisted.Template.Version != tpl.Version {
			t.Fatalf("template policy/version lost on save: %+v", persisted.Template)
		}
	}
	if err := service.Load(); err != nil {
		t.Fatal(err)
	}
	check := func(port uint16, status, hitCount int) {
		t.Helper()
		snapshot, ok := wafpipeline.Ports.Get(port)
		if !ok {
			t.Fatal("saved service has no runtime snapshot")
		}
		r := httptest.NewRequest("GET", "http://example.test/photos", nil)
		r.Header.Set("Referer", "http://169.254.169.254/latest/meta-data/")
		r.Header.Set("User-Agent", "iOS-Synology-Photos-iPhone")
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}))
		response := httptest.NewRecorder()
		before := forwarded
		live.Serve(snapshot, response, r)
		if response.Code != status || len(last.Hits) != hitCount {
			t.Fatalf("port=%d status=%d hits=%+v", port, response.Code, last.Hits)
		}
		if status == http.StatusForbidden && forwarded != before {
			t.Fatal("blocked SSRF reached upstream handler")
		}
		if hitCount > 0 && (last.Hits[0].Category != "ssrf" || last.Hits[0].Observe != (status == http.StatusNoContent)) {
			t.Fatalf("incorrect SSRF classification: %+v", last.Hits)
		}
	}
	check(45231, http.StatusNoContent, 0)
	check(45232, http.StatusNoContent, 1)
	nas, err := service.Get(45231)
	if err != nil {
		t.Fatal(err)
	}
	for i := range nas.Policy.RuleSet.Rules {
		if nas.Policy.RuleSet.Rules[i].Category == "ssrf" {
			nas.Policy.RuleSet.Rules[i].Mode = wafpipeline.ModeBlock
		}
	}
	if _, err := service.Save(PortSaveRequest{Config: nas}); err != nil {
		t.Fatal(err)
	}
	if err := service.Load(); err != nil {
		t.Fatal(err)
	}
	check(45231, http.StatusForbidden, 1)
	check(45232, http.StatusNoContent, 1)
}
