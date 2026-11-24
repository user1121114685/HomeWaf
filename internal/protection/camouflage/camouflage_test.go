package camouflage

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func TestDefaultConfigUsesGenericHomeNAS(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Profile != "nas-generic" || cfg.Port != DefaultTCPPort || cfg.BanSeconds != DefaultBanDays*24*60*60 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	normalized, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Profile != "nas-generic" || normalized.Port != 8080 {
		t.Fatalf("unexpected normalized defaults: %+v", normalized)
	}
}

func TestNormalizeConfigPreservesExplicitPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Profile = "https"
	cfg.Port = 9443
	normalized, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Port != 9443 || normalized.Profile != "https" {
		t.Fatalf("explicit port was changed: %+v", normalized)
	}
	if normalized.BanSeconds != DefaultBanSeconds || !normalized.BlockPublic {
		t.Fatalf("automatic block policy was changed: %+v", normalized)
	}
}

func TestClassifyNmapStyleProbes(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		payload []byte
		want    string
		min     int
	}{
		{"http", "http", []byte("GET / HTTP/1.0\r\n\r\n"), "GetRequest", 90},
		{"rtsp", "rtsp", []byte("OPTIONS / RTSP/1.0\r\n\r\n"), "RTSPRequest", 90},
		{"sip", "sip", []byte("OPTIONS sip:nm SIP/2.0\r\n\r\n"), "SIPOptions", 90},
		{"tls", "https", []byte{0x16, 0x03, 0x03, 0x00, 0x40}, "TLSSessionReq", 90},
		{"smb", "smb", []byte{0x00, 0x00, 0x00, 0x10, 0xff, 'S', 'M', 'B', 0x72}, "SMBProgNeg", 90},
		{"redis", "redis", []byte("*1\r\n$4\r\nINFO\r\n"), "redis-server", 90},
		{"dns", "dns", []byte("\x00\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x07version\x04bind\x00\x00\x10\x00\x03"), "DNSVersionBindReq", 90},
		{"snmp-v1", "snmp", []byte{0x30, 0x82, 0x00, 0x2f, 0x02, 0x01, 0x00}, "SNMPv1public", 90},
		{"snmp-v3", "snmp", []byte{0x30, 0x3a, 0x02, 0x01, 0x03}, "SNMPv3GetRequest", 90},
		{"ntp", "ntp", append([]byte{0xd9, 0x0a}, make([]byte, 46)...), "NTPRequest", 80},
		{"postgres-ssl", "postgresql", []byte{0x00, 0x00, 0x00, 0x08, 0x04, 0xd2, 0x16, 0x2f}, "PostgreSQLStartup", 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe, _, _, score, _ := classify(tc.profile, tc.payload, "fixture")
			if probe != tc.want || score < tc.min {
				t.Fatalf("probe=%q score=%d, want %q >= %d", probe, score, tc.want, tc.min)
			}
		})
	}
}

func TestDNSResponsePreservesTransactionAndVersion(t *testing.T) {
	query := []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07, 'v', 'e', 'r', 's', 'i', 'o', 'n', 0x04, 'b', 'i', 'n', 'd', 0x00, 0x00, 0x10, 0x00, 0x03}
	response := dnsResponse(query, "BIND 9.18.24")
	if len(response) == 0 || !bytes.Equal(response[:2], query[:2]) || !bytes.Contains(response, []byte("BIND 9.18.24")) {
		t.Fatalf("unexpected DNS response: %x", response)
	}
}

func TestMySQLHandshakeContainsVersion(t *testing.T) {
	response := mysqlHandshake("8.0.36")
	if len(response) < 5 || response[4] != 0x0a || !bytes.Contains(response, []byte("8.0.36")) {
		t.Fatalf("unexpected MySQL handshake: %x", response)
	}
	if got := int(response[0]) | int(response[1])<<8 | int(response[2])<<16; got != len(response)-4 {
		t.Fatalf("invalid MySQL packet length %d for %d bytes", got, len(response)-4)
	}
}

func TestProtocolResponsesUseProbeCompatibleFrames(t *testing.T) {
	redis := redisInfoResponse("7.2.4")
	if !bytes.HasPrefix(redis, []byte("$")) || !bytes.Contains(redis, []byte("redis_version:7.2.4")) {
		t.Fatalf("unexpected Redis response: %q", redis)
	}
	memcached := memcachedUDPResponse([]byte{0, 1, 0, 0, 0, 1, 0, 0}, "1.6.21")
	if len(memcached) < 8 || !bytes.Equal(memcached[:8], []byte{0, 1, 0, 0, 0, 1, 0, 0}) || !bytes.Contains(memcached, []byte("STAT version 1.6.21\r\n")) {
		t.Fatalf("unexpected Memcached UDP response: %x", memcached)
	}
	postgres := postgresErrorResponse()
	if len(postgres) < 5 || postgres[0] != 'E' || int(binary.BigEndian.Uint32(postgres[1:5])) != len(postgres)-1 {
		t.Fatalf("invalid PostgreSQL error packet: %x", postgres)
	}
	snmp := snmpResponse([]byte{0x30, 0x2f, 0x02, 0x01, 0x00, 0x04, 0x06, 'p', 'u', 'b', 'l', 'i', 'c'}, "Net-SNMP 5.9.4")
	if len(snmp) < 2 || snmp[0] != 0x30 || !bytes.Contains(snmp, []byte("Net-SNMP 5.9.4")) {
		t.Fatalf("unexpected SNMP response: %x", snmp)
	}
	smb := smbResponseV2()
	if len(smb) < 4 || int(smb[2])<<8|int(smb[3]) != len(smb)-4 || !bytes.Contains(smb, []byte("WORKGROUP")) || !bytes.Contains(smb, []byte("NAS")) {
		t.Fatalf("unexpected SMB negotiate response: %x", smb)
	}
}

func TestLocalLANUsesExactPrefixes(t *testing.T) {
	prefixes := []string{"192.168.12.0/24", "2001:db8:1234:5600::/64"}
	if !isLocalLANWithCIDRs(netip.MustParseAddr("192.168.12.250"), prefixes) {
		t.Fatal("same IPv4 /24 should be local")
	}
	if isLocalLANWithCIDRs(netip.MustParseAddr("192.168.13.1"), prefixes) {
		t.Fatal("adjacent IPv4 /24 should be non-local")
	}
	if !isLocalLANWithCIDRs(netip.MustParseAddr("2001:db8:1234:5600::42"), prefixes) {
		t.Fatal("same IPv6 /64 should be local")
	}
}

func TestManagementClientAddsOnlyPrivateLANPrefix(t *testing.T) {
	manager := NewManager(nil)
	manager.ObserveManagementClientIP("192.168.44.17")
	if !manager.isLocalLAN(netip.MustParseAddr("192.168.44.220")) {
		t.Fatal("private management client /24 should be local")
	}
	manager.ObserveManagementClientIP("8.8.8.9")
	if manager.isLocalLAN(netip.MustParseAddr("8.8.8.10")) {
		t.Fatal("public management client should not add a local prefix")
	}
}

func TestManagerSendsMySQLHandshakeImmediately(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(probe.Addr().(*net.TCPAddr).Port)
	_ = probe.Close()
	hits := make(chan Hit, 1)
	manager := NewManager(func(hit Hit) Hit {
		hits <- hit
		return hit
	})
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Profile = "mysql"
	cfg.Version = "8.0.36"
	cfg.Port = port
	cfg.ListenIPVersion = "ipv4"
	if err := manager.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	conn, err := net.DialTimeout("tcp4", "127.0.0.1:"+strconv.Itoa(int(port)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	response := make([]byte, 256)
	n, err := conn.Read(response)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(response[:n], []byte("8.0.36")) || response[4] != 0x0a {
		t.Fatalf("MySQL handshake was not sent immediately: %x", response[:n])
	}
	_ = conn.Close()
	select {
	case hit := <-hits:
		if hit.Profile != "mysql" || hit.Probe != "MySQLHandshake" {
			t.Fatalf("unexpected MySQL hit: %+v", hit)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not record MySQL hit")
	}
}
