package camouflage

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigUsesHTTPWithoutVendorIdentity(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Profile != "http" || cfg.Port != DefaultTCPPort || cfg.BanSeconds != DefaultBanDays*24*60*60 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	normalized, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Profile != "http" || normalized.Port != 8080 {
		t.Fatalf("unexpected normalized defaults: %+v", normalized)
	}
}

func TestNormalizeDefenseConfigValidatesProfilesAndLocalCIDRs(t *testing.T) {
	config := DefaultDefenseConfig()
	config.LocalCIDRMode = "custom"
	config.LocalCIDRs = []string{"192.168.12.9/24", "2001:db8:1234:5600::1/64"}
	config.Profiles = []ProfileConfig{{
		ID: "home-web", Name: "家庭 Web", Enabled: true, Port: 8080,
		ProtocolProfile: "synology-http", Version: "DSM 7.2.1", ServerName: "nas",
	}}

	normalized, err := NormalizeDefenseConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.Profiles) != 1 || normalized.LocalCIDRs[0] != "192.168.12.0/24" || normalized.LocalCIDRs[1] != "2001:db8:1234:5600::/64" {
		t.Fatalf("unexpected normalized defense config: %+v", normalized)
	}

	config.LocalCIDRs = []string{"192.168.0.0/16"}
	if _, err := NormalizeDefenseConfig(config); err == nil {
		t.Fatal("broader than /24 custom IPv4 prefix should be rejected")
	}
}

func TestLegacyGenericNASProfilesMigrateToHTTP(t *testing.T) {
	legacy, err := NormalizeProfileConfig(ProfileConfig{
		ID: "legacy-nas", Name: "通用家庭 NAS", ProtocolProfile: "nas-generic", Version: "NAS Web Console",
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.ProtocolProfile != "http" || legacy.Version != "nginx/1.24.0" {
		t.Fatalf("legacy profile was not migrated to the generic HTTP image: %+v", legacy)
	}

	legacySingle, err := NormalizeConfig(Config{Profile: "nas-generic", Version: "NAS Web Console", Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	if legacySingle.Profile != "http" || legacySingle.Version != "nginx/1.24.0" {
		t.Fatalf("legacy single-listener profile was not migrated: %+v", legacySingle)
	}
}

func TestNmapEvidenceSeparatesHTTPBrandAppearanceFromDetection(t *testing.T) {
	profile, ok := ProfileByCode("synology-http")
	if !ok {
		t.Fatal("Synology HTTP appearance profile is missing")
	}
	if profile.NmapService != "http" || !strings.Contains(profile.NmapEvidence, "不据此确认页面品牌") && !strings.Contains(profile.NmapEvidence, "只能识别为 HTTP") {
		t.Fatalf("Nmap evidence overstates vendor detection: %+v", profile)
	}
	if profile.NmapProbe != "GetRequest" || !profile.VersionEditable {
		t.Fatalf("HTTP image probe metadata is incomplete: %+v", profile)
	}

	for _, code := range []string{"rsync", "vnc", "tftp", "nfs", "upnp", "nat-pmp", "coap", "ipp", "telnet"} {
		if _, ok := ProfileByCode(code); !ok {
			t.Errorf("expected Nmap-backed home profile %q", code)
		}
	}
	if _, ok := ProfileByCode("mdns"); !ok {
		t.Fatal("expected Nmap-backed mDNS home profile")
	}
	tftp, _ := ProfileByCode("tftp")
	if tftp.VersionEditable {
		t.Fatal("TFTP profile should not offer a version field its response cannot report")
	}
	for code, expected := range map[string][2]string{
		"nfs":     {"nfs", "NFSPROC_NULL"},
		"upnp":    {"upnp", "UPNP_MSEARCH"},
		"nat-pmp": {"nat-pmp", "NAT_PMP_ADDR"},
		"coap":    {"coap", "coap-request"},
		"ipp":     {"ipp", "GetRequest"},
		"telnet":  {"telnet", "NULL / GenericLines"},
	} {
		profile, _ := ProfileByCode(code)
		if profile.NmapService != expected[0] || profile.NmapProbe != expected[1] {
			t.Errorf("unexpected Nmap mapping for %s: service=%q probe=%q", code, profile.NmapService, profile.NmapProbe)
		}
	}
}

func TestSynologyHTTPImageReportsConfigurableVersionWithNginxHeader(t *testing.T) {
	profile, _ := ProfileByCode("synology-http")
	cfg := Config{Profile: profile.Code, Version: "DSM 7.2.1"}
	response := httpResponse(cfg, "Synology DiskStation", "nginx")
	if !bytes.Contains(response, []byte("Server: nginx\r\n")) || !bytes.Contains(response, []byte("X-Syno-Version: DSM 7.2.1\r\n")) {
		t.Fatalf("HTTP image did not report its configured response fields: %s", response)
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
		{"rsync", "rsync", nil, "RsyncProtocolVersion", 90},
		{"vnc", "vnc", []byte("RFB 003.008\n"), "VNCProtocolVersion", 90},
		{"tftp", "tftp", []byte{0x00, 0x01, 'n', 'm', 'a', 'p', 0, 'o', 'c', 't', 'e', 't', 0}, "TFTPReadRequest", 80},
		{"upnp", "upnp", []byte("M-SEARCH * HTTP/1.1\r\nHost: 239.255.255.250:1900\r\nMan: \"ssdp:discover\"\r\nMX: 5\r\nST: ssdp:all\r\n\r\n"), "UPNP_MSEARCH", 90},
		{"coap", "coap", []byte{0x40, 0x01, 0x01, 0xce, 0xbb, '.', 'w', 'e', 'l', 'l', '-', 'k', 'n', 'o', 'w', 'n', 0x04, 'c', 'o', 'r', 'e'}, "coap-request", 90},
		{"nfs", "nfs", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 1, 0x86, 0xa3, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "NFSPROC_NULL", 90},
		{"ipp", "ipp", []byte("GET / HTTP/1.0\r\n\r\n"), "GetRequest", 90},
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

func TestNmapDatabaseProbeResponses(t *testing.T) {
	upnpProfile, _ := ProfileByCode("upnp")
	upnpProbe := []byte("M-SEARCH * HTTP/1.1\r\nHost: 239.255.255.250:1900\r\nMan: \"ssdp:discover\"\r\nMX: 5\r\nST: ssdp:all\r\n\r\n")
	upnp := udpResponse(upnpProfile, Config{ServerName: "nas", Version: "2.3.7"}, upnpProbe)
	if !bytes.HasPrefix(upnp, []byte("HTTP/1.1 200 OK\r\n")) || !bytes.Contains(upnp, []byte("MiniUPnPd/2.3.7\r\n")) || !bytes.Contains(upnp, []byte("ST: upnp:rootdevice\r\n")) {
		t.Fatalf("unexpected SSDP response: %q", upnp)
	}

	coapProfile, _ := ProfileByCode("coap")
	coapProbe := []byte{0x40, 0x01, 0x01, 0xce, 0xbb, '.', 'w', 'e', 'l', 'l', '-', 'k', 'n', 'o', 'w', 'n', 0x04, 'c', 'o', 'r', 'e'}
	coap := udpResponse(coapProfile, Config{Version: "libcoap/4.3.5"}, coapProbe)
	if len(coap) < 6 || !bytes.Equal(coap[:4], []byte{0x60, 0x45, 0x01, 0xce}) || !bytes.Contains(coap, []byte("libcoap/4.3.5")) {
		t.Fatalf("unexpected CoAP 2.05 response: %x", coap)
	}
	if invalid := udpResponse(coapProfile, Config{}, []byte("not CoAP")); len(invalid) != 0 {
		t.Fatalf("invalid CoAP request received a response: %x", invalid)
	}

	natPMPProfile, _ := ProfileByCode("nat-pmp")
	natPMP := udpResponse(natPMPProfile, Config{}, []byte{0, 0})
	if !bytes.Equal(natPMP, []byte{0, 0x80, 0, 0, 0, 0, 0x0e, 0x10, 192, 0, 2, 1}) {
		t.Fatalf("unexpected NAT-PMP public address response: %x", natPMP)
	}

	nfsProfile, _ := ProfileByCode("nfs")
	nfsProbe := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 1, 0x86, 0xa3, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	nfs := udpResponse(nfsProfile, Config{}, nfsProbe)
	if len(nfs) != 24 || !bytes.Equal(nfs[:8], []byte{0, 0, 0, 0, 0, 0, 0, 1}) || binary.BigEndian.Uint32(nfs[20:24]) != 0 {
		t.Fatalf("unexpected NFS NULL RPC response: %x", nfs)
	}
}

func TestNmapRecognizesCommonServiceGreetings(t *testing.T) {
	ftp, _ := ProfileByCode("ftp")
	if got := string(tcpGreeting(ftp, Config{Version: "vsftpd 3.0.5"})); got != "220 (vsFTPd 3.0.5)\r\n" {
		t.Fatalf("unexpected vsFTPd greeting: %q", got)
	}
	ftps, _ := ProfileByCode("ftps")
	if got := string(tcpGreeting(ftps, Config{Version: "FileZilla Server 1.8.2"})); got != "220 FileZilla Server version 1.8.2\r\n" {
		t.Fatalf("unexpected FileZilla greeting: %q", got)
	}
	imap, _ := ProfileByCode("imap")
	if got := string(tcpGreeting(imap, Config{})); got != "* OK [CAPABILITY IMAP4rev1] Dovecot ready.\r\n" {
		t.Fatalf("unexpected Dovecot IMAP greeting: %q", got)
	}
	pop3, _ := ProfileByCode("pop3")
	if got := string(tcpGreeting(pop3, Config{})); got != "+OK Dovecot ready.\r\n" {
		t.Fatalf("unexpected Dovecot POP3 greeting: %q", got)
	}
	telnet, _ := ProfileByCode("telnet")
	telnetGreeting := tcpGreeting(telnet, Config{})
	if !bytes.HasPrefix(telnetGreeting, []byte{0xff, 0xfd, 0x01, 0xff, 0xfd, 0x1f, 0xff, 0xfb, 0x01, 0xff, 0xfb, 0x03}) {
		t.Fatalf("unexpected Telnet IAC negotiation: %x", telnetGreeting)
	}

	ippProfile, _ := ProfileByCode("ipp")
	ipp := tcpResponse(ippProfile, Config{Version: "2.4.7"}, []byte("GET / HTTP/1.0\r\n\r\n"))
	if !bytes.HasPrefix(ipp, []byte("HTTP/1.0 200 OK\r\n")) || !bytes.Contains(ipp, []byte("Server: CUPS/2.4.7\r\n")) {
		t.Fatalf("unexpected CUPS response: %s", ipp)
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

func TestMQTTRespondsToNmapConnectProbe(t *testing.T) {
	profile, _ := ProfileByCode("mqtt")
	config := DefaultConfig()
	connect := []byte{0x10, 0x10, 0x00, 0x04, 'M', 'Q', 'T', 'T', 0x04, 0x02, 0x00, 0x1e, 0x00, 0x04, 'n', 'm', 'a', 'p'}
	response := tcpResponse(profile, config, connect)
	if !bytes.Equal(response, []byte{0x20, 0x02, 0x00, 0x00}) {
		t.Fatalf("unexpected MQTT CONNACK response: %x", response)
	}
	if got := tcpResponse(profile, config, []byte("not an MQTT CONNECT")); len(got) != 0 {
		t.Fatalf("invalid request received an unsolicited MQTT response: %x", got)
	}
}

func TestTFTPReturnsNmapSoftmatchErrorFrame(t *testing.T) {
	profile, _ := ProfileByCode("tftp")
	request := []byte{0x00, 0x01, 'r', '7', 't', 'f', 't', 'p', '.', 't', 'x', 't', 0, 'o', 'c', 't', 'e', 't', 0}
	response := udpResponse(profile, DefaultConfig(), request)
	if !bytes.HasPrefix(response, []byte{0x00, 0x05, 0x00, 0x02}) || response[len(response)-1] != 0 {
		t.Fatalf("unexpected TFTP ERROR response: %x", response)
	}
}

func TestMDNSRespondsToNmapDNSServiceDiscoveryProbe(t *testing.T) {
	profile, _ := ProfileByCode("mdns")
	query := []byte{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 9, '_', 's', 'e', 'r', 'v', 'i', 'c', 'e', 's', 7, '_', 'd', 'n', 's', '-', 's', 'd', 4, '_', 'u', 'd', 'p', 5, 'l', 'o', 'c', 'a', 'l', 0, 0, 12, 0x80, 1}
	response := udpResponse(profile, DefaultConfig(), query)
	if len(response) <= 12 || !bytes.Equal(response[:12], []byte{0, 0, 0x84, 0, 0, 0, 0, 5, 0, 0, 0, 0}) {
		t.Fatalf("unexpected mDNS response header: %x", response)
	}
	if !bytes.Contains(response, []byte("_http")) || !bytes.Contains(response, []byte("_smb")) {
		t.Fatalf("home service records are missing: %x", response)
	}
	probe, protocol, _, score, _ := classify("mdns", query, "DNS-SD")
	if probe != "DNS_SD_QU" || protocol != "mDNS" || score < 80 {
		t.Fatalf("unexpected mDNS score evidence: probe=%s protocol=%s score=%d", probe, protocol, score)
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

func TestCustomLocalCIDRsReplaceAutomaticPrefixes(t *testing.T) {
	manager := NewManager(nil)
	manager.SetLocalCIDRs("custom", []string{"192.168.12.0/24"})
	manager.ObserveManagementClientIP("192.168.99.17")
	if !manager.isLocalLAN(netip.MustParseAddr("192.168.12.42")) {
		t.Fatal("address in custom prefix should be local")
	}
	if manager.isLocalLAN(netip.MustParseAddr("192.168.99.17")) {
		t.Fatal("management client prefix should not override custom mode")
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

func TestManagerAnswersNmapUDPProbes(t *testing.T) {
	cases := []struct {
		profile string
		probe   string
		want    []byte
	}{
		{
			profile: "upnp",
			probe:   "UPNP_MSEARCH",
			want:    []byte("M-SEARCH * HTTP/1.1\r\nHost: 239.255.255.250:1900\r\nMan: \"ssdp:discover\"\r\nMX: 5\r\nST: ssdp:all\r\n\r\n"),
		},
		{
			profile: "coap",
			probe:   "coap-request",
			want:    []byte{0x40, 0x01, 0x01, 0xce, 0xbb, '.', 'w', 'e', 'l', 'l', '-', 'k', 'n', 'o', 'w', 'n', 0x04, 'c', 'o', 'r', 'e'},
		},
		{
			profile: "nat-pmp",
			probe:   "NAT_PMP_ADDR",
			want:    []byte{0, 0},
		},
		{
			profile: "nfs",
			probe:   "NFSPROC_NULL",
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 1, 0x86, 0xa3, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			reservation, err := net.ListenPacket("udp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := uint16(reservation.LocalAddr().(*net.UDPAddr).Port)
			_ = reservation.Close()

			hits := make(chan Hit, 1)
			manager := NewManager(func(hit Hit) Hit {
				hits <- hit
				return hit
			})
			config := DefaultConfig()
			config.Enabled = true
			config.Profile = tc.profile
			config.Port = port
			config.ListenIPVersion = "ipv4"
			if err := manager.Apply(config); err != nil {
				t.Fatal(err)
			}
			defer manager.Stop()

			conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(port)})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			if _, err := conn.Write(tc.want); err != nil {
				t.Fatal(err)
			}
			response := make([]byte, 2048)
			n, err := conn.Read(response)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				t.Fatal("listener returned an empty response")
			}
			select {
			case hit := <-hits:
				if hit.Probe != tc.probe || hit.Score < 80 {
					t.Fatalf("unexpected Nmap probe score: %+v", hit)
				}
			case <-time.After(time.Second):
				t.Fatal("listener did not record the probe")
			}
		})
	}
}
