package wafpipeline

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestHoneypotCaptureDeniedStepPolicyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		number     int
		step       string
		failClosed bool
		verdict    Verdict
		hit        bool
		observeHit bool
		fail       bool
		want       Action
	}{
		{"manual blacklist", 2, "ip_group", true, Verdict{Action: Deny, Status: 403, Reason: "blacklist:custom"}, false, false, false, Honeypot},
		{"shared blacklist", 2, "ip_group", true, Verdict{Action: Deny, Status: 403, Reason: "blacklist:group"}, false, false, false, Honeypot},
		{"automatic blacklist", 2, "ip_group", true, Verdict{Action: Deny, Status: 403, Reason: "blacklist:auto:sqli"}, false, false, false, Honeypot},
		{"unlisted", 2, "ip_group", true, Verdict{Action: Deny, Status: 403, Reason: "ip:unlisted"}, false, false, false, Honeypot},
		{"policy failure", 2, "ip_group", true, Verdict{Action: Deny, Status: 503, Reason: "ip:group_unavailable"}, false, false, false, Deny},
		{"lockdown", 2, "ip_group", true, Verdict{Action: Deny, Status: 503, Reason: "lockdown"}, false, false, false, Deny},
		{"access shutdown", 1, "access_switch", true, Verdict{Action: Deny, Status: 503, Reason: "access_disabled"}, false, false, false, Deny},
		{"unknown business entry", 2, "business_entry", true, Verdict{Action: Deny, Status: 404}, false, false, false, Deny},
		{"attack rule", 3, "sqli", false, Verdict{Action: Deny, Status: 403, RuleID: "sqli", Reason: "matched"}, true, false, false, Honeypot},
		{"rate limit", 3, "qps", false, Verdict{Action: Deny, Status: 429, RuleID: "qps"}, true, false, false, Honeypot},
		{"observed verdict", 3, "sqli", false, Verdict{Action: Deny, Observe: true}, true, true, false, Deny},
		{"observed evidence", 3, "sqli", false, Verdict{Action: Deny}, true, true, false, Deny},
		{"no hit", 3, "detector", false, Verdict{Action: Deny}, false, false, false, Deny},
		{"auth gate", 3, "auth_gate", true, Verdict{Action: Deny, Status: 403}, true, false, false, Deny},
		{"challenge", 3, "bot", false, Verdict{Action: Challenge, Status: 429}, true, false, false, Challenge},
		{"drop", 3, "custom", false, Verdict{Action: Drop}, true, false, false, Drop},
		{"detector failure", 3, "detector", false, Verdict{Action: Deny}, true, false, true, Deny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Snapshot{Config: PortConfig{Policy: DefaultPolicy()}}
			s.Config.Policy.Honeypot.Enabled = true
			s.Config.Policy.Honeypot.CaptureDenied = true
			originalErr := errors.New("unavailable")
			step := Step{Number: tc.number, Name: tc.step, FailClosed: tc.failClosed, Inspect: func(c *RequestContext) (Verdict, error) {
				if tc.hit {
					c.Hit("sqli", "sqli", High, tc.observeHit, "signature matched")
				}
				if tc.fail {
					return tc.verdict, originalErr
				}
				return tc.verdict, nil
			}}
			context := func() *RequestContext {
				return NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
			}
			c := context()
			v, err := captureDeniedStep(s, step).Inspect(c)
			if v.Action != tc.want || v.Reason != tc.verdict.Reason || v.RuleID != tc.verdict.RuleID || tc.fail && err != originalErr {
				t.Fatalf("verdict/evidence changed: %+v, %v", v, err)
			}
			if v.Action == Honeypot && v.Status != 200 {
				t.Fatal("honeypot did not advertise its response status")
			}
			for _, disabled := range []string{"capture", "honeypot"} {
				s.Config.Policy.Honeypot.CaptureDenied = disabled != "capture"
				s.Config.Policy.Honeypot.Enabled = disabled != "honeypot"
				v, _ := captureDeniedStep(s, step).Inspect(context())
				if v != tc.verdict {
					t.Fatalf("%s disabled but behavior changed: %+v", disabled, v)
				}
			}
		})
	}
}

func TestHoneypotCapturedRequestsStayIsolatedAndBoundMetadata(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	honey := NewHoneyHandler()
	f.runtime.Respond = honey.Respond
	results := make(chan Result, 8)
	f.runtime.OnResult = func(_ *Snapshot, _ *RequestContext, result Result) { results <- result }
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Honeypot.Enabled = true
		s.Config.Policy.Honeypot.CaptureDenied = true
		s.Config.Policy.Honeypot.MaxDelayMillis = 0
		s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "deny", Value: "127.0.0.1"}}
	})
	for _, method := range []string{"GET", "POST", "HEAD"} {
		headers := map[string]string{
			"X-Forwarded-For": "198.51.100.8", "X-Real-IP": "198.51.100.9",
			"User-Agent": strings.Repeat("agent", 100), "Authorization": "Bearer never-log-this",
			"Cookie": "session=never-log-this", "Content-Type": "application/json",
		}
		status, body, _ := proxyCall(t, f, method, "/admin?token=never-log-this", `{"username":"probe","password":"never-log-this"}`, headers)
		result := <-results
		if status != 200 || result.Verdict.Action != Honeypot || result.Verdict.Status != 200 || result.Verdict.Reason != "blacklist:custom" || f.requests.Load() != 0 {
			t.Fatalf("%s reached upstream or lost denial: %d %+v", method, status, result)
		}
		h := result.Honeypot
		if h == nil || h.Method != method || h.Path != "/admin" || h.NormalizedPath != "/admin" || h.PeerIP != "127.0.0.1" || len(h.UserAgent) != 256 {
			t.Fatalf("missing/boundless metadata: %+v", h)
		}
		if method == "HEAD" && len(body) != 0 {
			t.Fatal("HEAD returned a body")
		}
		raw, _ := json.Marshal(result)
		if strings.Contains(string(raw), "never-log-this") || strings.Contains(string(body), "never-log-this") {
			t.Fatal("credential or query leaked into honeypot evidence")
		}
		if method == "POST" {
			found := false
			for _, hit := range result.Hits {
				found = found || hit.Category == "honeypot_escalation" && strings.Contains(hit.Evidence, "keyed_digest=")
			}
			if !found {
				t.Fatal("submission escalation missing")
			}
		}
	}
	// Explain uses exactly the same routing policy and keeps the original reason.
	result, err := f.runtime.Explain(f.current.Load(), httptest.NewRequest("GET", "http://test/admin", nil), netip.MustParseAddr("127.0.0.1"))
	if err != nil || result.Verdict.Action != Honeypot || result.Trace[len(result.Trace)-1].Verdict.Action != Honeypot {
		t.Fatalf("preview disagrees with live routing: %+v %v", result, err)
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Honeypot.CaptureDenied = false })
	status, _, _ := proxyCall(t, f, "GET", "/admin", "", nil)
	result = <-results
	if status != 403 || result.Honeypot != nil || f.requests.Load() != 0 {
		t.Fatal("disabling capture did not restore the original denial")
	}
}

func TestHoneypotTrapNormalizedPathsAcrossSources(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.runtime.Respond = NewHoneyHandler().Respond
	original := f.runtime.RuleSteps
	f.runtime.RuleSteps = func(s *Snapshot) []Step { return append([]Step{TrapStep(s)}, original(s)...) }
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Honeypot.Enabled = true
		s.Config.Policy.Honeypot.MaxDelayMillis = 0
		s.Config.Policy.Honeypot.TrapPaths = []string{"/bait"}
		s.Config.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
	})
	for _, ip := range []string{"198.51.100.11", "198.51.100.12"} {
		for _, path := range []string{"/bait", "/%2562ait", "/x/../bait", "/bait;ignored"} {
			status, _, _ := proxyCall(t, f, "GET", path, "", map[string]string{"X-Forwarded-For": ip})
			if status != 200 || f.requests.Load() != 0 {
				t.Fatalf("rotated source or encoded trap escaped isolation: %s %s %d", ip, path, status)
			}
		}
	}
	status, _, _ := proxyCall(t, f, "GET", "/normal", "", nil)
	if status != 204 || f.requests.Load() != 1 {
		t.Fatal("trap affected a normal request")
	}
}

func TestHoneypotRuleDenialsAcrossSourcesAndObserveMode(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.runtime.Respond = NewHoneyHandler().Respond
	results := make(chan Result, 8)
	f.runtime.OnResult = func(_ *Snapshot, _ *RequestContext, result Result) { results <- result }
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Honeypot.Enabled = true
		s.Config.Policy.Honeypot.CaptureDenied = true
		s.Config.Policy.Honeypot.MaxDelayMillis = 0
		s.Config.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
		s.Config.Policy.RuleSet.URLs = []URLRule{{ID: "restricted", Match: "prefix", Value: "/private", Action: Deny, Mode: ModeBlock}}
	})
	for _, ip := range []string{"198.51.100.21", "198.51.100.22"} {
		for _, path := range []string{"/private", "/%2570rivate", "/x/../private", "/%2570rivate?probe=1"} {
			status, _, _ := proxyCall(t, f, "GET", path, "", map[string]string{"X-Forwarded-For": ip})
			result := <-results
			if status != 200 || result.Verdict.Action != Honeypot || result.Verdict.RuleID != "restricted" || result.Honeypot == nil || f.requests.Load() != 0 {
				t.Fatalf("rule denial escaped capture: %s %s %+v", ip, path, result)
			}
		}
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.URLs[0].Mode = ModeObserve })
	status, _, _ := proxyCall(t, f, "GET", "/private", "", map[string]string{"X-Forwarded-For": "198.51.100.21"})
	result := <-results
	if status != 204 || result.Honeypot != nil || f.requests.Load() != 1 {
		t.Fatal("observation mode was converted to enforcement")
	}
}

func TestHoneypotOverloadAndHead(t *testing.T) {
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: DefaultPolicy()}}
	s.Config.Policy.Honeypot.MaxConcurrent = 1
	s.Config.Policy.Honeypot.MaxDelayMillis = 0
	h := NewHoneyHandler()
	newContext := func() *RequestContext {
		return NewRequestContext(httptest.NewRequest("HEAD", "http://test/", nil), "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
	}
	c, w := newContext(), httptest.NewRecorder()
	if !h.Respond(s, c, w, Verdict{Action: Honeypot, Stage: 2}) || w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD was not handled without a body")
	}
	if !h.admission.Acquire(5000, 1) {
		t.Fatal("admission was not released")
	}
	defer h.admission.Release(5000)
	c, w = newContext(), httptest.NewRecorder()
	if !h.Respond(s, c, w, Verdict{Action: Honeypot, Stage: 2}) || w.Code != 503 || w.Header().Get("Retry-After") != "1" || c.Values["response_verdict"].(Verdict).Status != 503 {
		t.Fatal("overload was not bounded or reported accurately")
	}
}

type honeyDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *honeyDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestHoneypotSubmissionBoundsReadDeadline(t *testing.T) {
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: randomToken()}}
	s.Config.Policy.Honeypot.MaxDelayMillis = 0
	req := httptest.NewRequest("POST", "http://test/login", strings.NewReader(`{"username":"probe","password":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	c := NewRequestContext(req, "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
	w := &honeyDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	before := time.Now()
	if !NewHoneyHandler().Respond(s, c, w, Verdict{Action: Honeypot, Stage: 2}) {
		t.Fatal("submission was not handled")
	}
	if len(w.deadlines) != 2 || w.deadlines[0].Before(before.Add(4*time.Second)) || w.deadlines[0].After(before.Add(6*time.Second)) || !w.deadlines[1].IsZero() {
		t.Fatalf("submission deadline missing or not reset: %v", w.deadlines)
	}
}
