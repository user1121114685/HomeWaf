package wafpipeline

import (
	"io"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestSSRFLiteralAndHouseholdCorpus(t *testing.T) {
	bad := []string{"http://127.0.0.1/admin", "http://localhost/", "http://device.localhost/", "http://10.0.0.1/", "http://192.168.1.1/", "http://172.16.0.1/", "http://169.254.169.254/latest/meta-data/", "http://[::1]/", "http://[::ffff:127.0.0.1]/", "http://2130706433/", "http://0x7f000001/", "http://0177.0.0.1/", "http://127.1/", "http://0x7f.0.0.1/", "http://example.test@127.0.0.1/", "http://127.0.0.1/#@example.test", "gopher://127.0.0.1:6379/_fixture", "dict://127.0.0.1/", "file:///etc/passwd", "http://[fe80::1%25eth0]/", "http://100.64.1.1/"}
	good := []string{"https://example.test/image.jpg", "http://8.8.8.8/", "https://1.1.1.1/", "https://[2606:4700:4700::1111]/", "ordinary description", "/photos/album", "http://127.0.0.1.example.test/", "https://example.test/?next=http://127.0.0.1/", "https://example.test/#127.0.0.1", "https://example.test/photos/中文", "ftp://example.test/file", "https://192.168.1.1.example.test/"}
	for _, s := range bad {
		t.Run("bad/"+s, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.test/fetch?url="+url.QueryEscape(s), nil)
			if got := evalDetector(t, "ssrf", r); got.Verdict.Action != Deny {
				t.Fatalf("missed %q %+v", s, got)
			}
		})
	}
	for _, s := range good {
		t.Run("good/"+s, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.test/fetch?url="+url.QueryEscape(s), nil)
			if got := evalDetector(t, "ssrf", r); got.Verdict.Action != Allow {
				t.Fatalf("false positive %q %+v", s, got)
			}
		})
	}
}

type unreadBody struct{}

func (unreadBody) Read([]byte) (int, error) { panic("header-only CSRF read the body") }
func (unreadBody) Close() error             { return nil }
func TestCSRFCrossOriginAndNormalCorpus(t *testing.T) {
	bad := []string{"http://evil.test", "https://example.test", "http://example.test:5001", "http://example.test.evil.test", "http://evil.test@example.test", "null", "file:///tmp", "http://sub.example.test", "http://example.test/path", "http://example.test?x=1", "http://example.test#x"}
	for _, origin := range bad {
		t.Run("bad/"+origin, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://example.test/api", nil)
			r.Header.Set("Cookie", "session=business")
			r.Header.Set("Origin", origin)
			r.Body = unreadBody{}
			if got := evalDetector(t, "csrf", r); got.Verdict.Action != Deny {
				t.Fatalf("cross-origin accepted %q %+v", origin, got)
			}
		})
	}
	cases := []struct{ method, origin, referer, cookie string }{{"GET", "http://evil.test", "", "s=1"}, {"HEAD", "http://evil.test", "", "s=1"}, {"OPTIONS", "http://evil.test", "", "s=1"}, {"POST", "http://example.test", "", "s=1"}, {"PUT", "http://example.test:80", "", "s=1"}, {"PATCH", "http://EXAMPLE.test", "", "s=1"}, {"DELETE", "http://example.test", "", "s=1"}, {"POST", "", "http://example.test/photos?q=1", "s=1"}, {"POST", "http://evil.test", "", ""}, {"POST", "", "", ""}, {"PROPFIND", "http://evil.test", "", "s=1"}, {"POST", "http://example.test", "http://evil.test/", "s=1"}}
	for _, tt := range cases {
		r := httptest.NewRequest(tt.method, "http://example.test/api", nil)
		r.Header.Set("Cookie", tt.cookie)
		r.Header.Set("Origin", tt.origin)
		r.Header.Set("Referer", tt.referer)
		r.Body = unreadBody{}
		if got := evalDetector(t, "csrf", r); got.Verdict.Action != Allow {
			t.Fatalf("normal CSRF case rejected %+v %+v", tt, got)
		}
	}
}
func TestCSRFExceptionsMissingOriginAndHostScope(t *testing.T) {
	runtime, s := detectorFixture("csrf", ModeBlock)
	s.Config.Policy.RuleSet.CSRFOrigins = []string{"https://mobile.example.test"}
	request := func(origin string) Result {
		r := httptest.NewRequest("POST", "http://example.test/api", nil)
		r.Header.Set("Cookie", "session=x")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		result, err := runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := request("https://mobile.example.test"); got.Verdict.Action != Allow {
		t.Fatal("explicit origin exception ignored")
	}
	if got := request(""); got.Verdict.Action != Allow || len(got.Hits) != 1 || !got.Hits[0].Observe {
		t.Fatal("missing origin default must observe")
	}
	s.Config.Policy.RuleSet.CSRFMissingOrigin = "block"
	if got := request(""); got.Verdict.Action != Deny {
		t.Fatal("missing origin block ignored")
	}
	s.Config.Policy.RuleSet.CSRFMissingOrigin = "allow"
	if got := request(""); got.Verdict.Action != Allow || len(got.Hits) != 0 {
		t.Fatal("missing origin allow ignored")
	}
	s.Config.Policy.RuleSet.Rules[0].Scope.ExcludePaths = []string{"/api"}
	if got := request("http://evil.test"); got.Verdict.Action != Allow {
		t.Fatal("webhook path exception ignored")
	}
	s.Config.Policy.RuleSet.Rules[0].Scope.ExcludePaths = nil
	s.Config.Policy.RuleSet.Rules[0].Scope.Hosts = []string{"other.test"}
	if got := request("http://evil.test"); got.Verdict.Action != Allow {
		t.Fatal("host scope ignored")
	}
	r := httptest.NewRequest("POST", "http://example.test/api", io.NopCloser(strings.NewReader("")))
	r.Header.Set("Cookie", "session=x")
	r.Header.Add("Origin", "http://example.test")
	r.Header.Add("Origin", "http://evil.test")
	s.Config.Policy.RuleSet.Rules[0].Scope.Hosts = nil
	got, err := runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil || got.Verdict.Action != Deny {
		t.Fatal("duplicate origin ambiguity allowed")
	}
}
func BenchmarkOriginDetectors(b *testing.B) {
	for _, d := range []Detector{SSRFDetector(), CSRFDetector("http", "observe", nil)} {
		b.Run(d.Name(), func(b *testing.B) {
			r := httptest.NewRequest("POST", "http://example.test/?url=https://example.test/photo.jpg", nil)
			r.Header.Set("Cookie", "s=1")
			r.Header.Set("Origin", "http://example.test")
			v := NewNormalizedView(r, DefaultViewLimits())
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Inspect(v)
			}
		})
	}
}
