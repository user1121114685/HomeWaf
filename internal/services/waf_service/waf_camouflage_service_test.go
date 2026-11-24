package waf_service

import (
	"net"
	"strconv"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/protection/camouflage"
)

func TestCamouflageServiceBlocksEveryNonLocalHitForNinetyDays(t *testing.T) {
	service := newWafCamouflageService()
	defer service.Stop()
	oldLogQueue := global.GQEQUE_LOG_DB
	global.GQEQUE_LOG_DB = nil
	t.Cleanup(func() { global.GQEQUE_LOG_DB = oldLogQueue })

	var requests []request.WafFirewallIPBlockAddReq
	service.blockIP = func(req request.WafFirewallIPBlockAddReq) error {
		requests = append(requests, req)
		return nil
	}

	before := time.Now().Add(90 * 24 * time.Hour).Unix()
	first := service.handleHit(camouflage.Hit{ClientIP: "192.168.60.44", Port: 8080, Profile: "http", Score: 94, Probe: "GetRequest"})
	second := service.handleHit(camouflage.Hit{ClientIP: "192.168.60.44", Port: 8080, Profile: "http", Score: 94, Probe: "GetRequest"})
	if len(requests) != 1 {
		t.Fatalf("expected one firewall write for repeated hits, got %d", len(requests))
	}
	got := requests[0]
	if got.IP != "192.168.60.44" || got.BlockType != "auto" {
		t.Fatalf("unexpected firewall request: %+v", got)
	}
	if delta := got.ExpireTime - before; delta < -2 || delta > 2 {
		t.Fatalf("expected a 90-day expiry, difference=%d seconds", delta)
	}
	if !first.Blocked || !second.Blocked {
		t.Fatalf("non-local private-network hits were not blocked: first=%+v second=%+v", first, second)
	}
}

func TestCamouflageServiceDoesNotBlockLocalLANHit(t *testing.T) {
	service := newWafCamouflageService()
	defer service.Stop()
	oldLogQueue := global.GQEQUE_LOG_DB
	global.GQEQUE_LOG_DB = nil
	t.Cleanup(func() { global.GQEQUE_LOG_DB = oldLogQueue })

	called := false
	service.blockIP = func(request.WafFirewallIPBlockAddReq) error {
		called = true
		return nil
	}
	hit := service.handleHit(camouflage.Hit{ClientIP: "192.168.99.240", Port: 8080, Profile: "http", Score: 94, LocalLAN: true})
	if called {
		t.Fatal("local LAN hit should not be sent to the firewall")
	}
	if !hit.LocalLAN || hit.Blocked {
		t.Fatalf("unexpected local event: %+v", hit)
	}
}

func TestCamouflagePortConflictIsScopedToTransport(t *testing.T) {
	service := newWafCamouflageService()
	config := camouflage.DefenseConfig{
		LocalCIDRMode: "auto",
		Profiles: []camouflage.ProfileConfig{
			{ID: "web", Name: "Web", Enabled: true, Port: 8080, ProtocolProfile: "http"},
			{ID: "dns", Name: "DNS", Enabled: true, Port: 8080, ProtocolProfile: "dns"},
		},
	}
	if err := service.validatePortConflicts(config); err != nil {
		t.Fatalf("TCP and UDP listeners may share a port: %v", err)
	}
	config.Profiles[1].ProtocolProfile = "http"
	if err := service.validatePortConflicts(config); err == nil {
		t.Fatal("two enabled TCP listeners on the same port should conflict")
	}
}

func TestCamouflageServiceApplyStartsAndStopsEnabledListener(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(probe.Addr().(*net.TCPAddr).Port)
	_ = probe.Close()

	service := newWafCamouflageService()
	defer service.Stop()
	oldLogQueue, oldLogDB := global.GQEQUE_LOG_DB, global.GWAF_LOCAL_LOG_DB
	global.GQEQUE_LOG_DB, global.GWAF_LOCAL_LOG_DB = nil, nil
	t.Cleanup(func() { global.GQEQUE_LOG_DB, global.GWAF_LOCAL_LOG_DB = oldLogQueue, oldLogDB })
	config := camouflage.DefenseConfig{
		LocalCIDRMode: "auto",
		Profiles: []camouflage.ProfileConfig{{
			ID: "test-http", Name: "Test HTTP", Enabled: true, Port: port,
			ProtocolProfile: "http", ListenIPVersion: "ipv4",
		}},
	}
	if err := service.apply(config); err != nil {
		t.Fatal(err)
	}
	if statuses := service.Status(); len(statuses) != 1 || !statuses[0].Running {
		t.Fatalf("enabled profile did not start: %+v", statuses)
	}
	conn, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), time.Second)
	if err != nil {
		t.Fatalf("enabled profile is not reachable: %v", err)
	}
	_, _ = conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
	_ = conn.Close()
	deadline := time.Now().Add(time.Second)
	var events []camouflage.Hit
	for time.Now().Before(deadline) {
		events, err = service.Events(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(events) != 1 || events[0].Probe != "GetRequest" {
		t.Fatalf("runtime score event was not available before log queue flush: %+v", events)
	}

	config.Profiles[0].Enabled = false
	if err := service.apply(config); err != nil {
		t.Fatal(err)
	}
	if statuses := service.Status(); len(statuses) != 1 || statuses[0].Running {
		t.Fatalf("disabled profile kept its listener: %+v", statuses)
	}
	if events, err = service.Events(10); err != nil || len(events) != 1 {
		t.Fatalf("stopping the listener discarded its score history: events=%+v err=%v", events, err)
	}
}

func TestDecodeCamouflageConfigMigratesLegacyGenericProfile(t *testing.T) {
	config, err := decodeCamouflageConfig(`{"enabled":false,"port":8080,"profile":"nas-generic","version":"NAS Web Console","server_name":"nas","listen_ip_version":"dual"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Profiles) != 1 || config.Profiles[0].ProtocolProfile != "http" || config.Profiles[0].Enabled {
		t.Fatalf("legacy generic profile was not migrated as disabled HTTP: %+v", config)
	}
}
