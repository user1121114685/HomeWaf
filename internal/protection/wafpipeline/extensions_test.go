package wafpipeline

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestCustomRulesRunAfterGateAndDryRunNeverPunishes(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "business") }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.BudgetMillis = 1000
		s.Config.ScriptGroups = map[string][]string{"family": {"198.51.100.0/24"}}
		s.Config.Policy.RuleSet.Scripts = []ScriptRule{{ID: "family-script", Source: `rule RFamily "" salience 10 { when MF.URL == "/restricted" && RF.IPInGroup(MF.SRC_IP,"family") then RF.Ban(10); }`, Mode: ModeBlock, Severity: High}}
	})
	s := f.current.Load()
	req := httptest.NewRequest("GET", "http://family.test/restricted", nil)
	before := len(f.sink.snapshot())
	result, err := f.runtime.Explain(s, req, netip.MustParseAddr("198.51.100.25"))
	if err != nil || result.Verdict.Action != Deny || len(result.Hits) != 1 || result.Hits[0].BanSeconds != 600 {
		t.Fatalf("script not evaluated: %+v err=%v", result, err)
	}
	if len(f.sink.snapshot()) != before || f.requests.Load() != 0 {
		t.Fatal("dry-run caused event or upstream traffic")
	}
	copy := CloneConfig(s.Config)
	copy.ScriptGroups["family"][0] = "203.0.113.0/24"
	if s.Config.ScriptGroups["family"][0] != "198.51.100.0/24" {
		t.Fatal("script groups shared between snapshots")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Gate.Enabled = true })
	result, err = f.runtime.Explain(f.current.Load(), httptest.NewRequest("GET", "http://family.test/restricted", nil), netip.MustParseAddr("198.51.100.25"))
	if err != nil || result.Verdict.Reason != "gate:required" || result.Verdict.Action != Challenge {
		t.Fatalf("custom rule ran before login: %+v err=%v", result, err)
	}
	for _, trace := range result.Trace {
		if trace.Check == "custom_rules" {
			t.Fatal("unauthenticated request evaluated custom rules")
		}
	}
	res, err := f.front.Client().Get(f.front.URL + "/restricted")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if f.requests.Load() != 0 {
		t.Fatal("login failure reached real upstream")
	}
}

func TestURLPolicyAndCustomObservationPreserveOrdinaryTraffic(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "business") }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.BudgetMillis = 1000
		s.Config.Policy.RuleSet.URLs = []URLRule{{ID: "allow", Match: "prefix", Value: "/notes", Action: Allow, Mode: ModeBlock}, {ID: "deny", Match: "exact", Value: "/notes/private", Action: Deny, Mode: ModeBlock}}
		s.Config.Policy.RuleSet.Scripts = []ScriptRule{{ID: "observe", Source: `rule RObserve "" salience 1 { when MF.METHOD == "POST" then RF.Deny(); }`, Mode: ModeObserve, Severity: High}}
	})
	for _, tc := range []struct {
		path   string
		status int
		hits   int64
	}{{"/notes/private", 403, 0}, {"/notes/public", 200, 1}, {"/ordinary", 200, 2}} {
		res, err := f.front.Client().Post(f.front.URL+tc.path, "text/plain", strings.NewReader("family notes"))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != tc.status || f.requests.Load() != tc.hits {
			t.Fatalf("%s status=%d origins=%d", tc.path, res.StatusCode, f.requests.Load())
		}
	}
	if _, err := f.runtime.Explain(f.current.Load(), httptest.NewRequest("GET", "http://family.test/notes/private", nil), netip.MustParseAddr("203.0.113.5")); err != nil {
		t.Fatal(err)
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "deny-local", Type: "deny", Value: "127.0.0.1"}}
	})
	res, err := f.front.Client().Get(f.front.URL + "/notes/public")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 || f.requests.Load() != 2 {
		t.Fatal("URL whitelist bypassed IP deny")
	}
}

func TestCustomFailureCountersArePortLocalAndReadOnlyInExplain(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.BudgetMillis = 1000
		s.Config.Policy.RuleSet.Scripts = []ScriptRule{{ID: "failures", Source: `rule RFailures "" salience 10 { when MF.GetIPFailureCount(5) >= 2 then RF.Deny(); }`, Mode: ModeBlock, Severity: High}}
	})
	for i := 0; i < 2; i++ {
		res, err := f.front.Client().Get(f.front.URL + "/missing")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatal("blocked before threshold")
		}
	}
	res, err := f.front.Client().Get(f.front.URL + "/next")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 || f.requests.Load() != 2 {
		t.Fatal("port failures not used by custom rule")
	}
	s := f.current.Load()
	keys := f.runtime.failureStore().ActiveKeys()
	for i := 0; i < 2; i++ {
		result, err := f.runtime.Explain(s, httptest.NewRequest("GET", "http://family.test/next", nil), netip.MustParseAddr("127.0.0.1"))
		if err != nil || result.Verdict.Action != Deny {
			t.Fatal("dry-run did not read counter", err)
		}
	}
	other := &Snapshot{Config: CloneConfig(s.Config)}
	other.Config.Port++
	result, err := f.runtime.Explain(other, httptest.NewRequest("GET", "http://family.test/next", nil), netip.MustParseAddr("127.0.0.1"))
	if err != nil || result.Verdict.Action != Allow {
		t.Fatal("failure counter leaked to another port", err)
	}
	if f.runtime.failureStore().ActiveKeys() != keys {
		t.Fatal("dry-run created counter keys")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = f.runtime.Explain(s, httptest.NewRequest("GET", "http://family.test/", nil).WithContext(cancelled), netip.MustParseAddr("127.0.0.1"))
	if err != nil || result.Verdict.Action != Allow {
		t.Fatal("cancelled detector did not fail open", err)
	}
}

func TestMalformedAndUnsafeCustomRulesRejectBeforePublication(t *testing.T) {
	for _, source := range []string{`rule Broken { when then }`, `rule R "" { when MF.GetIPFailureCount(MF.CONTENT_LENGTH) > 1 then RF.Deny(); }`, `rule R "" { when MF.RecordIPFailureThreshold(5,1) then RF.Deny(); }`} {
		s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5300, Protocol: "http", Upstream: "http://127.0.0.1", Policy: DefaultPolicy()}}
		s.Config.Policy.RuleSet.Scripts = []ScriptRule{{ID: "invalid", Source: source, Mode: ModeBlock, Severity: High}}
		if (&Runtime{}).Prepare(s) == nil {
			t.Fatal("invalid or side-effecting custom source accepted")
		}
	}
}
