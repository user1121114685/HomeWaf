package manager

import (
	"HomeWaf/plugin/process"
	"HomeWaf/wafpipeline"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pluginSnapshot(port uint16, ids ...string) *wafpipeline.Snapshot {
	p := wafpipeline.DefaultPolicy()
	p.RuleSet.Plugins = &wafpipeline.PluginPolicy{Mode: wafpipeline.ModeBlock, IDs: ids}
	s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: port, ListenEnabled: true, Policy: p}, Compiled: &wafpipeline.CompiledPolicy{}, Secrets: wafpipeline.PortSecrets{PluginProfiles: map[string]wafpipeline.PluginLaunch{}}}
	for _, id := range ids {
		s.Secrets.PluginProfiles[id] = wafpipeline.PluginLaunch{Binary: "fixture", Params: map[string]any{"nested": map[string]any{"token": "secret"}}}
	}
	return s
}

func TestPortPluginIsolationReuseAndCandidateRollback(t *testing.T) {
	old := wafpipeline.Ports
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](10)
	t.Cleanup(func() { wafpipeline.Ports = old })
	pool := NewPipelinePluginPool(4)
	starts, stops := 0, 0
	pool.start = func(port uint16, id string, profile wafpipeline.PluginLaunch) (wafpipeline.PluginCheck, func(), error) {
		starts++
		if id == "broken" {
			return wafpipeline.PluginCheck{}, nil, errors.New("private/path/secret")
		}
		profile.Params["nested"].(map[string]any)["token"] = "mutated"
		count := 0
		return wafpipeline.PluginCheck{ID: id, Check: func(ctx context.Context, request wafpipeline.PluginRequest) (bool, error) {
			if request.PortID != wafpipeline.PortID(port) {
				t.Error("cross-port request")
			}
			count++
			return count > 1, nil
		}}, func() { stops++ }, nil
	}
	a, b := pluginSnapshot(5001, "filter"), pluginSnapshot(5002, "filter")
	for _, s := range []*wafpipeline.Snapshot{a, b} {
		if err := pool.Prepare(s); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Compiled.Plugins.Close)
		if err := wafpipeline.Ports.Publish(s.Config.Port, func() (*wafpipeline.Snapshot, error) { return s, nil }); err != nil {
			t.Fatal(err)
		}
		if blocked, _ := s.Compiled.Plugins.Checks[0].Check(context.Background(), wafpipeline.PluginRequest{PortID: wafpipeline.PortID(s.Config.Port)}); blocked {
			t.Fatal("plugin state shared across ports")
		}
		if s.Secrets.PluginProfiles["filter"].Params["nested"].(map[string]any)["token"] != "secret" {
			t.Fatal("plugin mutated snapshot configuration")
		}
	}
	if starts != 2 {
		t.Fatalf("starts=%d", starts)
	}
	if blocked, _ := a.Compiled.Plugins.Checks[0].Check(context.Background(), wafpipeline.PluginRequest{PortID: wafpipeline.PortID(5001)}); !blocked {
		t.Fatal("port instance lost its state")
	}
	reuse := pluginSnapshot(5001, "filter")
	if err := pool.Prepare(reuse); err != nil || reuse.Compiled.Plugins != a.Compiled.Plugins || starts != 2 {
		t.Fatal("unchanged configuration not reused")
	}
	wafpipeline.CloseRetiredProxy(a, reuse)
	if a.Compiled.Plugins.Closed() {
		t.Fatal("reused instance closed")
	}
	candidate := pluginSnapshot(5001, "filter", "broken")
	candidate.Config.Policy.RuleSet.Plugins.Generation++
	err := pool.Prepare(candidate)
	if err == nil || strings.Contains(err.Error(), "private/") {
		t.Fatal("failed launch accepted or leaked internal error")
	}
	if pool.Stats()["processes"] != 2 || stops != 1 || a.Compiled.Plugins.Closed() || b.Compiled.Plugins.Closed() {
		t.Fatal("failed candidate leaked or closed live plugins")
	}
	// Only one spare slot: a partially prepared replacement must release it.
	c := pluginSnapshot(5003, "filter")
	if err := pool.Prepare(c); err != nil {
		t.Fatal(err)
	}
	defer c.Compiled.Plugins.Close()
	candidate = pluginSnapshot(5001, "filter", "second")
	candidate.Config.Policy.RuleSet.Plugins.Generation++
	if err := pool.Prepare(candidate); err == nil || pool.Stats()["processes"] != 3 {
		t.Fatal("quota rollback failed")
	}
	a.Compiled.Plugins.Close()
	a.Compiled.Plugins.Close()
	if b.Compiled.Plugins.Closed() || pool.Stats()["processes"] != 2 {
		t.Fatal("close affected another port or released twice")
	}
}

func TestPortPluginRejectsUnsignedBinaryWithoutLeakingPath(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]string{"test": base64.RawURLEncoding.EncodeToString(public)})
	old := process.TrustedKeysJSON
	process.TrustedKeysJSON = string(keys)
	defer func() { process.TrustedKeysJSON = old }()
	if !process.TrustConfigured() {
		t.Fatal("test keyring invalid")
	}
	dir := t.TempDir()
	unsigned := filepath.Join(dir, "unsigned.exe")
	if err := os.WriteFile(unsigned, []byte("not executable or signed"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "missing.exe"), unsigned} {
		pool := NewPipelinePluginPool(2)
		s := pluginSnapshot(5010, "filter")
		s.Secrets.PluginProfiles["filter"] = wafpipeline.PluginLaunch{Binary: path}
		if err := pool.Prepare(s); err == nil || strings.Contains(err.Error(), dir) {
			t.Fatal("unsigned/missing binary accepted or path disclosed")
		}
		if pool.Stats()["processes"] != 0 {
			t.Fatal("failed real launcher leaked quota")
		}
	}
}
