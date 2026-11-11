package wafenginecore

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNativeSensitiveContentAndResponsePoliciesEndToEnd(t *testing.T) {
	var hits atomic.Int64
	var wire atomic.Value
	large := strings.Repeat("family-file-", 100000)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		raw, _ := io.ReadAll(r.Body)
		wire.Store([]string{string(raw), r.URL.RequestURI()})
		if r.Method == "POST" {
			w.Header().Add("Set-Cookie", "business=value; Path=/")
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		switch r.URL.Path {
		case "/privacy":
			_, _ = io.WriteString(w, "邮箱: family@example.com 手机: 13800138000")
		case "/deny-out":
			_, _ = io.WriteString(w, "out-deny and replace-word")
		case "/large":
			w.Header().Set("Content-Length", strconv.Itoa(len(large)))
			_, _ = io.WriteString(w, large)
		case "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: out-deny\n\n")
			w.(http.Flusher).Flush()
		default:
			_, _ = io.WriteString(w, strings.Repeat("replace-word ", 80))
		}
	}))
	defer up.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("replace-word static"), 0600); err != nil {
		t.Fatal(err)
	}
	var current atomic.Pointer[wafpipeline.Snapshot]
	forward := &wafpipeline.Forwarder{DecodeResponse: PipelineDecodeResponse, EncodeResponse: PipelineEncodeResponse, SecureCookie: PipelineSecureCookie, StaticHandler: PipelineStaticHandler}
	gate := wafpipeline.NewGate(nil)
	rules := wafpipeline.NewRulesA(gate)
	engine := wafpipeline.DefaultRuleEngine()
	runtime := &wafpipeline.Runtime{RuleSteps: func(s *wafpipeline.Snapshot) []wafpipeline.Step { return engine.WithControls(s, rules.Steps(s)) }, Respond: gate.Respond, ForwardPrepare: forward.Prepare, Forward: forward.Serve}
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { runtime.Serve(current.Load(), w, r) }))
	defer front.Close()
	port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
	config := wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: up.URL, Policy: wafpipeline.DefaultPolicy(), Routes: []wafpipeline.UpstreamRoute{{Hosts: []string{"*"}, PathPrefix: "/static/", TargetType: "static", Static: &wafpipeline.StaticBusiness{Settings: model.StaticSiteConfig{StaticSitePath: root, StaticSitePrefix: "/static/", AllowedExtensions: ".txt"}}}}}
	config.Policy.RuleSet.Rules = nil
	config.Policy.RuleSet.BudgetMillis = 1000
	config.Policy.RuleSet.QPS.Mode = wafpipeline.ModeOff
	config.Policy.RuleSet.CC.Mode = wafpipeline.ModeOff
	config.Policy.RuleSet.BOT = wafpipeline.ModeOff
	config.Policy.RuleSet.Sensitive = &wafpipeline.SensitivePolicy{Mode: wafpipeline.ModeBlock, Replacement: "[redacted]", Rules: []model.Sensitive{{CheckDirection: "out", Action: "deny", Content: "out-deny"}, {CheckDirection: "in", Action: "deny", Content: "in-deny"}, {CheckDirection: "all", Action: "replace", Content: "replace-word"}}}
	config.Policy.Network.Response.Cookie = model.CookieSecurityConfig{IsEnable: 1, HttpOnly: 1, Secure: 2, SameSite: "Lax"}
	var baselines map[string]wafpipeline.TamperBaseline
	set := func() {
		t.Helper()
		s := &wafpipeline.Snapshot{Config: wafpipeline.CloneConfig(config), Secrets: wafpipeline.PortSecrets{GateKey: "content-fixture-key-not-production", TamperBaselines: baselines}}
		if err := runtime.Prepare(s); err != nil {
			t.Fatal(err)
		}
		old := current.Swap(s)
		wafpipeline.CloseRetiredProxy(old, s)
	}
	defer func() { wafpipeline.CloseRetiredProxy(current.Load(), nil) }()
	set()
	front.Start()
	call := func(method, path, body, encoding string) (int, string, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, front.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		if encoding != "" {
			req.Header.Set("Accept-Encoding", encoding)
		}
		res, err := front.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var rd io.Reader = res.Body
		if res.Header.Get("Content-Encoding") == "gzip" {
			gz, e := gzip.NewReader(res.Body)
			if e != nil {
				t.Fatal(e)
			}
			defer gz.Close()
			rd = gz
		}
		raw, e := io.ReadAll(rd)
		if e != nil {
			t.Fatal(e)
		}
		return res.StatusCode, string(raw), res.Header
	}
	code, _, _ := call("POST", "/save", "in-deny", "")
	if code != 403 || hits.Load() != 0 {
		t.Fatal("request deny reached origin")
	}
	code, body, head := call("POST", "/save?q=replace-word", "replace-word", "")
	seen := wire.Load().([]string)
	if code != 200 || seen[0] != "[redacted]" || !strings.Contains(seen[1], "%5Bredacted%5D") || strings.Contains(body, "replace-word") {
		t.Fatal("request/response replacement lost")
	}
	if !strings.Contains(head.Get("Set-Cookie"), "HttpOnly") || strings.Contains(head.Get("Set-Cookie"), "Secure") {
		t.Fatal("cookie safety changed HTTP semantics")
	}
	code, body, _ = call("GET", "/deny-out", "", "")
	if code != 403 || strings.Contains(body, "out-deny") {
		t.Fatal("response deny leaked content")
	}
	code, body, _ = call("GET", "/static/file.txt", "", "")
	if code != 200 || body != "[redacted] static" {
		t.Fatalf("local static content skipped policy: %d %q", code, body)
	}
	code, body, _ = call("GET", "/large", "", "")
	if code != 200 || body != large {
		t.Fatal("large body was buffered or truncated")
	}
	code, body, _ = call("GET", "/sse", "", "")
	if code != 200 || body != "data: out-deny\n\n" {
		t.Fatal("SSE changed")
	}
	config.Policy.RuleSet.Sensitive.Mode = wafpipeline.ModeObserve
	set()
	code, body, _ = call("POST", "/save?q=replace-word", "replace-word", "")
	seen = wire.Load().([]string)
	if code != 200 || seen[0] != "replace-word" || !strings.Contains(seen[1], "replace-word") || !strings.Contains(body, "replace-word") {
		t.Fatal("observe mutated request or response")
	}
	config.Policy.Network.Response.Compression = model.ResponseCompressConfig{IsEnable: 1, Prefer: "gzip_only", MinLength: 1}
	set()
	code, body, head = call("GET", "/save", "", "gzip")
	if code != 200 || head.Get("Content-Encoding") != "gzip" || !strings.Contains(body, "replace-word") {
		t.Fatal("configured compression failed")
	}
	config.Policy.Network.Response.Compression.IsEnable = 0
	config.Policy.RuleSet.Sensitive.Mode = wafpipeline.ModeOff
	rule := wafpipeline.TamperRule{ID: "fixed-page", Path: "/save", Mode: wafpipeline.ModeBlock, Action: "replace", Generation: 1}
	config.Policy.Network.Response.Tamper = []wafpipeline.TamperRule{rule}
	baselines = map[string]wafpipeline.TamperBaseline{rule.ID: wafpipeline.NewTamperBaseline(rule, []byte("approved-page"), "text/plain", "utf-8")}
	set()
	code, body, _ = call("GET", "/save", "", "")
	if code != 200 || body != "approved-page" {
		t.Fatal("tampered response did not restore baseline")
	}
	config.Policy.Network.Response.Tamper[0].Mode = wafpipeline.ModeObserve
	set()
	code, body, _ = call("GET", "/save", "", "")
	if code != 200 || !strings.Contains(body, "replace-word") {
		t.Fatal("observe tamper policy altered response")
	}
	config.Policy.Network.Response.Tamper[0].Mode = wafpipeline.ModeBlock
	set()
	code, body, _ = call("GET", "/save", "", "")
	if code != 200 || body != "approved-page" {
		t.Fatal("observe-to-block transition invalidated approved baseline")
	}
	captured := make(chan wafpipeline.TamperBaseline, 1)
	forward.CaptureTamper = func(s *wafpipeline.Snapshot, rule wafpipeline.TamperRule, body []byte, ctype, charset string) bool {
		select {
		case captured <- wafpipeline.NewTamperBaseline(rule, body, ctype, charset):
			return true
		default:
			return false
		}
	}
	config.Policy.Network.Response.Tamper[0].Generation++
	set()
	code, body, _ = call("GET", "/save", "", "")
	if code != 200 || !strings.Contains(body, "replace-word") {
		t.Fatal("relearn reused stale baseline")
	}
	select {
	case baseline := <-captured:
		if !strings.Contains(string(baseline.Content), "replace-word") {
			t.Fatal("capture did not receive actual origin content")
		}
	default:
		t.Fatal("missing baseline capture")
	}
	config.Policy.Network.Response.Privacy = &wafpipeline.PrivacyPolicy{Mode: wafpipeline.ModeBlock, URLs: []wafpipeline.PrivacyURL{{Match: "exact", Value: "/privacy"}}}
	set()
	code, body, _ = call("GET", "/privacy", "", "")
	if code != 200 || strings.Contains(body, "family@example.com") {
		t.Fatal("response privacy adapter did not mask actual origin content")
	}
	config.Policy.Network.Response.Privacy.Mode = wafpipeline.ModeObserve
	set()
	code, body, _ = call("GET", "/privacy", "", "")
	if code != 200 || !strings.Contains(body, "family@example.com") {
		t.Fatal("observe privacy adapter changed original response")
	}
}
