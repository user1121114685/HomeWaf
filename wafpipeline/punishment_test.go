package wafpipeline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func attachPunisher(t *testing.T, f *proxyFixture, p *Punisher, checks *atomic.Int64) {
	t.Helper()
	original := f.runtime.RuleSteps
	honey := NewHoneyHandler()
	oldResponder := f.runtime.Respond
	f.runtime.State = p
	f.runtime.Sink = p
	f.runtime.Respond = func(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
		return honey.Respond(s, c, w, v) || oldResponder(s, c, w, v)
	}
	f.runtime.RuleSteps = func(s *Snapshot) []Step {
		return append(original(s), Step{Number: 3, Name: "test_critical", Inspect: func(c *RequestContext) (Verdict, error) {
			checks.Add(1)
			if c.View.Path == "/critical" {
				c.Hit("rce", "test_critical", Critical, false, "fixture")
				return Verdict{Action: Deny, Status: 403, Reason: "fixture"}, nil
			}
			return Verdict{Action: Continue}, nil
		}})
	}
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
	})
}
func penaltyRequest(t *testing.T, f *proxyFixture, ip, path string) int {
	code, _, _ := proxyCall(t, f, "GET", path, "", map[string]string{"X-Forwarded-For": ip})
	return code
}
func penaltyEvent(p *Punisher, port string, ip string, severity Severity, category, req string) {
	p.Submit(HitEvent{Time: p.Now(), PortID: port, ClientIP: netip.MustParseAddr(ip), Severity: severity, Category: category, ReqID: req})
	p.Drain(256)
}

func TestPunishmentCriticalExpiryRepeatPortScopeAndTrusted(t *testing.T) {
	p := NewPunisher()
	now := time.Now()
	p.Now = func() time.Time { return now }
	p.start = now
	a := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	b := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	var checksA, checksB atomic.Int64
	attachPunisher(t, a, p, &checksA)
	attachPunisher(t, b, p, &checksB)
	ip := "198.51.100.40"
	port := PortID(a.current.Load().Config.Port)
	if penaltyRequest(t, a, ip, "/critical") != 403 || a.requests.Load() != 0 {
		t.Fatal("initial attack forwarded")
	}
	p.Drain(256)
	rows := p.Records(port)
	if len(rows) != 1 || rows[0].ExpiresAt-now.Unix() != 3600 {
		t.Fatalf("critical duration wrong: %+v", rows)
	}
	checks := checksA.Load()
	if penaltyRequest(t, a, ip, "/ok") != 403 || checksA.Load() != checks || a.requests.Load() != 0 {
		t.Fatal("ban did not short-circuit detection")
	}
	if penaltyRequest(t, b, ip, "/ok") != 204 || b.requests.Load() != 1 {
		t.Fatal("automatic ban crossed port scope")
	}
	a.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "trusted", Type: "allow", Trust: "trusted", Value: ip}}
	})
	if penaltyRequest(t, a, ip, "/ok") != 204 || a.requests.Load() != 1 {
		t.Fatal("shared ban overrides trusted")
	}
	a.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = append(s.Config.Policy.IPGroup.Entries, IPEntry{ID: "deny", Type: "deny", Value: ip})
	})
	if penaltyRequest(t, a, ip, "/ok") != 403 || a.requests.Load() != 1 {
		t.Fatal("custom deny did not override trusted")
	}
	now = now.Add(time.Hour + time.Second)
	p.Drain(0)
	a.update(t, func(s *Snapshot) { s.Config.Policy.IPGroup.Entries = nil })
	if penaltyRequest(t, a, ip, "/ok") != 204 || a.requests.Load() != 2 {
		t.Fatal("expired ban did not release")
	}
	penaltyEvent(p, port, ip, Critical, "rce", "again")
	rows = p.Records(port)
	if len(rows) != 1 || rows[0].ExpiresAt-now.Unix() != 4*3600 || rows[0].Strikes != 2 {
		t.Fatalf("repeat multiplier lost: %+v", rows)
	}
	for i, want := range []int64{16 * 3600, 64 * 3600, 7 * 86400, 7 * 86400} {
		now = time.Unix(rows[0].ExpiresAt+1, 0)
		p.Drain(0)
		penaltyEvent(p, port, ip, Critical, "rce", fmt.Sprint("repeat", i))
		rows = p.Records(port)
		if len(rows) != 1 || rows[0].ExpiresAt-now.Unix() != want {
			t.Fatalf("repeat cap incorrect: %+v want=%d", rows, want)
		}
	}
	for _, protected := range []string{"127.0.0.1", "10.0.0.2", "169.254.1.2"} {
		penaltyEvent(p, port, protected, Critical, "rce", protected)
		if p.BanReason(port, netip.MustParseAddr(protected)) != "" {
			t.Fatal("protected address banned")
		}
	}
	p.Submit(HitEvent{PortID: port, ClientIP: netip.MustParseAddr("198.51.100.41"), Severity: Critical, Observe: true, Category: "rce"})
	p.Drain(1)
	if p.BanReason(port, netip.MustParseAddr("198.51.100.41")) != "" {
		t.Fatal("observe punished")
	}
}

func TestMillionSourceCountersStayBoundedAndPreserveOtherPort(t *testing.T) {
	p := NewPunisher()
	policy := DefaultPolicy()
	policy.HitPolicy.High.Count = 100
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: policy}}
	other := &Snapshot{Config: PortConfig{Port: 5001, Policy: policy}}
	p.Lookup = func(id string) *Snapshot {
		if id == "port:5001" {
			return other
		}
		return s
	}
	penaltyEvent(p, "port:5001", "198.51.100.9", High, "sqli", "b")
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 1000000; i++ {
		ip := netip.AddrFrom4([4]byte{11, byte(i >> 16), byte(i >> 8), byte(i)})
		p.Submit(HitEvent{PortID: "port:5000", ClientIP: ip, Severity: High, Category: "sqli"})
		if i%128 == 0 {
			p.Drain(256)
		}
	}
	for p.Drain(256) > 0 {
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	stats := p.Stats()
	if stats["counter_keys"] > 1001 || stats["state_records"] != 0 {
		t.Fatalf("unbounded state: %v", stats)
	}
	if after.HeapAlloc > before.HeapAlloc+96<<20 {
		t.Fatalf("excess retained heap: %d", after.HeapAlloc-before.HeapAlloc)
	}
	if p.counters["port:5001|198.51.100.9/32|severity:high"] == nil {
		t.Fatal("one port evicted another port's counters")
	}
	t.Logf("million sources: retained heap before=%d after=%d stats=%v", before.HeapAlloc, after.HeapAlloc, stats)
}

func TestIPv6AggregationPortScopeAndObservationSignals(t *testing.T) {
	p := NewPunisher()
	policy := DefaultPolicy()
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: policy}}
	p.Lookup = func(id string) *Snapshot { return s }
	penaltyEvent(p, "port:5000", "2001:db8:1:2::a", Critical, "rce", "a")
	if p.BanReason("port:5000", netip.MustParseAddr("2001:db8:1:2::b")) == "" || p.BanReason("port:5001", netip.MustParseAddr("2001:db8:1:2::b")) != "" {
		t.Fatal("IPv6 prefix or port scope incorrect")
	}
}

func TestEventFloodKeepsOtherPortServing(t *testing.T) {
	p := NewPunisher()
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	var checks atomic.Int64
	attachPunisher(t, f, p, &checks)
	floodPolicy := DefaultPolicy()
	floodPolicy.HitPolicy.High.Count = 100
	flood := &Snapshot{Config: PortConfig{Port: 5000, Policy: floodPolicy}}
	originalLookup := p.Lookup
	p.Lookup = func(id string) *Snapshot {
		if id == "port:flood" {
			return flood
		}
		return originalLookup(id)
	}
	client := f.front.Client()
	measure := func() time.Duration {
		times := make([]time.Duration, 200)
		for i := range times {
			started := time.Now()
			r, _ := http.NewRequest("GET", f.front.URL+"/ok", nil)
			r.Header.Set("X-Forwarded-For", "198.51.100.60")
			resp, err := client.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 204 {
				t.Fatal("other port stopped serving")
			}
			times[i] = time.Since(started)
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		return times[198]
	}
	baseline := measure()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); p.Run(ctx) }()
	floodDone := make(chan struct{})
	go func() {
		defer close(floodDone)
		for i := 0; i < 1000000; i++ {
			p.Submit(HitEvent{PortID: "port:flood", ClientIP: netip.AddrFrom4([4]byte{11, byte(i >> 16), byte(i >> 8), byte(i)}), Category: "sqli", Severity: High})
		}
	}()
	during := measure()
	<-floodDone
	cancel()
	<-workerDone
	if during > max(100*time.Millisecond, 10*baseline) {
		t.Fatalf("other port stalled baseline=%s flood=%s", baseline, during)
	}
	if f.requests.Load() != 400 {
		t.Fatal("normal requests lost")
	}
	if p.Stats()["dropped_high"] == 0 {
		t.Fatal("saturated queue did not expose dropped events")
	}
	t.Logf("real HTTP p99 baseline=%s flood=%s dropped=%d", baseline, during, p.Stats()["dropped_high"])
}
