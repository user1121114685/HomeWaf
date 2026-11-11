package wafpipeline

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

var injectionCorpus = map[string]struct{ bad, good []string }{
	"sqli": {
		bad:  []string{"1' OR '1'='1", "1 OR 1=1--", "1 UNION SELECT username,password FROM users", "1 UnIoN aLl SeLeCt NULL,NULL", "1/**/UNION/**/SELECT/**/password/**/FROM/**/users", "1; SELECT password FROM users", "' OR 1=1 #", "1 AND SLEEP(10)", "1; exec xp_cmdshell 'whoami'", "1 UNION SELECT 1,2,3--", "1%2527%2520OR%25201%253D1--", "1' AND 1=1--"},
		good: []string{"select your plan", "union station", "drop table 咖啡", "1=1 是什么意思", "O'Reilly books", "套餐 A & B", "2026-11-10", "https://example.test/search?q=tea", "a+b=c", "中文 文件(1)#%.txt", "{\"count\":42}", "Choose between blue and green"},
	},
	"xss": {
		bad:  []string{"<script>alert(1)</script>", "<img src=x onerror=alert(1)>", "<svg onload=alert(1)>", "<a href=javascript:alert(1)>x</a>", "<iframe srcdoc='<script>alert(1)</script>'></iframe>", "<ScRiPt>alert(1)</ScRiPt>", "&lt;img src=x onerror=alert(1)&gt;", "%253Cscript%253Ealert(1)%253C/script%253E", `\u003cscript\u003ealert(1)\u003c/script\u003e`, "<body onload=alert(1)>", "<input autofocus onfocus=alert(1)>", "<details open ontoggle=alert(1)>"},
		good: []string{"hello world", "1 < 2 and 3 > 2", "<p>family photos</p>", "<strong>important</strong>", "照片 2026.jpg", "https://example.test/photos", "a+b=c", "${this is documentation}", "{{ordinary template text}}", "Unexpected token '<'", "<div class=note>note</div>", "中文 文件(1)#%&.txt"},
	},
}

func detectorFixture(category string, mode Mode) (*Runtime, *Snapshot) {
	p := DefaultPolicy()
	p.RuleSet.Rules = []Rule{{ID: category, Category: category, Mode: mode, Severity: High}}
	p.RuleSet.BudgetMillis = 1000
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://upstream.test", Policy: p}}
	e := DefaultRuleEngine()
	return &Runtime{RuleSteps: e.Steps}, s
}
func evalDetector(t *testing.T, category string, r *http.Request) Result {
	t.Helper()
	runtime, s := detectorFixture(category, ModeBlock)
	result, err := runtime.Explain(s, r, netip.MustParseAddr("192.0.2.10"))
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestInjectionAttackAndHouseholdCorpus(t *testing.T) {
	for category, corpus := range injectionCorpus {
		t.Run(category, func(t *testing.T) {
			for _, bad := range corpus.bad {
				t.Run("bad/"+bad, func(t *testing.T) {
					r := httptest.NewRequest("GET", "http://example.test/search?q="+url.QueryEscape(bad), nil)
					if got := evalDetector(t, category, r); got.Verdict.Action != Deny {
						t.Fatalf("attack missed: %s trace=%+v", bad, got.Trace)
					}
				})
			}
			for _, good := range corpus.good {
				t.Run("good/"+good, func(t *testing.T) {
					r := httptest.NewRequest("GET", "http://example.test/search?q="+url.QueryEscape(good), nil)
					if got := evalDetector(t, category, r); got.Verdict.Action != Allow {
						t.Fatalf("false positive: %s %+v", good, got)
					}
				})
			}
		})
	}
}
func TestInjectionEncodedBodiesAndReplay(t *testing.T) {
	for _, encoding := range []string{"identity", "gzip", "deflate", "br", "zstd"} {
		t.Run(encoding, func(t *testing.T) {
			body := []byte(`{"nested":{"q":"1' OR '1'='1"}}`)
			var wire bytes.Buffer
			switch encoding {
			case "identity":
				wire.Write(body)
			case "gzip":
				w := gzip.NewWriter(&wire)
				w.Write(body)
				w.Close()
			case "deflate":
				w := zlib.NewWriter(&wire)
				w.Write(body)
				w.Close()
			case "br":
				w := brotli.NewWriter(&wire)
				w.Write(body)
				w.Close()
			case "zstd":
				w, _ := zstd.NewWriter(&wire)
				w.Write(body)
				w.Close()
			}
			original := append([]byte(nil), wire.Bytes()...)
			r := httptest.NewRequest("POST", "http://example.test/search", bytes.NewReader(original))
			r.ContentLength = -1
			r.TransferEncoding = []string{"chunked"}
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Encoding", encoding)
			if got := evalDetector(t, "sqli", r); got.Verdict.Action != Deny {
				t.Fatalf("encoded body missed: %+v", got)
			}
			// Explain owns its context request; direct shared-view verification covers
			// forwarding, including the full original encoded byte representation.
			r = httptest.NewRequest("POST", "http://example.test/search", bytes.NewReader(original))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Encoding", encoding)
			v := NewNormalizedView(r, DefaultViewLimits())
			if err := v.ReadBody(); err != nil {
				t.Fatal(err)
			}
			replayed, _ := io.ReadAll(r.Body)
			if !bytes.Equal(replayed, original) {
				t.Fatal("wire body changed")
			}
		})
	}
	for _, raw := range []string{`{"q":"1' OR '1'='1","q":"safe"}`, `{"<img src=x onerror=alert(1)>":"safe"}`, `{"nested":[{"q":"<script>alert(1)</script>"}]}`} {
		category := "xss"
		if strings.Contains(raw, "OR") {
			category = "sqli"
		}
		r := httptest.NewRequest("POST", "http://example.test/", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if got := evalDetector(t, category, r); got.Verdict.Action != Deny {
			t.Fatalf("JSON duplicate/key/nesting missed: %s %+v", raw, got)
		}
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("nested", "<svg onload=alert(1)>")
	form.Close()
	r := httptest.NewRequest("POST", "http://example.test/", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	if got := evalDetector(t, "xss", r); got.Verdict.Action != Deny {
		t.Fatal("multipart field missed")
	}
}

type slowDetector struct{}

func (slowDetector) Name() string     { return "slow" }
func (slowDetector) Category() string { return "xss" }
func (slowDetector) Inspect(*NormalizedView) []Match {
	time.Sleep(5 * time.Millisecond)
	return []Match{{"xss", "query", "slow"}}
}
func TestDetectorScopeObserveBudgetAndCancellation(t *testing.T) {
	runtime, s := detectorFixture("xss", ModeObserve)
	r := httptest.NewRequest("GET", "http://example.test/?q="+url.QueryEscape("<script>alert(1)</script>"), nil)
	result, err := runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict.Action != Allow || len(result.Hits) != 1 || !result.Hits[0].Observe {
		t.Fatal("observe semantics")
	}
	s.Config.Policy.RuleSet.Rules[0].Mode = ModeBlock
	s.Config.Policy.RuleSet.Rules[0].Scope.ExcludeFields = []string{"content"}
	data, _ := json.Marshal(map[string]string{"content": "<script>example documentation</script>"})
	r = httptest.NewRequest("POST", "http://example.test/notes", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	result, err = runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil || result.Verdict.Action != Allow {
		t.Fatal("rich text exclusion ignored")
	}
	s.Config.Policy.RuleSet.BudgetMillis = 1
	runtime.RuleSteps = NewRuleEngine(slowDetector{}).Steps
	r = httptest.NewRequest("GET", "http://example.test/", nil)
	result, err = runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil || result.Verdict.Action != Allow || len(result.Hits) == 0 || result.Hits[0].Category != "detector_timeout" {
		t.Fatalf("budget: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = r.WithContext(ctx)
	result, err = runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil || result.Verdict.Action != Allow {
		t.Fatal("cancellation semantics")
	}
}
func BenchmarkInjectionDetectors(b *testing.B) {
	for _, d := range []Detector{SQLDetector(), XSSDetector()} {
		b.Run(d.Name(), func(b *testing.B) {
			r := httptest.NewRequest("GET", "http://example.test/?q=family+photo+album", nil)
			v := NewNormalizedView(r, DefaultViewLimits())
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Inspect(v)
			}
		})
	}
}

func TestViewRetainsReadFailureAndFieldQuotaDoesNotSliceShortBody(t *testing.T) {
	sentinel := errors.New("fixture read failure")
	r := httptest.NewRequest("POST", "http://example.test/", nil)
	r.Body = io.NopCloser(io.MultiReader(strings.NewReader("partial"), &onceError{err: sentinel}))
	v := NewNormalizedView(r, DefaultViewLimits())
	if err := v.ReadBody(); err == nil {
		t.Fatal("read error lost")
	}
	raw, err := io.ReadAll(r.Body)
	if string(raw) != "partial" || !errors.Is(err, sentinel) {
		t.Fatalf("replay lost original error: %q %v", raw, err)
	}
	r = httptest.NewRequest("POST", "http://example.test/?a=1&b=2", strings.NewReader("short"))
	limits := DefaultViewLimits()
	limits.MaxFields = 1
	v = NewNormalizedView(r, limits)
	if err = v.ReadBody(); err != nil {
		t.Fatal(err)
	}
	if !v.BodyTruncated {
		t.Fatal("field quota not reported")
	}
	raw, _ = io.ReadAll(r.Body)
	if string(raw) != "short" {
		t.Fatal("short body was corrupted")
	}
}
