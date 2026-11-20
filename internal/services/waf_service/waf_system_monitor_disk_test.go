package waf_service

import (
	"HomeWaf/internal/model/response"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

func TestDiskMonitorExcludesAuxiliaryMountsButKeepsDataVolumes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		partition disk.PartitionStat
		keep      bool
	}{
		{"snap image", disk.PartitionStat{Device: "/dev/loop0", Mountpoint: "/snap/core24/2124", Fstype: "squashfs", Opts: []string{"ro"}}, false},
		{"snap path fallback", disk.PartitionStat{Mountpoint: "/snap/app/42", Fstype: "fuse"}, false},
		{"snap alternate root", disk.PartitionStat{Mountpoint: "/var/lib/snapd/snap/app/42", Fstype: "fuse"}, false},
		{"snap root trailing slash", disk.PartitionStat{Mountpoint: "/snap/"}, false},
		{"package image outside snap", disk.PartitionStat{Mountpoint: "/media/package", Fstype: "squashfs"}, false},
		{"installation media", disk.PartitionStat{Mountpoint: "/media/cdrom", Fstype: "iso9660"}, false},
		{"memory filesystem", disk.PartitionStat{Mountpoint: "/run", Fstype: "tmpfs"}, false},
		{"process filesystem", disk.PartitionStat{Mountpoint: "/proc", Fstype: "proc"}, false},
		{"cgroups", disk.PartitionStat{Mountpoint: "/sys/fs/cgroup", Fstype: "cgroup2"}, false},
		{"missing mount", disk.PartitionStat{Device: "/dev/sda1", Fstype: "ext4"}, false},
		{"system root", disk.PartitionStat{Mountpoint: "/", Fstype: "ext4"}, true},
		{"container root", disk.PartitionStat{Mountpoint: "/", Fstype: "overlay"}, true},
		{"image system root", disk.PartitionStat{Mountpoint: "/", Fstype: "squashfs"}, true},
		{"EFI volume", disk.PartitionStat{Mountpoint: "/boot/efi", Fstype: "vfat"}, true},
		{"readonly data", disk.PartitionStat{Mountpoint: "/mnt/archive", Fstype: "ext4", Opts: []string{"ro"}}, true},
		{"loop data volume", disk.PartitionStat{Device: "/dev/loop7", Mountpoint: "/mnt/data", Fstype: "ext4"}, true},
		{"similar directory name", disk.PartitionStat{Mountpoint: "/snapshots", Fstype: "btrfs"}, true},
		{"snap writable data", disk.PartitionStat{Mountpoint: "/var/lib/snapd", Fstype: "ext4"}, true},
		{"removable data", disk.PartitionStat{Mountpoint: "/run/media/user/data", Fstype: "exfat"}, true},
		{"network data", disk.PartitionStat{Mountpoint: "/mnt/nas", Fstype: "nfs4"}, true},
		{"Windows drive", disk.PartitionStat{Device: "C:", Mountpoint: "C:\\", Fstype: "NTFS"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := monitorDiskPartition(tc.partition); got != tc.keep {
				t.Fatalf("keep=%v want %v: %+v", got, tc.keep, tc.partition)
			}
		})
	}
}

func TestDiskMonitorFiltersBeforeSamplingAndPreservesDataUsage(t *testing.T) {
	var sampled []string
	service := &WafSystemMonitorService{}
	rows := service.diskInfoForPartitions([]disk.PartitionStat{
		{Device: "/dev/sda2", Mountpoint: "/", Fstype: "ext4"},
		{Device: "/dev/loop0", Mountpoint: "/snap/core24/2124", Fstype: "squashfs"},
		{Device: "tmpfs", Mountpoint: "/run", Fstype: "tmpfs"},
		{Device: "/dev/sdb1", Mountpoint: "/mnt/archive", Fstype: "ext4", Opts: []string{"ro"}},
	}, func(mount string) (*disk.UsageStat, error) {
		sampled = append(sampled, mount)
		return &disk.UsageStat{Total: 4 << 30, Free: 3 << 30, Used: 1 << 30, UsedPercent: 25}, nil
	})
	if !reflect.DeepEqual(sampled, []string{"/", "/mnt/archive"}) {
		t.Fatalf("sampled excluded mounts: %v", sampled)
	}
	if len(rows) != 2 || rows[0].FileSystem != "/dev/sda2" || rows[1].MountPoint != "/mnt/archive" || rows[1].UsagePercent != 25 || rows[1].Total != "4.0 GB" {
		t.Fatalf("data volume changed: %+v", rows)
	}
}

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
