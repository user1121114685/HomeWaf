package wafupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/releasetest"
)

func TestScheduledReleaseChecksAndNotifications(t *testing.T) {
	var calls atomic.Int32
	var published atomic.Value
	published.Store("v2.0.2")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		version := published.Load().(string)
		if version == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(releasetest.Metadata(version, "scheduled release"))
	}))
	defer server.Close()
	oldManifest, oldRoot, oldBeta, oldVersion := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION = oldManifest, oldRoot, oldBeta, oldVersion
	})
	global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION = "", server.URL, "", "v2.0.1"

	dir := t.TempDir()
	monitor := NewReleaseMonitor(dir)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var notifications []string
	notify := func(current, version string) {
		if current != "v2.0.1" {
			t.Errorf("unexpected installed version %q", current)
		}
		notifications = append(notifications, version)
	}
	readPlan := func() releaseSchedule {
		t.Helper()
		var plan releaseSchedule
		if err := readReleaseState(filepath.Join(dir, "release-check-schedule.json"), &plan); err != nil {
			t.Fatal(err)
		}
		if plan.Version != releaseScheduleVersion || !plan.NextCheck.After(plan.WindowStart) || !plan.NextCheck.Before(plan.WindowStart.Add(releaseCheckPeriod)) {
			t.Fatalf("automatic check outside its scheduled period: %+v", plan)
		}
		return plan
	}
	run := func(at time.Time) {
		t.Helper()
		if err := monitor.CheckDue(at, notify); err != nil {
			t.Fatal(err)
		}
	}
	run(start)
	first := readPlan()
	if calls.Load() != 0 || len(notifications) != 0 {
		t.Fatalf("first startup check was not delayed into its randomized window: calls=%d notifications=%v", calls.Load(), notifications)
	}
	monitor = NewReleaseMonitor(dir)
	run(first.NextCheck.Add(-time.Nanosecond))
	pending, err := monitor.Check(true)
	if err != nil || !strings.Contains(pending.Message, "尚未检查") || calls.Load() != 0 || !readPlan().NextCheck.Equal(first.NextCheck) {
		t.Fatalf("startup or console read changed the randomized schedule: %+v %v calls=%d", pending, err, calls.Load())
	}

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if err := monitor.CheckDue(first.NextCheck, notify); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	second := readPlan()
	if calls.Load() != 1 || !second.WindowStart.Equal(start.Add(releaseCheckPeriod)) || second.Notified != "v2.0.2" || !slices.Equal(notifications, []string{"v2.0.2"}) {
		t.Fatalf("duplicate or missing scheduled check: %+v calls=%d", second, calls.Load())
	}
	monitor = NewReleaseMonitor(dir)
	found, err := monitor.Check(true)
	if err != nil || found.Version != "v2.0.2" || calls.Load() != 1 {
		t.Fatalf("offline login did not recover the release: %+v %v", found, err)
	}
	run(second.NextCheck)
	if calls.Load() != 2 || !slices.Equal(notifications, []string{"v2.0.2"}) {
		t.Fatalf("duplicate notifications after restart: %v", notifications)
	}

	third := readPlan()
	published.Store("v2.0.3")
	manual, err := monitor.Check(false)
	if manual.Version != "v2.0.3" || err != nil || !third.NextCheck.Equal(readPlan().NextCheck) {
		t.Fatalf("manual check did not leave the automatic deadline intact: %+v %v", manual, err)
	}
	run(third.NextCheck)
	published.Store("v2.0.4-rc.1")
	run(readPlan().NextCheck)
	if !slices.Equal(notifications, []string{"v2.0.2", "v2.0.3"}) {
		t.Fatalf("incorrect release notifications: %v", notifications)
	}

	published.Store("v2.0.3")
	run(readPlan().NextCheck)
	published.Store("")
	due := readPlan().NextCheck
	if err := monitor.CheckDue(due, notify); err == nil {
		t.Fatal("publisher failure hidden")
	}
	before := calls.Load()
	failedPlan := readPlan()
	if failedPlan.RetryCheck.Before(due.Add(releaseCheckRetryMin)) || !failedPlan.RetryCheck.Before(due.Add(releaseCheckRetryMax)) {
		t.Fatalf("publisher failure did not persist a randomized retry deadline: %+v", failedPlan)
	}
	monitor = NewReleaseMonitor(dir)
	run(due)
	stale, err := monitor.Check(true)
	if err != nil || stale.Version != "v2.0.3" || stale.CheckWarning == "" || calls.Load() != before {
		t.Fatalf("failure lost known release or ignored the retry deadline: %+v %v", stale, err)
	}
	published.Store("v2.0.4")
	run(failedPlan.RetryCheck)
	if calls.Load() != before+1 || !slices.Equal(notifications, []string{"v2.0.2", "v2.0.3", "v2.0.4"}) || !readPlan().RetryCheck.IsZero() {
		t.Fatalf("scheduled retry did not recover and notify: calls=%d notifications=%v plan=%+v", calls.Load(), notifications, readPlan())
	}
	published.Store("v2.0.5")
	overdue := readPlan().NextCheck.Add(8 * 24 * time.Hour)
	run(overdue)
	run(overdue)
	if calls.Load() != before+2 || !readPlan().NextCheck.After(overdue) || !slices.Equal(notifications, []string{"v2.0.2", "v2.0.3", "v2.0.4", "v2.0.5"}) {
		t.Fatalf("check after downtime: calls=%d notifications=%v", calls.Load(), notifications)
	}
	global.GWAF_RELEASE_VERSION = "v2.0.5"
	current, err := monitor.Check(true)
	if err != nil || current.Version != "" || calls.Load() != before+2 {
		t.Fatalf("old release offered after upgrade: %+v %v", current, err)
	}
}

func TestScheduledReleaseCheckMigratesLegacyScheduleWithoutConcentratingRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(releasetest.Metadata("v3.1.0", "migrated release"))
	}))
	defer server.Close()
	oldManifest, oldRoot, oldBeta, oldVersion := global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION
	t.Cleanup(func() {
		global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION = oldManifest, oldRoot, oldBeta, oldVersion
	})
	global.GUPDATE_VERSION_URL, global.GUPDATE_RELEASE_URL, global.GUPDATE_GITHUB_VERSION_URL, global.GWAF_RELEASE_VERSION = "", server.URL, "", "v3.0.0"

	dir := t.TempDir()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	legacy := releaseSchedule{WindowStart: start.Add(-24 * time.Hour), NextCheck: start.Add(24 * time.Hour), Notified: "v2.9.0"}
	if err := writeReleaseState(filepath.Join(dir, "release-check-schedule.json"), legacy); err != nil {
		t.Fatal(err)
	}
	var notified string
	monitor := NewReleaseMonitor(dir)
	if err := monitor.CheckDue(start, func(_, version string) { notified = version }); err != nil {
		t.Fatal(err)
	}
	var migrated releaseSchedule
	if err := readReleaseState(filepath.Join(dir, "release-check-schedule.json"), &migrated); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || notified != "" || migrated.Version != releaseScheduleVersion || migrated.Notified != "v2.9.0" || !migrated.NextCheck.Equal(legacy.NextCheck) {
		t.Fatalf("legacy schedule migration lost its randomized deadline: calls=%d notified=%q plan=%+v", calls.Load(), notified, migrated)
	}
	if err := monitor.CheckDue(legacy.NextCheck, func(_, version string) { notified = version }); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || notified != "v3.1.0" {
		t.Fatalf("migrated schedule did not notify when due: calls=%d notified=%q", calls.Load(), notified)
	}
}
