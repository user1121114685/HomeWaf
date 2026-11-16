package firewall

import (
	"HomeWaf/common/wafexec"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const flowDropTag = "HomeWafDefense_"

// Exactly one accepted TCP connection, both directions. This is never an IP
// ban. Deadlines in owned comments allow reconciliation after a worker restart.
type FlowDropper struct {
	mu        sync.Mutex
	slots     chan struct{}
	command   func(string, ...string) (string, error)
	os        string
	now       func() time.Time
	lastError string
	leases    map[string]time.Time
	sequence  uint64
}

func NewFlowDropper() *FlowDropper {
	return &FlowDropper{slots: make(chan struct{}, 8), os: runtime.GOOS, now: time.Now, leases: map[string]time.Time{}}
}

type limitedCommandOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedCommandOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - b.Len()
	if len(p) > remaining {
		p = p[:max(0, remaining)]
		b.overflow = true
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func (f *FlowDropper) run(bin string, args ...string) (string, error) {
	if f.command != nil {
		return f.command(bin, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := wafexec.FixStdin(exec.CommandContext(ctx, bin, args...))
	var output limitedCommandOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if output.overflow {
		return "", errors.New("防火墙规则读取超过安全上限")
	}
	if err != nil {
		return "", fmt.Errorf("%s 操作失败: %w", bin, err)
	}
	return output.String(), nil
}

func (f *FlowDropper) Probe(version string) error {
	if f.os != "linux" {
		return errors.New("当前平台未提供连接级静默丢包，请选择立即断开；静默丢包目前需要Linux的iptables/ip6tables权限")
	}
	for _, bin := range flowBinaries(version) {
		for _, chain := range []string{"INPUT", "OUTPUT"} {
			if _, err := f.run(bin, "-w", "1", "-S", chain); err != nil {
				return fmt.Errorf("静默丢包不可用: %w", err)
			}
		}
	}
	return nil
}

func flowBinaries(version string) []string {
	if version == "ipv4" {
		return []string{"iptables"}
	}
	if version == "ipv6" {
		return []string{"ip6tables"}
	}
	return []string{"iptables", "ip6tables"}
}

func (f *FlowDropper) Error() string { f.mu.Lock(); defer f.mu.Unlock(); return f.lastError }
func (f *FlowDropper) fail(err error) error {
	f.mu.Lock()
	f.lastError = err.Error()
	f.mu.Unlock()
	return err
}

func flowArgs(chain, verb, tag string, local, remote netip.AddrPort) []string {
	src, dst := remote, local
	if chain == "OUTPUT" {
		src, dst = local, remote
	}
	args := []string{"-w", "1", verb, chain}
	if verb == "-I" {
		args = append(args, "1")
	}
	return append(args, "-p", "tcp", "-s", src.Addr().String(), "-d", dst.Addr().String(), "--sport", strconv.Itoa(int(src.Port())), "--dport", strconv.Itoa(int(dst.Port())), "-m", "comment", "--comment", tag, "-j", "DROP")
}

func (f *FlowDropper) Drop(local, remote netip.AddrPort) error {
	if f.os != "linux" {
		return f.fail(errors.New("连接级静默丢包在此平台不可用"))
	}
	if !local.IsValid() || !remote.IsValid() || local.Port() == 0 || remote.Port() == 0 || local.Addr().Zone() != "" || remote.Addr().Zone() != "" {
		return f.fail(errors.New("无效的连接地址"))
	}
	local = netip.AddrPortFrom(local.Addr().Unmap(), local.Port())
	remote = netip.AddrPortFrom(remote.Addr().Unmap(), remote.Port())
	if local.Addr().Is4() != remote.Addr().Is4() {
		return f.fail(errors.New("连接地址族不一致"))
	}
	select {
	case f.slots <- struct{}{}:
		defer func() { <-f.slots }()
	default:
		return f.fail(errors.New("静默丢包执行队列已满，连接已中止"))
	}
	now := f.now()
	f.mu.Lock()
	if len(f.leases) >= 512 {
		f.mu.Unlock()
		return f.fail(errors.New("静默丢包连接配额已满，连接已中止"))
	}
	f.sequence++
	tag := fmt.Sprintf("%s%d_%x%x", flowDropTag, now.Add(2*time.Minute).Unix(), now.UnixNano(), f.sequence)
	f.leases[tag] = now.Add(2 * time.Minute)
	f.mu.Unlock()
	bin := "iptables"
	if local.Addr().Is6() {
		bin = "ip6tables"
	}
	if _, err := f.run(bin, flowArgs("OUTPUT", "-I", tag, local, remote)...); err != nil {
		return f.fail(err)
	}
	if _, err := f.run(bin, flowArgs("INPUT", "-I", tag, local, remote)...); err != nil {
		_, _ = f.run(bin, flowArgs("OUTPUT", "-D", tag, local, remote)...)
		return f.fail(err)
	}
	f.mu.Lock()
	f.lastError = ""
	f.mu.Unlock()
	return nil
}

// Only expired rules in our exact comment namespace are removed.
func (f *FlowDropper) Reconcile() error {
	if f.os != "linux" {
		return nil
	}
	now := f.now()
	for _, bin := range flowBinaries("") {
		for _, chain := range []string{"INPUT", "OUTPUT"} {
			out, err := f.run(bin, "-w", "1", "-S", chain)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(out, "\n") {
				parts := strings.Fields(line)
				if len(parts) < 4 || parts[0] != "-A" || parts[1] != chain {
					continue
				}
				expired := false
				for i := 2; i+1 < len(parts); i++ {
					if parts[i] != "--comment" {
						continue
					}
					tag := strings.Trim(parts[i+1], "\"")
					if !strings.HasPrefix(tag, flowDropTag) {
						continue
					}
					fields := strings.Split(strings.TrimPrefix(tag, flowDropTag), "_")
					if len(fields) != 2 {
						continue
					}
					deadline, parseErr := strconv.ParseInt(fields[0], 10, 64)
					if parseErr == nil && deadline > now.Unix() {
						f.mu.Lock()
						f.leases[tag] = time.Unix(deadline, 0)
						f.mu.Unlock()
					}
					if parseErr == nil && deadline > 0 && deadline <= now.Unix() {
						expired = true
						parts[i+1] = tag
					}
				}
				if expired {
					parts[0] = "-D"
					if _, err := f.run(bin, append([]string{"-w", "1"}, parts...)...); err != nil {
						return f.fail(err)
					}
				}
			}
		}
	}
	f.mu.Lock()
	for tag, deadline := range f.leases {
		if !deadline.After(now) {
			delete(f.leases, tag)
		}
	}
	f.mu.Unlock()
	return nil
}
