package wafpipeline

import (
	"HomeWaf/wafenginecore/ipset"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func accessSnapshot() *Snapshot {
	return &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://localhost:8080", Policy: DefaultPolicy()}}
}

func TestSharedGroupsUpdateLiveAndDenyBeforeTrusted(t *testing.T) {
	t.Cleanup(func() { ipset.SetGroupSnapshot(nil) })
	ipset.UpsertGroupMatcher("allow", "家人", ipset.BuildMatchSet([]string{"203.0.113.1", "240e::1"}))
	ipset.UpsertGroupMatcher("deny", "拒绝", ipset.BuildMatchSet([]string{"203.0.113.2", "240e::2"}))
	s := accessSnapshot()
	s.Config.Policy.IPGroup.AllowGroups = []string{"allow"}
	s.Config.Policy.IPGroup.DenyGroups = []string{"deny"}
	s.Config.Policy.IPGroup.UnlistedAction = Deny
	r := &Runtime{}
	if err := r.Prepare(s); err != nil {
		t.Fatal(err)
	}
	check := func(ip string, want Action, reason string) {
		t.Helper()
		ctx := NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "5000", netip.MustParseAddr(ip), s.Config.Policy.RuleSet.View)
		result := s.Compiled.Pipeline.Evaluate(ctx)
		if result.Verdict.Action != want || result.Verdict.Reason != reason {
			t.Fatalf("%s: %+v", ip, result.Verdict)
		}
	}
	check("203.0.113.1", Allow, "passed")
	check("240e::1", Allow, "passed")
	ipset.UpsertGroupMatcher("deny", "拒绝", ipset.BuildMatchSet([]string{"203.0.113.1", "240e::1"}))
	check("203.0.113.1", Deny, "blacklist:group")
	check("240e::1", Deny, "blacklist:group")
	s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "allow", Trust: "trusted", Value: "203.0.113.1"}}
	if err := r.Prepare(s); err != nil {
		t.Fatal(err)
	}
	check("203.0.113.1", Deny, "blacklist:group")
	ipset.UpsertGroupMatcher("allow", "家人", ipset.BuildMatchSet([]string{"203.0.113.3"}))
	check("203.0.113.3", Allow, "passed")
	check("240e::3", Deny, "ip:unlisted")
	ipset.RemoveGroupMatcher("deny")
	check("203.0.113.3", Deny, "ip:group_unavailable")
}

func TestMainlandBlacklistBothFamiliesAndUnknown(t *testing.T) {
	for _, ip := range []string{"203.0.113.1", "240e::1", "::ffff:203.0.113.1"} {
		for _, tc := range []struct {
			country, province string
			known, allow      bool
		}{
			{"中国", "重庆市", true, true}, {"China", "", true, true}, {"CN", "Chongqing", true, true},
			{"中国", "香港特别行政区", true, false}, {"中国", "澳门", true, false}, {"中国", "臺灣省", true, false},
			{"香港", "", true, false}, {"台湾", "", true, false}, {"中国", "Hong Kong", true, false},
			{"United States", "California", true, false}, {"", "", false, false}, {"中国", "重庆", false, false},
		} {
			t.Run(ip+tc.country+tc.province, func(t *testing.T) {
				s := accessSnapshot()
				s.Config.Policy.IPGroup.BlockOutsideMainland = true
				// Even a trusted entry cannot override this explicit blacklist.
				s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "allow", Trust: "trusted", Value: ip}}
				r := &Runtime{Geo: NewGeoCache(func(netip.Addr) GeoResult { return GeoResult{Country: tc.country, Province: tc.province, OK: tc.known} }, nil)}
				result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
				if err != nil {
					t.Fatal(err)
				}
				if tc.allow && result.Verdict.Action != Allow || !tc.allow && result.Verdict.Reason != "blacklist:outside_mainland" {
					t.Fatalf("%+v", result.Verdict)
				}
			})
		}
	}
	s := accessSnapshot()
	s.Config.Policy.IPGroup.BlockOutsideMainland = true
	r := &Runtime{}
	for _, ip := range []string{"203.0.113.1", "240e::1"} {
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Action != Deny {
			t.Fatalf("missing database allowed public source: %+v %v", result, err)
		}
	}
	for _, ip := range []string{"192.168.1.1", "fd00::1"} {
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Action != Allow {
			t.Fatalf("private source treated as foreign: %+v %v", result, err)
		}
	}
}

func TestRegionBlacklistOverridesCountryWhitelistAndReloads(t *testing.T) {
	s := accessSnapshot()
	g := &s.Config.Policy.IPGroup
	g.UnlistedAction = Deny
	g.Regions = []Region{{Country: "中国"}}
	g.DeniedRegions = []Region{{Country: "中国", Province: "重庆"}}
	generation := uint64(1)
	province := "重庆"
	r := &Runtime{Geo: NewGeoCache(func(netip.Addr) GeoResult { return GeoResult{Country: "中国", Province: province, OK: true} }, func() uint64 { return generation })}
	for _, ip := range []string{"203.0.113.1", "240e::1"} {
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Reason != "blacklist:region" {
			t.Fatalf("%+v %v", result, err)
		}
	}
	province = "四川"
	generation++
	result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr("240e::1"))
	if err != nil || result.Verdict.Action != Allow {
		t.Fatalf("database reload did not change region decision: %+v %v", result, err)
	}
}

func TestSharedWhitelistKeepsGateAttackAndAutomaticProtection(t *testing.T) {
	t.Cleanup(func() { ipset.SetGroupSnapshot(nil) })
	ipset.UpsertGroupMatcher("family", "家人", ipset.BuildMatchSet([]string{"203.0.113.1", "240e::1"}))
	for _, ip := range []string{"203.0.113.1", "240e::1"} {
		s := accessSnapshot()
		s.Config.Policy.IPGroup.AllowGroups = []string{"family"}
		s.Config.Policy.IPGroup.UnlistedAction = Deny
		gate := NewGate(NewGateStore())
		rules := NewRulesA(gate)
		engine := DefaultRuleEngine()
		r := &Runtime{RuleSteps: func(s *Snapshot) []Step { return engine.WithControls(s, rules.Steps(s)) }, Respond: gate.Respond}
		s.Config.Policy.Gate.Enabled = true
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Action == Allow || result.Verdict.Stage != 3 {
			t.Fatalf("whitelist bypassed gate: %+v %v", result, err)
		}
		s.Config.Policy.Gate.Enabled = false
		result, err = r.Explain(s, httptest.NewRequest("GET", "http://test/?file=../../../../etc/passwd", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Action != Deny || result.Verdict.Stage != 3 {
			t.Fatalf("whitelist bypassed attack detection: %+v %v", result, err)
		}
		r.State = fixtureState{ban: true}
		result, err = r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(ip))
		if err != nil || result.Verdict.Reason != "blacklist:auto:fixture" {
			t.Fatalf("whitelist bypassed automatic ban: %+v %v", result, err)
		}
	}
}

func TestBlacklistReportsTraceAndMetricsWithoutForwarding(t *testing.T) {
	s := accessSnapshot()
	s.Config.Policy.IPGroup.BlockOutsideMainland = true
	forwarded, logged := false, false
	r := &Runtime{Forward: func(*Snapshot, *RequestContext, http.ResponseWriter) { forwarded = true }, OnResult: func(_ *Snapshot, _ *RequestContext, result Result) {
		logged = result.Verdict.Reason == "blacklist:outside_mainland" && len(result.Trace) > 0
	}}
	if err := r.Prepare(s); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://test/", nil)
	req.RemoteAddr = "[240e::1]:12345"
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 5000}))
	response := httptest.NewRecorder()
	r.Serve(s, response, req)
	if response.Code != 403 || forwarded || !logged || s.Compiled.Metrics.Denied.Load() != 1 || s.Compiled.Metrics.Stats()["today_denied"] != 1 {
		t.Fatalf("status=%d forwarded=%v logged=%v denied=%d", response.Code, forwarded, logged, s.Compiled.Metrics.Denied.Load())
	}
}

func TestSharedGroupTemplateIsolationAndValidation(t *testing.T) {
	s := accessSnapshot()
	s.Config.Policy.IPGroup.AllowGroups = []string{"family"}
	s.Config.Policy.IPGroup.DenyGroups = []string{"blocked"}
	s.Config.Policy.IPGroup.BlockOutsideMainland = true
	tpl := TemplateFromPort("test", "测试", s.Config)
	if len(tpl.Policy.IPGroup.AllowGroups)+len(tpl.Policy.IPGroup.DenyGroups) != 0 || !tpl.Policy.IPGroup.BlockOutsideMainland {
		t.Fatal("template leaked local references or lost geographical policy")
	}
	result, err := ApplyTemplate(s.Config, tpl, []string{"ip_group"})
	if err != nil || len(result.Policy.IPGroup.AllowGroups) != 1 || len(result.Policy.IPGroup.DenyGroups) != 1 {
		t.Fatalf("template dropped group references: %+v %v", result, err)
	}
	tpl.Policy.IPGroup.AllowGroups = []string{"local"}
	if ValidateTemplate(tpl) == nil {
		t.Fatal("local group reference accepted in portable template")
	}
	s.Config.Policy.IPGroup.AllowGroups = []string{"duplicate", "duplicate"}
	if ValidatePolicy(s.Config.Policy) == nil {
		t.Fatal("duplicate reference accepted")
	}
}
