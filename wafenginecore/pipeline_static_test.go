package wafenginecore

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNativeStaticBusinessRequiresGateAndConfinesFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "family.txt"), []byte("private-family-file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("family-spa"), 0600); err != nil {
		t.Fatal(err)
	}
	var current atomic.Pointer[wafpipeline.Snapshot]
	forward := &wafpipeline.Forwarder{StaticHandler: PipelineStaticHandler}
	gate := wafpipeline.NewGate(nil)
	rules := wafpipeline.NewRulesA(gate)
	runtime := &wafpipeline.Runtime{RuleSteps: rules.Steps, Respond: gate.Respond, ForwardPrepare: forward.Prepare, Forward: forward.Serve}
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { runtime.Serve(current.Load(), w, r) }))
	defer front.Close()
	port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
	config := wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Policy: wafpipeline.DefaultPolicy(), Routes: []wafpipeline.UpstreamRoute{{Hosts: []string{"*"}, PathPrefix: "/files/", TargetType: "static", Static: &wafpipeline.StaticBusiness{Settings: model.StaticSiteConfig{StaticSitePath: root, StaticSitePrefix: "/files/", AllowedExtensions: ".txt,.html"}, SPAFallback: true}}}}
	config.Policy.RuleSet.QPS.Mode = wafpipeline.ModeOff
	config.Policy.RuleSet.CC.Mode = wafpipeline.ModeOff
	config.Policy.RuleSet.BOT = wafpipeline.ModeOff
	set := func(enabled bool) {
		t.Helper()
		config.Policy.Gate.Enabled = enabled
		s := &wafpipeline.Snapshot{Config: wafpipeline.CloneConfig(config), Secrets: wafpipeline.PortSecrets{GateKey: "static-fixture-key-not-production"}}
		if err := runtime.Prepare(s); err != nil {
			t.Fatal(err)
		}
		old := current.Swap(s)
		wafpipeline.CloseRetiredProxy(old, s)
	}
	defer func() { wafpipeline.CloseRetiredProxy(current.Load(), nil) }()
	set(true)
	front.Start()
	get := func(path, rangeHeader string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", front.URL+path, nil)
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		res, err := front.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res.StatusCode, string(body)
	}
	code, body := get("/files/family.txt", "")
	if code != 401 || strings.Contains(body, "private-family-file") {
		t.Fatal("static business bypassed login")
	}
	set(false)
	code, body = get("/files/family.txt", "bytes=0-6")
	if code != 206 || body != "private" {
		t.Fatalf("Range lost: %d %q", code, body)
	}
	code, body = get("/files/gallery", "")
	if code != 200 || body != "family-spa" {
		t.Fatal("SPA fallback lost")
	}
	code, body = get("/files/%2e%2e/secret.txt", "")
	if code != 403 {
		t.Fatalf("path traversal accepted: %d %q", code, body)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err == nil {
		code, body = get("/files/link.txt", "")
		if code != 403 || strings.Contains(body, "outside-secret") {
			t.Fatal("static symlink escaped root")
		}
	} else {
		t.Log("symlink creation unavailable on host; traversal and os.Root confinement still exercised")
	}
}
