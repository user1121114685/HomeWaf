package wafpipeline

import (
	"net/http"
	"testing"
)

func TestExplicitCallbackExceptionCannotBypassLoginOrBlacklist(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Gate.Enabled = true
		s.Config.Policy.RuleSet.BudgetMillis = 1000
		s.Config.Policy.RuleSet.Callback = &CallbackPolicy{Mode: ModeBlock, Networks: []string{"127.0.0.1"}, Scope: Scope{Paths: []string{"/callback"}}}
		s.Config.Policy.RuleSet.Rules = []Rule{{ID: "sql", Category: "sqli", Severity: High, Mode: ModeBlock}}
	})
	attack := "?q=%27%20OR%201%3D1%20--"
	code, _, _ := proxyCall(t, f, "POST", "/callback"+attack, "", nil)
	if code != 401 || f.requests.Load() != 0 {
		t.Fatal("callback bypassed login")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Gate.Enabled = false })
	code, _, _ = proxyCall(t, f, "POST", "/callback"+attack, "", nil)
	if code != 204 || f.requests.Load() != 1 {
		t.Fatal("explicit callback exception did not apply")
	}
	code, _, _ = proxyCall(t, f, "GET", "/ordinary"+attack, "", nil)
	if code != 403 || f.requests.Load() != 1 {
		t.Fatal("callback exception escaped path scope")
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "deny", Type: "deny", Value: "127.0.0.1"}}
	})
	code, _, _ = proxyCall(t, f, "POST", "/callback"+attack, "", nil)
	if code != 403 || f.requests.Load() != 1 {
		t.Fatal("callback bypassed blacklist")
	}
	template := TemplateFromPort("callback", "callback", f.current.Load().Config)
	if len(template.Policy.RuleSet.Callback.Networks) != 0 {
		t.Fatal("template exported manual callback IPs")
	}
}
