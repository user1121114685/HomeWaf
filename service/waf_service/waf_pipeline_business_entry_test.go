package waf_service

import (
	"HomeWaf/model"
	"testing"
)

func TestLegacyBusinessEntryPreservesHTTPSRedirectAndStoppedHost(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	var hosts []model.Hosts
	if err := db.Find(&hosts).Error; err != nil {
		t.Fatal(err)
	}
	for _, host := range hosts {
		host.Ssl = 1
		host.Port = 5443
		host.AutoJumpHTTPS = 1
		if host.Code == "camera" {
			host.START_STATUS = 1
		}
		if err := db.Save(&host).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(80, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	kinds := map[string]string{}
	for _, route := range p.Config.Routes {
		for _, host := range route.Hosts {
			kinds[host] = route.TargetType
		}
		if route.TargetType == "https_redirect" && route.RedirectPort != 5443 {
			t.Fatal("lost TLS target listener")
		}
	}
	if kinds["nas.test"] != "https_redirect" || kinds["camera.test"] != "unavailable" || p.Config.Upstream != "" {
		t.Fatal("migration exposed an origin instead of preserving local entries")
	}
}
