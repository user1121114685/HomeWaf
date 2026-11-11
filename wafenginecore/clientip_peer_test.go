package wafenginecore

import (
	"HomeWaf/model"
	"testing"
)

func TestXFFTrustedListRejectsUntrustedDirectPeer(t *testing.T) {
	r := newReq("198.51.100.10:54321", map[string]string{"X-Forwarded-For": "203.0.113.99"})
	_, got, _ := (&WafEngine{}).getBizClientIP(r, model.Hosts{IPSourceMode: "xff_depth", IPTrustProxies: "10.0.0.0/8"})
	if got != "198.51.100.10" {
		t.Fatalf("untrusted peer supplied authoritative IP: %s", got)
	}
}
