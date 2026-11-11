// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package webplugin

import (
	"HomeWaf/utils"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LimitMode 限流模式
type LimitMode int

const (
	// RateMode 平均速率模式 - 每秒固定速率
	RateMode LimitMode = iota
	// WindowMode 滑动窗口模式 - N秒内最多M次
	WindowMode
)

const (
	defaultTrackedIPs   = 20000
	defaultWindowEvents = 200000
	maxEventsPerIP      = 4096
)

// Capacity exhaustion must not make unrelated visitors share a depleted bucket.
// Existing identities keep their limits; only untracked work degrades to allow.
var untrackedLimiter = rate.NewLimiter(rate.Inf, 1)

// IPRateLimiter .
type IPRateLimiter struct {
	ips                       map[string]*rate.Limiter
	mu                        *sync.RWMutex
	r                         rate.Limit
	b                         int
	mode                      LimitMode
	window                    int                    // 时间窗口大小(秒)
	requests                  map[string][]time.Time // 用于滑动窗口模式记录请求时间
	lastSeen                  map[string]time.Time   // 平均速率模式下每个IP的最后活跃时间，供清理判断空闲
	Rule                      *utils.RuleHelper
	maxIPs, maxEvents, events int
	quotaBypasses             uint64
	lastSweep                 time.Time
	clock                     func() time.Time
}

func (i *IPRateLimiter) now() time.Time {
	if i.clock != nil {
		return i.clock()
	}
	return time.Now()
}

func (i *IPRateLimiter) limits() {
	if i.maxIPs <= 0 {
		i.maxIPs = defaultTrackedIPs
	}
	if i.maxEvents <= 0 {
		i.maxEvents = defaultWindowEvents
	}
}

func (i *IPRateLimiter) sweepIfFull(now time.Time) {
	if now.Sub(i.lastSweep) >= time.Second {
		i.cleanupLocked(now)
		i.lastSweep = now
	}
}

// NewIPRateLimiter 创建一个新的IP限流器
// r: 每秒请求速率
// b: 突发请求数量
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	// 令牌桶从空到补满需要 b/r 秒。把它记成等价窗口：空闲超过这段时间后桶必然已满，
	// 此时回收限流器与保留它完全等价，清理才不会平白送给攻击者一次满桶。
	window := 0
	if r > 0 {
		window = int(float64(b)/float64(r)) + 1
	}
	i := &IPRateLimiter{
		ips:      make(map[string]*rate.Limiter),
		mu:       &sync.RWMutex{},
		r:        r,
		b:        b,
		mode:     RateMode, // 默认使用平均速率模式，保持向后兼容
		window:   window,
		requests: make(map[string][]time.Time),
		lastSeen: make(map[string]time.Time),
	}

	return i
}

// NewWindowIPRateLimiter 创建一个基于滑动窗口的IP限流器
// window: 时间窗口大小(秒)
// maxRequests: 窗口内最大请求数
func NewWindowIPRateLimiter(window, maxRequests int) *IPRateLimiter {
	i := &IPRateLimiter{
		ips:      make(map[string]*rate.Limiter),
		mu:       &sync.RWMutex{},
		r:        rate.Limit(float64(maxRequests) / float64(window)), // 保持兼容
		b:        maxRequests,
		mode:     WindowMode,
		window:   window,
		requests: make(map[string][]time.Time),
		lastSeen: make(map[string]time.Time),
	}

	return i
}

// AddIP returns the existing bucket or creates it atomically. Repeated calls
// must not replenish an active client's tokens.
func (i *IPRateLimiter) AddIP(ip string) *rate.Limiter {
	return i.GetLimiter(ip)
}

// GetLimiter resolves and creates a bucket while holding the same lock.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.limits()
	now := i.now()
	limiter, exists := i.ips[ip]
	if !exists {
		if len(i.ips) >= i.maxIPs {
			i.sweepIfFull(now)
		}
		if len(ip) > 128 || len(i.ips) >= i.maxIPs {
			i.quotaBypasses++
			return untrackedLimiter
		}
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}
	i.lastSeen[ip] = now
	return limiter
}

// Allow 检查是否允许请求通过
// 根据模式使用不同的限流策略
func (i *IPRateLimiter) Allow(ip string) bool {
	if i.mode == RateMode {
		// 使用令牌桶算法限流
		return i.GetLimiter(ip).Allow()
	} else {
		// 使用滑动窗口算法限流
		return i.allowByWindow(ip)
	}
}

// allowByWindow 使用滑动窗口算法检查是否允许请求通过
func (i *IPRateLimiter) allowByWindow(ip string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.limits()
	now := i.now()
	windowStart := now.Add(-time.Duration(i.window) * time.Second)
	if _, exists := i.requests[ip]; !exists && len(i.requests) >= i.maxIPs {
		i.sweepIfFull(now)
	}
	_, exists := i.requests[ip]
	if len(ip) > 128 || (!exists && len(i.requests) >= i.maxIPs) {
		i.quotaBypasses++
		return true
	}
	validRequests := i.trimWindow(ip, windowStart)

	// 检查是否超过限制
	if len(validRequests) >= i.b {
		return false
	}
	if len(validRequests) >= maxEventsPerIP || i.events >= i.maxEvents {
		i.sweepIfFull(now)
		validRequests = i.trimWindow(ip, windowStart)
		if len(validRequests) >= maxEventsPerIP || i.events >= i.maxEvents {
			i.quotaBypasses++
			return true
		}
	}
	i.requests[ip] = append(validRequests, now)
	i.events++
	return true
}

func (i *IPRateLimiter) trimWindow(ip string, cutoff time.Time) []time.Time {
	previous := i.requests[ip]
	valid := previous[:0]
	for _, at := range previous {
		if at.After(cutoff) {
			valid = append(valid, at)
		}
	}
	i.events -= len(previous) - len(valid)
	if len(valid) == 0 {
		delete(i.requests, ip)
		return nil
	}
	i.requests[ip] = valid
	return valid
}

// CleanupOldRecords 清理过期的请求记录
func (i *IPRateLimiter) CleanupOldRecords() {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cleanupLocked(i.now())
}

func (i *IPRateLimiter) cleanupLocked(now time.Time) {

	if i.mode == RateMode {
		// 平均速率模式下 requests 里没有时间戳，不能拿窗口去判过期——那样每次清理都会把
		// 所有令牌桶整表删掉重建，等于按清理周期给攻击者定时重置一次满桶。
		// 改成按空闲时长回收：只有空闲超过「补满一桶所需时间」的 IP 才删，此时删与不删等价。
		idleTTL := time.Duration(i.window) * time.Second
		if idleTTL <= 0 {
			idleTTL = time.Minute
		}
		for ip, seen := range i.lastSeen {
			if now.Sub(seen) >= idleTTL {
				delete(i.lastSeen, ip)
				delete(i.requests, ip)
				delete(i.ips, ip)
			}
		}
		return
	}

	windowStart := now.Add(-time.Duration(i.window) * time.Second)

	i.events = 0
	for _, times := range i.requests {
		i.events += len(times)
	}
	for ip := range i.requests {
		if len(i.trimWindow(ip, windowStart)) == 0 {
			delete(i.ips, ip)
			delete(i.lastSeen, ip)
		}
	}
}

// ClearWindowForIP 清空指定IP的滑动窗口记录
// 用于手动重置某个IP的限流状态
func (i *IPRateLimiter) ClearWindowForIP(ip string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// 清空滑动窗口记录
	if _, exists := i.requests[ip]; exists {
		i.events -= len(i.requests[ip])
		delete(i.requests, ip)
	}

	// 重置令牌桶（用于平均速率模式）
	if _, exists := i.ips[ip]; exists {
		// 创建一个新的限流器替换旧的
		newLimiter := rate.NewLimiter(i.r, i.b)
		i.ips[ip] = newLimiter
	}
}

// GetRequestCount 获取指定IP在当前窗口内的请求数量
func (i *IPRateLimiter) GetRequestCount(ip string) int {
	i.mu.RLock()
	defer i.mu.RUnlock()

	now := i.now()
	windowStart := now.Add(-time.Duration(i.window) * time.Second)

	if _, exists := i.requests[ip]; !exists {
		return 0
	}

	count := 0
	for _, t := range i.requests[ip] {
		if t.After(windowStart) {
			count++
		}
	}

	return count
}

// —— 只读访问器：供构造一致性校验与运行期可观测使用 ——

// RatePerSecond 返回令牌桶的每秒补充速率（滑动窗口模式下为等价速率）。
func (i *IPRateLimiter) RatePerSecond() rate.Limit { return i.r }

// Burst 返回突发额度（滑动窗口模式下即窗口内最大请求数）。
func (i *IPRateLimiter) Burst() int { return i.b }

// WindowSeconds 返回时间窗口(秒)。平均速率模式下是「令牌桶补满所需时间」的等价窗口。
func (i *IPRateLimiter) WindowSeconds() int { return i.window }

// IsWindowMode 是否为滑动窗口模式。
func (i *IPRateLimiter) IsWindowMode() bool { return i.mode == WindowMode }

// TrackedIPCount 当前正在跟踪的客户端数量，用于观察内存占用规模。
func (i *IPRateLimiter) TrackedIPCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.mode == WindowMode {
		return len(i.requests)
	}
	return len(i.ips)
}

func (i *IPRateLimiter) ResourceStats() (ips, events int, quotaBypasses uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	ips = len(i.ips)
	if i.mode == WindowMode {
		ips = len(i.requests)
	}
	return ips, i.events, i.quotaBypasses
}
