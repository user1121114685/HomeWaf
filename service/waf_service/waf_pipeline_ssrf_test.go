package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	check := func(port uint16, r *http.Request, status int, category string) {
		t.Helper()
		snapshot, ok := wafpipeline.Ports.Get(port)
		if !ok {
			t.Fatal("saved service has no runtime snapshot")
		}
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}))
		response := httptest.NewRecorder()
		before := forwarded
		live.Serve(snapshot, response, r)
		if response.Code != status {
			t.Fatalf("port=%d status=%d hits=%+v", port, response.Code, last.Hits)
		}
		if status == http.StatusForbidden && forwarded != before || status == http.StatusNoContent && forwarded != before+1 {
			t.Fatal("upstream call count disagrees with the expected decision")
		}
		if category == "" {
			if len(last.Hits) != 0 {
				t.Fatalf("unexpected rule hits on port %d: %+v", port, last.Hits)
			}
		} else if len(last.Hits) != 1 || last.Hits[0].Category != category || last.Hits[0].Observe != (status == http.StatusNoContent) {
			t.Fatalf("port %d: expected %s, got %+v", port, category, last.Hits)
		}
	}
	photosURL := func() *http.Request {
		r := httptest.NewRequest("GET", "http://example.test/photos", nil)
		r.Header.Set("Referer", "http://169.254.169.254/latest/meta-data/")
		r.Header.Set("User-Agent", "iOS-Synology-Photos-iPhone")
		return r
	}
	photosList := func() *http.Request {
		r := httptest.NewRequest("POST", "http://example.test/webapi/entry.cgi", strings.NewReader("api=SYNO.Foto.Browse.Diff&method=list"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "session", Value: "fixture"})
		return r
	}
	check(45231, photosURL(), http.StatusNoContent, "")
	check(45232, photosURL(), http.StatusNoContent, "ssrf")
	check(45231, photosList(), http.StatusNoContent, "")
	check(45232, photosList(), http.StatusNoContent, "csrf")
	otherOrigin := photosList()
	otherOrigin.Header.Set("Origin", "https://other.example.test")
	check(45231, otherOrigin, http.StatusNoContent, "csrf")
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
	check(45231, photosURL(), http.StatusForbidden, "ssrf")
	check(45232, photosURL(), http.StatusNoContent, "ssrf")
}
