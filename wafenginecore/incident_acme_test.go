package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafenginecore/accessgate"
	"net/http/httptest"
	"testing"
)

func TestIncidentACMEPrefixDoesNotBypassAccess(t *testing.T) {
	_, _, _ = legacyBodyFixture(t)
	old := accessgate.Get()
	t.Cleanup(func() { accessgate.SetConfig(old) })
	accessgate.SetConfig(&accessgate.Config{GlobalEnable: true, CenterOrigin: "https://login.example.test", CenterHost: "login.example.test", PathPrefix: "/access", CookiePrefix: "test_access", UnauthAction: model.AccessUnauth401})
	waf := newAccessTestEngine()
	h := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "app-code"}}
	for _, path := range []string{
		"/.well-known/acme-challenge/test", "/.well-known/acme-challenge/abcdef123", "/.well-known/acme-challenge/admin/" + testChallengeToken,
		"/api/health", "/healthz", "/health", "/ping", "/api/notify", "/callback", "/notify",
		"/sharetest0", "/sharetest0/", "/api/v2/core/auth/login", "/websocket",
	} {
		r := httptest.NewRequest("GET", "http://app.example.com"+path, nil)
		w := httptest.NewRecorder()
		result := waf.DoAccessGate(w, r, h, &innerbean.WebLog{}, "198.51.100.8")
		if result == accessPass || w.Code != 401 {
			t.Errorf("incident path bypassed Access without an explicit exemption: %s result=%d status=%d", path, result, w.Code)
		}
	}
}

func TestIncidentACMEFastPathRequiresExactRequest(t *testing.T) {
	host := setupChallenge(t, "incident-acme")
	for _, suffix := range []string{"a/" + testChallengeToken, testChallengeToken + "/", testChallengeToken + "?x=1", "%71" + testChallengeToken[1:], "/" + testChallengeToken} {
		r := acmeRequest("GET", suffix)
		if (&WafEngine{}).tryServeACMEChallenge(httptest.NewRecorder(), r, &innerbean.WebLog{HOST_CODE: host}) {
			t.Errorf("noncanonical ACME path exempted: %s", suffix)
		}
	}
}
