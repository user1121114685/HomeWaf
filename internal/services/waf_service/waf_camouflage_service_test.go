package waf_service

import (
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/protection/camouflage"
)

func TestCamouflageServiceBlocksNonLocalHitsForNinetyDays(t *testing.T) {
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
	config := camouflage.DefaultConfig()
	config.BlockPublic = false
	config.BanSeconds = 1
	if err := service.manager.Apply(config); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(90 * 24 * time.Hour).Unix()
	first := service.handleHit(camouflage.Hit{ClientIP: "198.51.100.44", Port: 8080, Profile: "nas-generic", Score: 94, Probe: "GetRequest"})
	second := service.handleHit(camouflage.Hit{ClientIP: "198.51.100.44", Port: 8080, Profile: "nas-generic", Score: 94, Probe: "GetRequest"})
	if len(requests) != 1 {
		t.Fatalf("expected one firewall write for repeated hits, got %d", len(requests))
	}
	got := requests[0]
	if got.IP != "198.51.100.44" || got.BlockType != "auto" {
		t.Fatalf("unexpected firewall request: %+v", got)
	}
	if delta := got.ExpireTime - before; delta < -2 || delta > 2 {
		t.Fatalf("expected a 90-day expiry, difference=%d seconds", delta)
	}
	if !first.Blocked || !second.Blocked {
		t.Fatalf("non-local events were not blocked: first=%+v second=%+v", first, second)
	}
}

func TestCamouflageServiceDoesNotBlockManagementLAN(t *testing.T) {
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
	config := camouflage.DefaultConfig()
	if err := service.manager.Apply(config); err != nil {
		t.Fatal(err)
	}
	hit := service.handleHit(camouflage.Hit{ClientIP: "192.168.99.240", Port: 8080, Profile: "nas-generic", Score: 94, LocalLAN: true})
	if called {
		t.Fatal("management LAN hit should not be sent to the firewall")
	}
	if !hit.LocalLAN || hit.Blocked {
		t.Fatalf("unexpected local event: %+v", hit)
	}
}
