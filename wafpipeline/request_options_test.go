package wafpipeline

import (
	"HomeWaf/model"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestConfiguredUploadPreservesChoicesAndCompleteUpstreamStream(t *testing.T) {
	var received string
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.WriteHeader(204)
	}, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.BudgetMillis = 1000
		s.Config.Policy.RuleSet.Rules = []Rule{{ID: "upload", Category: "upload", Severity: High, Mode: ModeBlock}}
		s.Config.Policy.RuleSet.UploadOptions = &model.UploadSecurityConfig{IsEnable: 1, CheckExt: 1, ExtBlacklist: "secret", CheckSize: 1, MaxSizeKB: 128, OverLimitAction: "pass", ExcludePaths: "/dav"}
	})
	for _, tc := range []struct {
		file, path string
		size, want int
	}{{"notes.sh", "/upload", 90 << 10, 204}, {"notes.secret", "/upload", 20, 403}, {"notes.secret", "/dav/file", 20, 204}, {"big.txt", "/upload", 150 << 10, 403}} {
		body := uploadRequest(t, tc.file, strings.Repeat("n", tc.size)).String()
		before := f.requests.Load()
		code, _, _ := proxyCall(t, f, "POST", tc.path, body, map[string]string{"Content-Type": "multipart/form-data; boundary=HomeWafFixtureBoundary"})
		if code != tc.want {
			t.Fatalf("%s %s: %d", tc.file, tc.path, code)
		}
		if tc.want == 204 {
			if f.requests.Load() != before+1 || received != body {
				t.Fatal("accepted upload changed or was truncated")
			}
		} else if f.requests.Load() != before {
			t.Fatal("denied upload reached upstream")
		}
	}
}
func TestConfiguredCSRFAndAntiLeechEnforceWithoutBlockingExceptions(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.Rules = []Rule{{ID: "csrf", Category: "csrf", Severity: Medium, Mode: ModeBlock}}
		s.Config.Policy.RuleSet.CSRFOptions = &model.CsrfConfig{IsEnable: 1, ProtectMethods: "POST, PUT", AllowedOrigins: "https://*.family.test", ExcludePaths: "/webhook"}
		s.Config.Policy.RuleSet.AntiLeech = &AntiLeechPolicy{Mode: ModeBlock, Settings: model.AntiLeechConfig{IsEnableAntiLeech: 1, FileTypes: "png", ValidReferers: "server_names;*.family.test", Action: "redirect", RedirectURL: "/no-ref"}}
	})
	for _, tc := range []struct {
		method, path, origin, referer string
		want                          int
	}{{"POST", "/save", "https://evil.test", "", 403}, {"POST", "/webhook/callback", "https://evil.test", "", 204}, {"PUT", "/save", "https://photo.family.test", "", 204}, {"GET", "/a.png", "", "https://evil.test/", 302}, {"GET", "/a.png", "", "https://photo.family.test/", 204}} {
		before := f.requests.Load()
		code, _, head := proxyCall(t, f, tc.method, tc.path, "", map[string]string{"Origin": tc.origin, "Referer": tc.referer})
		if code != tc.want {
			t.Fatalf("%s %s: %d", tc.method, tc.path, code)
		}
		if tc.want == 204 {
			if f.requests.Load() != before+1 {
				t.Fatal("normal request not forwarded")
			}
		} else if f.requests.Load() != before {
			t.Fatal("policy rejection forwarded")
		}
		if code == 302 && head.Get("Location") != "/no-ref" {
			t.Fatal("configured redirect lost")
		}
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.AntiLeech.Mode = ModeObserve })
	code, _, _ := proxyCall(t, f, "GET", "/a.png", "", nil)
	if code != 204 {
		t.Fatal("observe referer policy blocked")
	}
}
func TestMigratedAccessChallengeIsPortBoundAndExpires(t *testing.T) {
	f := newGateFixture(t, NewGateStore(), "form", "safe-test-password")
	f.snap.Config.Policy.Gate.Enabled = false
	f.snap.Config.Policy.RuleSet.Challenge = ChallengePolicy{Mode: ModeBlock}
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Accept": "text/html", "Sec-Fetch-Mode": "navigate", "User-Agent": "ordinary-browser"}
	res, _ := gateRequest(t, f, "GET", "/files", nil, headers)
	if res.StatusCode != 429 || f.hits.Load() != 0 {
		t.Fatal("challenge bypassed")
	}
	var pending *http.Cookie
	for _, cookie := range res.Cookies() {
		if strings.HasPrefix(cookie.Name, "hw_challenge_pending_") {
			pending = cookie
		}
	}
	if pending == nil {
		t.Fatal("challenge has no proof")
	}
	res, _ = gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/challenge", url.Values{"token": {pending.Value}, "next": {"/files"}}, headers, pending)
	if res.StatusCode != 303 {
		t.Fatalf("challenge completion failed: %d", res.StatusCode)
	}
	var passed *http.Cookie
	for _, cookie := range res.Cookies() {
		if strings.HasPrefix(cookie.Name, "hw_challenge_") && !strings.Contains(cookie.Name, "pending") {
			passed = cookie
		}
	}
	if passed == nil {
		t.Fatal("missing passed proof")
	}
	res, _ = gateRequest(t, f, "GET", "/files", nil, headers, passed)
	if res.StatusCode != 200 || f.hits.Load() != 1 {
		t.Fatal("passed challenge did not forward")
	}
	f.snap.Config.Policy.RuleSet.Challenge.Until = 1
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	res, _ = gateRequest(t, f, "GET", "/files", nil, headers)
	if res.StatusCode != 200 || f.hits.Load() != 2 {
		t.Fatal("expired challenge still blocked")
	}
}
