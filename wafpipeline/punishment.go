package wafpipeline

import (
	"HomeWaf/common/queue"
	"container/list"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (e HitEvent) QueuePriority() int {
	if e.Observe {
		return 0
	}
	if e.Severity.Rank() >= High.Rank() {
		return 2
	}
	return 1
}
func (e HitEvent) QueueKey() string { return e.PortID }
func (e HitEvent) QueueSize() int64 {
	return int64(256 + len(e.Evidence) + len(e.Path) + len(e.RuleID) + len(e.ReqID))
}

type PunishmentRecord struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	PortID        string `json:"port_id"`
	Scope         string `json:"scope"`
	Prefix        string `json:"prefix"`
	Reason        string `json:"reason"`
	RuleID        string `json:"rule_id"`
	Hits          int    `json:"hits"`
	Strikes       int    `json:"strikes"`
	CreatedAt     int64  `json:"created_at"`
	ExpiresAt     int64  `json:"expires_at"`
	CooldownUntil int64  `json:"cooldown_until,omitempty"`
}
type PunishmentChange struct {
	Record PunishmentRecord `json:"record"`
	Event  string           `json:"event"`
}
type punishmentView struct {
	Bans      map[string]map[netip.Prefix]PunishmentRecord
	Honeypots map[string]map[netip.Prefix]PunishmentRecord
	Locks     map[string]PunishmentRecord
	Widths    map[string][]int
}
type hitWindow struct {
	counts  [64]uint32
	epochs  [64]int64
	lastReq string
	width   time.Duration
	paths   map[[32]byte]time.Time
}
type counterEntry struct {
	key, port string
	window    hitWindow
	local     *list.Element
}
type statePort struct {
	order    *list.List
	distinct map[netip.Addr]time.Time
	cooldown time.Time
}

// Punisher owns bounded mutable counters and publishes immutable decision
// snapshots. Requests only enqueue events and read the atomic snapshot.
type Punisher struct {
	Queue                                           *queue.Queue
	Lookup                                          func(string) *Snapshot
	Protected                                       func(netip.Addr) bool
	Persist                                         func([]PunishmentRecord) error
	OnChange                                        func(PunishmentChange)
	OnEvent                                         func(HitEvent)
	Now                                             func() time.Time
	mu                                              sync.Mutex
	flushMu                                         sync.Mutex
	state                                           atomic.Pointer[punishmentView]
	records                                         map[string]PunishmentRecord
	recordCounts                                    map[string]int
	counters                                        map[string]*counterEntry
	ports                                           map[string]*statePort
	start                                           time.Time
	revision, flushed                               uint64
	maxCounters, maxRecords, perPort                int
	Processed, Ignored, QuotaDropped, PersistErrors atomic.Uint64
}

func NewPunisher() *Punisher {
	p := &Punisher{Queue: queue.NewBoundedQueue(4096, 256, 4<<20), Now: time.Now, start: time.Now(), records: map[string]PunishmentRecord{}, recordCounts: map[string]int{}, counters: map[string]*counterEntry{}, ports: map[string]*statePort{}, maxCounters: 20000, maxRecords: 20000, perPort: 1000}
	p.Lookup = func(id string) *Snapshot {
		n, e := strconv.ParseUint(strings.TrimPrefix(id, "port:"), 10, 16)
		if e != nil {
			return nil
		}
		s, _ := Ports.Get(uint16(n))
		return s
	}
	p.publish()
	return p
}
func bound(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
func (p *Punisher) Submit(e HitEvent) bool {
	if !e.ClientIP.IsValid() || len(e.PortID) > 32 {
		return false
	}
	e.ClientIP = e.ClientIP.Unmap()
	e.Path = bound(e.Path, 1024)
	e.Evidence = bound(e.Evidence, 256)
	e.RuleID = bound(e.RuleID, 128)
	e.ReqID = bound(e.ReqID, 64)
	e.Category = bound(e.Category, 64)
	return p.Queue.TryEnqueue(e)
}
func (p *Punisher) port(id string) *statePort {
	s := p.ports[id]
	if s == nil {
		s = &statePort{order: list.New(), distinct: map[netip.Addr]time.Time{}}
		p.ports[id] = s
	}
	return s
}
func prefixFor(ip netip.Addr, bits int) netip.Prefix {
	ip = ip.Unmap()
	if ip.Is4() {
		bits = 32
	}
	return netip.PrefixFrom(ip, bits).Masked()
}
func (p *Punisher) protected(s *Snapshot, e HitEvent) bool {
	if e.Trusted || protectedAddress(e.ClientIP) || p.Protected != nil && p.Protected(e.ClientIP) {
		return true
	}
	return s.Compiled != nil && s.Compiled.Trusted.Contains(net.IP(e.ClientIP.AsSlice()), p.Now().Unix())
}
func recordKey(kind, scope, port, prefix string) string {
	if scope == "global" {
		port = "*"
	}
	return kind + "|" + port + "|" + prefix
}
func active(r PunishmentRecord, now int64) bool { return r.ExpiresAt == 0 || r.ExpiresAt > now }
func (p *Punisher) put(r PunishmentRecord) bool {
	if old, exists := p.records[r.ID]; !exists {
		if len(p.records) >= p.maxRecords || p.recordCounts[r.PortID] >= p.perPort {
			p.QuotaDropped.Add(1)
			return false
		}
		p.recordCounts[r.PortID]++
	} else if old.PortID != r.PortID {
		if p.recordCounts[r.PortID] >= p.perPort {
			p.QuotaDropped.Add(1)
			return false
		}
		p.recordCounts[old.PortID]--
		p.recordCounts[r.PortID]++
	}
	p.records[r.ID] = r
	p.revision++
	return true
}
func (p *Punisher) remove(id string) {
	if r, ok := p.records[id]; ok {
		delete(p.records, id)
		p.recordCounts[r.PortID]--
		if p.recordCounts[r.PortID] == 0 {
			delete(p.recordCounts, r.PortID)
		}
		p.revision++
	}
}
func (p *Punisher) count(port, ip, category, req, path string, window int, distinct bool, limit int, now time.Time) int {
	key := port + "|" + ip + "|" + category
	state := p.port(port)
	entry := p.counters[key]
	if entry == nil {
		quota := min(p.perPort, limit)
		if state.order.Len() >= quota || len(p.counters) >= p.maxCounters {
			old := state.order.Back()
			if old == nil {
				p.QuotaDropped.Add(1)
				return 0
			}
			victim := old.Value.(*counterEntry)
			delete(p.counters, victim.key)
			state.order.Remove(old)
		}
		entry = &counterEntry{key: key, port: port}
		entry.local = state.order.PushFront(entry)
		p.counters[key] = entry
	}
	state.order.MoveToFront(entry.local)
	w := &entry.window
	if req != "" && w.lastReq == req {
		return 0
	}
	w.lastReq = req
	if distinct {
		if w.paths == nil {
			w.paths = map[[32]byte]time.Time{}
		}
		for key, seen := range w.paths {
			if now.Sub(seen) >= time.Duration(window)*time.Second {
				delete(w.paths, key)
			}
		}
		// At most 128 distinct path hashes per counter, never retain URLs.
		hash := sha256.Sum256([]byte(path))
		if _, ok := w.paths[hash]; ok {
			w.paths[hash] = now
			return len(w.paths)
		}
		if len(w.paths) >= 128 {
			return len(w.paths)
		}
		w.paths[hash] = now
		return len(w.paths)
	}
	width := (time.Duration(window)*time.Second + 63) / 64
	if w.width != width {
		*w = hitWindow{width: width, lastReq: req}
	}
	tick := max(int64(1), int64(now.Sub(p.start)/width)+1)
	slot := tick % 64
	if w.epochs[slot] != tick {
		w.counts[slot] = 0
		w.epochs[slot] = tick
	}
	if w.counts[slot] < 1000000 {
		w.counts[slot]++
	}
	total := 0
	for i, n := range w.counts {
		if w.epochs[i] > 0 && w.epochs[i] <= tick && now.Sub(p.start)-time.Duration(w.epochs[i]-1)*width < time.Duration(window)*time.Second {
			total += int(n)
		}
	}
	return total
}
func (p *Punisher) ban(s *Snapshot, e HitEvent, count, duration int, now time.Time) []PunishmentChange {
	policy := s.Config.Policy.HitPolicy
	prefix := prefixFor(e.ClientIP, policy.IPv6Prefix).String()
	id := recordKey("ban", policy.BanScope, e.PortID, prefix)
	if current, ok := p.records[id]; ok && active(current, now.Unix()) {
		return nil
	}
	historyID := recordKey("history", policy.BanScope, e.PortID, prefix)
	needed := 0
	for _, key := range []string{id, historyID} {
		if _, ok := p.records[key]; !ok {
			needed++
		}
	}
	if len(p.records)+needed > p.maxRecords || p.recordCounts[e.PortID]+needed > p.perPort {
		p.QuotaDropped.Add(1)
		return nil
	}
	history := p.records[historyID]
	strikes := 1
	if history.ID != "" && active(history, now.Unix()) {
		strikes = history.Strikes + 1
	}
	seconds := int64(duration)
	for i := 1; i < strikes && seconds < 7*86400; i++ {
		seconds = min(seconds*4, int64(7*86400))
	}
	r := PunishmentRecord{ID: id, Kind: "ban", PortID: e.PortID, Scope: policy.BanScope, Prefix: prefix, Reason: e.Category, RuleID: e.RuleID, Hits: count, Strikes: strikes, CreatedAt: now.Unix(), ExpiresAt: now.Unix() + seconds}
	if !p.put(r) {
		return nil
	}
	history = r
	history.ID = historyID
	history.Kind = "history"
	history.ExpiresAt = now.Add(30 * 24 * time.Hour).Unix()
	p.put(history)
	p.remove(recordKey("honeypot", "port", e.PortID, prefix))
	return []PunishmentChange{{r, "ban_created"}}
}
func (p *Punisher) honey(s *Snapshot, e HitEvent, count int, now time.Time) []PunishmentChange {
	if !s.Config.Policy.Honeypot.Enabled {
		return nil
	}
	prefix := prefixFor(e.ClientIP, s.Config.Policy.HitPolicy.IPv6Prefix).String()
	id := recordKey("honeypot", "port", e.PortID, prefix)
	if current, ok := p.records[id]; ok && active(current, now.Unix()) {
		return nil
	}
	r := PunishmentRecord{ID: id, Kind: "honeypot", PortID: e.PortID, Scope: "port", Prefix: prefix, Reason: e.Category, RuleID: e.RuleID, Hits: count, CreatedAt: now.Unix(), ExpiresAt: now.Add(time.Duration(s.Config.Policy.Honeypot.TTLSeconds) * time.Second).Unix()}
	if !p.put(r) {
		return nil
	}
	return []PunishmentChange{{r, "honeypot_created"}}
}
func (p *Punisher) lock(s *Snapshot, e HitEvent, now time.Time) []PunishmentChange {
	cfg := s.Config.Policy.HitPolicy.Lockdown
	if !cfg.Enabled || e.Severity.Rank() < High.Rank() || strings.HasPrefix(e.Category, "response_") {
		return nil
	}
	state := p.port(e.PortID)
	if now.Before(state.cooldown) {
		return nil
	}
	for ip, t := range state.distinct {
		if now.Sub(t) >= time.Duration(cfg.WindowSeconds)*time.Second {
			delete(state.distinct, ip)
		}
	}
	key := prefixFor(e.ClientIP, s.Config.Policy.HitPolicy.IPv6Prefix).Addr()
	if len(state.distinct) < 1000 {
		state.distinct[key] = now
	}
	if len(state.distinct) < cfg.DistinctIPs {
		return nil
	}
	id := recordKey("lockdown", "port", e.PortID, "")
	old := p.records[id]
	r := PunishmentRecord{ID: id, Kind: "lockdown", PortID: e.PortID, Scope: "port", Reason: "distributed_high_severity", Hits: len(state.distinct), CreatedAt: now.Unix(), ExpiresAt: now.Add(time.Duration(cfg.Seconds) * time.Second).Unix(), CooldownUntil: now.Add(time.Duration(cfg.Seconds+cfg.CooldownSeconds) * time.Second).Unix()}
	if old.ID != "" {
		r.CreatedAt = old.CreatedAt
	}
	if !p.put(r) {
		return nil
	}
	if old.ID != "" {
		return nil
	}
	return []PunishmentChange{{r, "lockdown_started"}}
}
func (p *Punisher) process(s *Snapshot, e HitEvent, now time.Time) []PunishmentChange {
	p.Processed.Add(1)
	if e.Observe || !s.Config.Policy.HitPolicy.Enabled || p.protected(s, e) {
		p.Ignored.Add(1)
		return nil
	}
	cfg := s.Config.Policy
	ip := prefixFor(e.ClientIP, cfg.HitPolicy.IPv6Prefix).String()
	changes := p.lock(s, e, now)
	if e.Category == "custom_rule" && e.BanSeconds > 0 {
		return append(changes, p.ban(s, e, 1, min(e.BanSeconds, 7*86400), now)...)
	}
	if e.Category == "honeypot_escalation" {
		return append(changes, p.ban(s, e, 1, cfg.HitPolicy.Critical.BanSeconds, now)...)
	}
	if e.Category == "trap_path" {
		return append(changes, p.honey(s, e, 1, now)...)
	}
	if e.Category == "auth_fail" {
		long := effectiveThreshold(cfg.HitPolicy.LoginBan, Threshold{600, 20, 1800})
		n := p.count(e.PortID, ip, "auth_ban", e.ReqID, "", long.WindowSeconds, false, cfg.HitPolicy.MaxKeys, now)
		if n >= long.Count {
			return append(changes, p.ban(s, e, n, long.BanSeconds, now)...)
		}
		n = p.count(e.PortID, ip, "auth_fail", e.ReqID, "", 300, false, cfg.HitPolicy.MaxKeys, now)
		if n >= cfg.Honeypot.AuthFailThreshold {
			return append(changes, p.honey(s, e, n, now)...)
		}
		return changes
	}
	if e.Category == "weak_pwd" {
		n := p.count(e.PortID, ip, e.Category, e.ReqID, "", 300, false, cfg.HitPolicy.MaxKeys, now)
		if n >= cfg.Honeypot.WeakPasswordThreshold {
			return append(changes, p.honey(s, e, n, now)...)
		}
		return changes
	}
	if strings.HasPrefix(e.Category, "response_") {
		if !cfg.HitPolicy.ResponseSignals {
			return changes
		}
		var threshold Threshold
		switch e.Category {
		case "response_404":
			threshold = effectiveThreshold(cfg.HitPolicy.Response404, Threshold{60, 20, 0})
		case "response_auth":
			threshold = effectiveThreshold(cfg.HitPolicy.ResponseAuth, Threshold{300, 10, 0})
		case "response_5xx":
			threshold = effectiveThreshold(cfg.HitPolicy.Response5xx, Threshold{300, 5, 0})
		default:
			return changes
		}
		n := p.count(e.PortID, ip, e.Category, e.ReqID, e.Path, threshold.WindowSeconds, e.Category == "response_404", cfg.HitPolicy.MaxKeys, now)
		if n >= threshold.Count {
			return append(changes, p.honey(s, e, n, now)...)
		}
		return changes
	}
	var threshold Threshold
	switch e.Severity {
	case Critical:
		threshold = cfg.HitPolicy.Critical
	case High:
		threshold = cfg.HitPolicy.High
	case Medium:
		threshold = cfg.HitPolicy.Medium
	default:
		return changes
	}
	n := p.count(e.PortID, ip, "severity:"+string(e.Severity), e.ReqID, "", threshold.WindowSeconds, false, cfg.HitPolicy.MaxKeys, now)
	if n >= threshold.Count {
		changes = append(changes, p.ban(s, e, n, threshold.BanSeconds, now)...)
	}
	return changes
}
func effectiveThreshold(t, fallback Threshold) Threshold {
	if t.WindowSeconds == 0 && t.Count == 0 && t.BanSeconds == 0 {
		return fallback
	}
	return t
}

func (p *Punisher) publish() {
	v := &punishmentView{Bans: map[string]map[netip.Prefix]PunishmentRecord{}, Honeypots: map[string]map[netip.Prefix]PunishmentRecord{}, Locks: map[string]PunishmentRecord{}, Widths: map[string][]int{}}
	for _, r := range p.records {
		if r.Kind == "lockdown" {
			v.Locks[r.PortID] = r
			continue
		}
		if r.Kind != "ban" && r.Kind != "honeypot" {
			continue
		}
		prefix, err := netip.ParsePrefix(r.Prefix)
		if err != nil {
			continue
		}
		scope := r.PortID
		if r.Scope == "global" {
			scope = "*"
		}
		groups := v.Bans
		if r.Kind == "honeypot" {
			groups = v.Honeypots
		}
		if groups[scope] == nil {
			groups[scope] = map[netip.Prefix]PunishmentRecord{}
		}
		groups[scope][prefix] = r
		width := prefix.Bits()
		if prefix.Addr().Is4() {
			width += 256
		}
		found := false
		for _, n := range v.Widths[scope] {
			if n == width {
				found = true
				break
			}
		}
		if !found {
			v.Widths[scope] = append(v.Widths[scope], width)
		}
	}
	p.state.Store(v)
}
func lookupState(groups map[string]map[netip.Prefix]PunishmentRecord, widths map[string][]int, port string, ip netip.Addr, now int64) (PunishmentRecord, bool) {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return PunishmentRecord{}, false
	}
	for _, key := range []string{port, "*"} {
		group := groups[key]
		if len(group) == 0 {
			continue
		}
		for _, width := range widths[key] {
			if (width >= 256) != ip.Is4() {
				continue
			}
			bits := width
			if bits >= 256 {
				bits -= 256
			}
			if r, ok := group[netip.PrefixFrom(ip, bits).Masked()]; ok && active(r, now) {
				return r, true
			}
		}
	}
	return PunishmentRecord{}, false
}
func (p *Punisher) BanReason(port string, ip netip.Addr) string {
	v := p.state.Load()
	r, ok := lookupState(v.Bans, v.Widths, port, ip, p.Now().Unix())
	if ok {
		return r.Reason
	}
	return ""
}
func (p *Punisher) InHoneypot(port string, ip netip.Addr) bool {
	v := p.state.Load()
	_, ok := lookupState(v.Honeypots, v.Widths, port, ip, p.Now().Unix())
	return ok
}

// ProtectedRestriction performs one read/clock for ignored-state diagnostics.
// Empty state is the common household case and requires neither lookup nor clock.
func (p *Punisher) ProtectedRestriction(port string, ip netip.Addr) bool {
	v := p.state.Load()
	if v == nil || len(v.Bans) == 0 && len(v.Honeypots) == 0 {
		return false
	}
	now := p.Now().Unix()
	if record, ok := lookupState(v.Bans, v.Widths, port, ip, now); ok && record.Reason != "" {
		return true
	}
	_, ok := lookupState(v.Honeypots, v.Widths, port, ip, now)
	return ok
}
func (p *Punisher) Locked(port string) bool {
	r, ok := p.state.Load().Locks[port]
	return ok && active(r, p.Now().Unix())
}

func (p *Punisher) expire(now time.Time) []PunishmentChange {
	var changes []PunishmentChange
	for id, r := range p.records {
		if !active(r, now.Unix()) {
			if r.Kind == "lockdown" {
				p.port(r.PortID).cooldown = now.Add(time.Duration(max(0, r.CooldownUntil-now.Unix())) * time.Second)
				p.port(r.PortID).distinct = map[netip.Addr]time.Time{}
				changes = append(changes, PunishmentChange{r, "lockdown_released"})
				cooldown := r
				cooldown.Kind = "cooldown"
				cooldown.ID = recordKey("cooldown", "port", r.PortID, "")
				cooldown.ExpiresAt = r.CooldownUntil
				p.put(cooldown)
			}
			p.remove(id)
		}
	}
	return changes
}

// Drain consumes a bounded batch; the production worker is the sole caller.
// It is also the deterministic barrier used by end-to-end tests.
func (p *Punisher) Drain(limit int) int {
	var changes []PunishmentChange
	var events []HitEvent
	p.mu.Lock()
	before := p.revision
	now := p.Now()
	changes = append(changes, p.expire(now)...)
	for len(events) < limit {
		v, ok := p.Queue.Dequeue()
		if !ok {
			break
		}
		e := v.(HitEvent)
		events = append(events, e)
		if s := p.Lookup(e.PortID); s != nil {
			changes = append(changes, p.process(s, e, now)...)
		}
	}
	if p.revision != before {
		p.publish()
	}
	p.mu.Unlock()
	if p.OnEvent != nil {
		for _, e := range events {
			p.OnEvent(e)
		}
	}
	if p.OnChange != nil {
		for _, change := range changes {
			p.OnChange(change)
		}
	}
	return len(events)
}
func (p *Punisher) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			for p.Drain(256) > 0 {
			}
			return
		case <-ticker.C:
			p.Drain(256)
		}
	}
}
func (p *Punisher) RunPersistence(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.Flush()
		}
	}
}
func (p *Punisher) Flush() error {
	p.flushMu.Lock()
	defer p.flushMu.Unlock()
	p.mu.Lock()
	revision := p.revision
	if p.Persist == nil || revision == p.flushed {
		p.mu.Unlock()
		return nil
	}
	rows := make([]PunishmentRecord, 0, len(p.records))
	for _, r := range p.records {
		rows = append(rows, r)
	}
	p.mu.Unlock()
	if err := p.Persist(rows); err != nil {
		p.PersistErrors.Add(1)
		return err
	}
	p.mu.Lock()
	p.flushed = revision
	p.mu.Unlock()
	return nil
}
func (p *Punisher) Restore(rows []PunishmentRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(rows) > p.maxRecords {
		return errors.New("punishment state exceeds quota")
	}
	for _, r := range rows {
		if r.Kind != "ban" && r.Kind != "honeypot" && r.Kind != "history" && r.Kind != "lockdown" && r.Kind != "cooldown" {
			return errors.New("unknown punishment kind")
		}
		if r.Scope != "global" && r.Scope != "port" || !strings.HasPrefix(r.PortID, "port:") || r.ExpiresAt < 0 {
			return errors.New("invalid punishment scope")
		}
		if r.Kind != "lockdown" && r.Kind != "cooldown" {
			prefix, err := netip.ParsePrefix(r.Prefix)
			if err != nil || prefix.Bits() < 32 {
				return errors.New("invalid punishment prefix")
			}
		}
		if r.ID != recordKey(r.Kind, r.Scope, r.PortID, r.Prefix) {
			return errors.New("punishment identity mismatch")
		}
	}
	for _, r := range rows {
		if active(r, p.Now().Unix()) {
			p.put(r)
			if r.Kind == "cooldown" {
				p.port(r.PortID).cooldown = p.Now().Add(time.Duration(r.ExpiresAt-p.Now().Unix()) * time.Second)
			}
		} else if r.Kind == "lockdown" && r.CooldownUntil > p.Now().Unix() {
			p.port(r.PortID).cooldown = p.Now().Add(time.Duration(r.CooldownUntil-p.Now().Unix()) * time.Second)
		}
	}
	p.publish()
	return nil
}
func (p *Punisher) Records(port string) []PunishmentRecord {
	p.mu.Lock()
	defer p.mu.Unlock()
	rows := []PunishmentRecord{}
	for _, r := range p.records {
		if r.Kind != "history" && r.Kind != "cooldown" && active(r, p.Now().Unix()) && (port == "" || r.PortID == port || r.Scope == "global") {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return rows
}
func (p *Punisher) Manage(port, id, action string) error {
	p.mu.Lock()
	r, ok := p.records[id]
	if !ok || (r.PortID != port && r.Scope != "global") {
		p.mu.Unlock()
		return errors.New("state record not found")
	}
	event := "released"
	switch action {
	case "release":
		p.remove(id)
		if r.Kind == "lockdown" {
			seconds := 60
			if s := p.Lookup(port); s != nil {
				seconds = s.Config.Policy.HitPolicy.Lockdown.CooldownSeconds
			}
			p.port(port).cooldown = p.Now().Add(time.Duration(seconds) * time.Second)
			p.port(port).distinct = map[netip.Addr]time.Time{}
			cooldown := r
			cooldown.Kind = "cooldown"
			cooldown.ID = recordKey("cooldown", "port", r.PortID, "")
			cooldown.ExpiresAt = p.Now().Add(time.Duration(seconds) * time.Second).Unix()
			p.put(cooldown)
		} else {
			prefix, err := netip.ParsePrefix(r.Prefix)
			if err == nil {
				for key, entry := range p.counters {
					parts := strings.SplitN(key, "|", 3)
					if len(parts) != 3 {
						continue
					}
					ip, e := netip.ParsePrefix(parts[1])
					if e == nil && prefix.Contains(ip.Addr()) && (r.Scope == "global" || entry.port == port) {
						p.port(entry.port).order.Remove(entry.local)
						delete(p.counters, key)
					}
				}
			}
		}
	case "permanent":
		if r.Kind != "ban" {
			p.mu.Unlock()
			return errors.New("only bans can be permanent")
		}
		r.ExpiresAt = 0
		p.put(r)
		event = "made_permanent"
	default:
		p.mu.Unlock()
		return errors.New("unknown state operation")
	}
	p.publish()
	p.mu.Unlock()
	if p.OnChange != nil {
		p.OnChange(PunishmentChange{r, event})
	}
	return p.Flush()
}
func (p *Punisher) Stats() map[string]uint64 {
	out := p.Queue.Stats()
	out["processed"] = p.Processed.Load()
	out["ignored"] = p.Ignored.Load()
	out["quota_dropped"] = p.QuotaDropped.Load()
	out["persist_errors"] = p.PersistErrors.Load()
	p.mu.Lock()
	out["counter_keys"] = uint64(len(p.counters))
	out["state_records"] = uint64(len(p.records))
	p.mu.Unlock()
	return out
}
