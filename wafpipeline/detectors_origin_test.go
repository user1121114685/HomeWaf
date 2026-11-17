package wafpipeline

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestSSRFAllowsBrowserContextHeaders(t *testing.T) {
	for _, origin := range []string{"http://192.168.1.10:8080", "http://10.0.0.10", "http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080", "http://[fd00::10]:8080"} {
		for _, header := range []string{"Origin", "Referer", "both"} {
			t.Run(origin+"/"+header, func(t *testing.T) {
				r := httptest.NewRequest("POST", origin+"/api/photos?page=1", strings.NewReader(`{"album":"family"}`))
				r.Header.Set("Content-Type", "application/json")
				if header != "Referer" {
					r.Header.Set("Origin", origin)
				}
				if header != "Origin" {
					r.Header.Set("Referer", origin+"/photos")
				}
				if got := evalDetector(t, "ssrf", r); got.Verdict.Action != Allow || len(got.Hits) != 0 {
					t.Fatalf("browser context treated as an SSRF target: %+v", got)
				}
			})
		}
	}
}

func TestSSRFAllowsSynologyPhotosRequest(t *testing.T) {
	const body = "api=SYNO.Foto.Upload.ConvertedFile&method=list_convert_needed&type=%5B%22photo%22%2C%22video_need_thumbnail%22%5D&version=2"
	for _, cookie := range []string{"", "session=opaque-session-fixture"} {
		r := httptest.NewRequest("POST", "https://nas.example.test/webapi/entry.cgi", strings.NewReader(body))
		r.Proto, r.ProtoMajor, r.ProtoMinor = "HTTP/2.0", 2, 0
		for name, value := range map[string]string{
			"Accept": "*/*", "Accept-Encoding": "gzip, deflate, br", "Accept-Language": "en-US;q=1, zh-Hans-US;q=0.9",
			"Content-Length": "123", "Content-Type": "application/x-www-form-urlencoded", "Priority": "u=3, i",
			"User-Agent": "Synology-Photos_Mobile_2.4.2_iPhone_15_iOS_27.0 (iPhone; iOS 27.0)",
		} {
			r.Header.Set(name, value)
		}
		// The reported cookie is redacted; only a synthetic opaque value is tested.
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		if got := evalDetector(t, "ssrf", r); got.Verdict.Action != Allow || len(got.Hits) != 0 {
			t.Fatalf("Synology Photos request falsely matched SSRF: %+v", got)
		}
	}
}

func TestSSRFContextHeadersDoNotExemptPayloads(t *testing.T) {
	for _, target := range []string{"http://192.168.1.10:8080/admin", "http://169.254.169.254/latest/meta-data/", "http://2130706433/", "file:///etc/passwd"} {
		for _, source := range []string{"query", "form", "json", "multipart", "body", "cookie", "header"} {
			t.Run(source+"/"+target, func(t *testing.T) {
				const origin = "http://192.168.1.10:8080"
				r := httptest.NewRequest("POST", origin+"/fetch", nil)
				field := "url"
				switch source {
				case "query":
					// Header exemptions must not apply to identically named parameters.
					field = "Referer"
					r.URL.RawQuery = url.Values{field: {url.QueryEscape(target)}}.Encode()
				case "form":
					field = "Origin"
					r = httptest.NewRequest("POST", origin+"/fetch", strings.NewReader(url.Values{field: {target}}.Encode()))
					r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				case "json":
					r = httptest.NewRequest("POST", origin+"/fetch", strings.NewReader(`{"url":"`+target+`","url":"https://example.test/"}`))
					r.Header.Set("Content-Type", "application/json")
				case "multipart":
					var body bytes.Buffer
					form := multipart.NewWriter(&body)
					if err := form.WriteField(field, target); err != nil {
						t.Fatal(err)
					}
					if err := form.Close(); err != nil {
						t.Fatal(err)
					}
					r = httptest.NewRequest("POST", origin+"/fetch", &body)
					r.Header.Set("Content-Type", form.FormDataContentType())
				case "body":
					field = ""
					r = httptest.NewRequest("POST", origin+"/fetch", strings.NewReader(target))
					r.Header.Set("Content-Type", "text/plain")
				case "cookie":
					field = "Origin"
					r.AddCookie(&http.Cookie{Name: field, Value: target})
				case "header":
					field = "X-Fetch-URL"
					r.Header.Set(field, target)
				}
				r.Header.Set("Origin", origin)
				r.Header.Set("Referer", origin+"/photos")
				v := NewNormalizedView(r, DefaultViewLimits())
				if err := v.ReadBody(); err != nil {
					t.Fatal(err)
				}
				matches := SSRFDetector().Inspect(v)
				if len(matches) != 1 || matches[0].Source != source || matches[0].Code != "internal_url" {
					t.Fatalf("SSRF payload in %s not detected: %+v", source, matches)
				}
				if field != "" {
					v.excludeFields = []string{field}
					if matches := SSRFDetector().Inspect(v); len(matches) != 0 {
						t.Fatalf("explicit field exclusion ignored: %+v", matches)
					}
				}
			})
		}
	}
}

func TestSSRFProxyContextHeadersAndEnforcement(t *testing.T) {
	for _, mode := range []Mode{ModeObserve, ModeBlock} {
		t.Run(string(mode), func(t *testing.T) {
			f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Referer") == "" || r.Method == "POST" && r.Header.Get("Origin") == "" {
					t.Error("browser context headers lost during forwarding")
				}
				_, _ = io.Copy(w, r.Body)
			}, false)
			f.update(t, func(s *Snapshot) {
				s.Config.Policy.RuleSet.BudgetMillis = 1000
				s.Config.Policy.RuleSet.Rules = []Rule{
					{ID: "ssrf", Category: "ssrf", Mode: mode, Severity: High},
					{ID: "csrf", Category: "csrf", Mode: ModeBlock, Severity: Medium},
				}
			})
			headers := map[string]string{"Referer": f.front.URL + "/photos", "Cookie": "session=business"}
			for _, path := range []string{"/photos", "/assets/app.js"} {
				status, _, _ := proxyCall(t, f, "GET", path, "", headers)
				if status != http.StatusOK {
					t.Fatalf("normal navigation blocked: %s status=%d", path, status)
				}
			}
			headers["Origin"] = f.front.URL
			headers["Content-Type"] = "application/json"
			const body = `{"album":"family"}`
			status, forwarded, _ := proxyCall(t, f, "POST", "/api/photos", body, headers)
			if status != http.StatusOK || string(forwarded) != body || f.requests.Load() != 3 || len(f.sink.snapshot()) != 0 {
				t.Fatalf("normal API request failed or logged an anomaly: status=%d body=%q requests=%d hits=%+v", status, forwarded, f.requests.Load(), f.sink.snapshot())
			}

			headers["Origin"] = "http://192.168.2.10"
			status, _, _ = proxyCall(t, f, "POST", "/api/photos", body, headers)
			hits := f.sink.snapshot()
			if status != http.StatusForbidden || f.requests.Load() != 3 || len(hits) != 1 || hits[0].Category != "csrf" {
				t.Fatalf("CSRF enforcement lost: status=%d requests=%d hits=%+v", status, f.requests.Load(), hits)
			}

			headers["Origin"] = f.front.URL
			status, _, _ = proxyCall(t, f, "GET", "/fetch?url="+url.QueryEscape("http://169.254.169.254/latest/meta-data/"), "", headers)
			wantStatus, wantRequests := http.StatusForbidden, int64(3)
			if mode == ModeObserve {
				wantStatus, wantRequests = http.StatusOK, 4
			}
			hits = f.sink.snapshot()
			if status != wantStatus || f.requests.Load() != wantRequests || len(hits) != 2 || hits[1].Category != "ssrf" || hits[1].Observe != (mode == ModeObserve) {
				t.Fatalf("SSRF enforcement lost: mode=%s status=%d requests=%d hits=%+v", mode, status, f.requests.Load(), hits)
			}
		})
	}
}

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

func TestSSRFPoliciesStayIsolatedAcrossServices(t *testing.T) {
	runtime, base := detectorFixture("ssrf", ModeBlock)
	nas := &Snapshot{Config: CloneConfig(base.Config)}
	nas.Config.Port = 15001
	nas.Config.Policy.RuleSet.Rules[0].Mode = ModeObserve
	nas.Config.Policy.RuleSet.Rules[0].Scope.ExcludePaths = []string{"/webapi/entry.cgi"}
	site := &Snapshot{Config: CloneConfig(base.Config)}
	site.Config.Port = 15002
	api := &Snapshot{Config: CloneConfig(base.Config)}
	api.Config.Port = 15003
	api.Config.Policy.RuleSet.Rules[0].Mode = ModeObserve

	for _, tc := range []struct {
		name     string
		snapshot *Snapshot
		path     string
		action   Action
		hits     int
		observe  bool
	}{
		{"nas_path_exception", nas, "/webapi/entry.cgi", Allow, 0, false},
		{"other_service_same_path", site, "/webapi/entry.cgi", Deny, 1, false},
		{"other_service_fetch", site, "/fetch", Deny, 1, false},
		{"nas_other_path_still_detected", nas, "/fetch", Allow, 1, true},
		{"api_observe_still_detected", api, "/webapi/entry.cgi", Allow, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://example.test"+tc.path, strings.NewReader(`{"url":"http://169.254.169.254/latest/meta-data/"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "iOS-Synology-Photos-iPhone")
			r.Header.Set("Origin", "http://192.168.1.10")
			r.Header.Set("Referer", "http://192.168.1.10/photos")
			result, err := runtime.Explain(tc.snapshot, r, netip.MustParseAddr("192.0.2.10"))
			if err != nil || result.Verdict.Action != tc.action || len(result.Hits) != tc.hits {
				t.Fatalf("service policy leaked or SSRF detection changed: result=%+v err=%v", result, err)
			}
			if tc.hits > 0 {
				hit := result.Hits[0]
				if hit.Category != "ssrf" || hit.PortID != PortID(tc.snapshot.Config.Port) || hit.Observe != tc.observe {
					t.Fatalf("wrong service or detection disposition: %+v", hit)
				}
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
