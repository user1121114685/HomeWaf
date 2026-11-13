package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/internal/proxyidentity"
	"HomeWaf/model"
	"context"
	"net"
	"testing"
)

// The first address is present in the supplied incident export. The other nine
// are user-provided leads, not confirmed events. None is a trusted proxy.
func TestIncidentUntrustedSourcesCannotSupplyIdentity(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_RECORD_PROXY_HEADER
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = "X-Forwarded-For,X-Real-IP" })
	t.Cleanup(func() { global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = old }) })
	sources := []string{"107.148.76.179", "34.34.253.72", "34.34.253.101", "94.154.123.196", "81.6.41.187", "62.106.95.224", "45.9.168.235", "45.149.78.219", "217.15.166.168", "195.211.27.85"}
	headers := map[string]string{
		"X-Forwarded-For": "127.0.0.1", "X-Real-IP": "127.0.0.1", "CF-Connecting-IP": "127.0.0.1",
		"Forwarded": "for=127.0.0.1", "True-Client-IP": "127.0.0.1", "Client-IP": "127.0.0.1",
		"X-Client-IP": "127.0.0.1", "X-Originating-IP": "127.0.0.1", "X-Remote-IP": "127.0.0.1",
		"X-Remote-Addr": "127.0.0.1", "Proxy-Client-IP": "127.0.0.1", "WL-Proxy-Client-IP": "127.0.0.1", "X-ProxyUser-IP": "127.0.0.1",
	}
	for _, source := range sources {
		for _, mode := range []string{"", "nic", "header", "xff_depth", "unknown"} {
			for _, trust := range []string{"", "10.0.0.0/8"} {
				t.Run(source+"/"+mode+"/"+trust, func(t *testing.T) {
					r := newReq(net.JoinHostPort(source, "54321"), headers)
					h := model.Hosts{IPSourceMode: mode, IPRealHeader: "X-Real-IP", IPTrustProxies: trust}
					err, got, _ := (&WafEngine{}).getBizClientIP(r, h)
					if err != nil || got != source {
						t.Fatalf("untrusted header became identity: got=%q err=%v, want=%s", got, err, source)
					}
				})
			}
		}
	}
}

func TestIncidentTrustedChainRejectsAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		want   string
	}{
		{"client supplied leftmost", []string{"127.0.0.1, 198.51.100.8"}, "198.51.100.8"},
		{"multiple field lines", []string{"127.0.0.1", "198.51.100.8"}, "198.51.100.8"},
		{"invalid rightmost", []string{"127.0.0.1, garbage"}, "10.0.0.2"},
		{"empty rightmost", []string{"127.0.0.1,"}, "10.0.0.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReq("10.0.0.2:3000", nil)
			r.Header["X-Forwarded-For"] = tc.values
			err, got, _ := (&WafEngine{}).getBizClientIP(r, model.Hosts{IPSourceMode: "xff_depth", IPTrustProxies: "10.0.0.0/8"})
			if err != nil || got != tc.want {
				t.Fatalf("got %s (%v), want %s", got, err, tc.want)
			}
		})
	}
}

func TestIncidentForwardIdentityUsesPerRequestPolicy(t *testing.T) {
	h := model.Hosts{Port: 15000, IPMode: "proxy", IPSourceMode: "header", IPRealHeader: "My-Client-IP", IPTrustProxies: "10.0.0.2/32"}
	for _, tc := range []struct{ peer, claimed, proto string }{
		{"198.51.100.8", "127.0.0.1", "http"}, {"10.0.0.2", "198.51.100.9", "https"},
	} {
		r := newReq(net.JoinHostPort(tc.peer, "2345"), map[string]string{"My-Client-IP": tc.claimed, "X-Forwarded-Proto": "https"})
		_, resolved, _ := (&WafEngine{}).getBizClientIP(r, h)
		id, ok := proxyidentity.FromContext(legacyForwardIdentity(context.Background(), r, h, resolved))
		if !ok || id.IP != resolved || id.Proto != tc.proto {
			t.Fatalf("unexpected downstream identity %+v", id)
		}
	}
	for _, trust := range []string{"0.0.0.0/0", "::/0"} {
		if IsIPSourceTrustedPeer(model.Hosts{IPSourceMode: "header", IPTrustProxies: trust}, "198.51.100.8") {
			t.Fatalf("universal trust accepted: %s", trust)
		}
	}
}

func TestIncidentCDNCustomXFFKeepsTrustedChain(t *testing.T) {
	h := model.Hosts{IPSourceMode: "cdn_preset", CDNProvider: "edgeone", IPRealHeader: "X-Forwarded-For", IPTrustProxies: "10.0.0.2/32"}
	r := newReq("10.0.0.2:443", map[string]string{"X-Forwarded-For": "127.0.0.1, 198.51.100.8"})
	_, got, _ := (&WafEngine{}).getBizClientIP(r, h)
	if got != "198.51.100.8" {
		t.Fatalf("trusted CDN chain resolved to %q", got)
	}
	r.RemoteAddr = "198.51.100.9:443"
	_, got, _ = (&WafEngine{}).getBizClientIP(r, h)
	if got != "198.51.100.9" {
		t.Fatalf("direct client spoofed CDN identity: %q", got)
	}
}
