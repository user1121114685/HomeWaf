package wafpipeline

import (
	"HomeWaf/cache"
	"sync"
	"sync/atomic"
	"time"
)

// Requests waiting on uploads, authentication or long-lived streams all use the
// same bounded admission budget. Management traffic uses a separate listener.
type Admission struct {
	mu              sync.Mutex
	ports           map[uint16]int
	total, capacity int
	Rejected        atomic.Uint64
}

func NewAdmission(capacity int) *Admission {
	if capacity < 2 {
		capacity = 256
	}
	return &Admission{ports: map[uint16]int{}, capacity: capacity}
}
func (a *Admission) Acquire(port uint16, limit int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	limit = positive(limit, 32)
	if limit > a.capacity/2 {
		limit = a.capacity / 2
	}
	if a.total >= a.capacity || a.ports[port] >= limit {
		a.Rejected.Add(1)
		return false
	}
	a.total++
	a.ports[port]++
	return true
}
func (a *Admission) Release(port uint16) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ports[port] > 0 {
		a.total--
		a.ports[port]--
		if a.ports[port] == 0 {
			delete(a.ports, port)
		}
	}
}
func (a *Admission) Stats() map[string]uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]uint64{"in_flight": uint64(a.total), "capacity": uint64(a.capacity), "rejected": a.Rejected.Load()}
}

type PortMetrics struct {
	DiagnosticAlertAt      atomic.Int64
	Requests, Denied, Hits atomic.Uint64
	Last                   atomic.Pointer[Result]
	dailyMu                sync.Mutex
	daily                  DailyMetrics
	businessCache          atomic.Pointer[cache.WafCache]
	CacheHits, CacheMisses atomic.Uint64
}

func (m *PortMetrics) cacheStore() *cache.WafCache {
	if current := m.businessCache.Load(); current != nil {
		return current
	}
	created := cache.NewBoundedLocalCache(64, (64<<20)/int64(max(1, Ports.Limit())))
	if m.businessCache.CompareAndSwap(nil, created) {
		return created
	}
	return m.businessCache.Load()
}

type DailyMetrics struct {
	Day      int    `json:"day"`
	Requests uint64 `json:"requests"`
	Denied   uint64 `json:"denied"`
	Hits     uint64 `json:"hits"`
}

func Today() int { now := time.Now(); return now.Year()*10000 + int(now.Month())*100 + now.Day() }
func (m *PortMetrics) AddDaily(requests, denied, hits uint64) {
	m.dailyMu.Lock()
	defer m.dailyMu.Unlock()
	day := Today()
	if m.daily.Day != day {
		m.daily = DailyMetrics{Day: day}
	}
	m.daily.Requests += requests
	m.daily.Denied += denied
	m.daily.Hits += hits
}
func (m *PortMetrics) Daily() DailyMetrics {
	m.AddDaily(0, 0, 0)
	m.dailyMu.Lock()
	defer m.dailyMu.Unlock()
	return m.daily
}
func (m *PortMetrics) RestoreDaily(d DailyMetrics) {
	m.dailyMu.Lock()
	defer m.dailyMu.Unlock()
	if d.Day == Today() {
		m.daily = d
	}
}

func (m *PortMetrics) Stats() map[string]uint64 {
	d := m.Daily()
	out := map[string]uint64{"requests": m.Requests.Load(), "denied": m.Denied.Load(), "hits": m.Hits.Load(), "day": uint64(d.Day), "today_requests": d.Requests, "today_denied": d.Denied, "today_hits": d.Hits, "business_cache_hits": m.CacheHits.Load(), "business_cache_misses": m.CacheMisses.Load()}
	if store := m.businessCache.Load(); store != nil {
		for name, value := range store.Stats() {
			out["business_cache_"+name] = value
		}
	}
	return out
}
