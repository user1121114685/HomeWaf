package wafpipeline

import (
	"HomeWaf/wafenginecore/cccounter"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
)

type FeedReader interface{ Contains(netip.Addr, int64) bool }
type Runtime struct {
	PluginPrepare        func(*Snapshot) error
	PluginStats          func() map[string]uint64
	CertificateChallenge func(*Snapshot, *RequestContext) (string, bool)
	AI                   AIPredictor
	Admission            *Admission
	Geo                  *GeoCache
	Feed                 FeedReader
	State                IPStateReader
	Protected            func(netip.Addr) bool
	Sink                 EventSink
	Forward              func(*Snapshot, *RequestContext, http.ResponseWriter)
	ForwardPrepare       func(*Snapshot) error
	OnResult             func(*Snapshot, *RequestContext, Result)
	MetricsForPort       func(uint16) *PortMetrics
	// Factory hooks are registered before any port snapshot is prepared.
	RuleSteps       func(*Snapshot) []Step
	RulePrepare     func(*Snapshot) error
	Respond         func(*Snapshot, *RequestContext, http.ResponseWriter, Verdict) bool
	Requests        atomic.Uint64
	Denied          atomic.Uint64
	failureInit     sync.Once
	failureCounters *cccounter.Counter
	failureCleanup  atomic.Int64
}

var Live = &Runtime{}

func (r *Runtime) Prepare(s *Snapshot) error {
	if err := ValidateConfig(s.Config, 0); err != nil {
		return err
	}
	if err := ValidatePluginProfiles(s.Secrets.PluginProfiles); err != nil {
		return err
	}
	if r.RulePrepare != nil {
		if err := r.RulePrepare(s); err != nil {
			return err
		}
	}
	compiled, err := compileIP(s.Config)
	if err != nil {
		return err
	}
	s.Compiled = compiled
	s.Compiled.GateLogo = s.Config.Policy.Gate.LogoData
	s.Compiled.BusinessSelector = compileBusinessSelector(s.Config)
	s.Compiled.Sensitive, err = compileSensitive(s.Config.Policy.RuleSet.Sensitive)
	if err != nil {
		return err
	}
	s.Compiled.Privacy, err = compilePrivacy(s.Config.Policy.Network.Response.Privacy, s.Secrets.PrivacyConfig)
	if err != nil {
		return err
	}
	s.Compiled.Extensions, err = compileExtensions(s.Config)
	if err != nil {
		return err
	}
	s.Compiled.Metrics = &PortMetrics{}
	if !s.Preview {
		if r.MetricsForPort != nil {
			s.Compiled.Metrics = r.MetricsForPort(s.Config.Port)
		}
		if old, ok := Ports.Get(s.Config.Port); ok && old.Compiled != nil && old.Compiled.Metrics != nil {
			s.Compiled.Metrics = old.Compiled.Metrics
		}
	}
	if r.ForwardPrepare != nil && !s.Preview {
		if err := r.ForwardPrepare(s); err != nil {
			return err
		}
	}
	if p := s.Config.Policy.RuleSet.Plugins; p != nil && p.Mode != ModeOff && !s.Preview && s.Config.ListenEnabled {
		if r.PluginPrepare == nil {
			old, _ := Ports.Get(s.Config.Port)
			CloseRetiredProxy(s, old)
			return errors.New("端口插件运行器未就绪")
		}
		if err := r.PluginPrepare(s); err != nil {
			old, _ := Ports.Get(s.Config.Port)
			CloseRetiredProxy(s, old)
			return err
		}
	}
	steps := []Step{{Number: 1, Name: "access_switch", FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
		if !s.Config.Policy.Access.Enabled {
			return Verdict{Action: s.Config.Policy.Access.DisabledAction, Reason: "access_disabled", Status: 503}, nil
		}
		return Verdict{Action: Continue, Reason: "access_enabled"}, nil
	}}}
	if len(s.Config.HostNames) > 0 {
		steps = append(steps, Step{Number: 1, Name: "host_identity", FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
			if !portHostAllowed(s.Config.HostNames, c.Request.Host) {
				return Verdict{Action: Deny, Status: 403, Reason: "host_not_allowed"}, nil
			}
			return Verdict{Action: Continue, Reason: "host_allowed"}, nil
		}})
	}
	if len(s.Config.ACMEHosts) > 0 && r.CertificateChallenge != nil {
		steps = append(steps, Step{Number: 2, Name: "certificate_challenge", Inspect: func(c *RequestContext) (Verdict, error) {
			if value, ok := r.CertificateChallenge(s, c); ok {
				c.Values["certificate_challenge"] = value
				return Verdict{Action: Allow, Reason: "registered_acme_challenge", Status: 200}, nil
			}
			return Verdict{Action: Continue}, nil
		}})
	}
	steps = append(steps, r.ipStep(s))
	if len(s.Config.Routes) > 0 {
		steps = append(steps, businessEntryStep(s))
	}
	if r.RuleSteps != nil {
		steps = append(steps, r.withExtensions(s, r.RuleSteps(s))...)
	} else {
		steps = append(steps, r.withExtensions(s, nil)...)
	}
	if s.Config.Policy.Network.MaxProxyHops > 0 {
		steps = append(steps, proxyLoopStep(s))
	}
	for i := range steps {
		steps[i] = captureDeniedStep(s, steps[i])
	}
	p, err := New(steps, r.Sink)
	if err != nil {
		old, _ := Ports.Get(s.Config.Port)
		CloseRetiredProxy(s, old)
		return err
	}
	s.Compiled.Pipeline = p
	return nil
}
func (r *Runtime) Explain(s *Snapshot, req *http.Request, ip netip.Addr) (Result, error) {
	if !ip.IsValid() {
		return Result{}, errors.New("invalid client IP")
	}
	copy := &Snapshot{Config: CloneConfig(s.Config), Secrets: s.Secrets, Preview: true}
	if err := r.Prepare(copy); err != nil {
		return Result{}, err
	}
	req = WithTrustedExternalScheme(req, copy.Config.Protocol)
	c := NewRequestContext(req, PortID(copy.Config.Port), ip, copy.Config.Policy.RuleSet.View)
	c.DryRun = true
	return copy.Compiled.Pipeline.Evaluate(c), nil
}
func (r *Runtime) Serve(s *Snapshot, w http.ResponseWriter, req *http.Request) {
	r.Requests.Add(1)
	if s.Compiled == nil || s.Compiled.Pipeline == nil {
		http.Error(w, "Port policy is unavailable", 503)
		return
	}
	if s.Compiled.Metrics != nil {
		s.Compiled.Metrics.Requests.Add(1)
		s.Compiled.Metrics.AddDaily(1, 0, 0)
	}
	if r.Admission != nil {
		if !r.Admission.Acquire(s.Config.Port, s.Config.Policy.Network.MaxInFlight) {
			r.Denied.Add(1)
			if s.Compiled.Metrics != nil {
				s.Compiled.Metrics.Denied.Add(1)
				s.Compiled.Metrics.AddDaily(0, 1, 0)
			}
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Service busy", 503)
			return
		}
		defer r.Admission.Release(s.Config.Port)
	}
	port, err := LocalPort(req)
	if err != nil || port != s.Config.Port {
		http.Error(w, "Invalid listener", 400)
		return
	}
	ip, err := ExtractClientIP(req, s.Config.Policy.Network, s.Compiled.Proxies)
	if err != nil {
		http.Error(w, "Invalid client address", 400)
		return
	}
	req = WithTrustedExternalScheme(req, forwardedScheme(s, req))
	c := NewRequestContext(req, PortID(s.Config.Port), ip, s.Config.Policy.RuleSet.View)
	c.Values["response_writer"] = w
	result := s.Compiled.Pipeline.evaluate(c, false)
	submitted := len(c.Hits)
	defer func() {
		// Gate responses and proxy observation can produce events after Evaluate.
		if r.Sink != nil && !c.DryRun {
			for _, event := range c.Hits[submitted:] {
				r.Sink.Submit(event)
			}
		}
		result.Hits = append([]HitEvent(nil), c.Hits...)
		result.Trace = c.Trace.Entries()
		if v, ok := c.Values["response_verdict"].(Verdict); ok {
			result.Verdict = v
		}
		if interaction, ok := c.Values["honeypot_interaction"].(*HoneypotInteraction); ok {
			result.Honeypot = interaction
		}
		r.recordScriptFailures(s, c, result)
		if result.Verdict.Action != Allow && result.Verdict.Action != Continue && result.Verdict.Action != Bypass {
			r.Denied.Add(1)
		}
		if m := s.Compiled.Metrics; m != nil {
			var dailyHits, dailyDenied uint64
			for _, event := range result.Hits {
				if event.Category != "auth_success" && event.Category != "business_auth_success" {
					m.Hits.Add(1)
					dailyHits++
				}
			}
			if result.Verdict.Action != Allow && result.Verdict.Action != Continue && result.Verdict.Action != Bypass {
				m.Denied.Add(1)
				dailyDenied++
			}
			m.AddDaily(0, dailyDenied, dailyHits)
			if len(result.Hits) > 0 || result.Verdict.Action != Allow {
				m.Last.Store(&result)
			}
		}
		if r.OnResult != nil {
			r.OnResult(s, c, result)
		}
	}()
	v := result.Verdict
	if v.Action == Allow || v.Action == Continue || v.Action == Bypass {
		if destination, ok := c.Values["https_redirect"].(string); ok {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, c.Request, destination, v.Status)
			final := v
			final.Stage = 5
			c.Values["response_verdict"] = final
			c.Trace.Add(TraceEntry{Stage: 5, Check: "local_https_redirect", Verdict: final})
			return
		}
		if challenge, ok := c.Values["certificate_challenge"].(string); ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Length", fmt.Sprint(len(challenge)))
			w.WriteHeader(200)
			if c.Request.Method != "HEAD" {
				_, _ = w.Write([]byte(challenge))
			}
			final := Verdict{Action: Allow, Stage: 5, Reason: "registered_acme_challenge", Status: 200}
			c.Values["response_verdict"] = final
			c.Trace.Add(TraceEntry{Stage: 5, Check: "local_certificate_challenge", Verdict: final})
			return
		}
		if r.Forward == nil {
			http.Error(w, "Upstream handler is not ready", 503)
			return
		}
		StripGateCredentials(c.Request, s.Config.Policy.Gate)
		if token, ok := c.Values["gate_cookie"].(string); ok {
			SetGateCookie(w, c.Request, gateCookieName(s.Config.Port), token, "/", 0)
		}
		r.Forward(s, c, w)
		return
	}
	if v.Reason == "anti_leech" && v.Status == 302 {
		if target, ok := c.Values["policy_redirect"].(string); ok {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, c.Request, target, 302)
			return
		}
	}
	if r.Respond != nil && r.Respond(s, c, w, v) {
		return
	}
	if v.Action == Drop {
		if h, ok := w.(http.Hijacker); ok {
			if conn, _, e := h.Hijack(); e == nil {
				_ = conn.Close()
				return
			}
		}
		panic(http.ErrAbortHandler)
	}
	status := v.Status
	if status < 400 || status > 599 {
		status = 403
	}
	result.Verdict.Status = status
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": http.StatusText(status), "request_id": c.ReqID})
}

func portHostAllowed(names []string, host string) bool {
	hostname := strings.ToLower(host)
	if strings.HasPrefix(hostname, "[") {
		if address, err := netip.ParseAddrPort(hostname); err == nil {
			hostname = address.Addr().String()
		} else {
			hostname = strings.TrimSuffix(strings.TrimPrefix(hostname, "["), "]")
		}
	} else if i := strings.LastIndexByte(hostname, ':'); i >= 0 {
		hostname = hostname[:i]
	}
	for _, allowed := range names {
		if strings.EqualFold(allowed, hostname) || strings.HasPrefix(allowed, "*.") && strings.HasSuffix(hostname, strings.ToLower(allowed[1:])) {
			return true
		}
	}
	return false
}
func (r *Runtime) Stats() map[string]uint64 {
	return map[string]uint64{"requests": r.Requests.Load(), "denied": r.Denied.Load()}
}
func (r *Runtime) CheckPrepared(s *Snapshot) error {
	if s.Compiled == nil {
		return fmt.Errorf("port policy has not been prepared")
	}
	return nil
}
