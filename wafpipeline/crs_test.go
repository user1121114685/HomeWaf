package wafpipeline

import (
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"testing"
)

func testIP() netip.Addr { return netip.MustParseAddr("192.0.2.1") }
func TestCRSRealRulesModeScopeAndFailedReload(t *testing.T) {
	p := &CRSProvider{Root: filepath.Join("..", "cmd", "homewaf", "exedata", "owasp")}
	cfg := PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://upstream.test", Policy: DefaultPolicy()}
	cfg.Policy.RuleSet.OWASP = ModeBlock
	cfg.Policy.RuleSet.BudgetMillis = 1000
	s := &Snapshot{Config: cfg}
	runtime := &Runtime{RulePrepare: p.Prepare, RuleSteps: func(s *Snapshot) []Step { return []Step{p.Step(s)} }}
	check := func(target string) Result {
		r := httptest.NewRequest("GET", target, nil)
		r.Header.Set("User-Agent", "Mozilla/5.0")
		r.Header.Set("Accept", "text/html")
		result, err := runtime.Explain(s, r, testIP())
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := check("http://example.test/?q=family+photo"); got.Verdict.Action != Allow {
		t.Fatalf("CRS normal request blocked %+v", got)
	}
	r := httptest.NewRequest("PROPFIND", "http://example.test/dav", nil)
	r.Header.Set("User-Agent", "NAS WebDAV")
	r.Header.Set("Accept", "*/*")
	dav, err := runtime.Explain(s, r, testIP())
	if err != nil || dav.Verdict.Action != Allow {
		t.Fatalf("configured WebDAV method blocked %+v %v", dav, err)
	}
	if got := check("http://example.test/?q=1%27+OR+1%3D1--"); got.Verdict.Action != Deny || len(got.Hits) == 0 {
		t.Fatalf("CRS SQL not detected %+v", got)
	}
	s.Config.Policy.RuleSet.OWASP = ModeObserve
	if got := check("http://example.test/?q=1%27+OR+1%3D1--"); got.Verdict.Action != Allow || len(got.Hits) == 0 || !got.Hits[0].Observe {
		t.Fatalf("CRS observe failed %+v", got)
	}
	s.Config.Policy.RuleSet.OWASP = ModeBlock
	s.Config.Policy.RuleSet.OWASPScope.ExcludePaths = []string{"/notes"}
	if got := check("http://example.test/notes?q=1%27+OR+1%3D1--"); got.Verdict.Action != Allow {
		t.Fatal("CRS path scope ignored")
	}
	old := p.current.Load()
	p.Root = filepath.Join(t.TempDir(), "missing")
	if err := p.Reload(); err == nil || p.current.Load() != old {
		t.Fatal("failed CRS reload replaced valid rules")
	}
}

func BenchmarkCRSRequest(b *testing.B) {
	p := &CRSProvider{Root: filepath.Join("..", "cmd", "homewaf", "exedata", "owasp")}
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://upstream.test", Policy: DefaultPolicy()}}
	s.Config.Policy.RuleSet.OWASP = ModeObserve
	s.Config.Policy.RuleSet.BudgetMillis = 1000
	if err := p.Prepare(s); err != nil {
		b.Fatal(err)
	}
	step := p.Step(s)
	r := httptest.NewRequest("GET", "http://example.test/?q=family+photo", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0")
	r.Header.Set("Accept", "text/html")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewRequestContext(r, PortID(5000), testIP(), DefaultViewLimits())
		if _, err := step.Inspect(c); err != nil {
			b.Fatal(err)
		}
	}
}
