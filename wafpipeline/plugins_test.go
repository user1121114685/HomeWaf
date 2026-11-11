package wafpipeline

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestPortPluginRunsAfterGateAndFailsOpenWithinBudget(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	var calls atomic.Int64
	var behavior atomic.Int32
	f.runtime.PluginPrepare = func(s *Snapshot) error {
		s.Compiled.Plugins = &PreparedPlugins{Checks: []PluginCheck{{ID: "filter", Check: func(ctx context.Context, r PluginRequest) (bool, error) {
			calls.Add(1)
			if r.PortID != PortID(s.Config.Port) || r.Path != "/files" {
				t.Error("incorrect plugin metadata")
			}
			switch behavior.Load() {
			case 1:
				return false, errors.New("credential-must-not-leak")
			case 2:
				<-ctx.Done()
				return false, ctx.Err()
			}
			return true, nil
		}}}}
		return nil
	}
	f.update(t, func(s *Snapshot) {
		s.Config.ListenEnabled = true
		s.Config.Policy.Gate.RequireToken = true
		s.Config.Policy.Gate.TokenHeader = "X-HomeWaf-Token"
		s.Secrets.ServiceTokenHashes = []string{tokenHash("correct-token")}
		s.Config.Policy.RuleSet.Plugins = &PluginPolicy{Mode: ModeBlock, IDs: []string{"filter"}, TimeoutMillis: 15}
		s.Config.Policy.RuleSet.BudgetMillis = 500
	})
	if code, _, _ := proxyCall(t, f, "GET", "/files", "", nil); code != 401 || calls.Load() != 0 || f.requests.Load() != 0 {
		t.Fatal("unauthenticated request called plugin or upstream")
	}
	headers := map[string]string{"X-HomeWaf-Token": "correct-token"}
	if code, _, _ := proxyCall(t, f, "GET", "/files", "", headers); code != 403 || calls.Load() != 1 || f.requests.Load() != 0 {
		t.Fatal("plugin deny did not prevent upstream")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.Plugins.Mode = ModeObserve })
	if code, _, _ := proxyCall(t, f, "GET", "/files", "", headers); code != 204 || f.requests.Load() != 1 {
		t.Fatal("observe blocked business")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.Plugins.Mode = ModeBlock })
	for _, mode := range []int32{1, 2} {
		behavior.Store(mode)
		before := f.requests.Load()
		start := time.Now()
		if code, _, _ := proxyCall(t, f, "GET", "/files", "", headers); code != 204 || f.requests.Load() != before+1 {
			t.Fatal("plugin error blocked business")
		}
		if time.Since(start) > time.Second {
			t.Fatal("plugin timeout escaped budget")
		}
	}
	before := calls.Load()
	req := httptest.NewRequest("GET", "http://home.test/files", nil)
	req.Header.Set("X-HomeWaf-Token", "correct-token")
	if _, err := f.runtime.Explain(f.current.Load(), req, netip.MustParseAddr("198.51.100.10")); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before {
		t.Fatal("dry-run executed plugin")
	}
	errorsSeen := 0
	for _, e := range f.sink.snapshot() {
		if e.Category == "plugin_error" {
			errorsSeen++
			if !e.Observe || e.Evidence != "local plugin unavailable or timed out" {
				t.Fatal("plugin error leaked or became punitive")
			}
		}
	}
	if errorsSeen != 2 {
		t.Fatalf("missing error events: %d", errorsSeen)
	}
}

func TestPluginResourcesCloseWhenListenerCannotStart(t *testing.T) {
	old := Ports
	Ports = NewRegistry[Snapshot](10)
	defer func() { Ports = old }()
	p := unusedPort(t)
	var stopped atomic.Int64
	s := &Snapshot{Config: PortConfig{Port: p, Protocol: "https", ListenEnabled: true, Policy: DefaultPolicy()}, Compiled: &CompiledPolicy{Plugins: &PreparedPlugins{Stop: func() { stopped.Add(1) }}}}
	s.Config.Policy.Network.ListenIPVersion = "ipv4"
	if err := Ports.Publish(p, func() (*Snapshot, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	m := &ListenerManager{}
	defer m.StopAll()
	if err := m.StartAll(); err == nil {
		t.Fatal("listener started with missing TLS certificate")
	}
	m.StopAll()
	if !s.Compiled.Plugins.Closed() || stopped.Load() != 1 {
		t.Fatal("failed listener retained plugin resources")
	}
}
