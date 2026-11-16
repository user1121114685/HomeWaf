package wafpipeline

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefenseQuiet            = "quiet"
	DefenseRestrict         = "restrict"
	DefenseClose            = "close"
	DefenseHoney            = "honeypot"
	SuccessHistorySeconds   = 30 * 86400
	MaxSuccessPrefixes      = 4096
	MaxTotalSuccessPrefixes = 32768
)

// Nil is accepted when loading older configurations. New service saves require an enabled policy.
type DefenseConfig struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	WindowSeconds   int    `json:"window_seconds"`
	Count           int    `json:"count"`
	DurationSeconds int    `json:"duration_seconds"`
	TCPAction       string `json:"tcp_action"`
}

func DefaultDefense() DefenseConfig {
	return DefenseConfig{Mode: DefenseQuiet, WindowSeconds: 1800, Count: 10, DurationSeconds: 10800, TCPAction: "reset"}
}

func ValidateDefense(c *DefenseConfig) error {
	if c == nil {
		return nil
	}
	if c.Mode != DefenseQuiet && c.Mode != DefenseRestrict && c.Mode != DefenseClose && c.Mode != DefenseHoney {
		return errors.New("请选择一种异常访问处置模式")
	}
	if c.WindowSeconds < 1 || c.WindowSeconds > 86400 || c.Count < 1 || c.Count > 100000 || c.DurationSeconds < 1 || c.DurationSeconds > 7*86400 {
		return errors.New("统计窗口须为1..86400秒，阈值1..100000次，持续时间1秒..7天")
	}
	if c.TCPAction != "reset" && c.TCPAction != "drop" {
		return errors.New("TCP处置须为reset或drop")
	}
	return nil
}

// Stored with the owning port configuration, never trusted from a client save.
type DefenseState struct {
	ID               string   `json:"id"`
	Mode             string   `json:"mode"`
	Reason           string   `json:"reason"`
	StartedAt        int64    `json:"started_at"`
	ExpiresAt        int64    `json:"expires_at"`
	Prefixes         []string `json:"prefixes,omitempty"`
	AccessWasEnabled bool     `json:"access_was_enabled"`
}

func ActiveDefense(c PortConfig, now int64) *DefenseState {
	if c.Policy.Defense == nil || !c.Policy.Defense.Enabled || c.DefenseState == nil {
		return nil
	}
	if c.DefenseState.ExpiresAt <= now && c.DefenseState.Mode != DefenseClose {
		return nil
	}
	return c.DefenseState
}

func ValidateDefenseState(s *DefenseState) error {
	if s == nil {
		return nil
	}
	if s.ID == "" || len(s.ID) > 128 || s.StartedAt <= 0 || s.ExpiresAt <= s.StartedAt || s.ExpiresAt-s.StartedAt > 7*86400 || len(s.Prefixes) > MaxSuccessPrefixes {
		return errors.New("invalid persisted defense state")
	}
	if s.Mode != DefenseQuiet && s.Mode != DefenseRestrict && s.Mode != DefenseClose {
		return errors.New("invalid persisted defense mode")
	}
	if s.Mode == DefenseRestrict && len(s.Prefixes) == 0 {
		return errors.New("empty persisted restricted scope")
	}
	previous := ""
	for _, prefix := range s.Prefixes {
		p, err := netip.ParsePrefix(prefix)
		if err != nil || SuccessfulPrefix(p.Addr()) != prefix || prefix <= previous {
			return errors.New("invalid persisted success prefix")
		}
		previous = prefix
	}
	return nil
}

type SuccessPrefix struct {
	Port     uint16 `json:"port"`
	Prefix   string `json:"prefix"`
	LastSeen int64  `json:"last_seen"`
}

type defenseCounter struct {
	Revision int64
	Config   DefenseConfig
	Failures []int64
	Pending  *DefenseState
	Error    string
}

type DefenseStatus struct {
	Mode            string        `json:"mode"`
	Enabled         bool          `json:"enabled"`
	Legacy          bool          `json:"legacy"`
	HoneypotReady   bool          `json:"honeypot_ready"`
	Revision        int64         `json:"revision"`
	AccessEnabled   bool          `json:"access_enabled"`
	Failures        int           `json:"failures"`
	SuccessPrefixes int           `json:"success_prefixes"`
	HistoryReady    bool          `json:"history_ready"`
	Error           string        `json:"error,omitempty"`
	State           *DefenseState `json:"state,omitempty"`
}

// Request work is bounded memory-only. State commits and history writes run on
// the lifecycle worker; an audit/log queue is never the source of this data.
type DefenseEngine struct {
	RequireRegistered bool
	mu                sync.Mutex
	Now               func() time.Time
	counters          map[uint16]*defenseCounter
	success           map[uint16]map[string]int64
	dirty             map[string]SuccessPrefix
	total             int
	historyReady      bool
	historyLoaded     bool
	overflow          map[uint16]int64
	historyError      string
}

func NewDefenseEngine() *DefenseEngine {
	return &DefenseEngine{Now: time.Now, counters: map[uint16]*defenseCounter{}, success: map[uint16]map[string]int64{}, dirty: map[string]SuccessPrefix{}, overflow: map[uint16]int64{}}
}

func SuccessfulPrefix(ip netip.Addr) string {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return ""
	}
	bits := 64
	if ip.Is4() {
		bits = 16
	}
	return netip.PrefixFrom(ip, bits).Masked().String()
}

func (e *DefenseEngine) LoadHistory(rows []SuccessPrefix, err error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil {
		e.historyReady = false
		e.historyError = "成功来源记录不可用，自动收紧暂不触发"
		return err
	}
	if len(rows) > MaxTotalSuccessPrefixes+Ports.Limit() {
		return e.historyFailure("成功来源记录超过容量限制")
	}
	next := map[uint16]map[string]int64{}
	overflow := map[uint16]int64{}
	for _, row := range e.dirty {
		rows = append(rows, row)
	}
	now := e.Now().Unix()
	total := 0
	for _, row := range rows {
		if row.Prefix == "!overflow" && row.Port != 0 {
			overflow[row.Port] = max(overflow[row.Port], row.LastSeen+SuccessHistorySeconds)
			continue
		}
		p, parseErr := netip.ParsePrefix(row.Prefix)
		if parseErr != nil || row.Port == 0 || SuccessfulPrefix(p.Addr()) != row.Prefix || row.LastSeen > now+300 {
			return e.historyFailure("成功来源记录损坏，自动收紧暂不触发")
		}
		if row.LastSeen <= now-SuccessHistorySeconds {
			continue
		}
		if next[row.Port] == nil {
			next[row.Port] = map[string]int64{}
		}
		if _, exists := next[row.Port][row.Prefix]; !exists {
			total++
		}
		next[row.Port][row.Prefix] = max(next[row.Port][row.Prefix], row.LastSeen)
		if len(next[row.Port]) > MaxSuccessPrefixes {
			return e.historyFailure("端口成功来源记录超过容量限制")
		}
	}
	if total > MaxTotalSuccessPrefixes {
		return e.historyFailure("成功来源记录超过容量限制")
	}
	e.success, e.total = next, total
	e.overflow, e.historyLoaded = overflow, true
	e.historyReady, e.historyError = true, ""
	return nil
}

func (e *DefenseEngine) historyFailure(message string) error {
	e.historyReady, e.historyError = false, message
	return errors.New(message)
}

func (e *DefenseEngine) NeedsHistoryLoad() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.historyLoaded
}

func (e *DefenseEngine) counter(s *Snapshot) *defenseCounter {
	port := s.Config.Port
	c := e.counters[port]
	if c == nil {
		c = &defenseCounter{}
		e.counters[port] = c
	}
	if d := s.Config.Policy.Defense; d != nil && c.Config != *d {
		c.Config = *d
		c.Failures = nil
		c.Pending = nil
		c.Error = ""
	}
	c.Revision = s.Config.Revision
	return c
}

// Request failures may carry several detector events. This is called exactly
// once for the final request outcome, rather than once for each HitEvent.
func DefenseFailure(c *RequestContext, v Verdict) bool {
	if c == nil || c.DryRun || v.Observe || strings.HasPrefix(v.Reason, "defense:") || v.Reason == "access_disabled" || v.Reason == "lockdown" || v.Reason == "policy_unavailable" {
		return false
	}
	for _, hit := range c.Hits {
		if !hit.Observe && hit.Category == "auth_fail" {
			return true
		}
	}
	if v.Status >= 500 || v.Action != Deny && v.Action != Drop && v.Action != Honeypot && v.Action != Tarpit {
		return false
	}
	if strings.HasPrefix(v.Reason, "blacklist:") || v.Reason == "ip:unlisted" || v.Reason == "host_not_allowed" || v.Reason == "request_body_limit" || v.Reason == "response_content_policy" {
		return true
	}
	for _, hit := range c.Hits {
		if !hit.Observe && hit.Category != "detector_error" && !strings.HasPrefix(hit.Category, "response_") && !strings.HasPrefix(hit.Category, "honeypot_") {
			return true
		}
	}
	return false
}

func SuccessfulBusiness(c *RequestContext, v Verdict) bool {
	if c == nil || c.DryRun || (v.Action != Allow && v.Action != Continue && v.Action != Bypass) || (v.Status != 101 && (v.Status < 200 || v.Status >= 400)) {
		return false
	}
	if v.Reason != "upstream_response" && v.Reason != "business_cache_hit" {
		return false
	}
	for _, h := range c.Hits {
		if h.Category != "auth_success" && h.Category != "business_auth_success" {
			return false
		}
	}
	return true
}

func (e *DefenseEngine) Observe(s *Snapshot, c *RequestContext, v Verdict) {
	if s == nil || s.Preview || c == nil || c.DryRun {
		return
	}
	if current, ok := Ports.Get(s.Config.Port); ok && current.Config.Revision > s.Config.Revision {
		return
	}
	if done, _ := c.Values["defense_observed"].(bool); done {
		return
	}
	c.Values["defense_observed"] = true
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.RequireRegistered {
		current, ok := Ports.Get(s.Config.Port)
		if !ok || current.Config.Revision != s.Config.Revision || current.Secrets.GateKey != s.Secrets.GateKey {
			return
		}
	}
	now := e.Now().Unix()
	state := e.counter(s)
	if SuccessfulBusiness(c, v) {
		prefix := SuccessfulPrefix(c.ClientIP)
		if prefix != "" {
			rows := e.success[s.Config.Port]
			if rows == nil {
				rows = map[string]int64{}
				e.success[s.Config.Port] = rows
			}
			if _, exists := rows[prefix]; !exists {
				if len(rows) >= MaxSuccessPrefixes || e.total >= MaxTotalSuccessPrefixes {
					e.overflow[s.Config.Port] = now + SuccessHistorySeconds
					e.dirty[fmt.Sprintf("%d/!overflow", s.Config.Port)] = SuccessPrefix{s.Config.Port, "!overflow", now}
					return
				}
				e.total++
			}
			rows[prefix] = now
			e.dirty[fmt.Sprintf("%d/%s", s.Config.Port, prefix)] = SuccessPrefix{s.Config.Port, prefix, now}
		}
	}
	d := s.Config.Policy.Defense
	if d == nil || !d.Enabled || d.Mode == DefenseHoney || !s.Config.Policy.Access.Enabled || s.Config.DefenseState != nil || state.Pending != nil || !DefenseFailure(c, v) {
		return
	}
	state.Failures = recentFailures(state.Failures, now-int64(d.WindowSeconds))
	if len(state.Failures) < d.Count {
		state.Failures = append(state.Failures, now)
	}
	if len(state.Failures) < d.Count {
		return
	}
	mode, reason := d.Mode, "failure_threshold"
	var prefixes []string
	if mode == DefenseRestrict {
		if !e.historyReady || e.overflow[s.Config.Port] > now {
			return
		}
		for prefix, seen := range e.success[s.Config.Port] {
			if seen > now-SuccessHistorySeconds {
				prefixes = append(prefixes, prefix)
			}
		}
		sort.Strings(prefixes)
		if len(prefixes) == 0 {
			mode, reason = DefenseClose, "no_success_history"
		}
	}
	state.Pending = &DefenseState{ID: randomToken(), Mode: mode, Reason: reason, StartedAt: now, ExpiresAt: now + int64(d.DurationSeconds), Prefixes: prefixes, AccessWasEnabled: s.Config.Policy.Access.Enabled}
}

func recentFailures(items []int64, cutoff int64) []int64 {
	i := 0
	for i < len(items) && items[i] <= cutoff {
		i++
	}
	if i > 0 {
		copy(items, items[i:])
		items = items[:len(items)-i]
	}
	return items
}

type DefenseTransition struct {
	Port     uint16
	Revision int64
	Config   DefenseConfig
	State    DefenseState
}

func (e *DefenseEngine) Pending() []DefenseTransition {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []DefenseTransition
	for port, c := range e.counters {
		if c.Pending != nil {
			out = append(out, DefenseTransition{port, c.Revision, c.Config, *c.Pending})
		}
	}
	return out
}

func (e *DefenseEngine) Reset(port uint16) { e.mu.Lock(); delete(e.counters, port); e.mu.Unlock() }

func (e *DefenseEngine) Forget(port uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.counters, port)
	delete(e.overflow, port)
	e.total -= len(e.success[port])
	delete(e.success, port)
	for key, row := range e.dirty {
		if row.Port == port {
			delete(e.dirty, key)
		}
	}
}

func (e *DefenseEngine) TransitionFailed(port uint16, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.counters[port] == nil {
		e.counters[port] = &defenseCounter{}
	}
	if c := e.counters[port]; c != nil {
		c.Error = err.Error()
		c.Pending = nil
	}
}

func (e *DefenseEngine) Status(s *Snapshot) DefenseStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.Now().Unix()
	out := DefenseStatus{Revision: s.Config.Revision, AccessEnabled: s.Config.Policy.Access.Enabled, HistoryReady: e.historyReady, Error: e.historyError, State: s.Config.DefenseState}
	out.Legacy, out.HoneypotReady = s.Config.Policy.Defense == nil, SharedHoneypotReady()
	if d := s.Config.Policy.Defense; d != nil {
		out.Mode, out.Enabled = d.Mode, d.Enabled
	}
	if e.overflow[s.Config.Port] > now {
		out.HistoryReady = false
		out.Error = "成功来源容量已满，自动收紧暂不触发"
	}
	if c := e.counters[s.Config.Port]; c != nil {
		c.Failures = recentFailures(c.Failures, now-int64(c.Config.WindowSeconds))
		out.Failures = len(c.Failures)
		if c.Error != "" {
			out.Error = c.Error
		}
	}
	for _, seen := range e.success[s.Config.Port] {
		if seen > now-SuccessHistorySeconds {
			out.SuccessPrefixes++
		}
	}
	return out
}

func (e *DefenseEngine) DirtyHistory() []SuccessPrefix {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]SuccessPrefix, 0, len(e.dirty))
	for _, row := range e.dirty {
		out = append(out, row)
	}
	return out
}

func (e *DefenseEngine) HistorySaved(rows []SuccessPrefix, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.historyLoaded {
		return
	}
	if err != nil {
		e.historyFailure("成功来源保存失败，自动收紧暂不触发")
		return
	}
	for _, row := range rows {
		key := fmt.Sprintf("%d/%s", row.Port, row.Prefix)
		if current, ok := e.dirty[key]; ok && current.LastSeen == row.LastSeen {
			delete(e.dirty, key)
		}
	}
	cutoff := e.Now().Unix() - SuccessHistorySeconds
	for port, entries := range e.success {
		for prefix, seen := range entries {
			if seen <= cutoff {
				delete(entries, prefix)
				e.total--
			}
		}
		if len(entries) == 0 {
			delete(e.success, port)
		}
	}
	e.historyReady, e.historyError = true, ""
}

func (e *DefenseEngine) Step(s *Snapshot) Step {
	return Step{Number: 2, Name: "defense_scope", FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
		state := ActiveDefense(s.Config, e.Now().Unix())
		if state == nil || state.Mode != DefenseRestrict {
			return Verdict{Action: Continue}, nil
		}
		prefix := SuccessfulPrefix(c.ClientIP)
		idx := sort.SearchStrings(state.Prefixes, prefix)
		if idx < len(state.Prefixes) && state.Prefixes[idx] == prefix {
			return Verdict{Action: Continue, Reason: "defense:known_network"}, nil
		}
		return Verdict{Action: Deny, Status: 403, Reason: "defense:unknown_network"}, nil
	}}
}
