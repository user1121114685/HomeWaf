package wafpipeline

import (
	"net/http"
	"testing"
)

func TestAPITemplateRequiresTokenWithoutLoginOrCredentialLeak(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-HomeWaf-Token") != "" {
			t.Error("service credential reached upstream")
		}
		w.WriteHeader(204)
	}, false)
	f.update(t, func(s *Snapshot) {
		for _, template := range BuiltinTemplates() {
			if template.ID == "api-webhook" {
				s.Config.Policy = template.Policy
			}
		}
		s.Secrets.ServiceTokenHashes = []string{tokenHash("service-test-credential")}
	})
	for _, value := range []string{"", "incorrect"} {
		code, _, header := proxyCall(t, f, "GET", "/api/health", "", map[string]string{"Accept": "text/html", "X-HomeWaf-Token": value})
		if code != 401 || header.Get("Location") != "" || f.requests.Load() != 0 {
			t.Fatal("token-only gate redirected or reached upstream without valid token")
		}
	}
	req, _ := http.NewRequest("GET", f.front.URL+"/api/health", nil)
	req.Header.Add("X-HomeWaf-Token", "service-test-credential")
	req.Header.Add("X-HomeWaf-Token", "incorrect")
	response, err := f.front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || f.requests.Load() != 0 {
		t.Fatal("ambiguous token accepted")
	}
	code, _, _ := proxyCall(t, f, "GET", "/api/health", "", map[string]string{"X-HomeWaf-Token": "service-test-credential"})
	if code != 204 || f.requests.Load() != 1 {
		t.Fatal("valid token did not reach business")
	}
	f.update(t, func(s *Snapshot) { s.Secrets.ServiceTokenHashes = nil })
	code, _, _ = proxyCall(t, f, "GET", "/api/health", "", map[string]string{"X-HomeWaf-Token": "service-test-credential"})
	if code != 401 || f.requests.Load() != 1 {
		t.Fatal("revoked token retained access")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Gate.RequireToken = false })
	code, _, _ = proxyCall(t, f, "GET", "/api/health", "", nil)
	if code != 204 || f.requests.Load() != 2 {
		t.Fatal("ordinary disabled gate changed semantics")
	}
}

func TestGateTokenHeaderMustNotConsumeBusinessOrForwardingCredentials(t *testing.T) {
	for _, name := range []string{"Authorization", "Cookie", "X-Real-IP", "X-Forwarded-For", "X-Waf-Custom-Xff-Control", "X-Bad\nHeader"} {
		p := DefaultPolicy()
		p.Gate.TokenHeader = name
		if ValidatePolicy(p) == nil {
			t.Fatalf("unsafe token namespace accepted: %q", name)
		}
	}
	p := DefaultPolicy()
	p.Gate.TokenHeader = "X-Source-Token"
	if err := ValidatePolicy(p); err != nil {
		t.Fatal(err)
	}
}
