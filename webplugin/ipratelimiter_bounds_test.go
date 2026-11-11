package webplugin

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateBucketsAreAtomicAndSaturationKeepsActiveLimits(t *testing.T) {
	l := NewIPRateLimiter(0, 1)
	l.maxIPs = 2
	var allowed atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for n := 0; n < 128; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if l.GetLimiter("active").Allow() {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if allowed.Load() != 1 {
		t.Fatalf("concurrent creation replenished tokens: %d", allowed.Load())
	}
	if l.AddIP("active").Allow() {
		t.Fatal("AddIP reset an existing depleted bucket")
	}
	if !l.Allow("family") {
		t.Fatal("independent visitor did not receive its own bucket")
	}
	for n := 0; n < 10000; n++ {
		if !l.Allow(fmt.Sprint(n)) {
			t.Fatal("resource saturation blocked an untracked visitor")
		}
	}
	if l.Allow("active") {
		t.Fatal("flood evicted the active restriction")
	}
	keys, events, bypasses := l.ResourceStats()
	if keys != 2 || events != 0 || bypasses != 10000 {
		t.Fatalf("unbounded state: %d/%d/%d", keys, events, bypasses)
	}
}

func TestWindowResourceBoundsAndExpiryDoNotOvercount(t *testing.T) {
	now := time.Now()
	l := NewWindowIPRateLimiter(60, 3)
	l.clock = func() time.Time { return now }
	l.maxIPs, l.maxEvents = 2, 4
	for n := 0; n < 3; n++ {
		if !l.Allow("active") {
			t.Fatal("premature limit")
		}
	}
	if l.Allow("active") {
		t.Fatal("configured threshold not enforced")
	}
	if !l.Allow("family") {
		t.Fatal("other visitor blocked")
	}
	for n := 0; n < 1000; n++ {
		if !l.Allow(fmt.Sprint(n)) {
			t.Fatal("capacity is not an attack verdict")
		}
	}
	keys, events, bypasses := l.ResourceStats()
	if keys != 2 || events != 4 || bypasses != 1000 {
		t.Fatalf("wrong quotas: %d/%d/%d", keys, events, bypasses)
	}
	if l.Allow("active") {
		t.Fatal("saturation reset an active window")
	}
	now = now.Add(60 * time.Second)
	if l.GetRequestCount("active") != 0 {
		t.Fatal("expired boundary counted")
	}
	l.CleanupOldRecords()
	keys, events, _ = l.ResourceStats()
	if keys != 0 || events != 0 || !l.Allow("new") {
		t.Fatal("expiry did not release resource quota")
	}

	large := NewWindowIPRateLimiter(60, maxEventsPerIP*2)
	for n := 0; n < maxEventsPerIP+100; n++ {
		large.Allow("one-ip")
	}
	keys, events, bypasses = large.ResourceStats()
	if keys != 1 || events != maxEventsPerIP || bypasses != 100 {
		t.Fatalf("per-IP storage unbounded: %d/%d/%d", keys, events, bypasses)
	}
}
