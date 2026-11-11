package wafenginecore

import (
	"HomeWaf/cache"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"net/http/httptest"
	"testing"
)

func TestLegacyCSRFUsesSchemePortAndTrustedProxy(t *testing.T) {
	host := &wafenginmodel.HostSafe{Host: model.Hosts{CsrfJSON: `{"is_enable":1,"protect_methods":"POST","allow_empty_ref":0}`, IPTrustProxies: "10.0.0.0/8"}}
	for _, tc := range []struct {
		target, origin, peer, forward string
		blocked                       bool
	}{
		{"https://self.test:8443/", "https://self.test:8443", "192.0.2.1:20", "", false},
		{"https://self.test:8443/", "https://self.test:9443", "192.0.2.1:20", "", true},
		{"https://self.test:8443/", "http://self.test:8443", "192.0.2.1:20", "", true},
		{"https://self.test/", "https://self.test:443", "192.0.2.1:20", "", false},
		{"http://self.test:8443/", "https://self.test:8443", "192.0.2.1:20", "https", true},
		{"http://self.test:8443/", "https://self.test:8443", "10.0.0.2:20", "https", false},
	} {
		r := httptest.NewRequest("POST", tc.target, nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Forwarded-Proto", tc.forward)
		got := (&WafEngine{}).CheckCsrf(r, &innerbean.WebLog{}, nil, host, nil)
		if got.IsBlock != tc.blocked {
			t.Fatalf("target=%s origin=%s peer=%s blocked=%v", tc.target, tc.origin, tc.peer, got.IsBlock)
		}
	}
	r := httptest.NewRequest("POST", "https://self.test/", nil)
	r.Header.Add("Origin", "https://self.test")
	r.Header.Add("Origin", "https://evil.test")
	if !(&WafEngine{}).CheckCsrf(r, &innerbean.WebLog{}, nil, host, nil).IsBlock {
		t.Fatal("accepted ambiguous Origin headers")
	}
}

func TestLegacyBotIdentityCannotBeReusedAfterUAChange(t *testing.T) {
	oldCache, oldDeny := global.GCACHE_WAFCACHE, global.CurrentSettings().GCONFIG_RECORD_SPIDER_DENY
	c := cache.InitWafCache()
	defer c.Close()
	global.GCACHE_WAFCACHE = c
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_SPIDER_DENY = 1 })
	t.Cleanup(func() {
		global.GCACHE_WAFCACHE = oldCache
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_SPIDER_DENY = oldDeny })
	})
	host := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "bot-fixture", Port: 5000}}
	w := &WafEngine{}
	r := httptest.NewRequest("GET", "http://self.test/", nil)
	log := &innerbean.WebLog{SRC_IP: "8.8.8.8", USER_AGENT: "Mozilla/5.0"}
	if w.CheckBot(r, log, nil, host, nil).IsBlock {
		t.Fatal("normal browser blocked")
	}
	c.Set(enums.CACHE_DNS_NORMAL_IP+log.SRC_IP, log.SRC_IP)
	log.USER_AGENT = "360Spider"
	if !w.CheckBot(r, log, nil, host, nil).IsBlock || log.BotVerifyStrong != 0 {
		t.Fatal("normal-IP cache bypassed changed bot identity")
	}
}
