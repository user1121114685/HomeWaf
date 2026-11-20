package wafpipeline

import (
	"bufio"
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type proxyFixture struct {
	runtime     *Runtime
	current     atomic.Pointer[Snapshot]
	front, back *httptest.Server
	sink        *safeSink
	requests    atomic.Int64
}

func newProxyFixture(t *testing.T, handler http.HandlerFunc, backendTLS bool) *proxyFixture {
	t.Helper()
	f := &proxyFixture{sink: &safeSink{}}
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.requests.Add(1); handler(w, r) })
	if backendTLS {
		f.back = httptest.NewTLSServer(backend)
	} else {
		f.back = httptest.NewServer(backend)
	}
	f.front = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.runtime.Serve(f.current.Load(), w, r) }))
	port := uint16(f.front.Listener.Addr().(*net.TCPAddr).Port)
	policy := DefaultPolicy()
	policy.Gate.Enabled = false
	policy.RuleSet.QPS.Mode = ModeOff
	policy.RuleSet.CC.Mode = ModeOff
	policy.RuleSet.BOT = ModeOff
	for i := range policy.RuleSet.Rules {
		policy.RuleSet.Rules[i].Mode = ModeOff
	}
	policy.RuleSet.Rules = []Rule{{ID: "upload", Category: "upload", Mode: ModeObserve, Severity: High}}
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: f.back.URL, Policy: policy}, Secrets: PortSecrets{GateKey: randomToken()}}
	if backendTLS {
		s.Secrets.UpstreamCAPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.back.Certificate().Raw}))
	}
	gate := NewGate(nil)
	rules := NewRulesA(gate)
	engine := DefaultRuleEngine()
	forwarder := &Forwarder{}
	f.runtime = &Runtime{Admission: NewAdmission(256), RuleSteps: func(s *Snapshot) []Step { return engine.WithControls(s, rules.Steps(s)) }, Respond: gate.Respond, ForwardPrepare: forwarder.Prepare, Forward: forwarder.Serve, Sink: f.sink}
	if err := f.runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	f.current.Store(s)
	if err := Ports.Publish(port, func() (*Snapshot, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	f.front.Start()
	t.Cleanup(func() {
		f.front.Close()
		if s := f.current.Load(); s != nil && s.Compiled.Proxy != nil {
			s.Compiled.Proxy.transport.CloseIdleConnections()
		}
		Ports.Remove(port)
		f.back.Close()
	})
	return f
}
func (f *proxyFixture) update(t *testing.T, change func(*Snapshot)) {
	t.Helper()
	old := f.current.Load()
	next := &Snapshot{Config: CloneConfig(old.Config), Secrets: old.Secrets}
	change(next)
	if err := f.runtime.Prepare(next); err != nil {
		t.Fatal(err)
	}
	if err := Ports.Publish(next.Config.Port, func() (*Snapshot, error) { return next, nil }); err != nil {
		t.Fatal(err)
	}
	f.current.Store(next)
	if old.Compiled.Proxy != next.Compiled.Proxy {
		old.Compiled.Proxy.transport.CloseIdleConnections()
	}
}
func proxyCall(t *testing.T, f *proxyFixture, method, path, body string, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, f.front.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw, response.Header
}

func TestForwardHeadersWireBytesTLSAndRequestLimit(t *testing.T) {
	seen := make(chan *http.Request, 4)
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.Copy(w, r.Body)
	}, true)
	status, body, _ := proxyCall(t, f, "POST", "/files/a%2Fb?q=a%2Bb", "wire-body", map[string]string{"Cookie": "hw_gate_5000=private; business=keep", "Authorization": "Bearer application", "X-HomeWaf-Token": "private", "X-Waf-Custom-XFF-Control": "skip-auto-append", "X-Forwarded-For": "203.0.113.10", "X-Real-IP": "203.0.113.10", "X-Forwarded-Proto": "https", "Forwarded": "for=203.0.113.10", "Connection": "X-Hop, X-Real-IP", "X-Hop": "remove"})
	if status != 200 || string(body) != "wire-body" || f.requests.Load() != 1 {
		t.Fatalf("forward failed %d %q", status, body)
	}
	r := <-seen
	if r.RequestURI != "/files/a%2Fb?q=a%2Bb" || r.Header.Get("Cookie") != "business=keep" || r.Header.Get("Authorization") != "Bearer application" || r.Header.Get("X-Forwarded-For") != "127.0.0.1" || r.Header.Get("X-Real-IP") != "127.0.0.1" || r.Header.Get("X-Forwarded-Proto") != "http" {
		t.Fatalf("forward identity or bytes changed: URI=%s headers=%v", r.RequestURI, r.Header)
	}
	for _, name := range []string{"X-Hop", "X-HomeWaf-Token", "X-Waf-Custom-XFF-Control", "Forwarded"} {
		if r.Header.Get(name) != "" {
			t.Fatalf("leaked header %s", name)
		}
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Network.TrustedProxies = []string{"127.0.0.1/32"} })
	status, _, _ = proxyCall(t, f, "GET", "/trusted", "", map[string]string{"X-Forwarded-For": "198.51.100.8", "X-Forwarded-Proto": "https"})
	r = <-seen
	if status != 200 || r.Header.Get("X-Forwarded-For") != "198.51.100.8" || r.Header.Get("X-Forwarded-Proto") != "https" {
		t.Fatal("trusted proxy identity lost")
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Network.MaxRequestBytes = 5 })
	before := f.requests.Load()
	status, _, _ = proxyCall(t, f, "POST", "/upload", "123456789", nil)
	if status != 413 || f.requests.Load() != before {
		t.Fatal("known oversized request reached upstream")
	}
}
func TestForwardRangeSSEAndWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/range":
			http.ServeContent(w, r, "file.txt", time.Time{}, strings.NewReader("0123456789"))
		case "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-time.After(250 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			io.WriteString(w, "data: second\n\n")
		case "/ws":
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer c.Close()
			kind, message, err := c.ReadMessage()
			if err == nil {
				_ = c.WriteMessage(kind, message)
			}
		}
	}, false)
	status, body, headers := proxyCall(t, f, "GET", "/range", "", map[string]string{"Range": "bytes=2-5"})
	if status != 206 || string(body) != "2345" || headers.Get("Content-Range") != "bytes 2-5/10" {
		t.Fatal("Range changed")
	}
	start := time.Now()
	response, err := f.front.Client().Get(f.front.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "data: first\n" || time.Since(start) > 200*time.Millisecond {
		response.Body.Close()
		t.Fatalf("SSE first event delayed: %q %v", first, err)
	}
	tail, err := io.ReadAll(reader)
	response.Body.Close()
	if err != nil || !strings.Contains(string(tail), "data: second") {
		t.Fatal("SSE tail lost")
	}
	conn, resp, err := websocket.DefaultDialer.Dial(strings.Replace(f.front.URL, "http:", "ws:", 1)+"/ws", nil)
	if err != nil {
		t.Fatalf("WebSocket handshake: %v response=%v", err, resp)
	}
	defer conn.Close()
	message := bytes.Repeat([]byte("frame"), 40000)
	if err = conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
		t.Fatal(err)
	}
	_, got, err := conn.ReadMessage()
	if err != nil || !bytes.Equal(got, message) {
		t.Fatalf("WebSocket frame damaged %d/%d %v", len(got), len(message), err)
	}
}
func TestResponseSignalsOutageAndBusinessLogin(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(500)
		case "/missing":
			w.WriteHeader(404)
		case "/challenge":
			w.WriteHeader(401)
		case "/login":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"success":false}`)
		default:
			io.WriteString(w, "ok")
		}
	}, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.HitPolicy.ResponseSignals = true
		s.Config.Policy.HitPolicy.BusinessLogin = []LoginObservation{{Path: "/login", Method: "POST", UsernameField: "username", PasswordField: "password", SuccessStatuses: []int{200}, SuccessBodyContains: `"success":true`, BodyPrefixBytes: 1024}}
	})
	proxyCall(t, f, "GET", "/challenge", "", nil)
	if len(f.sink.snapshot()) != 0 {
		t.Fatal("initial anonymous 401 counted")
	}
	for i := 0; i < 20; i++ {
		proxyCall(t, f, "GET", "/ok", "", nil)
	}
	for i := 0; i < 5; i++ {
		proxyCall(t, f, "GET", "/error", "", nil)
	}
	eligible := 0
	for _, e := range f.sink.snapshot() {
		if e.Category == "response_5xx" && !e.Observe {
			eligible++
		}
	}
	if eligible > 2 {
		t.Fatalf("outage could trigger five personal failures: %d", eligible)
	}
	proxyCall(t, f, "POST", "/login", url.Values{"username": {"family"}, "password": {"admin"}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	events := f.sink.snapshot()
	counts := map[string]int{}
	for _, e := range events {
		counts[e.Category]++
		if strings.Contains(e.Evidence, "admin") || strings.Contains(e.Evidence, "family") || strings.Contains(e.Evidence, "success") {
			t.Fatalf("response or credentials leaked: %s", e.Evidence)
		}
	}
	if counts["auth_fail"] != 1 || counts["weak_pwd"] != 1 {
		t.Fatalf("missing login signals %v", counts)
	}
}

type repeatingReader struct {
	value byte
	block []byte
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	if r.block == nil {
		r.block = bytes.Repeat([]byte{r.value}, 32<<10)
	}
	for n := 0; n < len(p); {
		n += copy(p[n:], r.block)
	}
	return len(p), nil
}

type checkingWriter struct {
	value byte
	count int64
	bad   bool
}

func (w *checkingWriter) Write(p []byte) (int, error) {
	if bytes.Count(p, []byte{w.value}) != len(p) {
		w.bad = true
	}
	w.count += int64(len(p))
	return len(p), nil
}
func TestProxyOneGiBUploadDownloadBoundedHeap(t *testing.T) {
	if testing.Short() {
		t.Skip("one-GiB stream integration")
	}
	const size int64 = 1 << 30
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			_, _ = io.CopyN(w, &repeatingReader{value: 'y'}, size)
			return
		}
		check := &checkingWriter{value: 'x'}
		_, err := io.Copy(check, r.Body)
		if err != nil || check.bad || check.count != size {
			http.Error(w, "bad stream", 400)
			return
		}
		fmt.Fprint(w, check.count)
	}, false)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for old := peak.Load(); m.HeapAlloc > old; old = peak.Load() {
					if peak.CompareAndSwap(old, m.HeapAlloc) {
						break
					}
				}
			}
		}
	}()
	defer func() { close(done); <-stopped }()
	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("POST", f.front.URL+"/upload", io.NopCloser(io.LimitReader(&repeatingReader{value: 'x'}, size)))
	req.ContentLength = size
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || string(body) != strconv.FormatInt(size, 10) {
		t.Fatalf("upload: status=%d body=%s err=%v", response.StatusCode, body, err)
	}
	response, err = client.Get(f.front.URL + "/download")
	if err != nil {
		t.Fatal(err)
	}
	check := &checkingWriter{value: 'y'}
	_, err = io.Copy(check, response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || check.bad || check.count != size {
		t.Fatalf("download: status=%d bytes=%d err=%v", response.StatusCode, check.count, err)
	}
	growth := peak.Load() - before.HeapAlloc
	t.Logf("streamed upload=%d download=%d sampled_heap_growth=%d", size, size, growth)
	if growth > 96<<20 {
		t.Fatalf("stream buffered excessive heap: %d", growth)
	}
}
