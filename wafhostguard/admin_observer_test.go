package wafhostguard

import (
	"HomeWaf/cache"
	"HomeWaf/global"
	"testing"
)

func TestAdminProtectionObserverRunsBeforeTouchReturns(t *testing.T) {
	old := global.GCACHE_WAFCACHE
	global.GCACHE_WAFCACHE = cache.InitWafCache()
	defer func() { global.GCACHE_WAFCACHE = old; SetAdminIPObserver(nil) }()
	seen := ""
	SetAdminIPObserver(func(ip string) { seen = ip })
	TouchAdminIP("198.51.100.64")
	if seen != "198.51.100.64" || !isActiveAdminIP(seen) {
		t.Fatal("administrator protection was delayed")
	}
	seen = ""
	TouchAdminIP("not-an-ip")
	if seen != "" {
		t.Fatal("invalid address reached observer")
	}
}
