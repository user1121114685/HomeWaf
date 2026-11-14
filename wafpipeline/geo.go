package wafpipeline

import (
	"container/list"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

type GeoResult struct {
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	OK       bool   `json:"ok"`
}

func matchesRegion(geo GeoResult, regions []Region) bool {
	if !geo.OK {
		return false
	}
	for _, region := range regions {
		if strings.EqualFold(region.Country, geo.Country) && (region.Province == "" || strings.EqualFold(region.Province, geo.Province)) {
			return true
		}
	}
	return false
}

// Country-only databases identify HK/MO/TW separately; province databases may
// place them under China. Recognize both forms, never guessing unknown locations.
func mainlandChina(geo GeoResult) bool {
	if !geo.OK {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(geo.Country)) {
	case "中国", "中國", "中国大陆", "中國大陸", "cn", "china", "mainland china":
	default:
		return false
	}
	province := strings.ToLower(strings.TrimSpace(geo.Province))
	for _, excluded := range []string{"香港", "澳门", "澳門", "台湾", "臺灣", "台灣", "hong kong", "hongkong", "macao", "macau", "taiwan"} {
		if strings.Contains(province, excluded) {
			return false
		}
	}
	return province != "hk" && province != "mo" && province != "tw"
}

type geoItem struct {
	ip         netip.Addr
	value      GeoResult
	until      time.Time
	generation uint64
}
type GeoCache struct {
	mu         sync.Mutex
	items      map[netip.Addr]*list.Element
	lru        *list.List
	lookup     func(netip.Addr) GeoResult
	generation func() uint64
	regions    map[Region]bool
	limit      int
	ttl        time.Duration
	epoch      uint64
}

func NewGeoCache(lookup func(netip.Addr) GeoResult, generation func() uint64) *GeoCache {
	return &GeoCache{items: make(map[netip.Addr]*list.Element), lru: list.New(), lookup: lookup, generation: generation, regions: make(map[Region]bool), limit: 10000, ttl: 10 * time.Minute}
}
func (g *GeoCache) Lookup(ip netip.Addr) GeoResult {
	ip = ip.Unmap()
	if protectedAddress(ip) || g.lookup == nil {
		return GeoResult{}
	}
	generation := uint64(0)
	if g.generation != nil {
		generation = g.generation()
	}
	now := time.Now()
	g.mu.Lock()
	g.resetGeneration(generation)
	if el := g.items[ip]; el != nil {
		item := el.Value.(geoItem)
		if item.generation == generation && now.Before(item.until) {
			g.lru.MoveToFront(el)
			g.mu.Unlock()
			return item.value
		}
		delete(g.items, ip)
		g.lru.Remove(el)
	}
	g.mu.Unlock()
	v := g.lookup(ip)
	if v.Country == "" || v.Country == "未知" || v.Country == "内网" || v.Country == "0" || strings.EqualFold(v.Country, "unknown") {
		v.OK = false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.generation != nil && g.generation() != generation {
		return v
	}
	if el := g.items[ip]; el != nil {
		g.lru.Remove(el)
	}
	g.items[ip] = g.lru.PushFront(geoItem{ip, v, now.Add(g.ttl), generation})
	for len(g.items) > g.limit {
		el := g.lru.Back()
		delete(g.items, el.Value.(geoItem).ip)
		g.lru.Remove(el)
	}
	if v.OK && len(g.regions) < 10000 {
		g.regions[Region{v.Country, v.Province}] = true
	}
	return v
}
func (g *GeoCache) Clear() {
	g.mu.Lock()
	g.items = make(map[netip.Addr]*list.Element)
	g.lru.Init()
	g.regions = make(map[Region]bool)
	g.mu.Unlock()
}

// Regions reports actual values observed from the active database, never a hardcoded country list.
func (g *GeoCache) Regions() []Region {
	generation := uint64(0)
	if g.generation != nil {
		generation = g.generation()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.resetGeneration(generation)
	out := make([]Region, 0, len(g.regions))
	for k := range g.regions {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country == out[j].Country {
			return out[i].Province < out[j].Province
		}
		return out[i].Country < out[j].Country
	})
	return out
}

func (g *GeoCache) resetGeneration(generation uint64) {
	if g.epoch != generation {
		g.items = make(map[netip.Addr]*list.Element)
		g.lru.Init()
		g.regions = make(map[Region]bool)
		g.epoch = generation
	}
}
