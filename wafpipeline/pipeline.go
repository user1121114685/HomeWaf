// Package wafpipeline owns the port-scoped request policy. It deliberately has
// no dependency on database, management HTTP, or proxy implementation packages.
package wafpipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

type Action string

const (
	Continue  Action = "continue"
	Allow     Action = "allow"
	Deny      Action = "deny"
	Drop      Action = "drop"
	Challenge Action = "challenge"
	Honeypot  Action = "honeypot"
	Tarpit    Action = "tarpit"
	Bypass    Action = "bypass"
)

type Severity string

const (
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

func (s Severity) Rank() int {
	switch s {
	case Critical:
		return 4
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	}
	return 0
}

type Verdict struct {
	Action  Action `json:"action"`
	Stage   int    `json:"stage"`
	Reason  string `json:"reason"`
	RuleID  string `json:"rule_id,omitempty"`
	Observe bool   `json:"observe,omitempty"`
	Status  int    `json:"status,omitempty"`
}
type HitEvent struct {
	Time       time.Time  `json:"time"`
	PortID     string     `json:"port_id"`
	ClientIP   netip.Addr `json:"client_ip"`
	Category   string     `json:"category"`
	RuleID     string     `json:"rule_id,omitempty"`
	Severity   Severity   `json:"severity"`
	Observe    bool       `json:"observe"`
	Trusted    bool       `json:"trusted"`
	Evidence   string     `json:"evidence,omitempty"`
	ReqID      string     `json:"req_id"`
	Path       string     `json:"path,omitempty"`
	Status     int        `json:"status,omitempty"`
	BanSeconds int        `json:"ban_seconds,omitempty"`
}
type TraceEntry struct {
	Stage         int     `json:"stage"`
	Check         string  `json:"check"`
	Verdict       Verdict `json:"verdict"`
	ElapsedMicros int64   `json:"elapsed_us"`
	Error         string  `json:"error,omitempty"`
}

// Recorder is bounded even if many detectors match. It can also be attached to
// a request context by adapters for the existing proxy and logging paths.
type Recorder struct {
	mu      sync.Mutex
	entries []TraceEntry
	dropped int
}

func (r *Recorder) Add(e TraceEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) < 128 {
		r.entries = append(r.entries, e)
	} else {
		r.dropped++
	}
}
func (r *Recorder) Entries() []TraceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]TraceEntry(nil), r.entries...)
}

func (r *Recorder) reserve(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n = min(n, 128)
	if cap(r.entries) < n {
		entries := make([]TraceEntry, len(r.entries), n)
		copy(entries, r.entries)
		r.entries = entries
	}
}

type recorderKey struct{}

func WithRecorder(ctx context.Context, r *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, r)
}
func RequestRecorder(r *http.Request) *Recorder {
	v, _ := r.Context().Value(recorderKey{}).(*Recorder)
	return v
}

type RequestContext struct {
	Started  time.Time
	Context  context.Context
	Request  *http.Request
	PortID   string
	ClientIP netip.Addr
	ReqID    string
	Trusted  bool
	DryRun   bool
	View     *NormalizedView
	Trace    *Recorder
	Hits     []HitEvent
	Values   map[string]any
}

func NewRequestContext(r *http.Request, portID string, ip netip.Addr, limits ViewLimits) *RequestContext {
	var id [16]byte
	_, _ = rand.Read(id[:])
	trace := &Recorder{}
	r = r.WithContext(WithRecorder(r.Context(), trace))
	return &RequestContext{Started: time.Now(), Context: r.Context(), Request: r, PortID: portID, ClientIP: ip.Unmap(), ReqID: hex.EncodeToString(id[:]), View: NewNormalizedView(r, limits), Trace: trace, Values: make(map[string]any)}
}
func (c *RequestContext) Hit(category, rule string, severity Severity, observe bool, evidence string) {
	if len(c.Hits) >= 64 {
		return
	}
	// Evidence is a detector-supplied description, never a copy of credentials or body.
	if len(evidence) > 256 {
		evidence = evidence[:256]
	}
	c.Hits = append(c.Hits, HitEvent{Time: time.Now(), PortID: c.PortID, ClientIP: c.ClientIP, Category: category, RuleID: rule, Severity: severity, Observe: observe, Trusted: c.Trusted, Evidence: evidence, ReqID: c.ReqID, Path: c.Request.URL.EscapedPath()})
}

type Step struct {
	Number     int
	Name       string
	Inspect    func(*RequestContext) (Verdict, error)
	FailClosed bool // Access control failures are not detector fail-open decisions.
	RunTrusted bool // Reserved endpoints and an explicitly required gate still run.
}
type EventSink interface{ Submit(HitEvent) bool }
type Result struct {
	Verdict       Verdict      `json:"verdict"`
	Trace         []TraceEntry `json:"trace"`
	Hits          []HitEvent   `json:"hits,omitempty"`
	ReqID         string       `json:"req_id"`
	BodyTruncated bool         `json:"body_truncated,omitempty"`
}
type Pipeline struct {
	steps []Step
	sink  EventSink
}

func New(steps []Step, sink EventSink) (*Pipeline, error) {
	previous := 0
	for _, s := range steps {
		if s.Number < 1 || s.Number > 5 || s.Number == 4 || s.Number < previous || s.Name == "" || s.Inspect == nil {
			return nil, fmt.Errorf("invalid pipeline step %q", s.Name)
		}
		previous = s.Number
	}
	return &Pipeline{steps: append([]Step(nil), steps...), sink: sink}, nil
}
func safeInspect(s Step, c *RequestContext) (v Verdict, err error) {
	defer func() {
		if recover() != nil {
			v = Verdict{Action: Continue, Stage: s.Number, Reason: "detector_error"}
			err = fmt.Errorf("detector panic")
		}
	}()
	return s.Inspect(c)
}
func (p *Pipeline) Evaluate(c *RequestContext) Result {
	return p.evaluate(c, true)
}

// Runtime captures trace/hits once after response observation; standalone
// evaluations still receive immutable copies immediately.
func (p *Pipeline) evaluate(c *RequestContext, capture bool) Result {
	final := Verdict{Action: Allow, Stage: 5, Reason: "passed"}
	reserved := false
	for _, s := range p.steps {
		if s.Number == 3 && !reserved {
			c.Trace.reserve(len(p.steps) + 1)
			reserved = true
		}
		if c.Trusted && s.Number == 3 && !s.RunTrusted {
			c.Trace.Add(TraceEntry{Stage: 3, Check: s.Name, Verdict: Verdict{Action: Bypass, Stage: 3, Reason: "whitelist:trusted"}})
			continue
		}
		started := time.Now()
		hitStart := len(c.Hits)
		v, err := safeInspect(s, c)
		v.Stage = s.Number
		if v.Action == "" {
			v.Action = Continue
		}
		entry := TraceEntry{Stage: s.Number, Check: s.Name, Verdict: v, ElapsedMicros: time.Since(started).Microseconds()}
		if err != nil {
			for i := hitStart; i < len(c.Hits); i++ {
				c.Hits[i].Observe = true
			}
			entry.Error = "detector unavailable"
			if err.Error() == "detector panic" {
				entry.Error = "detector panic"
			}
			c.Hit("detector_error", s.Name, Low, true, "inspection unavailable; see stage and rule for configuration review")
			v = Verdict{Action: Continue, Stage: s.Number, Reason: "detector_error"}
			if s.Number < 3 || s.FailClosed {
				v = Verdict{Action: Deny, Stage: s.Number, Reason: "policy_unavailable", Status: http.StatusServiceUnavailable}
			}
			entry.Verdict = v
		}
		c.Trace.Add(entry)
		if v.Observe {
			continue
		}
		if v.Action == Bypass {
			c.Trusted = true
			continue
		}
		if v.Action != Continue {
			final = v
			break
		}
	}
	if p.sink != nil && !c.DryRun {
		for _, event := range c.Hits {
			p.sink.Submit(event)
		}
	}
	result := Result{Verdict: final, ReqID: c.ReqID, BodyTruncated: c.View.BodyTruncated}
	if capture {
		result.Trace = c.Trace.Entries()
		result.Hits = append([]HitEvent(nil), c.Hits...)
	}
	return result
}

// LocalPort uses the accepted connection, never client-controlled Host/XFF.
func LocalPort(r *http.Request) (uint16, error) {
	a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || a == nil {
		return 0, fmt.Errorf("listener identity unavailable")
	}
	_, p, err := net.SplitHostPort(a.String())
	if err != nil {
		return 0, fmt.Errorf("invalid listener address")
	}
	n, err := strconv.ParseUint(p, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid listener port")
	}
	return uint16(n), nil
}
func PortID(port uint16) string { return "port:" + strconv.Itoa(int(port)) }
