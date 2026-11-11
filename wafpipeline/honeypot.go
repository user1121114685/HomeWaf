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
	"net/http"
	"strings"
	"time"
)

type HoneyHandler struct{ admission *Admission }

func NewHoneyHandler() *HoneyHandler { return &HoneyHandler{admission: NewAdmission(64)} }

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

var honeyPage = template.Must(template.New("honeypot").Parse(`<!doctype html><html lang="zh"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><title>{{.Title}}</title><style>body{font:16px system-ui;max-width:360px;margin:12vh auto;padding:24px}label,input,button{display:block;box-sizing:border-box;width:100%;margin:12px 0}input,button{padding:12px}small{opacity:.7}</style><h1>{{.Title}}</h1><form method="post"><label>账号<input name="username" autocomplete="username" maxlength="128"></label><label>密码<input type="password" name="password" autocomplete="current-password" maxlength="256"></label><button>登录</button></form>{{if .Failed}}<p>账号或密码错误，请稍后重试。</p>{{end}}<small>安全访问验证</small></html>`))

// Respond has no transport or upstream dependency. A captured IP can never
// accidentally reach the business handler from this code path.
func (h *HoneyHandler) Respond(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
	if v.Action != Honeypot && v.Action != Tarpit {
		return false
	}
	cfg := s.Config.Policy.Honeypot
	if !h.admission.Acquire(s.Config.Port, cfg.MaxConcurrent) {
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
	trap := false
	for _, path := range cfg.TrapPaths {
		if c.View.Path == path {
			trap = true
		}
	}
	if post || trap {
		evidence := "honeypot trap touched"
		if post {
			c.Request.Body = http.MaxBytesReader(w, c.Request.Body, 16<<10)
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
	_ = honeyPage.Execute(w, struct {
		Title  string
		Failed bool
	}{s.Config.Policy.Gate.Title, post})
	return true
}
