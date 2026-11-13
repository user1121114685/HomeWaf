package wafpipeline

import (
	"HomeWaf/internal/proxyidentity"
	"HomeWaf/model"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafproxy"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type healthBucket struct {
	second        int64
	total, failed uint64
}
type UpstreamHealth struct {
	mu                  sync.Mutex
	origin              time.Time
	buckets             [60]healthBucket
	consecutiveFailures uint64
	lastFailure         time.Time
}

func (h *UpstreamHealth) Record(status int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.origin.IsZero() {
		h.origin = time.Now()
	}
	second := int64(time.Since(h.origin) / time.Second)
	b := &h.buckets[second%60]
	if b.second != second {
		*b = healthBucket{second: second}
	}
	b.total++
	if status >= 500 {
		b.failed++
		h.consecutiveFailures++
		h.lastFailure = time.Now()
	} else {
		h.consecutiveFailures = 0
	}
}
func (h *UpstreamHealth) Snapshot() (total, failed uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.origin.IsZero() {
		return
	}
	second := int64(time.Since(h.origin) / time.Second)
	for _, b := range h.buckets {
		if second-b.second < 60 {
			total += b.total
			failed += b.failed
		}
	}
	return
}
func (h *UpstreamHealth) CanAttributeFailure() bool {
	total, failed := h.Snapshot()
	h.mu.Lock()
	consecutive := h.consecutiveFailures
	h.mu.Unlock()
	return total >= 20 && failed*10 < total*3 && consecutive < 3
}

type ProxyTarget struct {
	configRoute     *UpstreamRoute
	proxy           *wafproxy.ReverseProxy
	transport       *http.Transport
	Health          UpstreamHealth
	Errors          atomic.Uint64
	routes          []preparedBusinessRoute
	sharedHealth    *UpstreamHealth
	handler         http.Handler
	responseHeaders func(*http.Request, string) map[string]string
	balance         *businessBalance
	cache           *preparedBusinessCache
}

type preparedBusinessRoute struct {
	hosts      []string
	prefix     string
	target     *ProxyTarget
	match      string
	priority   int
	expression *regexp.Regexp
}
type Forwarder struct {
	CaptureTamper   func(*Snapshot, TamperRule, []byte, string, string) bool
	DecodeResponse  func(*http.Response, string) ([]byte, string, error)
	EncodeResponse  func(*http.Request, *http.Response, []byte, string, model.ResponseCompressConfig) ([]byte, error)
	SecureCookie    func(string, bool, model.CookieSecurityConfig) string
	StaticHandler   func(StaticBusiness) (http.Handler, error)
	RequestHeaders  func([]model.CustomHeaderItem, *http.Request, string) map[string]string
	ResponseHeaders func([]model.CustomResponseHeaderRule, *http.Request, string) map[string]string
}
type forwardingContextKey struct{}
type forwardRequest struct {
	snapshot *Snapshot
	request  *RequestContext
	response *forwardResponse
}
type forwardResponse struct {
	status                          int
	transportError, policyDenied    bool
	location, encoding, contentType string
	sample                          *responsePrefix
	login                           *LoginObservation
	weak                            bool
	credential                      bool
	cacheHit                        bool
	responseDenied                  bool
	userHash                        string
}
type responsePrefix struct {
	io.ReadCloser
	mu           sync.Mutex
	data         []byte
	limit, total int
	complete     bool
}

func (p *responsePrefix) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	p.mu.Lock()
	p.total += n
	if left := p.limit - len(p.data); left > 0 {
		p.data = append(p.data, b[:min(n, left)]...)
	}
	if err == io.EOF {
		p.complete = true
	}
	p.mu.Unlock()
	return n, err
}
func (p *responsePrefix) snapshot() ([]byte, bool, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.data...), p.total > p.limit, p.complete
}

type proxyBufferPool struct{ pool sync.Pool }

func (p *proxyBufferPool) Get() []byte {
	if b, ok := p.pool.Get().([]byte); ok {
		return b
	}
	return make([]byte, 32<<10)
}
func (p *proxyBufferPool) Put(b []byte) {
	if cap(b) == 32<<10 {
		p.pool.Put(b[:32<<10])
	}
}

var forwardingBuffers proxyBufferPool

func positive(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}
func (f *Forwarder) Prepare(s *Snapshot) error {
	if s.Preview {
		return nil
	}
	if err := ValidateBusinessHeaders(s.Secrets.BusinessHeaders); err != nil {
		return err
	}
	if err := ValidateTamperBaselines(s.Secrets.TamperBaselines); err != nil {
		return err
	}
	response := s.Config.Policy.Network.Response
	if response.Cookie.IsEnable == 1 && f.SecureCookie == nil {
		return errors.New("响应Cookie处理器未就绪")
	}
	sensitive := s.Compiled.Sensitive
	if (s.Compiled.Privacy != nil || len(response.Tamper) > 0 || response.Compression.IsEnable == 1 || sensitive != nil && sensitive.mode != ModeOff && (sensitive.out.deny != nil || sensitive.out.replace != nil)) && (f.DecodeResponse == nil || f.EncodeResponse == nil) {
		return errors.New("响应内容处理器未就绪")
	}
	if old, ok := Ports.Get(s.Config.Port); ok && old.Compiled != nil && old.Compiled.Proxy != nil && old.Config.Upstream == s.Config.Upstream && old.Config.UpstreamDialIP == s.Config.UpstreamDialIP && old.Config.HeaderProfile == s.Config.HeaderProfile && reflect.DeepEqual(old.Config.Routes, s.Config.Routes) && reflect.DeepEqual(old.Secrets.BusinessHeaders, s.Secrets.BusinessHeaders) && old.Secrets.UpstreamCAPEM == s.Secrets.UpstreamCAPEM && reflect.DeepEqual(old.Config.Policy.Network, s.Config.Policy.Network) {
		s.Compiled.Proxy = old.Compiled.Proxy
		return nil
	}
	root, err := f.prepareTarget(s, s.Config.Upstream, nil)
	if err != nil {
		return err
	}
	if err = f.attachHeaders(s, root, s.Config.HeaderProfile); err != nil {
		root.closeIdle()
		return err
	}
	overrides, err := upstreamDialOverrides(s.Config)
	if err != nil {
		root.closeIdle()
		return err
	}
	originalDial := root.transport.DialContext
	idleSlots := 31
	root.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if ip := overrides[strings.ToLower(address)]; ip != "" {
			_, port, e := net.SplitHostPort(address)
			if e != nil {
				return nil, e
			}
			address = net.JoinHostPort(ip, port)
		}
		return originalDial(ctx, network, address)
	}
	for _, route := range s.Config.Routes {
		upstream := route.Upstream
		if route.TargetType != "" && route.TargetType != "proxy" {
			upstream = ""
		}
		target, err := f.prepareTarget(s, upstream, root.transport)
		if err != nil {
			root.closeIdle()
			return err
		}
		target.sharedHealth = &root.Health
		if route.TargetType == "static" {
			if f.StaticHandler == nil || route.Static == nil {
				root.closeIdle()
				return errors.New("静态业务处理器未就绪")
			}
			target.handler, err = f.StaticHandler(*route.Static)
			if err != nil {
				root.closeIdle()
				return err
			}
		} else if route.TargetType == "unavailable" {
			target.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "Service unavailable", 503) })
		} else if route.TargetType == "https_redirect" {
			port, code := route.RedirectPort, route.RedirectCode
			if code == 0 {
				code = 301
			}
			target.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				location := httpsRedirectLocation(r, port)
				if location == "" {
					http.Error(w, "Invalid authority", 400)
					return
				}
				http.Redirect(w, r, location, code)
			})
		} else if route.TargetType == "redirect" {
			code := route.RedirectCode
			if code == 0 {
				code = 302
			}
			destination := route.RedirectURL
			target.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination, code) })
		} else if route.StripPrefix {
			director := target.proxy.Director
			prefix := route.PathPrefix
			target.proxy.Director = func(r *http.Request) {
				u := *r.URL
				u.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(u.Path, prefix), "/")
				u.RawPath = ""
				r.URL = &u
				director(r)
			}
		}
		if err = f.attachHeaders(s, target, route.HeaderProfile); err != nil {
			root.closeIdle()
			return err
		}
		if err = f.prepareBalance(s, root, target, route, originalDial, &idleSlots); err != nil {
			root.closeIdle()
			return err
		}
		prepareBusinessCache(s, target, route)
		hosts := make([]string, len(route.Hosts))
		for i, host := range route.Hosts {
			hosts[i] = strings.TrimSuffix(strings.ToLower(host), ".")
		}
		prepared := preparedBusinessRoute{hosts: hosts, prefix: route.PathPrefix, target: target, match: route.MatchType, priority: route.Priority}
		if route.MatchType == "regex" {
			prepared.expression, err = regexp.Compile(route.PathPrefix)
			if err != nil {
				root.closeIdle()
				return err
			}
		}
		root.routes = append(root.routes, prepared)
	}
	root.transport.MaxIdleConns = idleSlots + 1
	s.Compiled.Proxy = root
	return nil
}

func (f *Forwarder) prepareTarget(s *Snapshot, upstream string, shared *http.Transport) (*ProxyTarget, error) {
	u, err := url.Parse(upstream)
	if upstream != "" && (err != nil || u.Hostname() == "") {
		return nil, errors.New("invalid upstream")
	}
	n := s.Config.Policy.Network
	transport := shared
	if transport == nil {
		transport = &http.Transport{DialContext: (&net.Dialer{Timeout: time.Duration(positive(n.ConnectSeconds, 5)) * time.Second, KeepAlive: 30 * time.Second}).DialContext, ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: time.Duration(positive(n.ConnectSeconds, 5)) * time.Second, ResponseHeaderTimeout: time.Duration(positive(n.ResponseHeaderSeconds, 30)) * time.Second, MaxResponseHeaderBytes: int64(n.MaxHeaderBytes), MaxIdleConns: 32, MaxIdleConnsPerHost: 16, MaxConnsPerHost: positive(n.MaxInFlight, 32), IdleConnTimeout: 90 * time.Second, ExpectContinueTimeout: time.Second, DisableCompression: true}
	}
	p := &ProxyTarget{transport: transport}
	if shared == nil && s.Secrets.UpstreamCAPEM != "" {
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(s.Secrets.UpstreamCAPEM)) {
			return nil, errors.New("上游CA证书不合法")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	if upstream == "" {
		p.handler = http.NotFoundHandler()
		return p, nil
	}
	proxy := wafproxy.NewSingleHostReverseProxy(u)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		if !n.PreserveHost {
			r.Host = u.Host
		}
		r.Header.Del("X-Waf-Custom-XFF-Control")
	}
	proxy.Transport = forwardTransport{base: transport}
	proxy.BufferPool = &forwardingBuffers
	proxy.ErrorLog = log.New(io.Discard, "", 0)
	proxy.ModifyResponse = func(response *http.Response) error {
		state, ok := response.Request.Context().Value(forwardingContextKey{}).(*forwardRequest)
		if !ok {
			return errors.New("missing forwarding context")
		}
		record := state.response
		record.status = response.StatusCode
		record.location = response.Header.Get("Location")
		record.encoding = response.Header.Get("Content-Encoding")
		record.contentType = response.Header.Get("Content-Type")
		p.Health.Record(response.StatusCode)
		if p.sharedHealth != nil {
			p.sharedHealth.Record(response.StatusCode)
		}
		if record.login != nil && record.login.SuccessBodyContains != "" && response.StatusCode != 101 && !strings.HasPrefix(strings.ToLower(record.contentType), "text/event-stream") {
			record.sample = &responsePrefix{ReadCloser: response.Body, limit: record.login.BodyPrefixBytes}
			response.Body = record.sample
		}
		if p.responseHeaders != nil {
			applyBusinessHeaders(response.Header, p.responseHeaders(state.request.Request, state.request.ClientIP.String()))
		}
		f.transformResponse(state, response)
		if p.cache != nil {
			p.cache.capture(response, state)
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		state, ok := r.Context().Value(forwardingContextKey{}).(*forwardRequest)
		if !ok {
			http.Error(w, "Bad Gateway", 502)
			return
		}
		record := state.response
		if errors.Is(err, context.Canceled) {
			record.status = 499
			record.transportError = true
			return
		}
		status := 502
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = 413
			record.policyDenied = true
		} else {
			record.transportError = true
			p.Errors.Add(1)
			p.Health.Record(status)
			if p.sharedHealth != nil {
				p.sharedHealth.Record(status)
			}
		}
		record.status = status
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status), "request_id": state.request.ReqID})
	}
	p.proxy = proxy
	return p, nil
}

func (p *ProxyTarget) closeIdle() {
	seen := map[*http.Transport]bool{}
	var closeTarget func(*ProxyTarget)
	closeTarget = func(target *ProxyTarget) {
		if target == nil {
			return
		}
		if target.transport != nil && !seen[target.transport] {
			target.transport.CloseIdleConnections()
			seen[target.transport] = true
		}
		for _, route := range target.routes {
			closeTarget(route.target)
		}
		if target.balance != nil {
			for _, backend := range target.balance.targets {
				closeTarget(backend)
			}
		}
	}
	closeTarget(p)
}

func (p *ProxyTarget) forRequest(r *http.Request) *ProxyTarget {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	best, score := p, -1
	var winner *preparedBusinessRoute
	for i := range p.routes {
		route := &p.routes[i]
		matches := strings.HasPrefix(r.URL.Path, route.prefix)
		if route.match == "exact" {
			matches = r.URL.Path == route.prefix
		} else if route.expression != nil {
			matches = route.expression.MatchString(r.URL.Path)
		}
		if !matches {
			continue
		}
		for _, match := range route.hosts {
			specificity := -1
			switch {
			case match == host:
				specificity = 10000 + len(match)
			case match == "*":
				specificity = 0
			case strings.HasPrefix(match, "*.") && strings.HasSuffix(host, match[1:]) && len(host) > len(match)-1:
				specificity = len(match)
			}
			if specificity < 0 {
				continue
			}
			if specificity > score || specificity == score && betterBusinessRoute(*route, winner) {
				winner = route
				best, score = route.target, specificity
			}
		}
	}
	return best
}
func betterBusinessRoute(candidate preparedBusinessRoute, current *preparedBusinessRoute) bool {
	if current == nil {
		return true
	}
	if (candidate.prefix == "") != (current.prefix == "") {
		return candidate.prefix != ""
	}
	if candidate.priority != current.priority {
		return candidate.priority < current.priority
	}
	rank := func(kind string) int {
		if kind == "exact" {
			return 0
		}
		if kind == "regex" {
			return 2
		}
		return 1
	}
	if rank(candidate.match) != rank(current.match) {
		return rank(candidate.match) < rank(current.match)
	}
	return len(candidate.prefix) > len(current.prefix)
}
func CloseRetiredProxy(old, next *Snapshot) {
	if old == nil || old.Compiled == nil {
		return
	}
	if old.Compiled.Plugins != nil && (next == nil || !next.Config.ListenEnabled || next.Compiled == nil || next.Compiled.Plugins != old.Compiled.Plugins) {
		old.Compiled.Plugins.Close()
	}
	if old.Compiled.Proxy == nil {
		return
	}
	if next == nil || !next.Config.ListenEnabled || next.Compiled == nil || next.Compiled.Proxy != old.Compiled.Proxy {
		old.Compiled.Proxy.closeIdle()
	}
}

type forwardTransport struct{ base http.RoundTripper }

func (t forwardTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	state, ok := r.Context().Value(forwardingContextKey{}).(*forwardRequest)
	if !ok {
		return nil, errors.New("missing forwarding context")
	}
	c, s := state.request, state.snapshot
	// This runs after hop-by-hop removal, so an attacker cannot nominate the
	// trusted identity headers in Connection to remove or replace them.
	proxyidentity.Strip(r.Header, s.Config.Policy.Network.RealIPHeader)
	r.Header.Set("X-Forwarded-For", c.ClientIP.String())
	r.Header.Set("X-Real-IP", c.ClientIP.String())
	r.Header.Del("waf_req_hop")
	if s.Config.Policy.Network.MaxProxyHops > 0 {
		r.Header.Set("waf_req_hop", strconv.Itoa(requestProxyHop(c.Request)+1))
	}
	r.Header.Set("X-Forwarded-Proto", forwardedScheme(s, c.Request))
	r.Header.Set("X-Forwarded-Host", c.Request.Host)
	r.Header.Set("X-Forwarded-Port", strconv.Itoa(int(s.Config.Port)))
	StripGateCredentials(r, s.Config.Policy.Gate)
	if state.response.login != nil && state.response.login.SuccessBodyContains != "" {
		r.Header.Set("Accept-Encoding", "identity")
	}
	return t.base.RoundTrip(r)
}
func forwardedScheme(s *Snapshot, r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip, e := netip.ParseAddr(host); e == nil && s.Compiled != nil && s.Compiled.Proxies != nil && s.Compiled.Proxies.Contains(ip.Unmap().AsSlice()) {
			if value := r.Header.Get("X-Forwarded-Proto"); value == "http" || value == "https" {
				return value
			}
		}
	}
	return "http"
}
func (f *Forwarder) Serve(s *Snapshot, c *RequestContext, w http.ResponseWriter) {
	if proxyidentity.UnsupportedUpgrade(c.Request.Header) {
		v := Verdict{Action: Deny, Stage: 5, Reason: "unsupported_protocol_upgrade", Status: http.StatusForbidden}
		c.Values["response_verdict"] = v
		c.Trace.Add(TraceEntry{Stage: 5, Check: "protocol_upgrade", Verdict: v})
		c.Hit("malformed", "unsupported_protocol_upgrade", Medium, false, "unsupported protocol upgrade")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if s.Compiled == nil || s.Compiled.Proxy == nil {
		http.Error(w, "Upstream is unavailable", 503)
		return
	}
	target := s.Compiled.Proxy.forRequest(c.Request)
	if target.balance != nil {
		target = target.balance.choose(c.ClientIP.String())
	}
	record := &forwardResponse{credential: c.Request.Header.Get("Authorization") != "" || len(c.Request.Cookies()) > 0}
	for i := range s.Config.Policy.HitPolicy.BusinessLogin {
		login := s.Config.Policy.HitPolicy.BusinessLogin[i]
		if login.Path == c.Request.URL.Path && login.Method == c.Request.Method {
			record.login = &login
			if (login.PasswordField != "" || login.UsernameField != "") && !c.View.bodyReadStarted {
				_ = inspectRequestBody(c)
			}
			for _, field := range c.View.Fields {
				if login.UsernameField != "" && field.Name == login.UsernameField {
					record.userHash = dimension(s.Secrets.GateKey + "\x00business-account\x00" + field.RawValue)
				}
				if field.Name == login.PasswordField && login.PasswordField != "" && accessgate.WeakPassword(field.RawValue) {
					record.weak = true
				}
			}
			break
		}
	}
	started := time.Now()
	defer func() {
		v := Verdict{Action: Allow, Stage: 5, Reason: "upstream_response", Status: record.status}
		if record.cacheHit {
			v.Reason = "business_cache_hit"
		}
		if record.transportError {
			v.Reason = "upstream_error"
		}
		if record.policyDenied {
			v.Action = Deny
			v.Reason = "request_body_limit"
		}
		if record.responseDenied {
			v.Action = Deny
			v.Reason = "response_content_policy"
			v.Status = 403
		}
		c.Values["response_verdict"] = v
		c.Trace.Add(TraceEntry{Stage: 5, Check: "reverse_proxy", Verdict: v, ElapsedMicros: time.Since(started).Microseconds()})
		f.observe(s, c, target, record)
	}()
	maxBytes := s.Config.Policy.Network.MaxRequestBytes
	if maxBytes > 0 {
		if c.Request.ContentLength > maxBytes {
			record.status = 413
			record.policyDenied = true
			http.Error(w, "Request body too large", 413)
			return
		}
		c.Request.Body = http.MaxBytesReader(w, c.Request.Body, maxBytes)
	}
	ctx := context.WithValue(c.Context, forwardingContextKey{}, &forwardRequest{snapshot: s, request: c, response: record})
	if target.cache != nil && !bodyResponsePolicyActive(s, c) && target.cache.serve(w, c.Request, record) {
		return
	}
	if target.handler != nil {
		if target.responseHeaders != nil {
			applyBusinessHeaders(w.Header(), target.responseHeaders(c.Request, c.ClientIP.String()))
		}
		writer := &localResponseWriter{w: w, state: &forwardRequest{snapshot: s, request: c, response: record}, f: f, noBuffer: !bodyResponsePolicyActive(s, c) && s.Config.Policy.Network.Response.Compression.IsEnable != 1}
		target.handler.ServeHTTP(writer, c.Request.WithContext(ctx))
		writer.finish()
		record.status = writer.status
		if record.status == 0 {
			record.status = 200
		}
		record.location = w.Header().Get("Location")
		record.contentType = w.Header().Get("Content-Type")
		target.Health.Record(record.status)
		if target.sharedHealth != nil {
			target.sharedHealth.Record(record.status)
		}
		return
	}
	target.proxy.ServeHTTP(w, c.Request.WithContext(ctx))
}
func responseHit(c *RequestContext, category, rule string, observe bool, status int) {
	before := len(c.Hits)
	c.Hit(category, rule, Low, observe, "upstream response signal")
	if len(c.Hits) > before {
		c.Hits[len(c.Hits)-1].Status = status
	}
}
func (f *Forwarder) observe(s *Snapshot, c *RequestContext, target *ProxyTarget, r *forwardResponse) {
	if c.DryRun || r.status == 0 || r.status == 101 || r.transportError || r.policyDenied {
		return
	}
	policy := s.Config.Policy.HitPolicy
	if r.login != nil && r.status < 500 {
		login := r.login
		success := false
		for _, status := range login.SuccessStatuses {
			if status == r.status {
				success = true
			}
		}
		if success && login.SuccessLocationPrefix != "" {
			success = strings.HasPrefix(r.location, login.SuccessLocationPrefix)
		}
		if success && login.SuccessBodyContains != "" {
			if r.sample == nil {
				return
			}
			raw, truncated, complete := r.sample.snapshot()
			req := &http.Request{Method: "POST", URL: &url.URL{Path: "/response"}, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(raw)))}
			req.Header.Set("Content-Encoding", r.encoding)
			view := NewNormalizedView(req, ViewLimits{BodyBytes: int64(login.BodyPrefixBytes), ExpansionRatio: 32})
			_ = view.ReadBody()
			success = strings.Contains(string(view.Body), login.SuccessBodyContains)
			if !success && (truncated || !complete || view.BodyTruncated || view.ParseError != "") {
				responseHit(c, "response_observation_incomplete", "business_login", true, r.status)
				return
			}
		}
		if success {
			responseHit(c, "business_auth_success", "business_login", true, r.status)
		} else {
			responseHit(c, "auth_fail", "business_login", !policy.ResponseSignals, r.status)
			if r.weak {
				responseHit(c, "weak_pwd", "business_login", !policy.ResponseSignals, r.status)
			}
		}
		if r.userHash != "" && len(c.Hits) > 0 {
			c.Hits[len(c.Hits)-1].Evidence = "business account fingerprint=" + r.userHash
		}
		return
	}
	if accessgate.MatchPathPrefix(c.View.Path, policy.IgnorePaths) || strings.HasSuffix(c.View.Path, ".map") {
		return
	}
	observe := !policy.ResponseSignals
	switch {
	case r.status == 404:
		responseHit(c, "response_404", "response", observe, r.status)
	case r.status == 401:
		if r.credential {
			responseHit(c, "response_auth", "response", observe, r.status)
		}
	case r.status == 403:
		responseHit(c, "response_auth", "response", observe, r.status)
	case r.status >= 500:
		responseHit(c, "response_5xx", "response", observe || !target.Health.CanAttributeFailure(), r.status)
	}
}
