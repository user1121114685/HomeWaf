package wafpipeline

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

func assessmentFixture() (*Runtime, *Snapshot, *RulesA) {
	controls := NewRulesA(NewGate(nil))
	engine := DefaultRuleEngine()
	r := &Runtime{RuleSteps: func(s *Snapshot) []Step { return engine.WithControls(s, controls.Steps(s)) }}
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 15000, Protocol: "http", Upstream: "http://127.0.0.1:18080", Policy: DefaultPolicy()}}
	return r, s, controls
}

func TestAssessmentScoreTracksDefenseWithoutPunishingLiveTraffic(t *testing.T) {
	r, s, controls := assessmentFixture()
	s.Config.Policy.RuleSet.QPS.Burst = 2
	s.Config.Policy.RuleSet.QPS.PerIP = 1
	s.Config.Policy.RuleSet.QPS.PerPort = 1
	input := AssessmentInput{Sources: []AssessmentSource{{IP: "198.51.100.25", ExpectAccess: true}}, Pressure: 25, Samples: []AssessmentSample{
		{ID: "normal", Kind: "benign", Category: "normal", Path: "/", Method: "GET"},
		{ID: "sql", Kind: "attack", Category: "sqli", Path: "/?id=1%27%20OR%20%271%27=%271%27--", Method: "GET"},
	}}
	before := CloneConfig(s.Config)
	blocked, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Score == nil || *blocked.Score != 100 || blocked.Pressure.Limited == 0 {
		t.Fatalf("unexpected blocked assessment: %+v", blocked)
	}
	if len(controls.Rates.entries) != 0 || r.Requests.Load() != 0 || r.Denied.Load() != 0 || s.Compiled != nil || s.Config.Revision != before.Revision {
		t.Fatal("assessment changed live counters or snapshot")
	}
	for i := range s.Config.Policy.RuleSet.Rules {
		s.Config.Policy.RuleSet.Rules[i].Mode = ModeObserve
	}
	observed, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Score == nil || *observed.Score >= *blocked.Score {
		t.Fatalf("observation earned blocking credit: %+v", observed)
	}
	s.Config.Policy.Access.Enabled = false
	closed, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Score == nil || *closed.Score != 0 {
		t.Fatal("reject-all earned a high score")
	}
	input.Samples[0].Kind = "unknown"
	input.Samples[1].Kind = "unknown"
	unknown, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Score != nil || len(unknown.Categories) != 0 {
		t.Fatal("unlabelled historical blocks created a score")
	}
}

func TestAssessmentUsesSelectedIdentityAndRealProxyTrust(t *testing.T) {
	r, s, _ := assessmentFixture()
	s.Config.HostNames = []string{"selected.example.test"}
	s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "deny", Type: "deny", Value: "198.51.100.25"}}
	input := AssessmentInput{Sources: []AssessmentSource{{IP: "198.51.100.25", ExpectAccess: false}}, Samples: []AssessmentSample{{ID: "spoof", Kind: "attack", Category: "source", Method: "GET", Path: "/", Headers: map[string]string{"Host": "private.invalid", "Origin": "https://private.invalid", "Referer": "https://private.invalid/secret", "Cookie": "PRIVATE", "X-Forwarded-For": "203.0.113.20"}}}}
	input.Samples[0].Imported = true
	report, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Target != "http://selected.example.test:15000" || report.Rows[0].ClientIP != "198.51.100.25" || report.Rows[0].Outcome != "blocked" {
		t.Fatalf("identity or proxy trust lost: %+v", report)
	}
	req, err := assessmentRequest(context.Background(), report.Target, input.Samples[0], input.Sources[0].IP)
	if err != nil {
		t.Fatal(err)
	}
	if req.Host != "selected.example.test:15000" || req.Header.Get("Origin") != report.Target || strings.Contains(req.Header.Get("Referer"), "private") || req.Header.Get("Cookie") != "" {
		t.Fatal("imported identity or credentials survived")
	}
	manual := input.Samples[0]
	manual.Imported = false
	manualRequest, err := assessmentRequest(context.Background(), report.Target, manual, input.Sources[0].IP)
	if err != nil {
		t.Fatal(err)
	}
	if manualRequest.Header.Get("Cookie") != "PRIVATE" {
		t.Fatal("manual diagnosis lost credential testing")
	}
	s.Config.HostNames = nil
	if target := AssessmentTarget(s.Config); target != "http://127.0.0.1:15000" {
		t.Fatalf("missing IP fallback: %s", target)
	}
	s.Config.Policy.Network.ListenIPVersion = "ipv6"
	if target := AssessmentTarget(s.Config); target != "http://[::1]:15000" {
		t.Fatalf("missing IPv6 fallback: %s", target)
	}
	s.Config.Policy.Network.TrustedProxies = []string{"198.51.100.25/32"}
	report, err = r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].ClientIP != "203.0.113.20" {
		t.Fatal("explicit trusted proxy was ignored")
	}
}

func TestAssessmentIncompleteInvalidDuplicateAndCancelledInputs(t *testing.T) {
	r, s, _ := assessmentFixture()
	input := AssessmentInput{Sources: []AssessmentSource{{IP: "198.51.100.25", ExpectAccess: true}}, Samples: []AssessmentSample{{ID: "a", Kind: "benign", Path: "/", Method: "GET"}, {ID: "b", Kind: "benign", Path: "/", Method: "GET"}, {ID: "c", Kind: "attack", Category: "sqli", Path: "/?q=1%27%20OR%20%271%27=%271%27--", Truncated: true}}}
	report, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if report.NormalTotal != 1 || report.Rows[2].Outcome != "incomplete" || report.Score == nil || *report.Score != 0 {
		t.Fatal("duplicate or truncated input inflated score")
	}
	input.Samples[0].Path = "//remote.invalid/"
	if _, err := r.Assess(context.Background(), s, input); err == nil {
		t.Fatal("absolute network target accepted")
	}
	input.Samples[0].Path = "/"
	input.Pressure = 2001
	if _, err := r.Assess(context.Background(), s, input); err == nil {
		t.Fatal("unbounded pressure accepted")
	}
	input.Pressure = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Assess(ctx, s, input); err == nil {
		t.Fatal("cancel ignored")
	}
	input.Sources[0].IP = netip.IPv4Unspecified().String()
	if _, err := r.Assess(context.Background(), s, input); err == nil {
		t.Fatal("invalid source accepted")
	}
}

func TestAssessmentUnavailableControlsEarnNoDefenseCredit(t *testing.T) {
	r, s, _ := assessmentFixture()
	s.Config.Policy.IPGroup.AllowGroups = []string{"missing-assessment-group"}
	input := AssessmentInput{Sources: []AssessmentSource{{IP: "198.51.100.25", ExpectAccess: true}}, Builtins: true}
	report, err := r.Assess(context.Background(), s, input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].Outcome != "unavailable" || report.Score == nil || *report.Score != 0 {
		t.Fatal("unavailable IP group earned defense credit")
	}
}
