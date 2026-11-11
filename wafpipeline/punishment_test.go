package wafpipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"runtime"
	"sort"
	"strings"
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
		s.Config.Policy.Honeypot.Enabled = true
		s.Config.Policy.Honeypot.MaxDelayMillis = 0
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

func TestPunishmentCriticalExpiryRepeatSharingAndTrusted(t *testing.T) {
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
	if penaltyRequest(t, b, ip, "/ok") != 403 || b.requests.Load() != 0 {
		t.Fatal("global ban not shared")
	}
	b.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "trusted", Type: "allow", Trust: "trusted", Value: ip}}
	})
	if penaltyRequest(t, b, ip, "/ok") != 204 || b.requests.Load() != 1 {
		t.Fatal("shared ban overrides trusted")
	}
	b.update(t, func(s *Snapshot) {
		s.Config.Policy.IPGroup.Entries = append(s.Config.Policy.IPGroup.Entries, IPEntry{ID: "deny", Type: "deny", Value: ip})
	})
	if penaltyRequest(t, b, ip, "/ok") != 403 || b.requests.Load() != 1 {
		t.Fatal("custom deny did not override trusted")
	}
	now = now.Add(time.Hour + time.Second)
	p.Drain(0)
	if penaltyRequest(t, a, ip, "/ok") != 204 || a.requests.Load() != 1 {
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

func TestPunishmentLockdownIsolationCooldownAndRestore(t *testing.T) {
	p := NewPunisher()
	now := time.Now()
	p.Now = func() time.Time { return now }
	p.start = now
	a := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	b := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	var ca, cb atomic.Int64
	attachPunisher(t, a, p, &ca)
	attachPunisher(t, b, p, &cb)
	a.update(t, func(s *Snapshot) {
		s.Config.Policy.HitPolicy.Lockdown.Enabled = true
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "trusted", Type: "allow", Trust: "trusted", Value: "198.51.100.99"}}
	})
	port := PortID(a.current.Load().Config.Port)
	for i := 1; i <= 5; i++ {
		penaltyEvent(p, port, fmt.Sprintf("198.51.100.%d", i), High, "sqli", fmt.Sprint(i))
	}
	if !p.Locked(port) || penaltyRequest(t, a, "198.51.100.90", "/ok") != 503 || a.requests.Load() != 0 {
		t.Fatal("lockdown not enforced")
	}
	if penaltyRequest(t, a, "198.51.100.99", "/ok") != 204 || a.requests.Load() != 1 {
		t.Fatal("lockdown blocked trusted")
	}
	if penaltyRequest(t, b, "198.51.100.90", "/ok") != 204 || b.requests.Load() != 1 {
		t.Fatal("lockdown crossed ports")
	}
	var saved []PunishmentRecord
	p.Persist = func(rows []PunishmentRecord) error { saved = append([]PunishmentRecord(nil), rows...); return nil }
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	restored := NewPunisher()
	restored.Now = p.Now
	restored.Lookup = p.Lookup
	if err := restored.Restore(saved); err != nil || !restored.Locked(port) {
		t.Fatalf("restore failed: %v", err)
	}
	var id string
	for _, r := range p.Records(port) {
		if r.Kind == "lockdown" {
			id = r.ID
		}
	}
	if err := p.Manage(port, id, "release"); err != nil {
		t.Fatal(err)
	}
	for i := 11; i <= 15; i++ {
		penaltyEvent(p, port, fmt.Sprintf("198.51.100.%d", i), High, "sqli", fmt.Sprint(i))
	}
	if p.Locked(port) {
		t.Fatal("manual release lacked cooldown")
	}
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	restored = NewPunisher()
	restored.Now = p.Now
	restored.Lookup = p.Lookup
	restored.start = now
	if err := restored.Restore(saved); err != nil {
		t.Fatal(err)
	}
	for i := 21; i <= 25; i++ {
		penaltyEvent(restored, port, fmt.Sprintf("198.51.100.%d", i), High, "sqli", fmt.Sprint(i))
	}
	if restored.Locked(port) {
		t.Fatal("restart lost cooldown")
	}
	now = now.Add(61 * time.Second)
	p.Drain(0)
	for i := 31; i <= 35; i++ {
		penaltyEvent(p, port, fmt.Sprintf("198.51.100.%d", i), High, "sqli", fmt.Sprint(i))
	}
	if !p.Locked(port) {
		t.Fatal("post-cooldown lock missing")
	}
	now = now.Add(601 * time.Second)
	p.Drain(0)
	if p.Locked(port) {
		t.Fatal("lock did not expire")
	}
}

func TestRealLoginFailuresHoneypotReleaseAndEscalation(t *testing.T) {
	f := newGateFixture(t, NewGateStore(), "basic", "correct-family-password")
	p := NewPunisher()
	p.Lookup = func(id string) *Snapshot {
		if id == PortID(f.snap.Config.Port) {
			return f.snap
		}
		return nil
	}
	f.snap.Config.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
	f.snap.Config.Policy.Honeypot.Enabled = true
	f.snap.Config.Policy.Honeypot.MaxDelayMillis = 0
	f.runtime.Sink = p
	f.runtime.State = p
	h := NewHoneyHandler()
	old := f.runtime.Respond
	f.runtime.Respond = func(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
		return h.Respond(s, c, w, v) || old(s, c, w, v)
	}
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	ip := "198.51.100.48"
	port := PortID(f.snap.Config.Port)
	call := func(method, path, password string) int {
		r, _ := http.NewRequest(method, f.server.URL+path, strings.NewReader("username=owner&password=honey-secret"))
		r.Header.Set("X-Forwarded-For", ip)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if password != "" {
			r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("owner:"+password)))
		}
		response, err := f.server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		p.Drain(256)
		return response.StatusCode
	}
	for range 5 {
		if call("GET", "/private", "incorrect-family-password") != 401 {
			t.Fatal("failed login response changed")
		}
	}
	if !p.InHoneypot(port, netip.MustParseAddr(ip)) || call("GET", "/private", "") != 200 || f.hits.Load() != 0 {
		t.Fatal("honeypot failed or reached upstream")
	}
	for _, r := range p.Records(port) {
		if r.Kind == "honeypot" {
			if err := p.Manage(port, r.ID, "release"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if call("GET", "/private", "correct-family-password") != 200 || f.hits.Load() != 1 {
		t.Fatal("legitimate user did not recover after release")
	}
	for range 3 {
		call("GET", "/private", "admin")
	}
	if !p.InHoneypot(port, netip.MustParseAddr(ip)) {
		t.Fatal("weak password threshold missing")
	}
	call("POST", "/admin", "")
	if p.BanReason(port, netip.MustParseAddr(ip)) == "" || f.hits.Load() != 1 {
		t.Fatal("honeypot submission did not escalate or leaked upstream")
	}
}

func TestMillionSourceCountersStayBoundedAndPreserveOtherPort(t *testing.T) {
	p := NewPunisher()
	policy := DefaultPolicy()
	policy.HitPolicy.High.Count = 100
	policy.Honeypot.Enabled = true
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
	policy.HitPolicy.BanScope = "port"
	policy.Honeypot.Enabled = true
	policy.HitPolicy.ResponseSignals = true
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: policy}}
	p.Lookup = func(id string) *Snapshot { return s }
	penaltyEvent(p, "port:5000", "2001:db8:1:2::a", Critical, "rce", "a")
	if p.BanReason("port:5000", netip.MustParseAddr("2001:db8:1:2::b")) == "" || p.BanReason("port:5001", netip.MustParseAddr("2001:db8:1:2::b")) != "" {
		t.Fatal("IPv6 prefix or port scope incorrect")
	}
	for i := 0; i < 30; i++ {
		p.Submit(HitEvent{PortID: "port:5000", ClientIP: netip.MustParseAddr("198.51.100.51"), Category: "response_5xx", Severity: Low, Observe: true, ReqID: fmt.Sprint(i)})
	}
	p.Drain(256)
	if p.InHoneypot("port:5000", netip.MustParseAddr("198.51.100.51")) {
		t.Fatal("upstream outage punished a person")
	}
	for i := 0; i < 20; i++ {
		p.Submit(HitEvent{PortID: "port:5000", ClientIP: netip.MustParseAddr("198.51.100.52"), Category: "response_404", Severity: Low, Path: "/same", ReqID: fmt.Sprint(i)})
	}
	p.Drain(256)
	if p.InHoneypot("port:5000", netip.MustParseAddr("198.51.100.52")) {
		t.Fatal("repeated same 404 counted as distinct paths")
	}
	for i := 0; i < 20; i++ {
		p.Submit(HitEvent{PortID: "port:5000", ClientIP: netip.MustParseAddr("198.51.100.52"), Category: "response_404", Severity: Low, Path: fmt.Sprint("/path/", i), ReqID: fmt.Sprint("n", i)})
	}
	p.Drain(256)
	if !p.InHoneypot("port:5000", netip.MustParseAddr("198.51.100.52")) {
		t.Fatal("distinct 404 threshold missing")
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
