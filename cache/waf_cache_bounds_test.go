package cache

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWafCacheBoundsKeepHotEntriesAndAccountExpiry(t *testing.T) {
	c := &WafCache{maxEntries: 4, maxBytes: 2048}
	for i := 0; i < 4; i++ {
		c.Set(fmt.Sprint(i), "value")
	}
	if c.Get("0") != "value" {
		t.Fatal("missing initial entry")
	}
	c.Set("4", "next")
	if c.Get("0") != "value" || c.Get("1") != nil {
		t.Fatal("eviction ignored recency")
	}
	for i := 0; i < 10000; i++ {
		c.Set(fmt.Sprint(i), "value")
	}
	if s := c.Stats(); s["keys"] > 4 || s["bytes"] > 2048 || s["evicted"] == 0 {
		t.Fatalf("cache unbounded: %v", s)
	}
	c.Set("oversize", strings.Repeat("x", 4096))
	if c.Get("oversize") != nil || c.Stats()["rejected"] != 1 {
		t.Fatal("oversize object cached")
	}
	c.SetWithTTlRenewTime("expired", "short", -time.Second)
	before := c.Stats()["bytes"]
	c.ClearExpirationCache()
	if c.Get("expired") != nil || c.Stats()["bytes"] >= before {
		t.Fatal("expiry did not release accounting")
	}
}
