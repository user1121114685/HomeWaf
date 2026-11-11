package wafpipeline

import (
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafenginecore/cccounter"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Gate struct {
	Store  *GateStore
	Limits *cccounter.Counter
}

func NewGate(store *GateStore) *Gate {
	if store == nil {
		store = NewGateStore()
	}
	return &Gate{Store: store, Limits: cccounter.New(200000, 500)}
}
func (g *Gate) Cleanup() { g.Limits.Cleanup(600); g.Store.Cleanup() }

type gateClaims struct {
	Kind    string `json:"k"`
	Port    string `json:"p"`
	User    string `json:"u,omitempty"`
	Nonce   string `json:"n"`
	Version string `json:"v,omitempty"`
	Expiry  int64  `json:"e"`
	Binding string `json:"b,omitempty"`
}

func randomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func tokenHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func validGateTokenHeader(name string) bool {
	n := strings.ToLower(name)
	return len(n) <= 128 && strings.HasPrefix(n, "x-") && permittedBusinessHeader(name) &&
		!strings.HasPrefix(n, "x-forwarded-") && n != "x-real-ip" && !strings.HasPrefix(n, "x-waf-") || n == "x-homewaf-token"
}
func gateSign(key string, c gateClaims) string {
	b, _ := json.Marshal(c)
	payload := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("HomeWaf port gate v1\n" + payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func gateVerify(key, token, port, kind string) (gateClaims, bool) {
	var c gateClaims
	if len(key) < 32 || len(token) > 2048 {
		return c, false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return c, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, false
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("HomeWaf port gate v1\n" + parts[0]))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return c, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Port != port || c.Kind != kind || c.Expiry <= time.Now().Unix() {
		return c, false
	}
	return c, true
}
func gateCookieName(port uint16) string { return "hw_gate_" + strconv.Itoa(int(port)) }
func oneCookie(r *http.Request, name string) string {
	value := ""
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == name {
			value = c.Value
			count++
		}
	}
	if count != 1 {
		return ""
	}
	return value
}
func (g *Gate) session(s *Snapshot, c *RequestContext) (GateSessionRecord, bool) {
	token := oneCookie(c.Request, gateCookieName(s.Config.Port))
	claims, ok := gateVerify(s.Secrets.GateKey, token, c.PortID, "session")
	if !ok {
		return GateSessionRecord{}, false
	}
	v, ok := g.Store.Valid(tokenHash(token), c.PortID, c.ClientIP.String(), s.Config.Policy.Gate, c.DryRun)
	return v, ok && v.Username == claims.User && v.Version == claims.Version && v.ExpiresAt == claims.Expiry && v.ID == claims.Nonce
}
func (g *Gate) Step(s *Snapshot) Step {
	return Step{Number: 3, Name: "auth_gate", FailClosed: true, RunTrusted: true, Inspect: func(c *RequestContext) (Verdict, error) {
		cfg := s.Config.Policy.Gate
		if accessgate.MatchPathPrefix(c.View.Path, []string{cfg.Prefix}) {
			return Verdict{Action: Challenge, Reason: "gate:endpoint", Status: 401}, nil
		}
		protectedScope := len(cfg.Protect.Hosts) > 0 || len(cfg.Protect.Paths) > 0 || len(cfg.Protect.Methods) > 0
		if (!cfg.Enabled && !cfg.RequireToken) || c.Trusted && cfg.BypassTrusted || cfg.AllowOPTIONS && c.Request.Method == http.MethodOptions || protectedScope && !scopeMatches(cfg.Protect, c.Request) || s.Compiled != nil && s.Compiled.GateExemptIPs.Contains(net.IP(c.ClientIP.AsSlice())) || (len(cfg.Exempt.Hosts) > 0 || len(cfg.Exempt.Paths) > 0 || len(cfg.Exempt.Methods) > 0) && scopeMatches(cfg.Exempt, c.Request) {
			return Verdict{Action: Continue, Reason: "gate:exempt"}, nil
		}
		if cfg.TokenHeader != "" {
			tokens := c.Request.Header.Values(cfg.TokenHeader)
			if len(tokens) == 1 && tokens[0] != "" && len(tokens[0]) <= 4096 {
				digest := tokenHash(tokens[0])
				valid := 0
				for _, expected := range s.Secrets.ServiceTokenHashes {
					valid |= subtle.ConstantTimeCompare([]byte(expected), []byte(digest))
				}
				if valid == 1 {
					return Verdict{Action: Continue, Reason: "gate:service_token"}, nil
				}
			}
		}
		if cfg.RequireToken {
			return Verdict{Action: Challenge, Status: 401, Reason: "gate:token_required"}, nil
		}
		if len(s.Secrets.GateKey) < 32 && !c.DryRun {
			return Verdict{}, fmt.Errorf("gate key unavailable")
		}
		if _, ok := g.session(s, c); ok {
			return Verdict{Action: Continue, Reason: "gate:session"}, nil
		}
		if cfg.Mode == "basic" {
			if user, pass, ok := c.Request.BasicAuth(); ok {
				if c.DryRun {
					return Verdict{Action: Challenge, Status: 401, Reason: "gate:credentials_not_tested"}, nil
				}
				u, status := g.authenticate(s, c, user, pass)
				if status != 0 {
					return Verdict{Action: Challenge, Reason: "gate:basic_failed", Status: status}, nil
				}
				token, _, err := g.issue(s, c, u, false)
				if err != nil {
					return Verdict{}, err
				}
				c.Values["gate_cookie"] = token
				return Verdict{Action: Continue, Reason: "gate:basic"}, nil
			}
		}
		if g.Store.Probe(c) {
			c.Hit("gate_probe", "gate_probe", Low, true, "20 distinct unauthenticated paths in one minute")
		}
		return Verdict{Action: Challenge, Status: 401, Reason: "gate:required"}, nil
	}}
}
func (g *Gate) authenticate(s *Snapshot, c *RequestContext, username, password string, otp ...string) (GateAccount, int) {
	if !g.reserveLogin(s, c) || !g.Store.Acquire(c.PortID) {
		return GateAccount{}, 429
	}
	defer g.Store.Release(c.PortID)
	if len(username) > 128 {
		username = username[:128]
	}
	u, found := g.Store.Account(c.PortID, username)
	hash := accessgate.DummyPasswordHash
	if found {
		hash = u.PasswordHash
	}
	// Oversize passwords must not bypass the dummy bcrypt work.
	validLength := len(password) <= 72
	checked := password
	if !validLength {
		checked = "invalid-oversize-password"
	}
	valid := accessgate.VerifyPassword(hash, checked) && validLength
	secret := "JBSWY3DPEHPK3PXP"
	if found && u.TOTPSecret != "" {
		secret = u.TOTPSecret
	}
	code := "000000"
	providedOTP := len(otp) > 0 && len(strings.TrimSpace(otp[0])) == 6
	if providedOTP {
		code = strings.TrimSpace(otp[0])
	}
	validOTP := utils.ValidateOtpCode(code, secret) && providedOTP
	if u.TOTPSecret != "" && !validOTP {
		valid = false
	}
	if !valid || !found || !u.Enabled || u.ExpiresAt > 0 && u.ExpiresAt <= time.Now().Unix() {
		c.Hit("auth_fail", "auth_gate", Low, false, fmt.Sprintf("account=%q; credential verification failed", username))
		if !found {
			c.Hit("auth_unknown_user", "auth_gate", Medium, false, "unknown account")
		}
		if accessgate.WeakPassword(password) {
			c.Hit("weak_pwd", "auth_gate", Low, false, "common weak password attempted")
		}
		n := g.Limits.Incr(c.PortID+":account_delay", dimension(username), 300).Count
		if n > 6 {
			n = 6
		}
		if n < 1 {
			n = 1
		}
		t := time.NewTimer(time.Duration(n) * 25 * time.Millisecond)
		defer t.Stop()
		select {
		case <-c.Context.Done():
		case <-t.C:
		}
		return GateAccount{}, 401
	}
	c.Hit("auth_success", "auth_gate", Low, true, "account="+username)
	return u, 0
}

func (g *Gate) reserveLogin(s *Snapshot, c *RequestContext) bool {
	if _, ok := c.Values["gate_login_reserved"]; ok {
		return true
	}
	limit := g.Limits.Incr(c.PortID+":login", c.ClientIP.String(), 60)
	if limit.Rejected || limit.Degraded || limit.Count > int64(s.Config.Policy.Gate.LoginPerMinute) {
		return false
	}
	c.Values["gate_login_reserved"] = true
	return true
}
func (g *Gate) issue(s *Snapshot, c *RequestContext, u GateAccount, remember bool) (string, int, error) {
	ttl := s.Config.Policy.Gate.AbsoluteSeconds
	age := 0
	if remember && s.Config.Policy.Gate.RememberDays > 0 {
		ttl = s.Config.Policy.Gate.RememberDays * 86400
		age = ttl
	}
	now := time.Now().Unix()
	claims := gateClaims{Kind: "session", Port: c.PortID, User: u.Username, Nonce: randomToken(), Version: u.Version, Expiry: now + int64(ttl)}
	token := gateSign(s.Secrets.GateKey, claims)
	row := GateSessionRecord{ID: claims.Nonce, PortID: c.PortID, Username: u.Username, TokenHash: tokenHash(token), Version: u.Version, ClientIP: c.ClientIP.String(), CreatedAt: now, LastSeen: now, ExpiresAt: claims.Expiry}
	return token, age, g.Store.Add(row, s.Config.Policy.Gate.MaxSessions)
}
func SetGateCookie(w http.ResponseWriter, r *http.Request, name, value, path string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, MaxAge: age, HttpOnly: true, Secure: RequestScheme(r) == "https", SameSite: http.SameSiteLaxMode})
}
func StripGateCredentials(r *http.Request, cfg GateConfig) {
	// Keep the exact syntax and values of unrelated cookies.
	var kept []string
	for _, line := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			name, _, _ := strings.Cut(part, "=")
			lower := strings.ToLower(name)
			legacyCredential := false
			for _, prefix := range cfg.CredentialCookiePrefixes {
				if strings.HasPrefix(name, prefix+"_") {
					legacyCredential = true
					break
				}
			}
			if strings.HasPrefix(lower, "hw_gate_") || strings.HasPrefix(lower, "hw_csrf_") || strings.HasPrefix(lower, "hw_challenge_") || strings.HasPrefix(lower, "samwaf_ac_") || strings.HasPrefix(lower, "samwaf_httpauth_token_") || lower == "samwaf_httpauth_token" || legacyCredential {
				continue
			}
			if part != "" {
				kept = append(kept, part)
			}
		}
	}
	r.Header.Del("Cookie")
	if len(kept) > 0 {
		r.Header.Set("Cookie", strings.Join(kept, "; "))
	}
	if cfg.Enabled && cfg.Mode == "basic" {
		r.Header.Del("Authorization")
	}
	r.Header.Del("X-HomeWaf-Token")
	if cfg.TokenHeader != "" {
		r.Header.Del(cfg.TokenHeader)
	}
	for _, name := range cfg.CredentialHeaders {
		r.Header.Del(name)
	}
}
func SafeGateNext(raw string) string {
	if raw == "" || len(raw) > 4096 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n\x00") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" {
		return "/"
	}
	decoded := u.Path
	for i := 0; i < 3; i++ {
		value, e := url.PathUnescape(decoded)
		if e != nil {
			return "/"
		}
		decoded = value
	}
	if strings.HasPrefix(decoded, "//") || strings.ContainsAny(decoded, "\\\r\n\x00") {
		return "/"
	}
	return raw
}
func browserNavigation(r *http.Request) bool {
	return r.Method == "GET" && strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest") && r.Header.Get("Upgrade") == "" && !strings.Contains(r.Header.Get("Accept"), "text/event-stream") && (r.Header.Get("Sec-Fetch-Dest") == "" || r.Header.Get("Sec-Fetch-Dest") == "document")
}
func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return SameOrigin(origin, RequestScheme(r), r.Host, false)
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func gateJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func (g *Gate) Respond(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
	if !strings.HasPrefix(v.Reason, "gate:") && v.Action != Challenge {
		return false
	}
	rw := &gateResponseWriter{ResponseWriter: w}
	w = rw
	defer func() {
		if rw.status == 0 {
			rw.status = 200
		}
		v.Status = rw.status
		if v.Reason == "gate:endpoint" && rw.status < 400 {
			v.Action = Allow
		}
		c.Trace.Add(TraceEntry{Stage: 3, Check: "gate_response", Verdict: v})
		c.Values["response_verdict"] = v
	}()
	r := c.Request
	if r.Method == "POST" {
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
		defer controller.SetReadDeadline(time.Time{})
	}
	cfg := s.Config.Policy.Gate
	prefix := strings.TrimRight(cfg.Prefix, "/")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if v.Reason == "gate:endpoint" {
		if c.View.Path == prefix+"/challenge" {
			g.challengeEndpoint(s, c, w)
			return true
		}
		if !cfg.Enabled {
			http.NotFound(w, r)
			return true
		}
		if c.View.Path != prefix+"/login" && c.View.Path != prefix+"/logout" {
			http.NotFound(w, r)
			return true
		}
		if r.Method == "GET" && c.View.Path == prefix+"/login" {
			token := oneCookie(r, "hw_csrf_"+strconv.Itoa(int(s.Config.Port)))
			if _, ok := gateVerify(s.Secrets.GateKey, token, c.PortID, "csrf"); !ok {
				token = gateSign(s.Secrets.GateKey, gateClaims{Kind: "csrf", Port: c.PortID, Nonce: randomToken(), Expiry: time.Now().Add(10 * time.Minute).Unix()})
			}
			SetGateCookie(w, r, "hw_csrf_"+strconv.Itoa(int(s.Config.Port)), token, prefix, 600)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			logo := template.URL("")
			if s.Compiled != nil {
				logo = template.URL(s.Compiled.GateLogo)
			}
			_ = gatePage.Execute(w, map[string]any{"Title": cfg.Title, "Hint": cfg.Hint, "Logo": logo, "Action": prefix + "/login", "CSRF": token, "Next": SafeGateNext(r.URL.Query().Get("next")), "Remember": cfg.RememberDays > 0})
			return true
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if c.View.Path == prefix+"/login" && !g.reserveLogin(s, c) {
			w.Header().Set("Retry-After", "60")
			gateJSON(w, 429, "请稍后重试")
			return true
		}
		if r.ParseForm() != nil || !sameOrigin(r) {
			gateJSON(w, 403, "请求验证失败")
			return true
		}
		csrf := r.PostForm.Get("csrf")
		cookie := oneCookie(r, "hw_csrf_"+strconv.Itoa(int(s.Config.Port)))
		_, ok := gateVerify(s.Secrets.GateKey, csrf, c.PortID, "csrf")
		if !ok || cookie == "" || subtle.ConstantTimeCompare([]byte(csrf), []byte(cookie)) != 1 {
			gateJSON(w, 403, "请求验证失败")
			return true
		}
		if c.View.Path == prefix+"/logout" {
			if sess, ok := g.session(s, c); ok {
				g.Store.Revoke(c.PortID, sess.ID)
			}
			SetGateCookie(w, r, gateCookieName(s.Config.Port), "", "/", -1)
			http.Redirect(w, r, prefix+"/login", 303)
			return true
		}
		u, status := g.authenticate(s, c, r.PostForm.Get("username"), r.PostForm.Get("password"), r.PostForm.Get("otp"))
		r.PostForm.Del("password")
		r.Form.Del("password")
		r.PostForm.Del("otp")
		r.Form.Del("otp")
		if status != 0 {
			if status == 429 {
				w.Header().Set("Retry-After", "60")
			}
			gateJSON(w, status, "用户名或密码错误，请稍后重试")
			return true
		}
		token, age, err := g.issue(s, c, u, r.PostForm.Get("remember") == "1")
		if err != nil {
			gateJSON(w, 503, "暂时无法建立会话")
			return true
		}
		SetGateCookie(w, r, gateCookieName(s.Config.Port), token, "/", age)
		SetGateCookie(w, r, "hw_csrf_"+strconv.Itoa(int(s.Config.Port)), "", prefix, -1)
		next := SafeGateNext(r.PostForm.Get("next"))
		if accessgate.MatchPathPrefix(NormalizePath(next), []string{prefix}) {
			next = "/"
		}
		http.Redirect(w, r, next, 303)
		return true
	}
	if !strings.HasPrefix(v.Reason, "gate:") {
		g.challengePage(s, c, w)
		return true
	}
	if v.Reason == "gate:token_required" {
		gateJSON(w, 401, "需要有效服务令牌")
		return true
	}
	if cfg.Mode == "basic" {
		w.Header().Set("WWW-Authenticate", `Basic realm="HomeWaf", charset="UTF-8"`)
	}
	if v.Status == 429 {
		w.Header().Set("Retry-After", "60")
		gateJSON(w, 429, "请稍后重试")
		return true
	}
	if cfg.Mode == "form" && browserNavigation(r) {
		http.Redirect(w, r, prefix+"/login?next="+url.QueryEscape(SafeGateNext(r.URL.RequestURI())), 302)
		return true
	}
	gateJSON(w, 401, "需要登录")
	return true
}

type gateResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *gateResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *gateResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *gateResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}

var gatePage = template.Must(template.New("gate").Parse(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>:root{color-scheme:light dark;font:16px system-ui}body{max-width:24rem;margin:10vh auto;padding:1.5rem}form{display:grid;gap:1rem}input,button{font:inherit;padding:.8rem;border:1px solid #888;border-radius:.4rem}label{display:grid;gap:.4rem}button{cursor:pointer;background:#245dcc;color:white}small{opacity:.75}.logo{display:block;max-width:96px;max-height:96px;margin-bottom:1rem}</style>{{if .Logo}}<img class="logo" src="{{.Logo}}" alt="站点标识">{{end}}<h1>{{.Title}}</h1><p>{{.Hint}}</p><form method="post" action="{{.Action}}"><input type="hidden" name="csrf" value="{{.CSRF}}"><input type="hidden" name="next" value="{{.Next}}"><label>用户名<input name="username" autocomplete="username" maxlength="128" required></label><label>密码<input type="password" name="password" autocomplete="current-password" maxlength="72" required></label><label>动态码（已启用二次验证时填写）<input name="otp" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6"></label>{{if .Remember}}<label><span><input type="checkbox" name="remember" value="1"> 记住我</span></label>{{end}}<button type="submit">登录</button><small>这是 HomeWaf 访问验证。登录后仍会进行安全检测。</small></form></html>`))

func challengeBinding(c *RequestContext) string {
	return dimension(c.ClientIP.String() + "\n" + c.Request.UserAgent())
}
func (g *Gate) ChallengePassed(s *Snapshot, c *RequestContext) bool {
	token := oneCookie(c.Request, "hw_challenge_"+strconv.Itoa(int(s.Config.Port)))
	v, ok := gateVerify(s.Secrets.GateKey, token, c.PortID, "passed")
	return ok && v.Binding == challengeBinding(c)
}
func (g *Gate) challengePage(s *Snapshot, c *RequestContext, w http.ResponseWriter) {
	if !browserNavigation(c.Request) {
		w.Header().Set("Retry-After", "10")
		gateJSON(w, 429, "请通过浏览器完成访问验证")
		return
	}
	prefix := strings.TrimRight(s.Config.Policy.Gate.Prefix, "/")
	token := gateSign(s.Secrets.GateKey, gateClaims{Kind: "challenge", Port: c.PortID, Nonce: randomToken(), Expiry: time.Now().Add(2 * time.Minute).Unix(), Binding: challengeBinding(c)})
	SetGateCookie(w, c.Request, "hw_challenge_pending_"+strconv.Itoa(int(s.Config.Port)), token, prefix, 120)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(429)
	_ = challengePage.Execute(w, map[string]string{"Action": prefix + "/challenge", "Token": token, "Next": SafeGateNext(c.Request.URL.RequestURI())})
}
func (g *Gate) challengeEndpoint(s *Snapshot, c *RequestContext, w http.ResponseWriter) {
	r := c.Request
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if r.ParseForm() != nil || !sameOrigin(r) {
		gateJSON(w, 403, "请求验证失败")
		return
	}
	token := r.PostForm.Get("token")
	v, ok := gateVerify(s.Secrets.GateKey, token, c.PortID, "challenge")
	pending := oneCookie(r, "hw_challenge_pending_"+strconv.Itoa(int(s.Config.Port)))
	if !ok || v.Binding != challengeBinding(c) || pending == "" || subtle.ConstantTimeCompare([]byte(token), []byte(pending)) != 1 {
		gateJSON(w, 403, "验证已失效")
		return
	}
	ttl := s.Config.Policy.RuleSet.CC.ChallengeSeconds
	v.Kind = "passed"
	v.Expiry = time.Now().Add(time.Duration(ttl) * time.Second).Unix()
	SetGateCookie(w, r, "hw_challenge_"+strconv.Itoa(int(s.Config.Port)), gateSign(s.Secrets.GateKey, v), "/", ttl)
	SetGateCookie(w, r, "hw_challenge_pending_"+strconv.Itoa(int(s.Config.Port)), "", strings.TrimRight(s.Config.Policy.Gate.Prefix, "/"), -1)
	http.Redirect(w, r, SafeGateNext(r.PostForm.Get("next")), 303)
}

var challengePage = template.Must(template.New("challenge").Parse(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>访问验证</title><style>:root{color-scheme:light dark;font:16px system-ui}body{max-width:24rem;margin:10vh auto;padding:2rem}button{font:inherit;padding:1rem}</style><h1>请确认继续访问</h1><p>请求频率或浏览器特征触发了访问验证。</p><form method="post" action="{{.Action}}"><input type="hidden" name="token" value="{{.Token}}"><input type="hidden" name="next" value="{{.Next}}"><button>继续访问</button></form></html>`))
