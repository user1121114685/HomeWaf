// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package cache

import (
	"HomeWaf/common/zlog"
	"container/list"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type WafCache struct {
	cache             map[string]WafCacheItem
	mu                sync.Mutex
	order             *list.List
	nodes             map[string]*list.Element
	bytes             int64
	maxEntries        int
	maxBytes          int64
	evicted, rejected uint64
	stop              chan struct{}
	closeOnce         sync.Once
}
type WafCacheItem struct {
	value      interface{}
	size       int64
	createTime time.Time
	expireTime time.Time
	ttl        time.Duration
}

func InitWafCache() *WafCache {
	wafcache := &WafCache{
		cache: make(map[string]WafCacheItem),
		mu:    sync.Mutex{},
		stop:  make(chan struct{}),
	}
	go wafcache.ClearExpirationCacheRoutine()
	return wafcache
}

// NewBoundedLocalCache is a process-local cache without a cleanup goroutine.
// Reads expire entries lazily; writes evict under both byte and entry quotas.
func NewBoundedLocalCache(maxEntries int, maxBytes int64) *WafCache {
	return &WafCache{cache: make(map[string]WafCacheItem), maxEntries: max(1, maxEntries), maxBytes: max(1024, maxBytes)}
}
func (wafCache *WafCache) Set(key string, value interface{}) {
	wafCache.SetWithTTl(key, value, 100*365*24*time.Hour)
}

// SetWithTTl 写入并设置存活时长。
//
// 注意语义（踩过坑，别再改回去）：对**已存在**的 key 会沿用原 createTime，
// 即到期时刻 = 原 createTime + 新 ttl，而不是 now + ttl。所以它只适合
// "首次写入" 或 "刷新值但不想改变到期时刻" 的场景；凡是要"续命"的地方
// 一律用 SetWithTTlRenewTime，否则会把在线会话凭空缩短甚至当场判死
// （历史上改密续期就是这样让令牌立刻失效的）。
//
// 另需注意：Redis 后端(RedisCache.SetWithTTl)没有 createTime 概念，等价于 now + ttl。
// 两个后端在"已存在 key"上的行为并不一致，写业务代码时不要依赖该差异。
func (wafCache *WafCache) SetWithTTl(key string, value interface{}, ttl time.Duration) {
	wafCache.store(key, value, ttl, false)
}
func (wafCache *WafCache) SetWithTTlRenewTime(key string, value interface{}, ttl time.Duration) {
	wafCache.store(key, value, ttl, true)
}
func cacheValueSize(value interface{}) int64 {
	switch v := value.(type) {
	case []byte:
		return int64(len(v))
	case string:
		return int64(len(v))
	}
	data, err := json.Marshal(value)
	if err != nil {
		return -1
	}
	return int64(len(data))
}
func (wafCache *WafCache) store(key string, value interface{}, ttl time.Duration, renew bool) {
	size := cacheValueSize(value) + int64(len(key)) + 192
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	if wafCache.maxEntries <= 0 {
		wafCache.maxEntries = 65536
	}
	if wafCache.maxBytes <= 0 {
		wafCache.maxBytes = 64 << 20
	}
	if size > 8<<20 || size > wafCache.maxBytes || size < 192+int64(len(key)) {
		wafCache.rejected++
		return
	}
	if wafCache.cache == nil {
		wafCache.cache = map[string]WafCacheItem{}
	}
	if wafCache.order == nil {
		wafCache.order = list.New()
		wafCache.nodes = map[string]*list.Element{}
	}
	created := time.Now()
	if old, ok := wafCache.cache[key]; ok {
		if !renew {
			created = old.createTime
		}
		wafCache.remove(key)
	}
	for len(wafCache.cache) >= wafCache.maxEntries || wafCache.bytes+size > wafCache.maxBytes {
		if node := wafCache.order.Front(); node != nil {
			wafCache.remove(node.Value.(string))
		} else {
			for k := range wafCache.cache {
				wafCache.remove(k)
				break
			}
		}
		wafCache.evicted++
	}
	wafCache.cache[key] = WafCacheItem{value: value, size: size, createTime: created, expireTime: created.Add(ttl), ttl: ttl}
	wafCache.bytes += size
	wafCache.nodes[key] = wafCache.order.PushBack(key)
}
func (wafCache *WafCache) remove(key string) {
	if item, ok := wafCache.cache[key]; ok {
		wafCache.bytes -= item.size
		delete(wafCache.cache, key)
	}
	if node := wafCache.nodes[key]; node != nil {
		wafCache.order.Remove(node)
		delete(wafCache.nodes, key)
	}
}
func (wafCache *WafCache) used(key string) {
	if node := wafCache.nodes[key]; node != nil {
		wafCache.order.MoveToBack(node)
	}
}
func (wafCache *WafCache) Stats() map[string]uint64 {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	return map[string]uint64{"keys": uint64(len(wafCache.cache)), "bytes": uint64(wafCache.bytes), "evicted": wafCache.evicted, "rejected": wafCache.rejected}
}
func (wafCache *WafCache) Close() {
	wafCache.closeOnce.Do(func() {
		if wafCache.stop != nil {
			close(wafCache.stop)
		}
	})
}
func (wafCache *WafCache) GetAs(key string, out interface{}) error {
	val := wafCache.Get(key)
	if val == nil {
		return ErrCacheMiss
	}
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (wafCache *WafCache) GetBytes(key string) ([]byte, error) {
	key1Value := wafCache.Get(key)
	if str, ok := key1Value.([]byte); ok {
		return str, nil
	}
	return nil, ErrCacheMiss
}
func (wafCache *WafCache) GetString(key string) (string, error) {
	key1Value := wafCache.Get(key)
	if str, ok := key1Value.(string); ok {
		return str, nil
	}
	return "", ErrCacheMiss
}
func (wafCache *WafCache) GetInt(key string) (int, error) {
	key1Value := wafCache.Get(key)
	if str, ok := key1Value.(int); ok {
		return str, nil
	}
	return -1, ErrCacheMiss
}
func (wafCache *WafCache) IsKeyExist(key string) bool {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	item, found := wafCache.cache[key]
	if !found {
		return false
	}
	if time.Since(item.createTime) <= item.ttl {
		wafCache.used(key)
		return true
	}
	wafCache.remove(key)
	return false
}

// ExistsE 内存实现不会有后端故障，error 恒为 nil
func (wafCache *WafCache) ExistsE(key string) (bool, error) {
	return wafCache.IsKeyExist(key), nil
}

// GetAsEx 读取并续期。与 SetWithTTlRenewTime 一致，续期从当前时刻重新计时。
func (wafCache *WafCache) GetAsEx(key string, out interface{}, ttl time.Duration) error {
	wafCache.mu.Lock()
	item, found := wafCache.cache[key]
	if found && time.Since(item.createTime) > item.ttl {
		wafCache.remove(key)
		found = false
	}
	if found && ttl > 0 {
		wafCache.used(key)
		now := time.Now()
		wafCache.cache[key] = WafCacheItem{
			value:      item.value,
			size:       item.size,
			createTime: now,
			expireTime: now.Add(ttl),
			ttl:        ttl,
		}
	}
	wafCache.mu.Unlock()

	if !found {
		return ErrCacheMiss
	}
	b, err := json.Marshal(item.value)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Touch 只续期不取值
func (wafCache *WafCache) Touch(key string, ttl time.Duration) error {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	item, found := wafCache.cache[key]
	if !found {
		return ErrCacheMiss
	}
	if time.Since(item.createTime) > item.ttl {
		wafCache.remove(key)
		return ErrCacheMiss
	}
	if ttl > 0 {
		wafCache.used(key)
		now := time.Now()
		wafCache.cache[key] = WafCacheItem{
			value:      item.value,
			size:       item.size,
			createTime: now,
			expireTime: now.Add(ttl),
			ttl:        ttl,
		}
	}
	return nil
}

func (wafCache *WafCache) Get(key string) interface{} {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	item, found := wafCache.cache[key]
	if !found {
		return nil
	}
	if time.Since(item.createTime) <= item.ttl {
		wafCache.used(key)
		return item.value
	}
	wafCache.remove(key)
	return nil
}

func (wafCache *WafCache) Remove(key string) interface{} {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	_, found := wafCache.cache[key]
	if !found {
		return nil
	}
	wafCache.remove(key)
	return nil
}
func (wafCache *WafCache) GetExpireTime(key string) (time.Time, error) {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	item, found := wafCache.cache[key]
	if !found {
		return time.Time{}, ErrCacheMiss
	}
	if time.Since(item.createTime) <= item.ttl {
		return item.expireTime, nil
	}
	zlog.Debug("GetExpireTime CLEAR CACHE EXPIRE :" + key)
	wafCache.remove(key)
	return time.Time{}, ErrCacheMiss
}
func (wafCache *WafCache) ClearExpirationCache() {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	now := time.Now()
	for key, item := range wafCache.cache {
		if now.Sub(item.createTime) > item.ttl {
			zlog.Debug("ClearExpirationCache CLEAR CACHE EXPIRE :" + key)
			wafCache.remove(key)
		}
	}
}
func (wafCache *WafCache) ClearExpirationCacheRoutine() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-wafCache.stop:
			return
		case <-ticker.C:
			wafCache.ClearExpirationCache()
		}
	}
}

// ListAvailableKeys 列出所有可用的键和剩余时间
func (wafCache *WafCache) ListAvailableKeys() map[string]time.Duration {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	now := time.Now()
	availableKeys := make(map[string]time.Duration)

	for key, item := range wafCache.cache {
		remainingTime := item.ttl - now.Sub(item.createTime)
		if remainingTime > 0 {
			availableKeys[key] = remainingTime
		} else {
			wafCache.remove(key) // 删除过期项
		}
	}
	return availableKeys
}

// ListAvailableKeysWithPrefix 列出指定前缀的可用键和剩余时间
func (wafCache *WafCache) ListAvailableKeysWithPrefix(prefix string) map[string]time.Duration {
	wafCache.mu.Lock()
	defer wafCache.mu.Unlock()
	now := time.Now()
	availableKeys := make(map[string]time.Duration)

	for key, item := range wafCache.cache {
		if strings.HasPrefix(key, prefix) {
			remainingTime := item.ttl - now.Sub(item.createTime)
			if remainingTime > 0 {
				availableKeys[key] = remainingTime
			} else {
				wafCache.remove(key) // 删除过期项
			}
		}
	}
	return availableKeys
}
