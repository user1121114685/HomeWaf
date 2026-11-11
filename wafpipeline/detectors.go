package wafpipeline

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Match struct{ Category, Source, Code string }
type Detector interface {
	Name() string
	Category() string
	Inspect(*NormalizedView) []Match
}
type valueDetector struct {
	category string
	match    func(string) bool
}

func (d valueDetector) Name() string     { return d.category }
func (d valueDetector) Category() string { return d.category }
func (d valueDetector) Inspect(v *NormalizedView) []Match {
	// Normalization of a path removes traversal; retain its decoded raw spelling
	// as a separate detection input. Neither representation changes forwarding.
	if scanValue(v, NormalizeValue(v.RawPath), d.match) {
		return []Match{{d.category, "path", d.category}}
	}
	for _, f := range v.Fields {
		if v.expired() {
			break
		}
		if v.excluded(f.Name) {
			continue
		}
		if scanValue(v, f.Value, d.match) || f.Source != "json_key" && scanValue(v, f.Name, d.match) {
			return []Match{{d.category, f.Source, d.category}}
		}
	}
	return nil
}
func scanValue(v *NormalizedView, value string, match func(string) bool) bool {
	// Keep each uninterrupted library call bounded. Adjacent windows overlap
	// to preserve signatures across boundaries. Long cross-window expressions
	// remain a documented limit of a bounded reverse-proxy inspection budget.
	const window = 8192
	const overlap = 1024
	for start := 0; start < len(value); start += window - overlap {
		if v.expired() {
			return false
		}
		end := start + window
		if end > len(value) {
			end = len(value)
		}
		if match(value[start:end]) {
			return true
		}
		if end == len(value) {
			break
		}
	}
	return false
}

type RuleEngine struct{ detectors map[string]Detector }

func NewRuleEngine(detectors ...Detector) *RuleEngine {
	e := &RuleEngine{detectors: map[string]Detector{}}
	for _, d := range detectors {
		if d != nil {
			e.detectors[d.Category()] = d
		}
	}
	return e
}
func DefaultRuleEngine() *RuleEngine {
	return NewRuleEngine(ProtocolDetector(nil, 32768), ScannerDetector(nil), SQLDetector(), XSSDetector(), UploadDetector(), LFIDetector(), RCEDetector(), JavaInjectionDetector(), JavaDeserializationDetector(), PHPDeserializationDetector(), PHPInjectionDetector(), ASPInjectionDetector(), SSTIDetector(), SSRFDetector(), CSRFDetector("http", "observe", nil))
}

// WithControls fixes protocol/QPS/CC/scanner/BOT/gate/attack order independent
// of the order in which rules happen to appear in a saved configuration.
func (e *RuleEngine) WithControls(s *Snapshot, controls []Step) []Step {
	categories := map[string]string{}
	for _, rule := range s.Config.Policy.RuleSet.Rules {
		categories["rule:"+rule.ID] = rule.Category
	}
	var protocol, scanner, headerChecks, attacks []Step
	for _, step := range e.Steps(s) {
		switch categories[step.Name] {
		case "malformed":
			protocol = append(protocol, step)
		case "scanner":
			scanner = append(scanner, step)
		case "csrf":
			headerChecks = append(headerChecks, step)
		default:
			attacks = append(attacks, step)
		}
	}
	out := append([]Step{}, protocol...)
	added := false
	for _, step := range controls {
		if !added && (step.Name == "bot" || step.Name == "auth_gate") {
			out = append(out, scanner...)
			added = true
		}
		out = append(out, step)
	}
	if !added {
		out = append(out, scanner...)
	}
	out = append(out, headerChecks...)
	return append(out, attacks...)
}
func (e *RuleEngine) Steps(s *Snapshot) []Step {
	steps := []Step{}
	for _, rule := range s.Config.Policy.RuleSet.Rules {
		detector, ok := e.detectors[rule.Category]
		if !ok || rule.Mode == ModeOff {
			continue
		}
		rule := rule
		if rule.Category == "csrf" {
			detector = CSRFDetector(s.Config.Protocol, s.Config.Policy.RuleSet.CSRFMissingOrigin, s.Config.Policy.RuleSet.CSRFOrigins)
			if options := s.Config.Policy.RuleSet.CSRFOptions; options != nil {
				missing := "block"
				if options.AllowEmptyRef == 1 {
					missing = "allow"
				}
				origins := append([]string{}, s.Config.Policy.RuleSet.CSRFOrigins...)
				for _, origin := range splitOptionList(options.AllowedOrigins) {
					if !strings.Contains(origin, "://") {
						origin = s.Config.Protocol + "://" + origin
					}
					origins = append(origins, origin)
				}
				d := CSRFDetector(s.Config.Protocol, missing, origins).(csrfDetector)
				d.options = options
				detector = d
			}
		}
		if rule.Category == "upload" {
			detector = uploadDetector{options: s.Config.Policy.RuleSet.UploadOptions}
		}
		if rule.Category == "malformed" {
			detector = ProtocolDetector(s.Config.Policy.RuleSet.AllowedMethods, s.Config.Policy.Network.MaxHeaderBytes)
		}
		if rule.Category == "scanner" {
			detector = ScannerDetector(scannerPaths(rule))
		}
		needsBody := true
		if d, ok := detector.(interface{ NeedsBody() bool }); ok {
			needsBody = d.NeedsBody()
		}
		steps = append(steps, Step{Number: 3, Name: "rule:" + rule.ID, Inspect: func(c *RequestContext) (Verdict, error) {
			if skipAttack(c, rule.Category) {
				return Verdict{Action: Continue, Reason: "rule_exception"}, nil
			}
			if !scopeMatches(rule.Scope, c.Request) {
				return Verdict{Action: Continue, Reason: "out_of_scope"}, nil
			}
			if rule.Category == "malformed" || rule.Category == "scanner" {
				previous := c.View.deadline
				c.View.deadline = time.Time{}
				defer func() { c.View.deadline = previous }()
			}
			if c.View.deadline.IsZero() {
				c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
			}
			c.View.scanContext = c.Context
			c.View.excludeFields = rule.Scope.ExcludeFields
			if !c.View.expired() && needsBody {
				if err := inspectRequestBody(c); err != nil {
					c.Hit("detector_error", rule.ID, Low, true, "body inspection unavailable")
					return Verdict{Action: Continue, Reason: "body_read_error"}, nil
				}
			}
			matches := detector.Inspect(c.View)
			if c.View.expired() {
				if _, seen := c.Values["budget_reported"]; !seen {
					c.Hit("detector_timeout", rule.ID, Low, true, "request inspection budget exhausted")
					c.Values["budget_reported"] = true
				}
				return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
			}
			if c.View.ParseError != "" {
				if _, seen := c.Values["parse_reported"]; !seen {
					c.Hit("body_parse", rule.ID, Low, true, c.View.ParseError)
					c.Values["parse_reported"] = true
				}
			}
			if len(matches) == 0 {
				return Verdict{Action: Continue}, nil
			}
			observe := rule.Mode == ModeObserve || matches[0].Code == "missing_origin_observe" || matches[0].Code == "protocol_observe"
			c.Hit(rule.Category, rule.ID, rule.Severity, observe, fmt.Sprintf("%s signature in %s (%s)", rule.Category, matches[0].Source, matches[0].Code))
			status := 403
			if rule.Category == "malformed" {
				status = 400
			}
			return Verdict{Action: Deny, Reason: "rule:" + rule.ID, RuleID: rule.ID, Status: status, Observe: observe}, nil
		}})
	}
	return steps
}
func (v *NormalizedView) expired() bool {
	if v.scanContext != nil {
		select {
		case <-v.scanContext.Done():
			return true
		default:
		}
	}
	return !v.deadline.IsZero() && time.Now().After(v.deadline)
}
func (v *NormalizedView) excluded(name string) bool {
	for _, field := range v.excludeFields {
		if strings.EqualFold(field, name) {
			return true
		}
	}
	return false
}

// The wire read timeout is independent of the CPU inspection budget.
func inspectRequestBody(c *RequestContext) error {
	first := !c.View.bodyReadStarted
	if writer, ok := c.Values["response_writer"].(http.ResponseWriter); ok && first {
		controller := http.NewResponseController(writer)
		_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
		defer controller.SetReadDeadline(time.Time{})
	}
	err := c.View.ReadBody()
	if first && !c.View.deadline.IsZero() {
		c.View.deadline = c.View.deadline.Add(c.View.bodyReadDuration)
	}
	return err
}
