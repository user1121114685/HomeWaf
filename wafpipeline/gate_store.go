package wafpipeline

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type GateAccount struct {
	PortID       string `json:"port_id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	TOTPSecret   string `json:"-"`
	Version      string `json:"-"`
	Enabled      bool   `json:"enabled"`
	ExpiresAt    int64  `json:"expires_at"`
}
type GateSessionRecord struct {
	ID        string `json:"id"`
	PortID    string `json:"port_id"`
	Username  string `json:"username"`
	TokenHash string `json:"-"`
	Version   string `json:"-"`
	ClientIP  string `json:"client_ip"`
	CreatedAt int64  `json:"created_at"`
	LastSeen  int64  `json:"last_seen"`
	ExpiresAt int64  `json:"expires_at"`
}
type gateProbe struct {
	paths map[string]time.Time
	seen  time.Time
	port  string
}
type GateStore struct {
	mu            sync.Mutex
	users         map[string]GateAccount
	sessions      map[string]GateSessionRecord
	probes        map[string]*gateProbe
	probePorts    map[string]int
	active        map[string]int
	sessionPorts  map[string]int
	totalActive   int
	version       uint64
	persisted     uint64
	persistMu     sync.Mutex
	Persist       func([]GateSessionRecord) error
	PersistErrors atomic.Uint64
}

func NewGateStore() *GateStore {
	return &GateStore{users: map[string]GateAccount{}, sessions: map[string]GateSessionRecord{}, probes: map[string]*gateProbe{}, probePorts: map[string]int{}, active: map[string]int{}, sessionPorts: map[string]int{}}
}
func accountKey(port, user string) string { return port + "\x00" + user }
func (s *GateStore) ReplaceAccounts(port string, users []GateAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, u := range s.users {
		if u.PortID == port {
			delete(s.users, k)
		}
	}
	for _, u := range users {
		if u.PortID == port {
			s.users[accountKey(port, u.Username)] = u
		}
	}
	for k, v := range s.sessions {
		if v.PortID == port {
			u, ok := s.users[accountKey(port, v.Username)]
			if !ok || !u.Enabled || u.Version != v.Version {
				s.remove(k)
			}
		}
	}
}
func (s *GateStore) Account(port, user string) (GateAccount, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[accountKey(port, user)]
	return u, ok
}
func (s *GateStore) Acquire(port string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.totalActive >= 32 || s.active[port] >= 4 {
		return false
	}
	s.totalActive++
	s.active[port]++
	return true
}
func (s *GateStore) Release(port string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalActive--
	s.active[port]--
	if s.active[port] == 0 {
		delete(s.active, port)
	}
}
func (s *GateStore) remove(key string) {
	if v, ok := s.sessions[key]; ok {
		delete(s.sessions, key)
		s.sessionPorts[v.PortID]--
		if s.sessionPorts[v.PortID] == 0 {
			delete(s.sessionPorts, v.PortID)
		}
		s.version++
	}
}
func (s *GateStore) Add(v GateSessionRecord, max int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[accountKey(v.PortID, v.Username)]
	if !ok || !u.Enabled || u.Version != v.Version || u.ExpiresAt > 0 && u.ExpiresAt <= time.Now().Unix() {
		return errors.New("account changed")
	}
	var same []GateSessionRecord
	for k, old := range s.sessions {
		if old.ExpiresAt <= time.Now().Unix() {
			s.remove(k)
			continue
		}
		if old.PortID == v.PortID && old.Username == v.Username {
			same = append(same, old)
		}
	}
	if max > 0 && len(same) >= max {
		sort.Slice(same, func(i, j int) bool { return same[i].CreatedAt < same[j].CreatedAt })
		for _, old := range same[:len(same)-max+1] {
			s.remove(old.TokenHash)
		}
	}
	if len(s.sessions) >= 20000 || s.sessionPorts[v.PortID] >= 1000 {
		return errors.New("session capacity reached")
	}
	s.sessions[v.TokenHash] = v
	s.sessionPorts[v.PortID]++
	s.version++
	return nil
}
func (s *GateStore) Valid(hash, port, ip string, cfg GateConfig, dry bool) (GateSessionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[hash]
	now := time.Now().Unix()
	u, exists := s.users[accountKey(port, v.Username)]
	if !ok || !exists || !u.Enabled || v.PortID != port || u.Version != v.Version || u.ExpiresAt > 0 && u.ExpiresAt <= now || v.ExpiresAt <= now || cfg.BindIP && v.ClientIP != ip || cfg.IdleSeconds > 0 && now-v.LastSeen >= int64(cfg.IdleSeconds) {
		return GateSessionRecord{}, false
	}
	if !dry && now > v.LastSeen {
		v.LastSeen = now
		s.sessions[hash] = v
		s.version++
	}
	return v, true
}
func (s *GateStore) Revoke(port, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if v.PortID == port && v.ID == id {
			s.remove(k)
			return true
		}
	}
	return false
}
func (s *GateStore) Sessions(port string) []GateSessionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []GateSessionRecord{}
	for _, v := range s.sessions {
		if (port == "" || v.PortID == port) && v.ExpiresAt > time.Now().Unix() {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}
func (s *GateStore) Restore(rows []GateSessionRecord) {
	for _, v := range rows {
		_ = s.Add(v, 0)
	}
}
func (s *GateStore) Flush() error {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	if s.Persist == nil {
		return nil
	}
	s.mu.Lock()
	version := s.version
	if version == s.persisted {
		s.mu.Unlock()
		return nil
	}
	rows := make([]GateSessionRecord, 0, len(s.sessions))
	for _, v := range s.sessions {
		rows = append(rows, v)
	}
	s.mu.Unlock()
	if err := s.Persist(rows); err != nil {
		s.PersistErrors.Add(1)
		return err
	}
	s.mu.Lock()
	s.persisted = version
	s.mu.Unlock()
	return nil
}
func (s *GateStore) Run(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	defer s.Flush()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.Flush()
		}
	}
}
func (s *GateStore) Probe(c *RequestContext) bool {
	if c.DryRun {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := c.PortID + ":" + c.ClientIP.String()
	now := time.Now()
	p := s.probes[key]
	if p == nil {
		if len(s.probes) >= 20000 || s.probePorts[c.PortID] >= 100 {
			return false
		}
		p = &gateProbe{paths: map[string]time.Time{}, port: c.PortID}
		s.probes[key] = p
		s.probePorts[c.PortID]++
	}
	p.seen = now
	for k, t := range p.paths {
		if now.Sub(t) > time.Minute {
			delete(p.paths, k)
		}
	}
	hash := dimension(c.View.Path)
	if _, ok := p.paths[hash]; ok {
		return false
	}
	if len(p.paths) >= 20 {
		return false
	}
	p.paths[hash] = now
	return len(p.paths) == 20
}
func (s *GateStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.sessions {
		if v.ExpiresAt <= now.Unix() {
			s.remove(k)
		}
	}
	for k, p := range s.probes {
		if now.Sub(p.seen) > time.Minute {
			delete(s.probes, k)
			s.probePorts[p.port]--
			if s.probePorts[p.port] == 0 {
				delete(s.probePorts, p.port)
			}
		}
	}
}
