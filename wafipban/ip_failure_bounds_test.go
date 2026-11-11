package wafipban

import (
	"HomeWaf/cache"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"testing"
	"time"
)

func TestFailureEventsBoundedWithoutChangingSavedSnapshot(t *testing.T) {
	store := cache.InitWafCache()
	defer store.Close()
	m := newTestManager(t, store)
	now := time.Now()
	old := &IPFailureRecord{IP: "192.0.2.10", Events: make([]time.Time, maxFailureEvents+20), Count: maxFailureEvents + 20}
	for n := range old.Events {
		old.Events[n] = now.Add(-time.Second)
	}
	store.Set(enums.CACHE_IP_FAILURE_PRE+old.IP, old)
	m.RecordFailure(failureLog(old.IP, 403))
	got := m.GetFailureInfo(old.IP)
	if len(got.Events) != maxFailureEvents || got.Count != maxFailureEvents || !got.EventsTruncated {
		t.Fatalf("unbounded snapshot: %d/%d", len(got.Events), got.Count)
	}
	if len(old.Events) != maxFailureEvents+20 || old.EventsTruncated {
		t.Fatal("published snapshot was mutated")
	}
	m.RecordFailureThreshold(old.IP, 1, 10)
	if m.GetFailureCount(old.IP, 1) != maxFailureEvents {
		t.Fatal("threshold update discarded an additional event")
	}
	stale := &IPFailureRecord{IP: old.IP, Events: []time.Time{now.Add(-20 * time.Minute)}}
	store.Set(enums.CACHE_IP_FAILURE_PRE+old.IP, stale)
	m.RecordFailure(failureLog(old.IP, 403))
	if m.GetFailureCount(old.IP, 1) != 1 {
		t.Fatal("expired events counted as new failures")
	}
}

func TestFailureStatusReloadDiscardsOldRegex(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_IP_FAILURE_STATUS_CODES
	t.Cleanup(func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_IP_FAILURE_STATUS_CODES = old })
	})
	m := &IPFailureManager{}
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_IP_FAILURE_STATUS_CODES = "^4[0-9]{2}$" })
	m.ReloadStatusCodes()
	if !m.IsFailureStatusCode(404) {
		t.Fatal("initial regex not active")
	}
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_IP_FAILURE_STATUS_CODES = "503" })
	m.ReloadStatusCodes()
	if m.IsFailureStatusCode(404) || !m.IsFailureStatusCode(503) {
		t.Fatal("removed failure policy still affects normal traffic")
	}
}

func TestUnverifiedCrawlerDoesNotBypassFailureAccounting(t *testing.T) {
	store := cache.InitWafCache()
	defer store.Close()
	m := newTestManager(t, store)
	claim := &innerbean.WebLog{SRC_IP: "192.0.2.20", IsBot: 1, RISK_LEVEL: 0}
	if claim.IsSafeBot() {
		t.Fatal("unverified crawler is considered safe")
	}
	m.RecordFailure(claim)
	if m.GetFailureCount(claim.SRC_IP, 1) != 1 {
		t.Fatal("weak identity bypassed failure accounting")
	}
	claim.BotVerifyStrong = 1
	if !claim.IsSafeBot() {
		t.Fatal("strong identity did not retain exemption")
	}
	m.RecordFailure(claim)
	if m.GetFailureCount(claim.SRC_IP, 1) != 1 {
		t.Fatal("verified harmless crawler counted as failure")
	}
}
