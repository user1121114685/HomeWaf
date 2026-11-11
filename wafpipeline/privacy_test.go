package wafpipeline

import (
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestPrivacySnapshotScopeAndObservation(t *testing.T) {
	policy := DefaultPolicy()
	policy.Network.Response.Privacy = &PrivacyPolicy{Mode: ModeBlock, URLs: []PrivacyURL{{Match: "prefix", Value: "/private"}}}
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5121, Protocol: "http", Upstream: "http://127.0.0.1:8000", Policy: policy}}
	if err := (&Runtime{}).Prepare(s); err != nil {
		t.Fatal(err)
	}
	c := NewRequestContext(httptest.NewRequest("GET", "http://family.test/private/profile", nil), "port:5121", netip.MustParseAddr("198.51.100.4"), DefaultViewLimits())
	if !privacyResponseActive(s, c) {
		t.Fatal("configured privacy URL did not match")
	}
	const raw = "邮箱: family@example.com 手机: 13800138000"
	masked := privacyResponse(s, c, []byte(raw))
	if string(masked) == raw || strings.Contains(string(masked), "family@example.com") {
		t.Fatalf("privacy engine did not mask fixture: %q", string(masked))
	}
	s.Config.Policy.Network.Response.Privacy.Mode = ModeObserve
	if got := privacyResponse(s, c, []byte(raw)); string(got) != raw {
		t.Fatal("observe privacy policy modified response")
	}
	for _, hit := range c.Hits {
		if !hit.Observe || strings.Contains(hit.Evidence, "family@example.com") {
			t.Fatal("privacy event punished client or exposed personal data")
		}
	}
	c.Request.URL.Path = "/public"
	if privacyResponseActive(s, c) {
		t.Fatal("privacy policy exceeded scope")
	}
}
