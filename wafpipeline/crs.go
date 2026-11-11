package wafpipeline

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/corazawaf/coraza/v3/experimental/plugins/plugintypes"
	"github.com/corazawaf/coraza/v3/types"
)

type crsProgram struct{ waf coraza.WAF }

// CRSProvider shares only immutable rule definitions. Each request gets a new
// Coraza transaction; port modes, exclusions, counters and events remain local.
type CRSProvider struct {
	Root    string
	mu      sync.Mutex
	current atomic.Pointer[crsProgram]
}

func (p *CRSProvider) Prepare(s *Snapshot) error {
	if s.Config.Policy.RuleSet.OWASP == ModeOff {
		return nil
	}
	if p.current.Load() != nil {
		return nil
	}
	return p.load(false)
}
func (p *CRSProvider) Reload() error { return p.load(true) }
func (p *CRSProvider) load(force bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !force && p.current.Load() != nil {
		return nil
	}
	if p.Root == "" {
		return errors.New("CRS directory unavailable")
	}
	cfg := coraza.NewWAFConfig().WithDirectivesFromFile(filepath.Join(p.Root, "coraza.conf")).WithDirectivesFromFile(filepath.Join(p.Root, "coreruleset", "crs-setup.conf")).WithDirectivesFromFile(filepath.Join(p.Root, "coreruleset", "rules", "*.conf"))
	cfg = cfg.WithDirectives("SecRuleEngine DetectionOnly\nSecAuditEngine Off\nSecDebugLogLevel 0\nSecRequestBodyLimit 16777216\nSecRequestBodyInMemoryLimit 16777216\nSecRequestBodyLimitAction ProcessPartial\nSecResponseBodyAccess Off").WithErrorCallback(func(types.MatchedRule) {}).WithDebugLogger(debuglog.Noop())
	waf, err := coraza.NewWAF(cfg)
	if err != nil {
		return fmt.Errorf("CRS compilation failed: %w", err)
	}
	p.current.Store(&crsProgram{waf: waf})
	return nil
}
func crsCategory(id int) string {
	switch id / 1000 {
	case 913:
		return "scanner"
	case 920, 921:
		return "malformed"
	case 930, 931:
		return "lfi"
	case 932:
		return "rce"
	case 933:
		return "php_inject"
	case 934:
		return "ssti"
	case 941:
		return "xss"
	case 942:
		return "sqli"
	case 944:
		return "java_inject"
	}
	return ""
}
func (p *CRSProvider) Step(s *Snapshot) Step {
	return Step{Number: 3, Name: "owasp_crs", Inspect: func(c *RequestContext) (Verdict, error) {
		if skipAttack(c, "owasp") {
			return Verdict{Action: Continue, Reason: "rule_exception"}, nil
		}
		mode := s.Config.Policy.RuleSet.OWASP
		if mode == ModeOff || !scopeMatches(s.Config.Policy.RuleSet.OWASPScope, c.Request) {
			return Verdict{Action: Continue}, nil
		}
		program := p.current.Load()
		if program == nil {
			return Verdict{}, errors.New("CRS unavailable")
		}
		if c.View.deadline.IsZero() {
			c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
		}
		c.View.scanContext = c.Context
		if c.View.expired() {
			return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
		}
		if err := inspectRequestBody(c); err != nil {
			return Verdict{}, errors.New("CRS body inspection unavailable")
		}
		tx := program.waf.NewTransaction()
		defer tx.Close()
		if state, ok := tx.(plugintypes.TransactionState); ok {
			state.Variables().TX().Set("allowed_methods", []string{strings.Join(s.Config.Policy.RuleSet.AllowedMethods, " ")})
		}
		r := c.Request
		tx.ProcessConnection(c.ClientIP.String(), 0, "127.0.0.1", int(s.Config.Port))
		tx.ProcessURI(r.URL.RequestURI(), r.Method, r.Proto)
		tx.SetServerName(r.Host)
		for k, values := range r.Header {
			if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Encoding") || strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Proxy-Authorization") || strings.EqualFold(k, "X-HomeWaf-Token") {
				continue
			}
			if c.View.BodyTruncated && strings.EqualFold(k, "Content-Type") {
				continue
			}
			for _, value := range values {
				tx.AddRequestHeader(k, value)
			}
		}
		tx.AddRequestHeader("Host", r.Host)
		for _, f := range c.View.Fields {
			switch f.Source {
			case "query":
				tx.AddGetRequestArgument(f.Name, f.Value)
			case "form", "json", "json_key", "multipart":
				tx.AddPostRequestArgument(f.Name, f.Value)
			}
		}
		tx.ProcessRequestHeaders()
		if !c.View.BodyTruncated && len(c.View.Body) > 0 {
			if _, _, err := tx.WriteRequestBody(c.View.Body); err != nil {
				return Verdict{}, errors.New("CRS body inspection failed")
			}
		}
		if _, err := tx.ProcessRequestBody(); err != nil {
			return Verdict{}, errors.New("CRS processing failed")
		}
		if c.View.expired() {
			c.Hit("detector_timeout", "owasp_crs", Low, true, "CRS budget exhausted")
			return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
		}
		score, threshold := 0, 5
		if state, ok := tx.(plugintypes.TransactionState); ok {
			for _, key := range []string{"blocking_inbound_anomaly_score", "inbound_anomaly_score", "anomaly_score"} {
				if values := state.Variables().TX().Get(key); len(values) > 0 {
					if value, _ := strconv.Atoi(values[0]); value > score {
						score = value
					}
				}
			}
			if values := state.Variables().TX().Get("inbound_anomaly_score_threshold"); len(values) > 0 {
				if value, _ := strconv.Atoi(values[0]); value > 0 {
					threshold = value
				}
			}
		}
		seen := map[string]bool{}
		for _, match := range tx.MatchedRules() {
			id := match.Rule().ID()
			category := crsCategory(id)
			if category == "" || seen[category] {
				continue
			}
			seen[category] = true
			severity := High
			if category == "rce" || category == "java_inject" {
				severity = Critical
			}
			c.Hit(category, strconv.Itoa(id), severity, mode == ModeObserve || score < threshold, "CRS category matched")
		}
		if score >= threshold && len(seen) > 0 {
			return Verdict{Action: Deny, Reason: "owasp:anomaly_threshold", Status: 403, Observe: mode == ModeObserve}, nil
		}
		return Verdict{Action: Continue}, nil
	}}
}
