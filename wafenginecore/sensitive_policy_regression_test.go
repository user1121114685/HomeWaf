package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/internal/multimatch"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacySensitiveDenyPriorityObserveAndReplacementBounds(t *testing.T) {
	m := &multimatch.Machine{}
	data := map[string]any{"replace": model.Sensitive{Content: "replace", Action: "replace", CheckDirection: "in"}, "reject": model.Sensitive{Content: "reject", Action: "deny", CheckDirection: "in"}, "output": model.Sensitive{Content: "output", Action: "deny", CheckDirection: "out"}}
	if err := m.BuildByCustom([][]rune{[]rune("replace"), []rune("reject"), []rune("output")}, data); err != nil {
		t.Fatal(err)
	}
	waf := &WafEngine{}
	waf.sensitive.Store(&sensitiveSnapshot{machine: m, in: true, out: true})
	host := &wafenginmodel.HostSafe{}
	r := httptest.NewRequest("POST", "http://example.test/?q=replace", strings.NewReader("reject"))
	result := waf.CheckSensitive(r, &innerbean.WebLog{URL: r.URL.RequestURI(), BODY: "output reject"}, nil, host, nil)
	if !result.IsBlock || r.URL.Query().Get("q") != "replace" {
		t.Fatal("URL replacement hid body deny")
	}
	host.Host.LogOnlyMode = 1
	r = httptest.NewRequest("POST", "http://example.test/?q=replace", strings.NewReader("replace"))
	result = waf.CheckSensitive(r, &innerbean.WebLog{URL: r.URL.RequestURI(), BODY: "replace"}, nil, host, nil)
	body, _ := io.ReadAll(r.Body)
	if result.IsBlock || string(body) != "replace" || r.URL.Query().Get("q") != "replace" {
		t.Fatal("observe mutated legacy request")
	}
	r = httptest.NewRequest("GET", "http://example.test/replace?q=replace&empty=", nil)
	if err := waf.ReplaceURLContent(r, []string{"replace", "not-present"}, "safe"); err != nil {
		t.Fatal(err)
	}
	if r.URL.Path != "/safe" || r.URL.Query().Get("q") != "safe" {
		t.Fatal("subsequent replacement restored original URL")
	}
	large := strings.Repeat("x", (1<<20)+7)
	r = httptest.NewRequest("POST", "http://example.test/", strings.NewReader(large))
	if err := waf.ReplaceBodyContent(r, []string{"x"}, "yy"); err == nil {
		t.Fatal("unbounded replacement accepted")
	}
	body, _ = io.ReadAll(r.Body)
	if string(body) != large {
		t.Fatal("bounded legacy replacement truncated original body")
	}
}
