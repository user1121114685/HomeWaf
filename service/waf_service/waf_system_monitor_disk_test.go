package waf_service

import (
	"HomeWaf/model/response"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiskMonitorSlowDriveDoesNotBlockOrMultiplyCollectors(t *testing.T) {
	var cache diskMonitorCache
	var calls atomic.Int32
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	collect := func() ([]response.WafDiskInfo, error) {
		calls.Add(1)
		<-release
		return []response.WafDiskInfo{{MountPoint: "C:", UsagePercent: 42}}, nil
	}
	started := time.Now()
	var callers sync.WaitGroup
	for i := 0; i < 16; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			rows, err := cache.snapshot(collect, time.Millisecond)
			if len(rows) != 0 || !errors.Is(err, errDiskMonitorPending) {
				t.Error("blocked disk must yield a pending status, not block system monitoring")
			}
		}()
	}
	callers.Wait()
	if time.Since(started) > time.Second {
		t.Fatal("system monitor waited for a blocked drive")
	}
	cache.mu.Lock()
	done := cache.pending
	cache.mu.Unlock()
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("released collector did not finish")
	}
	if calls.Load() != 1 {
		t.Fatalf("spawned %d collectors for one blocked drive", calls.Load())
	}
	rows, err := cache.snapshot(collect, time.Millisecond)
	if err != nil || len(rows) != 1 || rows[0].UsagePercent != 42 {
		t.Fatal("completed disk snapshot was lost")
	}
	rows[0].UsagePercent = 99
	rows, _ = cache.snapshot(collect, time.Millisecond)
	if rows[0].UsagePercent != 42 || calls.Load() != 1 {
		t.Fatal("cached snapshot was mutated or refreshed too soon")
	}
}

func TestDiskMonitorRefreshFailureKeepsLastSnapshotAndRetries(t *testing.T) {
	cache := diskMonitorCache{rows: []response.WafDiskInfo{{MountPoint: "D:", UsagePercent: 35}}, ready: true}
	failed := errors.New("drive unavailable")
	release := make(chan struct{})
	rows, err := cache.snapshot(func() ([]response.WafDiskInfo, error) { <-release; return nil, failed }, time.Hour)
	if err != nil || len(rows) != 1 || rows[0].UsagePercent != 35 {
		t.Fatal("refresh did not immediately return the last disk sample")
	}
	cache.mu.Lock()
	done := cache.pending
	cache.mu.Unlock()
	close(release)
	<-done
	rows, err = cache.snapshot(func() ([]response.WafDiskInfo, error) {
		t.Error("failed disk retried without backoff")
		return nil, nil
	}, time.Millisecond)
	if !errors.Is(err, failed) || len(rows) != 1 {
		t.Fatal("disk failure must preserve both the old sample and error")
	}
	cache.mu.Lock()
	cache.lastRead = time.Now().Add(-diskMonitorRefresh)
	cache.mu.Unlock()
	cache.snapshot(func() ([]response.WafDiskInfo, error) {
		return []response.WafDiskInfo{{MountPoint: "D:", UsagePercent: 36}}, nil
	}, time.Millisecond)
	cache.mu.Lock()
	done = cache.pending
	cache.mu.Unlock()
	if done != nil {
		<-done
	}
	rows, err = cache.snapshot(nil, time.Millisecond)
	if err != nil || len(rows) != 1 || rows[0].UsagePercent != 36 {
		t.Fatal("disk sampling did not recover")
	}
}

func TestDiskMonitorCollectorPanicDoesNotCrashWorker(t *testing.T) {
	var cache diskMonitorCache
	_, err := cache.snapshot(func() ([]response.WafDiskInfo, error) { panic("OS collector") }, time.Second)
	if err == nil || errors.Is(err, errDiskMonitorPending) {
		t.Fatal("collector panic must be reported as a disk error")
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.pending != nil {
		t.Fatal("collector panic left a stuck refresh")
	}
}
