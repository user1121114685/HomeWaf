package wafpipeline

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type HoneyHandler struct{ admission *Admission }

func NewHoneyHandler() *HoneyHandler { return &HoneyHandler{admission: NewAdmission(64)} }

// HoneypotInteraction contains server-observed request evidence. Never copy
// cookies, authorization or query values; optional body samples must pass the
// bounded, credential-redacting archive path.
type HoneypotInteraction struct {
	Method         string           `json:"method"`
	Path           string           `json:"path"`
	NormalizedPath string           `json:"normalized_path"`
	UserAgent      string           `json:"user_agent"`
	PeerIP         string           `json:"peer_ip,omitempty"`
	LureID         string           `json:"lure_id,omitempty"`
	Archive        *HoneypotArchive `json:"archive,omitempty"`
}

func honeypotInteraction(c *RequestContext) *HoneypotInteraction {
	interaction := &HoneypotInteraction{Method: bound(c.Request.Method, 32), Path: bound(c.Request.URL.EscapedPath(), 1024), NormalizedPath: bound(c.View.Path, 1024), UserAgent: bound(c.Request.UserAgent(), 256)}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		if peer, err := netip.ParseAddr(host); err == nil {
			interaction.PeerIP = peer.Unmap().String()
		}
	}
	c.Values["honeypot_interaction"] = interaction
	return interaction
}

// captureDeniedStep changes only the response to an existing policy denial.
// Access shutdown, unavailable policy, lockdown, auth gates and observe mode
// keep their original behavior. There is no new route to the business handler.
func captureDeniedStep(s *Snapshot, step Step) Step {
	if !s.Config.Policy.Honeypot.Enabled || !s.Config.Policy.Honeypot.CaptureDenied ||
		!(step.Number == 2 && step.Name == "ip_group" || step.Number == 3 && !step.FailClosed) {
		return step
	}
	inspect := step.Inspect
	step.Inspect = func(c *RequestContext) (Verdict, error) {
		start := len(c.Hits)
		v, err := inspect(c)
		if err != nil || v.Observe || v.Action != Deny || v.Status >= 500 {
			return v, err
		}
		eligible := step.Number == 2 && (strings.HasPrefix(v.Reason, "blacklist:") || v.Reason == "ip:unlisted")
		if step.Number == 3 {
			for _, hit := range c.Hits[start:] {
				if !hit.Observe && hit.Category != "detector_error" {
					eligible = true
					break
				}
			}
		}
		if eligible {
			v.Action, v.Status = Honeypot, http.StatusOK
		}
		return v, nil
	}
	return step
}

func TrapStep(s *Snapshot) Step {
	return Step{Number: 3, Name: "trap_path", Inspect: func(c *RequestContext) (Verdict, error) {
		policy := s.Config.Policy
		if !policy.Honeypot.Enabled || !policy.HitPolicy.Enabled {
			return Verdict{Action: Continue}, nil
		}
		for _, path := range policy.Honeypot.TrapPaths {
			if c.View.Path == path {
				c.Hit("trap_path", "trap_path", Low, false, "configured nonexistent path")
				return Verdict{Action: Honeypot, Reason: "trap_path", RuleID: "trap_path"}, nil
			}
		}
		return Verdict{Action: Continue}, nil
	}}
}

var honeyPage = template.Must(template.New("honeypot").Parse(`<!doctype html><html lang="zh"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><title>{{.Title}}</title><style>body{font:16px system-ui;max-width:360px;margin:12vh auto;padding:24px}label,input,button{display:block;box-sizing:border-box;width:100%;margin:12px 0}input,button{padding:12px}small{opacity:.7}</style><h1>{{.Title}}</h1><p>{{.Message}}</p><form method="post"><label>账号<input name="username" autocomplete="off" maxlength="128"></label><label>密码<input type="password" name="password" autocomplete="off" maxlength="256"></label><button>登录</button></form>{{if .Failed}}<p>账号或密码错误，请稍后重试。</p>{{end}}</html>`))

// Respond has no transport or upstream dependency. A captured IP can never
// accidentally reach the business handler from this code path.
func (h *HoneyHandler) Respond(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
	if v.Action != Honeypot && v.Action != Tarpit {
		return false
	}
	interaction := honeypotInteraction(c)
	lure := selectHoneypotLure(s, c)
	interaction.LureID = lure.ID
	c.Hit("honeypot_visit", "honeypot", Low, false, "isolated honeypot request")
	v.Status = http.StatusOK
	defer func() {
		c.Values["response_verdict"] = v
		c.Trace.Add(TraceEntry{Stage: v.Stage, Check: "honeypot_response", Verdict: v})
	}()
	cfg := effectiveHoneypot(s.Config.Policy.Honeypot)
	if !h.admission.Acquire(s.Config.Port, cfg.MaxConcurrent) {
		v.Status = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "1")
		http.Error(w, "Service busy", 503)
		return true
	}
	defer h.admission.Release(s.Config.Port)
	if cfg.MaxDelayMillis > 0 {
		var b [2]byte
		_, _ = rand.Read(b[:])
		delay := time.Duration((int(b[0])*256+int(b[1]))%(cfg.MaxDelayMillis+1)) * time.Millisecond
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-c.Context.Done():
			return true
		}
	}
	post := c.Request.Method == "POST"
	if post || cfg.ArchiveBody && c.Request.Body != nil && c.Request.Body != http.NoBody {
		readHoneypotBody(w, c, cfg)
	}
	trap := false
	for _, path := range cfg.TrapPaths {
		if c.View.Path == path {
			trap = true
		}
	}
	if post || trap {
		evidence := "honeypot trap touched"
		if post {
			username, password := "", ""
			if strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json") {
				var values map[string]string
				if json.NewDecoder(c.Request.Body).Decode(&values) == nil {
					username, password = values["username"], values["password"]
				}
			} else if c.Request.ParseForm() == nil {
				username, password = c.Request.PostForm.Get("username"), c.Request.PostForm.Get("password")
				c.Request.PostForm.Del("password")
				c.Request.Form.Del("password")
			}
			mac := hmac.New(sha256.New, []byte(s.Secrets.GateKey))
			_, _ = io.WriteString(mac, password)
			evidence = fmt.Sprintf("account=%q; password_length=%d; keyed_digest=%s", bound(username, 64), len(password), hex.EncodeToString(mac.Sum(nil))[:24])
		}
		c.Hit("honeypot_escalation", "honeypot", Critical, false, evidence)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	if c.Request.Method == http.MethodHead {
		return true
	}
	_ = honeyPage.Execute(w, struct {
		Title   string
		Message string
		Failed  bool
	}{lure.Title, lure.Message, post})
	return true
}
