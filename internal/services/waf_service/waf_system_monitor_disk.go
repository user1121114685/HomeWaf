package waf_service

import (
	"HomeWaf/internal/model/response"
	"errors"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

const diskMonitorRefresh = 30 * time.Second

var errDiskMonitorPending = errors.New("磁盘采样仍在进行中")

// Partition enumeration includes read-only package images such as Snap. These
// are not independent capacity to monitor. Do not exclude every read-only disk
// or loop device: data volumes can legitimately use either, and root is useful
// even on an image-backed or overlay-based system.
func monitorDiskPartition(partition disk.PartitionStat) bool {
	if partition.Mountpoint == "" {
		return false
	}
	mount := path.Clean(partition.Mountpoint)
	if mount == "/" {
		return true
	}
	for _, prefix := range []string{"/snap", "/var/lib/snapd/snap"} {
		if mount == prefix || strings.HasPrefix(mount, prefix+"/") {
			return false
		}
	}
	switch strings.ToLower(partition.Fstype) {
	case "squashfs", "iso9660", "fuse.squashfuse", "fuse.squashfuse_ll", "fuse.snapfuse",
		"proc", "procfs", "sysfs", "devfs", "devtmpfs", "tmpfs", "ramfs",
		"cgroup", "cgroup2", "debugfs", "securityfs", "configfs", "tracefs",
		"pstore", "hugetlbfs", "mqueue", "autofs", "binfmt_misc", "nsfs",
		"fusectl", "rpc_pipefs", "bpf", "efivarfs":
		return false
	}
	return true
}

// Windows volume enumeration can block on a disconnected mapped drive beyond
// the HTTP timeout. Keep at most one collector running and serve the last disk
// snapshot while it finishes, independently of CPU and memory sampling.
type diskMonitorCache struct {
	mu       sync.Mutex
	rows     []response.WafDiskInfo
	lastRead time.Time
	ready    bool
	err      error
	pending  chan struct{}
}

func (c *diskMonitorCache) snapshot(collect func() ([]response.WafDiskInfo, error), wait time.Duration) ([]response.WafDiskInfo, error) {
	c.mu.Lock()
	if !c.lastRead.IsZero() && time.Since(c.lastRead) < diskMonitorRefresh {
		rows, err := append([]response.WafDiskInfo{}, c.rows...), c.err
		c.mu.Unlock()
		return rows, err
	}
	if c.pending == nil {
		c.pending = make(chan struct{})
		go c.refresh(collect, c.pending)
	}
	done := c.pending
	if c.ready {
		rows, err := append([]response.WafDiskInfo{}, c.rows...), c.err
		c.mu.Unlock()
		return rows, err
	}
	c.mu.Unlock()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return append([]response.WafDiskInfo{}, c.rows...), c.err
	case <-timer.C:
		return []response.WafDiskInfo{}, errDiskMonitorPending
	}
}

func (c *diskMonitorCache) refresh(collect func() ([]response.WafDiskInfo, error), done chan struct{}) {
	var rows []response.WafDiskInfo
	var err error
	defer func() {
		if recover() != nil {
			err = errors.New("磁盘采样失败")
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if err == nil {
			c.rows = append([]response.WafDiskInfo{}, rows...)
			c.ready = true
		}
		c.err, c.lastRead, c.pending = err, time.Now(), nil
		close(done)
	}()
	rows, err = collect()
}
