package wafpipeline

import (
	"HomeWaf/innerbean"
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/cccounter"
	"HomeWaf/wafenginecore/ipset"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ScriptRule struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"`
	Mode     Mode     `json:"mode"`
	Severity Severity `json:"severity"`
}
type URLRule struct {
	ID     string `json:"id"`
	Match  string `json:"match"`
	Value  string `json:"value"`
	Action Action `json:"action"`
	Mode   Mode   `json:"mode"`
}
type compiledScript struct {
	rule      ScriptRule
	helper    *utils.RuleHelper
	body, geo bool
}
type compiledExtensions struct {
	scripts []compiledScript
	groups  map[string]*ipset.MatchSet
	windows []int
}

var failureWindowPattern = regexp.MustCompile(`\bMF\s*\.\s*GetIPFailureCount\s*\(\s*(\d+)\s*\)`)
var failureCallPattern = regexp.MustCompile(`\bMF\s*\.\s*GetIPFailureCount\s*\(`)
var scriptFactPattern = regexp.MustCompile(`\bMF\s*\.\s*([A-Za-z_][A-Za-z_0-9]*)`)

func validateExtensions(r RuleSetConfig) error {
	if len(r.Scripts) > 128 || len(r.URLs) > 2000 {
		return errors.New("custom rule quota exceeded")
	}
	ids := map[string]bool{}
	total := 0
	for _, rule := range r.Scripts {
		total += len(rule.Source)
		if rule.ID == "" || len(rule.ID) > 128 || ids[rule.ID] || !ValidMode(rule.Mode) || rule.Severity.Rank() == 0 || len(rule.Source) == 0 || len(rule.Source) > 32768 || total > 256<<10 {
			return errors.New("invalid custom rule or source quota exceeded")
		}
		ids[rule.ID] = true
		skeleton := utils.BuildGrlSkeleton(rule.Source)
		for _, match := range scriptFactPattern.FindAllStringSubmatch(skeleton, -1) {
			_, field := reflect.TypeOf(scriptFacts{}).FieldByName(match[1])
			_, method := reflect.TypeOf(&scriptFacts{}).MethodByName(match[1])
			if !field && !method {
				return fmt.Errorf("unsupported port rule fact: %s", match[1])
			}
		}
		calls := failureCallPattern.FindAllString(skeleton, -1)
		windows := failureWindowPattern.FindAllStringSubmatch(skeleton, -1)
		if len(calls) != len(windows) {
			return errors.New("GetIPFailureCount requires a constant 1..1440 minute window")
		}
		for _, window := range windows {
			n, _ := strconv.Atoi(window[1])
			if n < 1 || n > 1440 {
				return errors.New("failure window outside 1..1440 minutes")
			}
		}
	}
	ids = map[string]bool{}
	for _, rule := range r.URLs {
		if rule.ID == "" || len(rule.ID) > 128 || ids[rule.ID] || rule.Value == "" || len(rule.Value) > 2048 || !ValidMode(rule.Mode) || (rule.Action != Allow && rule.Action != Deny) {
			return errors.New("invalid URL policy")
		}
		ids[rule.ID] = true
		switch rule.Match {
		case "exact", "prefix", "suffix", "contains":
		default:
			return errors.New("invalid URL comparison")
		}
	}
	return nil
}
func compileExtensions(c PortConfig) (*compiledExtensions, error) {
	if len(c.Policy.RuleSet.Scripts) == 0 && len(c.Policy.RuleSet.URLs) == 0 {
		return nil, nil
	}
	out := &compiledExtensions{groups: map[string]*ipset.MatchSet{}}
	for name, values := range c.ScriptGroups {
		matcher := ipset.BuildMatchSet(values)
		if matcher.WildcardLen() > 512 {
			return nil, errors.New("script IP group wildcard quota exceeded")
		}
		out.groups[name] = matcher
	}
	windows := map[int]bool{}
	entries := 0
	for _, rule := range c.Policy.RuleSet.Scripts {
		if rule.Mode == ModeOff {
			continue
		}
		helper := &utils.RuleHelper{}
		helper.InitRuleEngine()
		if err := helper.LoadRuleString(rule.Source); err != nil {
			return nil, fmt.Errorf("自定义规则 %s 编译失败", rule.ID)
		}
		entries += len(helper.KnowledgeBase.RuleEntries)
		if entries > 256 {
			return nil, errors.New("custom rule entry quota exceeded")
		}
		skeleton := utils.BuildGrlSkeleton(rule.Source)
		for _, window := range failureWindowPattern.FindAllStringSubmatch(skeleton, -1) {
			n, _ := strconv.Atoi(window[1])
			windows[n] = true
		}
		out.scripts = append(out.scripts, compiledScript{rule, helper, strings.Contains(skeleton, "BODY"), strings.Contains(skeleton, "COUNTRY") || strings.Contains(skeleton, "PROVINCE") || strings.Contains(skeleton, "CITY")})
	}
	if len(windows) > 16 {
		return nil, errors.New("at most 16 distinct failure windows per port")
	}
	for window := range windows {
		out.windows = append(out.windows, window)
	}
	sort.Ints(out.windows)
	return out, nil
}

// A new defined type deliberately does not inherit WebLog's cache/Redis methods.
// All facts are request-local; the only helper reads an in-memory port counter.
type scriptFacts struct {
	HOST, URL, REFERER, USER_AGENT, METHOD, COOKIES, BODY, PORT, SRC_IP, COUNTRY, PROVINCE, CITY string
	CONTENT_LENGTH                                                                               int64
	headers                                                                                      http.Header
	verified                                                                                     bool
	failures                                                                                     func(int64) int64
}

func (f *scriptFacts) GetHeaderValue(name string) string     { return f.headers.Get(name) }
func (f *scriptFacts) IsSafeBot() bool                       { return f.verified }
func (f *scriptFacts) GetIPFailureCount(minutes int64) int64 { return f.failures(minutes) }

type scriptFunctions struct {
	*innerbean.RuleFunc
	groups map[string]*ipset.MatchSet
}

func (f *scriptFunctions) IPInGroup(ip, name string) bool {
	group := f.groups[name]
	return group != nil && group.Contains(net.ParseIP(ip))
}

func (r *Runtime) failureStore() *cccounter.Counter {
	r.failureInit.Do(func() { r.failureCounters = cccounter.New(20000, 1000) })
	return r.failureCounters
}
func (r *Runtime) recordScriptFailures(s *Snapshot, c *RequestContext, result Result) {
	ext := s.Compiled.Extensions
	if c.DryRun || c.Trusted || ext == nil || len(ext.windows) == 0 {
		return
	}
	failed := result.Verdict.Stage == 5 && (result.Verdict.Status == 401 || result.Verdict.Status == 403 || result.Verdict.Status == 404)
	for _, event := range result.Hits {
		if event.Category == "auth_fail" && !event.Observe {
			failed = true
			break
		}
	}
	if !failed {
		return
	}
	store := r.failureStore()
	now := time.Now().Unix()
	last := r.failureCleanup.Load()
	if now-last > 60 && r.failureCleanup.CompareAndSwap(last, now) {
		store.Cleanup(86400)
	}
	for _, minutes := range ext.windows {
		store.Incr(c.PortID, c.ClientIP.String()+":"+strconv.Itoa(minutes), minutes*60)
	}
}
func skipAttack(c *RequestContext, category string) bool {
	if category == "malformed" || category == "scanner" {
		return false
	}
	skips, _ := c.Values["rule_skips"].(map[string]bool)
	return skips["ALL"] || skips[strings.ToUpper(category)] || category == "lfi" && skips["DIR"]
}
func markRuleSkips(c *RequestContext, modules []string) {
	skips, _ := c.Values["rule_skips"].(map[string]bool)
	if skips == nil {
		skips = map[string]bool{}
		c.Values["rule_skips"] = skips
	}
	for _, module := range modules {
		skips[module] = true
	}
}
func (r *Runtime) withExtensions(s *Snapshot, steps []Step) []Step {
	steps = append(steps, antiLeechStep(s)...)
	if mode := s.Config.Policy.RuleSet.AI; mode != "" && mode != ModeOff {
		steps = append(steps, r.aiStep(s))
	}
	var additions []Step
	additions = append(additions, callbackStep(s)...)
	additions = append(additions, pluginStep(s)...)
	if s.Compiled.Extensions != nil {
		additions = append(additions, Step{Number: 3, Name: "custom_rules", Inspect: func(c *RequestContext) (Verdict, error) { return r.inspectExtensions(s, c) }})
	}
	additions = append(additions, sensitiveStep(s)...)
	if len(additions) == 0 {
		return steps
	}
	for i, step := range steps {
		if step.Name == "auth_gate" {
			out := append([]Step{}, steps[:i+1]...)
			out = append(out, additions...)
			return append(out, steps[i+1:]...)
		}
	}
	return append(steps, additions...)
}
func matchesURLPolicy(rule URLRule, candidate string) bool {
	value := strings.ToLower(rule.Value)
	switch rule.Match {
	case "exact":
		return candidate == value
	case "prefix":
		return strings.HasPrefix(candidate, value)
	case "suffix":
		return strings.HasSuffix(candidate, value)
	case "contains":
		return strings.Contains(candidate, value)
	}
	return false
}

func (r *Runtime) inspectExtensions(s *Snapshot, c *RequestContext) (Verdict, error) {
	urlValue := strings.ToLower(c.Request.URL.RequestURI())
	canonicalURL := strings.ToLower(c.View.Path)
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		canonicalURL += "?" + strings.ToLower(c.Request.URL.RawQuery)
	}
	allowed := false
	for _, rule := range s.Config.Policy.RuleSet.URLs {
		if rule.Mode == ModeOff {
			continue
		}
		// Match denials against both views so encoded paths cannot evade them.
		// Allow rules retain their raw-URI semantics: normalization must never
		// create a new permission or a broader attack-detection exception.
		matched := matchesURLPolicy(rule, urlValue) || rule.Action == Deny && matchesURLPolicy(rule, canonicalURL)
		if !matched {
			continue
		}
		observe := rule.Mode == ModeObserve
		c.Hit("url_policy", rule.ID, Medium, observe || rule.Action == Allow, "configured URL policy matched")
		if rule.Action == Deny && !observe {
			return Verdict{Action: Deny, Reason: "url_deny", RuleID: rule.ID, Status: 403}, nil
		}
		if rule.Action == Allow && !observe {
			allowed = true
		}
	}
	if allowed {
		markRuleSkips(c, []string{"ALL"})
		return Verdict{Action: Continue, Reason: "url_allow"}, nil
	}
	if skipAttack(c, "custom_rules") {
		return Verdict{Action: Continue, Reason: "attack_exception"}, nil
	}
	ext := s.Compiled.Extensions
	if len(ext.scripts) == 0 {
		return Verdict{Action: Continue}, nil
	}
	if c.View.deadline.IsZero() {
		c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
	}
	c.View.scanContext = c.Context
	needsBody, needsGeo := false, false
	for _, script := range ext.scripts {
		needsBody = needsBody || script.body
		needsGeo = needsGeo || script.geo
	}
	if needsBody {
		if err := inspectRequestBody(c); err != nil {
			return Verdict{}, errors.New("custom rule body inspection unavailable")
		}
	}
	facts := &scriptFacts{HOST: c.Request.Host, URL: c.Request.URL.RequestURI(), REFERER: c.Request.Referer(), USER_AGENT: c.Request.UserAgent(), METHOD: c.Request.Method, COOKIES: c.Request.Header.Get("Cookie"), BODY: string(c.View.Body), PORT: strconv.Itoa(int(s.Config.Port)), SRC_IP: c.ClientIP.String(), CONTENT_LENGTH: c.Request.ContentLength, headers: c.Request.Header, failures: func(minutes int64) int64 {
		return r.failureStore().Peek(c.PortID, c.ClientIP.String()+":"+strconv.FormatInt(minutes, 10), int(minutes*60)).Count
	}}
	if value, ok := c.Values["bot_verified"].(bool); ok {
		facts.verified = value
	}
	if needsGeo && r.Geo != nil {
		geo := r.Geo.Lookup(c.ClientIP)
		if geo.OK {
			facts.COUNTRY, facts.PROVINCE, facts.CITY = geo.Country, geo.Province, geo.City
		}
	}
	ctx, cancel := context.WithDeadline(c.Context, c.View.deadline)
	defer cancel()
	type winner struct {
		script         ScriptRule
		action         utils.RuleActionInfo
		priority, rank int
	}
	var best *winner
	for _, script := range ext.scripts {
		if script.geo && facts.COUNTRY == "" {
			continue
		}
		matches, err := script.helper.MatchFactsContext(ctx, "MF", facts, &scriptFunctions{innerbean.NewRuleFunc(), ext.groups})
		if err != nil {
			c.Hit("detector_error", script.rule.ID, Low, true, "custom rule evaluation unavailable")
			return Verdict{}, errors.New("custom rule evaluation unavailable")
		}
		for _, match := range matches {
			action := script.helper.GetRuleAction(match.RuleName)
			observe := script.rule.Mode == ModeObserve || action.Action == utils.RuleActionLog
			if observe {
				c.Hit("custom_rule", script.rule.ID, script.rule.Severity, true, "custom rule observed")
				continue
			}
			rank := 1
			if action.Action == utils.RuleActionDeny || action.Action == utils.RuleActionBan {
				rank = 2
			}
			candidate := &winner{script.rule, action, int(match.Salience), rank}
			if best == nil || candidate.priority > best.priority || candidate.priority == best.priority && (candidate.rank > best.rank || candidate.rank == best.rank && (candidate.action.BanMinutes > best.action.BanMinutes || candidate.action.BanMinutes == best.action.BanMinutes && candidate.script.ID < best.script.ID)) {
				best = candidate
			}
		}
	}
	if ctx.Err() != nil {
		c.Hit("detector_timeout", "custom_rules", Low, true, "request inspection budget exhausted")
		return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
	}
	if best == nil {
		return Verdict{Action: Continue}, nil
	}
	if best.action.Action == utils.RuleActionAllow {
		markRuleSkips(c, best.action.SkipModules)
		return Verdict{Action: Continue, Reason: "custom_rule_allow", RuleID: best.script.ID}, nil
	}
	before := len(c.Hits)
	c.Hit("custom_rule", best.script.ID, best.script.Severity, false, "custom rule matched")
	if best.action.Action == utils.RuleActionBan && len(c.Hits) > before {
		c.Hits[len(c.Hits)-1].BanSeconds = int(min(max(best.action.BanMinutes, 1), 10080)) * 60
	}
	return Verdict{Action: Deny, Reason: "custom_rule", RuleID: best.script.ID, Status: 403}, nil
}
