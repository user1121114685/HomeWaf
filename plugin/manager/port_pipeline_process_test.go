package manager

import (
	"HomeWaf/internal/licensing"
	plugininterface "HomeWaf/plugin/interface"
	"HomeWaf/plugin/process"
	"HomeWaf/plugin/proto"
	"HomeWaf/wafpipeline"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("HOMEWAF_TEST_PORT_PLUGIN") == "fixture-child" {
		if err := process.Serve(&proto.GRPCServer{Impl: &isolatedTestPlugin{}}); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type isolatedTestPlugin struct {
	mu    sync.Mutex
	port  string
	calls int
}

func (*isolatedTestPlugin) Name() string                      { return "fixture" }
func (*isolatedTestPlugin) Type() string                      { return "ip_filter" }
func (*isolatedTestPlugin) Version() string                   { return "1" }
func (*isolatedTestPlugin) Shutdown() error                   { return nil }
func (*isolatedTestPlugin) HealthCheck(context.Context) error { return nil }
func (p *isolatedTestPlugin) Init(config map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.port, _ = config["port_id"].(string)
	return nil
}
func (p *isolatedTestPlugin) Filter(ctx context.Context, request *plugininterface.IPFilterRequest) (*plugininterface.IPFilterResponse, error) {
	if request.RequestPath == "/wait" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return &plugininterface.IPFilterResponse{Allowed: p.calls == 1 && request.Extra["port_id"] == p.port}, nil
}

func TestPortPluginRealSignedProcessesAreIsolated(t *testing.T) {
	t.Setenv("HOMEWAF_TEST_PORT_PLUGIN", "fixture-child")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]string{"fixture": base64.RawURLEncoding.EncodeToString(public)})
	old := process.TrustedKeysJSON
	process.TrustedKeysJSON = string(keys)
	defer func() { process.TrustedKeysJSON = old }()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	binary := filepath.Join(t.TempDir(), filepath.Base(executable))
	target, err := os.OpenFile(binary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(target, hash), source)
	closeErr := target.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	signature, err := licensing.SignPlugin(hex.EncodeToString(hash.Sum(nil)), "fixture", private)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(binary+".sig.json", signature, 0600); err != nil {
		t.Fatal(err)
	}
	pool := NewPipelinePluginPool(2)
	a, b := pluginSnapshot(5111, "real"), pluginSnapshot(5112, "real")
	for _, s := range []*wafpipeline.Snapshot{a, b} {
		s.Secrets.PluginProfiles["real"] = wafpipeline.PluginLaunch{Binary: binary}
		if err = pool.Prepare(s); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Compiled.Plugins.Close)
		for i := 0; i < 2; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			blocked, err := s.Compiled.Plugins.Checks[0].Check(ctx, wafpipeline.PluginRequest{PortID: wafpipeline.PortID(s.Config.Port), Path: "/files"})
			cancel()
			if err != nil || blocked != (i == 1) {
				t.Fatalf("real isolated RPC: port=%d call=%d blocked=%v err=%v", s.Config.Port, i, blocked, err)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = a.Compiled.Plugins.Checks[0].Check(ctx, wafpipeline.PluginRequest{Path: "/wait"}); err == nil {
		t.Fatal("real RPC ignored deadline")
	}
	a.Compiled.Plugins.Close()
	b.Compiled.Plugins.Close()
	if pool.Stats()["processes"] != 0 {
		t.Fatal("real processes retained quota")
	}
}
