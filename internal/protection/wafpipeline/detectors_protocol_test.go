package wafpipeline

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocolMalformedAndNormalCorpus(t *testing.T) {
	bad := []func(*http.Request){
		func(r *http.Request) {
			r.Header.Set("Content-Length", "5")
			r.Header.Set("Transfer-Encoding", "chunked")
		},
		func(r *http.Request) { r.Header["Content-Length"] = []string{"1", "2"} },
		func(r *http.Request) { r.TransferEncoding = []string{"gzip"} },
		func(r *http.Request) { r.TransferEncoding = []string{"chunked", "chunked"} },
		func(r *http.Request) { r.Header["Host"] = []string{"example.test", "other.test"} },
		func(r *http.Request) { r.Header.Set("bad header", "value") },
		func(r *http.Request) { r.Header.Set("X-Test", "a\nb") },
		func(r *http.Request) { r.Header.Set("X-Test", "a\x00b") },
		func(r *http.Request) { r.RequestURI = "/a\rnext" },
		func(r *http.Request) { r.URL.RawQuery = "q=%00" },
	}
	for i, mutate := range bad {
		r := httptest.NewRequest("GET", "http://example.test/", nil)
		mutate(r)
		r.Body = unreadBody{}
		if got := evalDetector(t, "malformed", r); got.Verdict.Action != Deny || got.Verdict.Status != 400 {
			t.Fatalf("malformed case %d accepted %+v", i, got)
		}
	}
	for _, method := range DefaultPolicy().RuleSet.AllowedMethods {
		r := httptest.NewRequest(method, "http://example.test/files/%E4%B8%AD%E6%96%87", nil)
		r.Header.Set("Cookie", "session="+strings.Repeat("a", 4096))
		r.Body = unreadBody{}
		if got := evalDetector(t, "malformed", r); got.Verdict.Action != Allow {
			t.Fatalf("normal %s blocked %+v", method, got)
		}
	}
	r := httptest.NewRequest("POST", "http://example.test/", nil)
	r.ContentLength = -1
	r.TransferEncoding = []string{"chunked"}
	if got := evalDetector(t, "malformed", r); got.Verdict.Action != Allow {
		t.Fatal("normal chunked framing blocked")
	}
	r = httptest.NewRequest("M-SEARCH", "http://example.test/", nil)
	if got := evalDetector(t, "malformed", r); got.Verdict.Action != Allow || len(got.Hits) != 1 || !got.Hits[0].Observe {
		t.Fatal("non-smuggling anomaly must observe")
	}
}
func TestScannerCorpusAndExplicitAbsentPaths(t *testing.T) {
	for _, ua := range []string{"sqlmap", "nikto", "nmap", "masscan", "nuclei", "acunetix", "nessus", "wpscan", "gobuster", "dirbuster", "ffuf", "zgrab"} {
		r := httptest.NewRequest("GET", "http://example.test/", nil)
		r.Header.Set("User-Agent", ua)
		r.Body = unreadBody{}
		if got := evalDetector(t, "scanner", r); got.Verdict.Action != Deny {
			t.Fatalf("scanner missed: %s", ua)
		}
	}
	for _, ua := range []string{"Mozilla/5.0", "Chrome/120", "Safari/600", "Firefox/100", "Edge/100", "curl/8", "python-requests/2", "Go-http-client/1.1", "HomeAssistant", "Synology Drive", "Microsoft-WebDAV-MiniRedir", "CameraClient"} {
		r := httptest.NewRequest("GET", "http://example.test/articles/sqlmap-and-nuclei", nil)
		r.Header.Set("User-Agent", ua)
		if got := evalDetector(t, "scanner", r); got.Verdict.Action != Allow {
			t.Fatalf("normal client blocked %s", ua)
		}
	}
	runtime, s := detectorFixture("scanner", ModeBlock)
	r := httptest.NewRequest("GET", "http://example.test/wp-login.php", nil)
	result, err := runtime.Explain(s, r, testIP())
	if err != nil || result.Verdict.Action != Allow {
		t.Fatal("existing application path assumed absent")
	}
	s.Config.Policy.RuleSet.Rules[0].Params = map[string]json.RawMessage{"probe_paths": json.RawMessage(`["/wp-login.php"]`)}
	result, err = runtime.Explain(s, r, testIP())
	if err != nil || result.Verdict.Action != Deny {
		t.Fatal("explicit absent path not detected")
	}
}
func TestActualPipelineOrderAndPostLoginBudget(t *testing.T) {
	f := newGateFixture(t, nil, "basic", "long-safe-pass-42")
	engine := DefaultRuleEngine()
	f.runtime.RuleSteps = func(s *Snapshot) []Step { return engine.WithControls(s, f.rules.Steps(s)) }
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	r, _ := gateRequest(t, f, "GET", "/", nil, map[string]string{"User-Agent": "sqlmap"})
	if r.StatusCode != 403 || f.hits.Load() != 0 {
		t.Fatal("scanner did not run before gate")
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("owner:long-safe-pass-42"))
	r, _ = gateRequest(t, f, "GET", "/?q=1%27%20OR%201%3D1--", nil, map[string]string{"Authorization": auth})
	if r.StatusCode != 403 || f.hits.Load() != 0 {
		t.Fatal("authentication consumed later inspection budget or skipped inspection")
	}
	steps := f.snap.Compiled.Pipeline.steps
	index := map[string]int{}
	for i, s := range steps {
		index[s.Name] = i
	}
	names := []string{"rule:malformed", "qps", "cc", "rule:scanner", "bot", "auth_gate", "rule:sqli"}
	for i := 1; i < len(names); i++ {
		if index[names[i-1]] >= index[names[i]] {
			t.Fatalf("incorrect order %v", index)
		}
	}
}

func BenchmarkProtocolDetectors(b *testing.B) {
	for _, d := range []Detector{ProtocolDetector(DefaultPolicy().RuleSet.AllowedMethods, 32768), ScannerDetector(nil)} {
		b.Run(d.Name(), func(b *testing.B) {
			r := httptest.NewRequest("GET", "http://example.test/photos", nil)
			r.Header.Set("User-Agent", "Mozilla/5.0")
			v := NewNormalizedView(r, DefaultViewLimits())
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Inspect(v)
			}
		})
	}
}
