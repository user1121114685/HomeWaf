package camouflage

import (
	"net/netip"
	"testing"
)

func TestPreviewUsesClassifierWithoutFirewallOrEvents(t *testing.T) {
	p := ProfileConfig{ProtocolProfile: "http", Port: 18081, Version: "nginx/1.24.0", Enabled: true}
	payload := []byte("GET / HTTP/1.1\r\nUser-Agent: sqlmap/1.8\r\n\r\n")
	hit := Preview(p, netip.MustParseAddr("198.51.100.25"), payload, []string{"192.168.0.0/16"})
	probe, protocol, _, score, _ := classify(p.ProtocolProfile, payload, p.Version)
	if hit.Probe != probe || hit.Protocol != protocol || hit.Score != score || hit.Blocked || hit.ID != "" || hit.Time != 0 {
		t.Fatalf("preview diverged or claimed effects: %+v", hit)
	}
	local := Preview(p, netip.MustParseAddr("192.168.1.2"), payload, []string{"192.168.0.0/16"})
	if !local.LocalLAN {
		t.Fatal("lost LAN exemption")
	}
}
