package accessgate

import (
	"container/list"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"sync"
	"time"
)

type passwordCacheEntry struct {
	key     [32]byte
	expires time.Time
}

// PasswordCache caches successful bcrypt proofs only, under a process-local
// HMAC key. Changed password hashes cannot hit old entries; misses and unknown
// users still perform bcrypt. No passwords or reusable bearer tokens are kept.
type PasswordCache struct {
	mu       sync.Mutex
	key      [32]byte
	entries  map[[32]byte]*list.Element
	order    *list.List
	capacity int
}

func NewPasswordCache(capacity int) *PasswordCache {
	if capacity < 1 {
		capacity = 10000
	}
	c := &PasswordCache{entries: map[[32]byte]*list.Element{}, order: list.New(), capacity: capacity}
	if _, err := rand.Read(c.key[:]); err != nil {
		panic(err)
	}
	return c
}
func (c *PasswordCache) fingerprint(hash, password string) [32]byte {
	mac := hmac.New(sha256.New, c.key[:])
	mac.Write([]byte(hash))
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	var key [32]byte
	copy(key[:], mac.Sum(nil))
	return key
}
func (c *PasswordCache) Cached(hash, password string) bool {
	if hash == "" || hash == DummyPasswordHash || len(password) > 72 {
		return false
	}
	key := c.fingerprint(hash, password)
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key]
	if e == nil {
		return false
	}
	if time.Now().After(e.Value.(passwordCacheEntry).expires) {
		delete(c.entries, key)
		c.order.Remove(e)
		return false
	}
	c.order.MoveToFront(e)
	return true
}
func (c *PasswordCache) Verify(hash, password string) bool {
	if c.Cached(hash, password) {
		return true
	}
	if len(password) > 72 {
		_ = VerifyPassword(DummyPasswordHash, "invalid-oversize-password")
		return false
	}
	if hash == "" || hash == DummyPasswordHash {
		_ = VerifyPassword(DummyPasswordHash, password)
		return false
	}
	ok := VerifyPassword(hash, password)
	if !ok {
		return false
	}
	key := c.fingerprint(hash, password)
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.order.MoveToFront(e)
		return true
	}
	if len(c.entries) >= c.capacity {
		last := c.order.Back()
		delete(c.entries, last.Value.(passwordCacheEntry).key)
		c.order.Remove(last)
	}
	c.entries[key] = c.order.PushFront(passwordCacheEntry{key: key, expires: time.Now().Add(time.Minute)})
	return true
}
