package wafpipeline

import (
	"HomeWaf/wafbot"
	"context"
	"sync"
	"time"
)

type botQuery struct{ key, port, ua, ip string }
type botCacheEntry struct {
	verified, spoofed bool
	expires           time.Time
	port              string
}

// BOTCache limits DNS work to four workers and a bounded queue. It is invoked
// only by ports whose administrator explicitly enabled bot_verify_dns.
type BOTCache struct {
	mu      sync.Mutex
	entries map[string]botCacheEntry
	ports   map[string]int
	queue   chan botQuery
	Resolve func(ua, ip string) wafbot.BotResult
}

func NewBOTCache() *BOTCache {
	return &BOTCache{entries: map[string]botCacheEntry{}, ports: map[string]int{}, queue: make(chan botQuery, 128), Resolve: wafbot.DetermineNormalSearch}
}
func crawlerUA(ua string) string {
	return wafbot.CanonicalCrawlerUA(ua)
}
func (b *BOTCache) Lookup(port, ua, ip string, dry bool) (bool, bool) {
	ua = crawlerUA(ua)
	if ua == "" {
		return false, false
	}
	key := port + ":" + ua + ":" + ip
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if e, ok := b.entries[key]; ok {
		if e.expires.After(now) {
			return e.verified, e.spoofed
		}
		if !dry {
			delete(b.entries, key)
			b.ports[e.port]--
			if b.ports[e.port] == 0 {
				delete(b.ports, e.port)
			}
		}
	}
	if dry || len(b.entries) >= 10000 || b.ports[port] >= 500 {
		return false, false
	}
	select {
	case b.queue <- botQuery{key: key, port: port, ua: ua, ip: ip}:
		b.entries[key] = botCacheEntry{expires: now.Add(time.Minute), port: port}
		b.ports[port]++
	default:
	}
	return false, false
}
func (b *BOTCache) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case q := <-b.queue:
					result := b.Resolve(q.ua, q.ip)
					ttl := 10 * time.Minute
					if !result.StrongVerified && (!result.IsBot || result.IsNormalBot) {
						ttl = time.Minute
					}
					b.mu.Lock()
					if _, ok := b.entries[q.key]; ok {
						b.entries[q.key] = botCacheEntry{verified: result.StrongVerified, spoofed: result.IsBot && !result.IsNormalBot, expires: time.Now().Add(ttl), port: q.port}
					}
					b.mu.Unlock()
				}
			}
		}()
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			workers.Wait()
			return
		case <-ticker.C:
			b.mu.Lock()
			for key, e := range b.entries {
				if time.Now().After(e.expires) {
					delete(b.entries, key)
					b.ports[e.port]--
					if b.ports[e.port] == 0 {
						delete(b.ports, e.port)
					}
				}
			}
			b.mu.Unlock()
		}
	}
}
