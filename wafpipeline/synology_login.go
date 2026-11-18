package wafpipeline

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	synologyLockKind        = "synology_login_lockdown"
	synologyAdminCategory   = "synology_admin"
	synologyFailureCategory = "synology_login_failure"
)

type SynologyLoginPolicy struct {
	Admin       Mode      `json:"admin_mode"`
	FailureMode Mode      `json:"failure_mode"`
	Failures    Threshold `json:"failures"`
}

func newSynologyRule(category string) Rule {
	rule := Rule{ID: category, Category: category, Severity: Low, Mode: ModeOff}
	if category == synologyFailureCategory {
		rule.LoginFailures = &Threshold{WindowSeconds: 300, Count: 10, BanSeconds: 10800}
	}
	return rule
}

func (rs RuleSetConfig) ruleByCategory(category string) *Rule {
	for i := range rs.Rules {
		if rs.Rules[i].Category == category {
			return &rs.Rules[i]
		}
	}
	return nil
}

func (rs RuleSetConfig) LocksDSMLogins() bool {
	rule := rs.ruleByCategory(synologyFailureCategory)
	return rule != nil && rule.Mode == ModeBlock
}

// CompleteSynologyRules exposes the new detectors as off on older services.
// Preserve every existing rule, including custom IDs and full rulesets.
func (rs *RuleSetConfig) CompleteSynologyRules() {
	legacy := rs.SynologyLogin
	rs.SynologyLogin = nil
	for _, category := range []string{synologyAdminCategory, synologyFailureCategory} {
		defaults := newSynologyRule(category)
		if rule := rs.ruleByCategory(category); rule != nil {
			if legacy != nil {
				if category == synologyAdminCategory {
					rule.Mode = legacy.Admin
				} else {
					rule.Mode = legacy.FailureMode
				}
			}
			if rule.LoginFailures == nil {
				if category == synologyFailureCategory && legacy != nil {
					rule.LoginFailures = &legacy.Failures
				} else {
					rule.LoginFailures = defaults.LoginFailures
				}
			} else if category == synologyFailureCategory && legacy != nil {
				rule.LoginFailures = &legacy.Failures
			}
			continue
		}
		if legacy != nil {
			if category == synologyAdminCategory {
				defaults.Mode = legacy.Admin
			} else {
				defaults.Mode = legacy.FailureMode
				defaults.LoginFailures = &legacy.Failures
			}
		}
		if len(rs.Rules) >= 258 {
			continue
		}
		for n := 2; slices.ContainsFunc(rs.Rules, func(r Rule) bool { return r.ID == defaults.ID }); n++ {
			defaults.ID = fmt.Sprintf("%s_%d", category, n)
		}
		rs.Rules = append(rs.Rules, defaults)
	}
}

func synologyEndpoint(path string) (string, bool) {
	for _, endpoint := range []string{"/webapi/entry.cgi", "/webapi/auth.cgi"} {
		switch path {
		case endpoint:
			return "", true
		case endpoint + "/SYNO.API.Auth":
			return "SYNO.API.Auth", true
		case endpoint + "/SYNO.API.Auth.Type":
			return "SYNO.API.Auth.Type", true
		}
	}
	return "", false
}

// DSM sends quoted form strings in the account lookup and ordinary strings in
// the login operation. Only top-level request parameters identify an operation.
type synologyForm struct {
	auth, authType, get, login, admin bool
	otherAPI, otherMethod             bool
}

func (f *synologyForm) add(key, value string) {
	if key != "api" && key != "method" && key != "account" || len(value) > 128 {
		return
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if json.Unmarshal([]byte(value), &decoded) == nil {
			value = decoded
		}
	}
	switch key {
	case "api":
		switch value {
		case "SYNO.API.Auth":
			f.auth = true
		case "SYNO.API.Auth.Type":
			f.authType = true
		default:
			f.otherAPI = true
		}
	case "method":
		switch value {
		case "get":
			f.get = true
		case "login":
			f.login = true
		default:
			f.otherMethod = true
		}
	case "account":
		f.admin = f.admin || strings.EqualFold(value, "admin")
	}
}

func (r *Runtime) synologyLoginStep(s *Snapshot, admin, failure *Rule) Step {
	state, _ := r.State.(*Punisher)
	enforce := failure != nil && failure.Mode == ModeBlock
	if enforce && !s.Preview {
		// Unrelated saves share the window. Re-enabling the rule, changing
		// its thresholds or recreating this service starts a new window.
		limits := failure.LoginFailures
		if limits == nil {
			limits = newSynologyRule(synologyFailureCategory).LoginFailures
		}
		window := &synologyFailureWindow{limit: *limits}
		if previous, ok := Ports.Get(s.Config.Port); ok && previous.Compiled != nil {
			if old := previous.Compiled.synologyWindow; old != nil && old.limit == window.limit {
				window = old
			}
		}
		s.Compiled.synologyWindow = window
	}
	return Step{Number: 3, Name: "synology_login", RunTrusted: true, FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
		next := Verdict{Action: Continue}
		api, ok := synologyEndpoint(c.View.Path)
		if !ok || (c.Request.Method != http.MethodPost && c.Request.Method != http.MethodGet) {
			return next, nil
		}
		var form synologyForm
		if api != "" {
			form.add("api", api)
		}
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				form.add(key, value)
			}
		}
		media, _, _ := mime.ParseMediaType(c.Request.Header.Get("Content-Type"))
		if c.Request.Method == http.MethodPost && (media == "application/x-www-form-urlencoded" || media == "multipart/form-data") {
			if err := inspectRequestBody(c); err != nil {
				return Verdict{}, err
			}
			for _, field := range c.View.Fields {
				if field.Source == "form" || field.Source == "multipart" {
					form.add(field.Name, field.RawValue)
				}
			}
		}
		login := form.auth && form.login
		complete := !c.View.BodyTruncated && c.View.ParseError == ""
		if enforce && (login || !complete && (form.auth || form.login || !form.otherAPI)) {
			if state == nil && !c.DryRun {
				return Verdict{Action: Deny, Status: http.StatusServiceUnavailable, Reason: "synology_state_unavailable"}, nil
			}
			if state != nil {
				if remaining := state.synologyRetryAfter(c.PortID); remaining > 0 {
					c.Values["synology_retry_after"] = remaining
					return Verdict{Action: Deny, Status: http.StatusTooManyRequests, Reason: synologyLockKind, RuleID: failure.ID}, nil
				}
			}
		}
		if admin != nil && admin.Mode != ModeOff && form.admin && (login || form.authType && form.get) && scopeMatches(admin.Scope, c.Request) {
			observed := admin.Mode == ModeObserve
			c.Hit(admin.Category, admin.ID, admin.Severity, observed, "Built-in administrator in DSM authentication request")
			if !observed {
				return Verdict{Action: Deny, Status: http.StatusForbidden, Reason: synologyAdminCategory, RuleID: admin.ID}, nil
			}
		}
		if failure != nil && failure.Mode != ModeOff && login && complete && !form.otherAPI && !form.authType && !form.get && !form.otherMethod && !c.DryRun && scopeMatches(failure.Scope, c.Request) {
			c.synologyFailure = func() {
				responseHit(c, failure.Category, failure.ID, true, http.StatusBadRequest)
				if enforce {
					state.recordSynologyFailure(s, failure.ID, c.ReqID)
				}
			}
		}
		return next, nil
	}}
}

// Sample the unmodified upstream envelope and preserve the original wire body.
// An incomplete response, an OTP challenge or a successful HTTP exchange alone
// supplies no credential-failure evidence.
func inspectSynologyResponse(c *RequestContext, response *http.Response) error {
	const limit = 8192
	if c.synologyFailure == nil || response.StatusCode != http.StatusOK || response.ContentLength > limit {
		return nil
	}
	media, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if media != "application/json" && !strings.HasSuffix(media, "+json") {
		return nil
	}
	body := response.Body
	req := &http.Request{Method: http.MethodPost, URL: &url.URL{}, Header: http.Header{}, Body: body}
	req.Header.Set("Content-Encoding", response.Header.Get("Content-Encoding"))
	view := NewNormalizedView(req, ViewLimits{BodyBytes: limit})
	timer := time.AfterFunc(5*time.Second, func() { _ = body.Close() })
	err := view.ReadBody()
	timer.Stop()
	response.Body = req.Body
	if err != nil || view.BodyTruncated || view.ParseError != "" {
		// A response inspection failure must not turn an upstream DSM error
		// into a synthetic gateway error or consume the response body.
		return nil
	}
	var envelope struct {
		Success *bool `json:"success"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(view.Body, &envelope) == nil && envelope.Success != nil && !*envelope.Success && envelope.Error.Code == 400 {
		c.synologyFailure()
	}
	return nil
}

func (p *Punisher) synologyRetryAfter(port string) int64 {
	if record, ok := p.state.Load().SynologyLocks[port]; ok {
		return max(0, record.ExpiresAt-p.Now().Unix())
	}
	return 0
}

type synologyFailure struct {
	at      time.Time
	request string
}

type synologyFailureWindow struct {
	limit    Threshold
	failures []synologyFailure // guarded by the service punisher's mutex
}

// ClearSynologyLock removes only this service's persisted DSM login lock.
func (p *Punisher) ClearSynologyLock(port uint16) error {
	id := recordKey(synologyLockKind, "port", PortID(port), "")
	p.mu.Lock()
	_, exists := p.records[id]
	if exists {
		p.remove(id)
		p.publish()
	}
	p.mu.Unlock()
	if exists {
		return p.Flush()
	}
	return nil
}

// Login attempts from all addresses share one exact sliding window per port.
// Publish the transition before returning the failure response. The ordinary
// IP-ban queue may be delayed and its exemptions apply only to IP punishment.
func (p *Punisher) recordSynologyFailure(s *Snapshot, ruleID, request string) {
	port := PortID(s.Config.Port)
	id := recordKey(synologyLockKind, "port", port, "")
	p.mu.Lock()
	now := p.Now()
	window := s.Compiled.synologyWindow
	current := p.Lookup(port)
	if window == nil || current == nil || current.Compiled == nil || current.Compiled.synologyWindow != window {
		p.mu.Unlock()
		return
	}
	if record, ok := p.records[id]; ok && active(record, now.Unix()) {
		p.mu.Unlock()
		return
	}
	limit := window.limit
	cutoff := now.Add(-time.Duration(limit.WindowSeconds) * time.Second)
	fresh := 0
	for fresh < len(window.failures) && !window.failures[fresh].at.After(cutoff) {
		fresh++
	}
	window.failures = window.failures[fresh:]
	if slices.ContainsFunc(window.failures, func(f synologyFailure) bool { return f.request == request }) {
		p.mu.Unlock()
		return
	}
	window.failures = append(window.failures, synologyFailure{now, request})
	if len(window.failures) < limit.Count {
		p.mu.Unlock()
		return
	}
	record := PunishmentRecord{ID: id, Kind: synologyLockKind, PortID: port, Scope: "port", RuleID: ruleID, Reason: "synology_login_failures", Hits: limit.Count, CreatedAt: now.Unix(), ExpiresAt: now.Unix() + int64(limit.BanSeconds)}
	p.put(record) // A port-owned DSM lock always has a reserved slot.
	window.failures = nil
	p.publish()
	p.mu.Unlock()
	_ = p.Flush() // The persistence worker retries failures.
	if p.OnChange != nil {
		p.OnChange(PunishmentChange{record, "synology_login_lockdown_started"})
	}
}
