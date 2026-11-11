package wafpipeline

import (
	"net/http"
	"strings"
	"testing"
)

func TestLocalHTTPSRedirectAndDisabledRouteNeverReachOrigin(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Gate.Enabled = true
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"*"}, TargetType: "https_redirect", RedirectPort: 443, RedirectCode: 308}}
	})
	code, _, header := proxyCall(t, f, "GET", "/share/a%2Fb?q=x%20y", "", nil)
	if code != 308 || header.Get("Location") != "https://127.0.0.1/share/a%2Fb?q=x%20y" || f.requests.Load() != 0 {
		t.Fatal("transport redirect lost wire URI, required HTTP login, or reached origin")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Access.Enabled = false })
	code, _, _ = proxyCall(t, f, "GET", "/", "", nil)
	if code != 503 {
		t.Fatal("redirect bypassed master switch")
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Access.Enabled = true
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "deny", Type: "deny", Value: "127.0.0.1"}}
	})
	code, _, _ = proxyCall(t, f, "GET", "/", "", nil)
	if code != 403 {
		t.Fatal("redirect bypassed blacklist")
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = nil
		s.Config.Policy.Gate.Enabled = false
		s.Config.Routes[0].TargetType = "unavailable"
	})
	code, _, _ = proxyCall(t, f, "GET", "/", "", nil)
	if code != 503 || f.requests.Load() != 0 {
		t.Fatal("disabled source became an active upstream")
	}
}
func TestProxyLoopTerminatesAndHopHeaderSurvivesConnectionRemoval(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(r.Header.Get("waf_req_hop"))) }, false)
	f.update(t, func(s *Snapshot) { s.Config.Policy.Network.MaxProxyHops = 3 })
	code, body, _ := proxyCall(t, f, "GET", "/once", "", map[string]string{"waf_req_hop": "1", "Connection": "waf_req_hop"})
	if code != 200 || strings.TrimSpace(string(body)) != "2" {
		t.Fatal("hop metadata was dropped or not incremented")
	}
	before := f.runtime.Requests.Load()
	originBefore := f.requests.Load()
	f.update(t, func(s *Snapshot) { s.Config.Upstream = f.front.URL })
	code, _, _ = proxyCall(t, f, "GET", "/loop", "", nil)
	if code != 508 || f.runtime.Requests.Load()-before != 4 || f.requests.Load() != originBefore {
		t.Fatal("self-proxy loop did not stop at configured bound")
	}
	for _, hit := range f.sink.snapshot() {
		if hit.Category == "proxy_loop" && !hit.Observe {
			t.Fatal("configuration loop could punish visitor")
		}
	}
}
