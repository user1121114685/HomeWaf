package wafpipeline

import (
	"HomeWaf/wafenginecore/accessgate"
	"github.com/pquerna/otp/totp"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGateRequiresMigratedTOTPAndRevokesOldSession(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	f := newGateFixture(t, NewGateStore(), "form", "long-safe-pass-42")
	user, ok := f.gate.Store.Account(PortID(f.snap.Config.Port), "owner")
	if !ok {
		t.Fatal("missing fixture user")
	}
	user.TOTPSecret = secret
	user.Version = "mfa-one"
	f.gate.Store.ReplaceAccounts(user.PortID, []GateAccount{user})
	for _, code := range []string{"", "abcdef"} {
		csrf := csrfCookie(t, f)
		res, _ := gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"csrf": {csrf.Value}, "username": {"owner"}, "password": {"long-safe-pass-42"}, "otp": {code}}, nil, csrf)
		if res.StatusCode != 401 || f.hits.Load() != 0 {
			t.Fatal("password alone or invalid OTP bypassed MFA")
		}
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	csrf := csrfCookie(t, f)
	res, _ := gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"csrf": {csrf.Value}, "username": {"owner"}, "password": {"long-safe-pass-42"}, "otp": {code}, "next": {"/"}}, nil, csrf)
	if res.StatusCode != 303 {
		t.Fatalf("valid dynamic code rejected: %d", res.StatusCode)
	}
	var cookie *http.Cookie
	for _, candidate := range res.Cookies() {
		if candidate.Name == gateCookieName(f.snap.Config.Port) {
			cookie = candidate
		}
	}
	if cookie == nil {
		t.Fatal("MFA login did not establish a session")
	}
	res, _ = gateRequest(t, f, "GET", "/", nil, nil, cookie)
	if res.StatusCode != 200 || f.hits.Load() != 1 {
		t.Fatal("authenticated session did not reach business")
	}
	user.TOTPSecret = "KRUGS4ZANFZSAYJA"
	user.Version = "mfa-two"
	f.gate.Store.ReplaceAccounts(user.PortID, []GateAccount{user})
	res, _ = gateRequest(t, f, "GET", "/", nil, nil, cookie)
	if res.StatusCode != 401 || f.hits.Load() != 1 {
		t.Fatal("old session survived OTP rotation")
	}
	for _, event := range f.sink.snapshot() {
		if strings.Contains(event.Evidence, secret) || strings.Contains(event.Evidence, code) {
			t.Fatal("OTP credential appeared in event evidence")
		}
	}
	basic := newGateFixture(t, NewGateStore(), "basic", "long-safe-pass-42")
	account, _ := basic.gate.Store.Account(PortID(basic.snap.Config.Port), "owner")
	account.TOTPSecret = secret
	basic.gate.Store.ReplaceAccounts(account.PortID, []GateAccount{account})
	request, _ := http.NewRequest("GET", basic.server.URL+"/", nil)
	request.SetBasicAuth("owner", "long-safe-pass-42")
	reply, err := basic.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reply.Body.Close()
	if reply.StatusCode != 401 || basic.hits.Load() != 0 {
		t.Fatal("Basic authentication bypassed OTP")
	}
}

func TestGateOnlyIPExceptionsNeverBypassAttackOrBlacklist(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Gate.Enabled = true
		s.Config.Policy.Gate.ExemptIPs = []string{"127.0.0.1"}
		s.Config.Policy.RuleSet.Rules = []Rule{{ID: "sql", Category: "sqli", Severity: High, Mode: ModeBlock}}
		s.Config.Policy.RuleSet.BudgetMillis = 1000
	})
	code, _, _ := proxyCall(t, f, "GET", "/ordinary", "", nil)
	if code != 204 || f.requests.Load() != 1 {
		t.Fatal("gate-only IP exception did not permit normal request")
	}
	code, _, _ = proxyCall(t, f, "GET", "/?q=%27%20OR%201%3D1%20--", "", nil)
	if code != 403 || f.requests.Load() != 1 {
		t.Fatal("gate exception bypassed attack detection")
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "deny", Type: "deny", Value: "127.0.0.1"}}
	})
	code, _, _ = proxyCall(t, f, "GET", "/ordinary", "", nil)
	if code != 403 || f.requests.Load() != 1 {
		t.Fatal("gate exception bypassed IP blacklist")
	}
}

func TestLegacyCredentialStrippingUsesPortSnapshotNotGlobalSettings(t *testing.T) {
	previous := accessgate.Get()
	defer accessgate.SetConfig(previous)
	accessgate.SetConfig(&accessgate.Config{CookiePrefix: "business"})
	a := httptest.NewRequest("GET", "http://a.test/", nil)
	a.Header.Set("Cookie", "old_sso=value; business_sso=ordinary")
	a.Header.Set("X-Old-Token", "private")
	StripGateCredentials(a, GateConfig{CredentialCookiePrefixes: []string{"old"}, CredentialHeaders: []string{"X-Old-Token"}})
	if a.Header.Get("Cookie") != "business_sso=ordinary" || a.Header.Get("X-Old-Token") != "" {
		t.Fatal("port credential policy lost or global settings changed its business cookies")
	}
	b := httptest.NewRequest("GET", "http://b.test/", nil)
	b.Header.Set("Cookie", "old_sso=business")
	StripGateCredentials(b, GateConfig{})
	if b.Header.Get("Cookie") != "old_sso=business" {
		t.Fatal("another port inherited credential namespace")
	}
}
