package wafpipeline

import (
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafenginecore/cccounter"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type rateEntry struct {
	limiter *rate.Limiter
	seen    time.Time
	port    string
}
type RateStore struct {
	mu       sync.Mutex
	entries  map[string]*rateEntry
	ports    map[string]int
	Rejected atomic.Uint64
}

func NewRateStore() *RateStore {
	return &RateStore{entries: map[string]*rateEntry{}, ports: map[string]int{}}
}
func dimension(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:16])
}
func (s *RateStore) Allow(port, key string, perSecond float64, burst int, dry bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	k := port + ":" + key
	e := s.entries[k]
	if e == nil {
		if dry {
			return true
		}
		if len(s.entries) >= 200000 || s.ports[port] >= 1000 {
			s.Rejected.Add(1)
			return false
		}
		e = &rateEntry{limiter: rate.NewLimiter(rate.Limit(perSecond), burst), port: port}
		s.entries[k] = e
		s.ports[port]++
	}
	if dry {
		return e.limiter.TokensAt(now) >= 1
	}
	e.seen = now
	if e.limiter.Limit() != rate.Limit(perSecond) {
		e.limiter.SetLimitAt(now, rate.Limit(perSecond))
	}
	if e.limiter.Burst() != burst {
		e.limiter.SetBurstAt(now, burst)
	}
	return e.limiter.AllowN(now, 1)
}
func (s *RateStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for k, e := range s.entries {
		if e.seen.Before(cutoff) {
			delete(s.entries, k)
			s.ports[e.port]--
			if s.ports[e.port] == 0 {
				delete(s.ports, e.port)
			}
		}
	}
}

type RulesA struct {
	Rates    *RateStore
	Counters *cccounter.Counter
	Gate     *Gate
	// An optional, bounded asynchronous DNS cache; nil performs zero DNS traffic.
	BotIdentity func(port, ua, ip string, dry bool) (verified, spoofed bool)
}

func NewRulesA(gate *Gate) *RulesA {
	return &RulesA{Rates: NewRateStore(), Counters: cccounter.New(200000, 500), Gate: gate}
}
func (a *RulesA) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.Rates.Cleanup()
			a.Counters.Cleanup(600)
			if a.Gate != nil {
				a.Gate.Cleanup()
			}
		}
	}
}
func scopeMatches(scope Scope, r *http.Request) bool {
	if len(scope.Hosts) > 0 {
		host := strings.ToLower(r.Host)
		authority := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		found := false
		for _, pattern := range scope.Hosts {
			pattern = strings.ToLower(pattern)
			if authority == pattern || host == pattern || strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, pattern[1:]) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	p := NormalizePath(r.URL.EscapedPath())
	if len(scope.Paths) > 0 && !accessgate.MatchPathPrefix(p, scope.Paths) {
		return false
	}
	if accessgate.MatchPathPrefix(p, scope.ExcludePaths) {
		return false
	}
	if len(scope.Methods) > 0 {
		for _, m := range scope.Methods {
			if strings.EqualFold(m, r.Method) {
				return true
			}
		}
		return false
	}
	return true
}
func isStatic(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".css", ".js", ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".ico", ".woff", ".woff2":
		return true
	}
	return false
}
func ruleHit(c *RequestContext, mode Mode, category string, severity Severity, action Action, status int, reason string) Verdict {
	observe := mode == ModeObserve
	c.Hit(category, category, severity, observe, reason)
	return Verdict{Action: action, Reason: reason, RuleID: category, Observe: observe, Status: status}
}
func (a *RulesA) count(c *RequestContext, suffix, value string, window int) cccounter.Result {
	key := c.PortID + ":" + suffix
	if c.DryRun {
		v := a.Counters.Peek(key, dimension(value), window)
		v.Count++
		return v
	}
	return a.Counters.Incr(key, dimension(value), window)
}
func (a *RulesA) Steps(s *Snapshot) []Step {
	rules := s.Config.Policy.RuleSet
	paths := make([]string, 0, len(rules.QPS.Paths))
	for p := range rules.QPS.Paths {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) == len(paths[j]) {
			return paths[i] < paths[j]
		}
		return len(paths[i]) > len(paths[j])
	})
	steps := []Step{
		{Number: 3, Name: "qps", Inspect: func(c *RequestContext) (Verdict, error) {
			q := rules.QPS
			if q.Mode == ModeOff {
				return Verdict{Action: Continue}, nil
			}
			mult := 1.0
			lane := "dynamic"
			if isStatic(c.View.Path) {
				mult = q.StaticMultiplier
				lane = "static"
			}
			ok := a.Rates.Allow(c.PortID, "port", q.PerPort, q.Burst, c.DryRun)
			ipOK := a.Rates.Allow(c.PortID, "ip:"+c.ClientIP.String()+":"+lane, q.PerIP*mult, int(float64(q.Burst)*mult), c.DryRun)
			ok = ok && ipOK
			for _, p := range paths {
				if accessgate.MatchPathPrefix(c.View.Path, []string{p}) {
					allowed := a.Rates.Allow(c.PortID, "path:"+dimension(p)+":"+c.ClientIP.String(), q.Paths[p], q.Burst, c.DryRun)
					ok = ok && allowed
					break
				}
			}
			if !ok {
				return ruleHit(c, q.Mode, "qps", Medium, Deny, 429, "rate_limit"), nil
			}
			return Verdict{Action: Continue}, nil
		}},
		{Number: 3, Name: "cc", Inspect: func(c *RequestContext) (Verdict, error) {
			q := rules.CC
			if q.Mode == ModeOff || q.ExcludeStatic && isStatic(c.View.Path) || a.challengeExempt(s, c) {
				return Verdict{Action: Continue}, nil
			}
			key := c.ClientIP.String()
			if q.ByPath {
				key += "|" + c.View.Path
			}
			n := a.count(c, fmt.Sprintf("cc:%d", q.WindowSeconds), key, q.WindowSeconds)
			if n.Degraded {
				n = a.count(c, "cc_fallback", c.ClientIP.String(), q.WindowSeconds)
			}
			if n.Rejected || n.Count > int64(q.Threshold) {
				return ruleHit(c, q.Mode, "cc", Medium, q.Action, 429, "cc_frequency"), nil
			}
			return Verdict{Action: Continue}, nil
		}},
		{Number: 3, Name: "bot", Inspect: func(c *RequestContext) (Verdict, error) {
			if rules.BOT == ModeOff || a.challengeExempt(s, c) {
				return Verdict{Action: Continue}, nil
			}
			ua := strings.ToLower(c.Request.UserAgent())
			verified, spoofed := false, false
			if rules.BOTVerifyDNS && a.BotIdentity != nil {
				verified, spoofed = a.BotIdentity(c.PortID, ua, c.ClientIP.String(), c.DryRun)
			}
			if verified {
				c.Values["bot_verified"] = true
				return Verdict{Action: Continue, Reason: "bot:verified"}, nil
			}
			if spoofed || strings.Contains(ua, "headlesschrome") || strings.Contains(ua, "phantomjs") {
				return ruleHit(c, rules.BOT, "bot", Medium, Challenge, 429, "bot:challenge"), nil
			}
			return Verdict{Action: Continue}, nil
		}},
	}
	if a.Gate != nil {
		if challenge := rules.Challenge; challenge.Mode != "" && challenge.Mode != ModeOff {
			steps = append(steps, Step{Number: 3, Name: "access_challenge", Inspect: func(c *RequestContext) (Verdict, error) {
				if challenge.Until > 0 && challenge.Until <= time.Now().Unix() || !scopeMatches(challenge.Scope, c.Request) || a.challengeExempt(s, c) {
					return Verdict{Action: Continue}, nil
				}
				return ruleHit(c, challenge.Mode, "access_challenge", Low, Challenge, 429, "access_challenge"), nil
			}})
		}
		steps = append(steps, a.Gate.Step(s))
	}
	return steps
}
func (a *RulesA) challengeExempt(s *Snapshot, c *RequestContext) bool {
	return c.View.Path == strings.TrimRight(s.Config.Policy.Gate.Prefix, "/")+"/challenge" || a.Gate != nil && a.Gate.ChallengePassed(s, c)
}
