package wafpipeline

import (
	"HomeWaf/wafbot"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type safeSink struct {
	mu     sync.Mutex
	events []HitEvent
}

func (s *safeSink) Submit(e HitEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return true
}
func (s *safeSink) snapshot() []HitEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]HitEvent(nil), s.events...)
}

type gateFixture struct {
	snap    *Snapshot
	runtime *Runtime
	gate    *Gate
	rules   *RulesA
	server  *httptest.Server
	hits    atomic.Int64
	headers chan http.Header
	sink    *safeSink
}

func newGateFixture(t *testing.T, store *GateStore, mode, password string) *gateFixture {
	t.Helper()
	f := &gateFixture{headers: make(chan http.Header, 256), sink: &safeSink{}}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.headers <- r.Header.Clone()
		w.WriteHeader(200)
		_, _ = w.Write([]byte("upstream"))
	}))
	t.Cleanup(up.Close)
	target, _ := url.Parse(up.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	f.gate = NewGate(store)
	f.rules = NewRulesA(f.gate)
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.runtime.Serve(f.snap, w, r) }))
	_, port, _ := net.SplitHostPort(f.server.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	f.snap = &Snapshot{Config: PortConfig{Schema: 1, Port: uint16(n), Protocol: "http", Upstream: up.URL, Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: randomToken()}}
	f.snap.Config.Policy.Gate.Enabled = true
	f.snap.Config.Policy.Gate.Mode = mode
	f.runtime = &Runtime{RuleSteps: f.rules.Steps, Respond: f.gate.Respond, Sink: f.sink, Forward: func(s *Snapshot, c *RequestContext, w http.ResponseWriter) { proxy.ServeHTTP(w, c.Request) }}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	f.gate.Store.ReplaceAccounts(PortID(uint16(n)), []GateAccount{{PortID: PortID(uint16(n)), Username: "owner", PasswordHash: string(hash), Version: "v1", Enabled: true}})
	if err = f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	f.server.Start()
	t.Cleanup(f.server.Close)
	return f
}
func gateRequest(t *testing.T, f *gateFixture, method, p string, body url.Values, headers map[string]string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		rd = strings.NewReader(body.Encode())
	}
	req, err := http.NewRequest(method, f.server.URL+p, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(bytes)
}
func csrfCookie(t *testing.T, f *gateFixture) *http.Cookie {
	t.Helper()
	r, b := gateRequest(t, f, "GET", f.snap.Config.Policy.Gate.Prefix+"/login", nil, nil)
	if r.StatusCode != 200 || strings.Contains(b, f.snap.Config.Upstream) {
		t.Fatalf("login page: %d %s", r.StatusCode, b)
	}
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "hw_csrf_") {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatal("unsafe csrf cookie")
			}
			return c
		}
	}
	t.Fatal("missing CSRF cookie")
	return nil
}
func loginGate(t *testing.T, f *gateFixture, user, password string) (*http.Response, string) {
	t.Helper()
	csrf := csrfCookie(t, f)
	return gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"csrf": {csrf.Value}, "username": {user}, "password": {password}, "next": {"/files?name=中文"}}, nil, csrf)
}
func sessionCookie(t *testing.T, f *gateFixture) *http.Cookie {
	t.Helper()
	r, b := loginGate(t, f, "owner", "long-safe-pass-42")
	location, parseErr := url.Parse(r.Header.Get("Location"))
	if r.StatusCode != 303 || parseErr != nil || location.Path != "/files" || location.Query().Get("name") != "中文" {
		t.Fatalf("login %d location=%s body=%s", r.StatusCode, r.Header.Get("Location"), b)
	}
	for _, c := range r.Cookies() {
		if c.Name == gateCookieName(f.snap.Config.Port) {
			return c
		}
	}
	t.Fatal("no session")
	return nil
}
func TestGateHTTPIsolationCSRFAndCredentialStripping(t *testing.T) {
	store := NewGateStore()
	a := newGateFixture(t, store, "form", "long-safe-pass-42")
	b := newGateFixture(t, store, "form", "other-port-pass-42")
	for _, headers := range []map[string]string{{}, {"Upgrade": "websocket", "Connection": "Upgrade", "Accept": "text/html"}, {"Accept": "text/event-stream"}, {"Accept": "text/html", "X-Requested-With": "XMLHttpRequest"}} {
		r, _ := gateRequest(t, a, "GET", "/private", nil, headers)
		if r.StatusCode != 401 {
			t.Fatalf("API/WS/SSE status %d", r.StatusCode)
		}
	}
	r, _ := gateRequest(t, a, "GET", "/private?q=1", nil, map[string]string{"Accept": "text/html"})
	if r.StatusCode != 302 || !strings.Contains(r.Header.Get("Location"), "next=") {
		t.Fatal("navigation redirect missing")
	}
	r, _ = gateRequest(t, a, "POST", a.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"username": {"owner"}, "password": {"long-safe-pass-42"}}, nil)
	if r.StatusCode != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if a.hits.Load() != 0 || b.hits.Load() != 0 {
		t.Fatal("unauthenticated request reached upstream")
	}
	cookie := sessionCookie(t, a)
	r, _ = gateRequest(t, a, "GET", "/private", nil, map[string]string{"Authorization": "Bearer business-token", "X-HomeWaf-Token": "secret"}, cookie, &http.Cookie{Name: "business", Value: "keep"}, &http.Cookie{Name: gateCookieName(b.snap.Config.Port), Value: "other"})
	if r.StatusCode != 200 || a.hits.Load() != 1 {
		t.Fatal("valid session did not reach upstream")
	}
	h := <-a.headers
	if h.Get("Cookie") != "business=keep" || h.Get("Authorization") != "Bearer business-token" || h.Get("X-HomeWaf-Token") != "" {
		t.Fatalf("credential stripping: %+v", h)
	}
	cross := *cookie
	cross.Name = gateCookieName(b.snap.Config.Port)
	r, _ = gateRequest(t, b, "GET", "/private", nil, nil, &cross)
	if r.StatusCode != 401 || b.hits.Load() != 0 {
		t.Fatal("cross-port token accepted")
	}
	parts := strings.Split(cookie.Value, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var claims gateClaims
	_ = json.Unmarshal(raw, &claims)
	claims.Port = PortID(b.snap.Config.Port)
	raw, _ = json.Marshal(claims)
	cross.Value = base64.RawURLEncoding.EncodeToString(raw) + "." + parts[1]
	r, _ = gateRequest(t, b, "GET", "/private", nil, nil, &cross)
	if r.StatusCode != 401 || b.hits.Load() != 0 {
		t.Fatal("tampered port accepted")
	}
	u, _ := store.Account(PortID(a.snap.Config.Port), "owner")
	u.Version = "v2"
	store.ReplaceAccounts(u.PortID, []GateAccount{u})
	r, _ = gateRequest(t, a, "GET", "/private", nil, nil, cookie)
	if r.StatusCode != 401 || a.hits.Load() != 1 {
		t.Fatal("revoked session accepted")
	}
}
func TestGateFailuresRateEventsTrustedAndBasic(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	r1, b1 := loginGate(t, f, "owner", "wrong-password")
	r2, b2 := loginGate(t, f, "missing", "wrong-password")
	if r1.StatusCode != 401 || r2.StatusCode != 401 || b1 != b2 {
		t.Fatal("account enumeration in failure response")
	}
	_, _ = loginGate(t, f, "owner", "admin")
	events := f.sink.snapshot()
	cats := map[string]int{}
	for _, e := range events {
		cats[e.Category]++
		if strings.Contains(e.Evidence, "wrong-password") || strings.Contains(e.Evidence, "admin") {
			t.Fatal("password in event")
		}
	}
	if cats["auth_fail"] != 3 || cats["auth_unknown_user"] != 1 || cats["weak_pwd"] != 1 {
		t.Fatalf("missing response-stage events: %v", cats)
	}
	f.snap.Config.Policy.Gate.LoginPerMinute = 3
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	r, _ := loginGate(t, f, "owner", "long-safe-pass-42")
	if r.StatusCode != 429 || f.hits.Load() != 0 {
		t.Fatal("strict login limit bypass")
	}
	f.snap.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "loopback", Type: "allow", Trust: "trusted", Value: "127.0.0.1"}}
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	r, _ = gateRequest(t, f, "GET", "/private", nil, nil)
	if r.StatusCode != 200 {
		t.Fatal("trusted not bypassed")
	}
	f.snap.Config.Policy.Gate.BypassTrusted = false
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	r, _ = gateRequest(t, f, "GET", "/private", nil, nil)
	if r.StatusCode != 401 || f.hits.Load() != 1 {
		t.Fatal("required trusted gate bypassed")
	}
	basic := newGateFixture(t, nil, "basic", "long-safe-pass-42")
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("owner:long-safe-pass-42"))
	r, _ = gateRequest(t, basic, "GET", "/private", nil, map[string]string{"Authorization": auth})
	if r.StatusCode != 200 || basic.hits.Load() != 1 {
		t.Fatal("basic failed")
	}
	if h := <-basic.headers; h.Get("Authorization") != "" {
		t.Fatal("basic credentials reached upstream")
	}
}
func TestGateDryRunExpiryNextAndSessions(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	cookie := sessionCookie(t, f)
	before := f.gate.Limits.ActiveKeys()
	sessions := len(f.gate.Store.Sessions(""))
	r := httptest.NewRequest("POST", "http://preview/private", nil)
	r.SetBasicAuth("owner", "long-safe-pass-42")
	_, err := f.runtime.Explain(f.snap, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil {
		t.Fatal(err)
	}
	if f.gate.Limits.ActiveKeys() != before || len(f.gate.Store.Sessions("")) != sessions {
		t.Fatal("dryrun mutated auth")
	}
	for _, next := range []string{"https://evil.test/", "//evil.test/", "/%2f%2fevil.test/", "/\\evil.test/", "/%5cevil.test/", "/%0d%0aevil"} {
		if got := SafeGateNext(next); got != "/" {
			t.Fatalf("unsafe next accepted %q", next)
		}
	}
	if SafeGateNext("/files?next=%2Fphotos") != "/files?next=%2Fphotos" {
		t.Fatal("valid relative next rejected")
	}
	f.gate.Store.mu.Lock()
	for k, v := range f.gate.Store.sessions {
		v.LastSeen = time.Now().Add(-3 * time.Hour).Unix()
		f.gate.Store.sessions[k] = v
	}
	f.gate.Store.mu.Unlock()
	rsp, _ := gateRequest(t, f, "GET", "/private", nil, nil, cookie)
	if rsp.StatusCode != 401 || f.hits.Load() != 0 {
		t.Fatal("idle expiry ignored")
	}
}
func TestRatesObserveDryRunBrowserBurstAndIsolation(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	f.snap.Config.Policy.Gate.Enabled = false
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		r, _ := gateRequest(t, f, "GET", "/app.js", nil, nil)
		if r.StatusCode != 200 {
			t.Fatalf("normal burst blocked at %d", i)
		}
	}
	f.snap.Config.Policy.RuleSet.QPS.Mode = ModeOff
	f.snap.Config.Policy.RuleSet.CC.Threshold = 2
	f.snap.Config.Policy.RuleSet.CC.Action = Deny
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		r, _ := gateRequest(t, f, "GET", "/dynamic", nil, nil)
		if r.StatusCode != 200 {
			t.Fatal("observe blocked")
		}
	}
	for _, e := range f.sink.snapshot() {
		if e.Category == "cc" && !e.Observe {
			t.Fatal("observe became punitive")
		}
	}
	f.snap.Config.Policy.RuleSet.CC.Mode = ModeBlock
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	before := f.hits.Load()
	r, _ := gateRequest(t, f, "GET", "/dynamic", nil, nil)
	if r.StatusCode != 429 || f.hits.Load() != before {
		t.Fatal("block reached upstream")
	}
	a := f.rules.Counters.ActiveKeys()
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "http://preview/unseen", nil)
		_, err := f.runtime.Explain(f.snap, req, netip.MustParseAddr("198.51.100.1"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if a != f.rules.Counters.ActiveKeys() {
		t.Fatal("dryrun allocated keys")
	}
	if !f.rules.Rates.Allow("port:other", "same", 1, 1, false) {
		t.Fatal("other port affected")
	}
}
func TestBOTDNSCacheRequiresForwardConfirmationAndNoDryRunNetwork(t *testing.T) {
	b := NewBOTCache()
	var calls atomic.Int64
	b.Resolve = func(ua, ip string) wafbot.BotResult {
		calls.Add(1)
		return wafbot.BotResult{IsBot: true, IsNormalBot: ip == "192.0.2.1", StrongVerified: ip == "192.0.2.1"}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	b.Lookup("a", "googlebot", "192.0.2.1", true)
	time.Sleep(time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("dryrun made DNS query")
	}
	b.Lookup("a", "googlebot", "192.0.2.1", false)
	b.Lookup("a", "googlebot", "192.0.2.2", false)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		good, _ := b.Lookup("a", "googlebot", "192.0.2.1", true)
		_, bad := b.Lookup("a", "googlebot", "192.0.2.2", true)
		if good && bad {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("DNS identity did not publish")
}

func TestGateChallengeLoginCSRFLogoutAndToken(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	f.snap.Config.Policy.Gate.Enabled = false
	f.snap.Config.Policy.RuleSet.BOT = ModeBlock
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Accept": "text/html", "User-Agent": "HeadlessChrome fixture"}
	r, b := gateRequest(t, f, "GET", "/files", nil, headers)
	if r.StatusCode != 429 || f.hits.Load() != 0 || !strings.Contains(b, "继续访问") {
		t.Fatal("missing challenge")
	}
	var pending *http.Cookie
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "hw_challenge_pending_") {
			pending = c
		}
	}
	if pending == nil {
		t.Fatal("missing pending challenge")
	}
	r, _ = gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/challenge", url.Values{"token": {pending.Value}, "next": {"/files"}}, headers, pending)
	if r.StatusCode != 303 {
		t.Fatalf("challenge response %d", r.StatusCode)
	}
	var pass *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name == "hw_challenge_"+strconv.Itoa(int(f.snap.Config.Port)) {
			pass = c
		}
	}
	if pass == nil {
		t.Fatal("missing proof cookie")
	}
	r, _ = gateRequest(t, f, "GET", "/files", nil, headers, pass)
	if r.StatusCode != 200 || f.hits.Load() != 1 {
		t.Fatal("challenge proof rejected")
	}
	if h := <-f.headers; h.Get("Cookie") != "" {
		t.Fatal("challenge cookie leaked")
	}
	f.snap.Config.Policy.Gate.Enabled = true
	f.snap.Config.Policy.Gate.TokenHeader = "X-HomeWaf-Token"
	token := randomToken()
	f.snap.Secrets.ServiceTokenHashes = []string{tokenHash(token)}
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	r, _ = gateRequest(t, f, "GET", "/files", nil, map[string]string{"X-HomeWaf-Token": token})
	if r.StatusCode != 200 || f.hits.Load() != 2 {
		t.Fatal("service token not accepted")
	}
	if h := <-f.headers; h.Get("X-HomeWaf-Token") != "" {
		t.Fatal("service token leaked")
	}
	csrf := csrfCookie(t, f)
	r, _ = gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"csrf": {csrf.Value}, "username": {"owner"}, "password": {"long-safe-pass-42"}}, map[string]string{"Origin": "https://evil.test"}, csrf)
	if r.StatusCode != 403 {
		t.Fatal("cross-origin login allowed")
	}
	cookie := sessionCookie(t, f)
	csrf = csrfCookie(t, f)
	r, _ = gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/logout", url.Values{"csrf": {csrf.Value}}, nil, cookie, csrf)
	if r.StatusCode != 303 {
		t.Fatal("logout failed")
	}
	r, _ = gateRequest(t, f, "GET", "/files", nil, nil, cookie)
	if r.StatusCode != 401 || f.hits.Load() != 2 {
		t.Fatal("logout token still valid")
	}
}

func BenchmarkRulesARequest(b *testing.B) {
	g := NewGate(nil)
	a := NewRulesA(g)
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: strings.Repeat("k", 32)}}
	s.Config.Policy.RuleSet.QPS.PerIP = 100000
	s.Config.Policy.RuleSet.QPS.PerPort = 1000000
	s.Config.Policy.RuleSet.QPS.Mode = ModeObserve
	s.Config.Policy.RuleSet.CC.Mode = ModeOff
	runtime := &Runtime{RuleSteps: a.Steps}
	if err := runtime.Prepare(s); err != nil {
		b.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://example.test/photos", nil)
	ip := netip.MustParseAddr("192.0.2.1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewRequestContext(req, PortID(5000), ip, DefaultViewLimits())
		_ = s.Compiled.Pipeline.Evaluate(c)
	}
}
