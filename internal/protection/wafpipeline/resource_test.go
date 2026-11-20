package wafpipeline

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHundredManagedPortsResourceAndIsolation(t *testing.T) {
	old := Ports
	Ports = NewRegistry[Snapshot](200)
	t.Cleanup(func() { Ports = old })
	manager := &ListenerManager{}
	t.Cleanup(manager.StopAll)
	var upstream atomic.Int64
	back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { upstream.Add(1); w.WriteHeader(204) }))
	defer back.Close()
	gate := NewGate(nil)
	rules := NewRulesA(gate)
	engine := DefaultRuleEngine()
	forward := &Forwarder{}
	live := &Runtime{Forward: forward.Serve, ForwardPrepare: forward.Prepare, Respond: gate.Respond, Admission: NewAdmission(256)}
	live.RuleSteps = func(s *Snapshot) []Step { return engine.WithControls(s, rules.Steps(s)) }
	manager.SetHandler(live.Serve)
	runtime.GC()
	var before, ready runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	ports := make([]uint16, 0, 100)
	for len(ports) < 100 {
		port := unusedPort(t)
		if _, exists := Ports.Get(port); exists {
			continue
		}
		policy := DefaultPolicy()
		policy.IPGroup.UnlistedAction = Deny
		policy.IPGroup.Entries = []IPEntry{{ID: "loopback-only", Type: "allow", Trust: "allow_only", Value: "127.0.0.1/32"}}
		policy.Access.DrainSeconds = 0
		s := &Snapshot{Config: PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: back.URL, ListenEnabled: true, Policy: policy}}
		if err := live.Prepare(s); err != nil {
			t.Fatal(err)
		}
		if err := Ports.Publish(port, func() (*Snapshot, error) { return s, nil }); err != nil {
			t.Fatal(err)
		}
		if err := manager.Start(s); err != nil {
			t.Fatal(err)
		}
		ports = append(ports, port)
	}
	runtime.GC()
	runtime.ReadMemStats(&ready)
	t.Logf("MANAGED_100_PORTS startup_ms=%d heap_delta=%d goroutines=%d", time.Since(start).Milliseconds(), int64(ready.HeapAlloc)-int64(before.HeapAlloc), runtime.NumGoroutine())
	transport := &http.Transport{MaxIdleConns: 200, MaxIdleConnsPerHost: 2}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	check := func(port uint16) {
		t.Helper()
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
		req.Host = fmt.Sprintf("forged:%d", ports[0])
		res, err := client.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != 204 {
			t.Errorf("port=%d status=%d", port, res.StatusCode)
		}
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, 16)
	for _, port := range ports {
		slots <- struct{}{}
		wg.Add(1)
		go func(p uint16) { defer wg.Done(); defer func() { <-slots }(); check(p) }(port)
	}
	wg.Wait()
	if upstream.Load() != 100 {
		t.Fatalf("upstream=%d want100", upstream.Load())
	}
	if err := manager.Stop(ports[0]); err != nil {
		t.Fatal(err)
	}
	for _, port := range ports[1:] {
		check(port)
	}
	if upstream.Load() != 199 {
		t.Fatalf("stopping one port affected others: upstream=%d", upstream.Load())
	}
	manager.StopAll()
	transport.CloseIdleConnections()
	for _, port := range ports {
		s, _ := Ports.Get(port)
		CloseRetiredProxy(s, nil)
	}
	if manager.Count() != 0 {
		t.Fatal("listeners or accepted connections did not close")
	}
}
