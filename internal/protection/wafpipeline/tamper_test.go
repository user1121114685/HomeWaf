package wafpipeline

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestTamperDoesNotPunishVisitorAndRetainsApprovedBaseline(t *testing.T) {
	rule := TamperRule{ID: "home", Path: "/", Mode: ModeBlock, Action: "replace"}
	baseline := NewTamperBaseline(rule, []byte("approved"), "text/plain", "utf-8")
	s := &Snapshot{Secrets: PortSecrets{TamperBaselines: map[string]TamperBaseline{rule.ID: baseline}}}
	c := NewRequestContext(httptest.NewRequest("GET", "http://family.test/", nil), "port:5000", netip.MustParseAddr("203.0.113.4"), DefaultViewLimits())
	response := &http.Response{Header: http.Header{}}
	charset := "utf-8"
	data := (&Forwarder{}).tamperResponse(s, c, rule, response, []byte("changed"), &charset)
	if string(data) != "approved" || len(c.Hits) != 1 || !c.Hits[0].Observe {
		t.Fatal("server tampering was not isolated from visitor punishment")
	}
	rule.Mode = ModeObserve
	rule.Action = "alert"
	if TamperBinding(rule) != baseline.Binding {
		t.Fatal("changing action discarded established baseline")
	}
	rule.Generation++
	if TamperBinding(rule) == baseline.Binding {
		t.Fatal("explicit relearn retained old baseline")
	}
	rule.Generation--
	rule.Mode = ModeBlock
	rule.Action = "replace"
	for _, header := range []string{"Set-Cookie", "Vary", "Cache-Control"} {
		response.Header = http.Header{}
		switch header {
		case "Set-Cookie":
			response.Header.Set(header, "private-user=session")
		case "Vary":
			response.Header.Set(header, "Authorization")
		default:
			response.Header.Set(header, "private")
		}
		data = (&Forwarder{}).tamperResponse(s, c, rule, response, []byte("private-user-page"), &charset)
		if string(data) != "private-user-page" {
			t.Fatal("personalized response was replaced with another visitor's baseline")
		}
	}
}
