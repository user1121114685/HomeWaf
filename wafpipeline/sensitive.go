package wafpipeline

import (
	"HomeWaf/internal/multimatch"
	"HomeWaf/model"
	"bytes"
	"errors"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SensitivePolicy struct {
	Mode        Mode              `json:"mode"`
	Replacement string            `json:"replacement"`
	Rules       []model.Sensitive `json:"rules"`
}
type sensitiveDirection struct{ deny, replace *multimatch.Machine }
type compiledSensitive struct {
	in, out     sensitiveDirection
	mode        Mode
	replacement string
}

func compileSensitive(p *SensitivePolicy) (*compiledSensitive, error) {
	if p == nil {
		return nil, nil
	}
	if !ValidMode(p.Mode) || len(p.Rules) > 5000 || len(p.Replacement) > 256 {
		return nil, errors.New("invalid sensitive policy or quota")
	}
	sets := map[string][]model.Sensitive{"in:deny": {}, "in:replace": {}, "out:deny": {}, "out:replace": {}}
	total := 0
	for _, rule := range p.Rules {
		total += len(rule.Content)
		if rule.Content == "" || len(rule.Content) > 4096 || total > 256<<10 || (rule.Action != "deny" && rule.Action != "replace") || (rule.CheckDirection != "in" && rule.CheckDirection != "out" && rule.CheckDirection != "all") {
			return nil, errors.New("invalid sensitive rule")
		}
		for _, direction := range []string{"in", "out"} {
			if rule.CheckDirection == direction || rule.CheckDirection == "all" {
				key := direction + ":" + rule.Action
				sets[key] = append(sets[key], rule)
			}
		}
	}
	build := func(key string) *multimatch.Machine {
		rows := sets[key]
		if len(rows) == 0 {
			return nil
		}
		words := make([][]rune, 0, len(rows))
		data := map[string]any{}
		for _, row := range rows {
			words = append(words, []rune(row.Content))
			data[row.Content] = row
		}
		m := &multimatch.Machine{}
		_ = m.BuildByCustom(words, data)
		return m
	}
	return &compiledSensitive{in: sensitiveDirection{build("in:deny"), build("in:replace")}, out: sensitiveDirection{build("out:deny"), build("out:replace")}, mode: p.Mode, replacement: p.Replacement}, nil
}
func (d sensitiveDirection) match(value string) (bool, []string) {
	if len(d.deny.MultiPatternSearch([]rune(value), true)) > 0 {
		return true, nil
	}
	hits := d.replace.MultiPatternSearch([]rune(value), false)
	words := make([]string, 0, len(hits))
	for _, hit := range hits {
		words = append(words, string(hit.Word))
	}
	sort.Slice(words, func(i, j int) bool {
		if len(words[i]) == len(words[j]) {
			return words[i] < words[j]
		}
		return len(words[i]) > len(words[j])
	})
	return false, words
}
func sensitiveReplace(value string, words []string, replacement string, limit int) (string, bool) {
	bound := len(value)
	for _, word := range words {
		if len(replacement) > len(word) {
			bound += strings.Count(value, word) * (len(replacement) - len(word))
			if bound > limit {
				return value, false
			}
		}
	}
	pairs := make([]string, 0, len(words)*2)
	for _, word := range words {
		pairs = append(pairs, word, replacement)
	}
	if len(pairs) == 0 {
		return value, true
	}
	return strings.NewReplacer(pairs...).Replace(value), true
}
func sensitiveStep(s *Snapshot) []Step {
	p := s.Compiled.Sensitive
	if p == nil || p.mode == ModeOff || p.in.deny == nil && p.in.replace == nil {
		return nil
	}
	return []Step{{Number: 3, Name: "sensitive_request", Inspect: func(c *RequestContext) (Verdict, error) {
		if skipAttack(c, "sensitive") {
			return Verdict{Action: Continue}, nil
		}
		if c.View.deadline.IsZero() {
			c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
		}
		c.View.scanContext = c.Context
		if err := inspectRequestBody(c); err != nil {
			return Verdict{}, err
		}
		deny, words := p.in.match(NormalizeValue(c.Request.URL.RequestURI()))
		bodyDeny, bodyWords := p.in.match(string(c.View.Body))
		deny = deny || bodyDeny
		if c.View.expired() {
			return Verdict{}, errors.New("sensitive inspection budget exhausted")
		}
		if !deny && len(words) == 0 && len(bodyWords) == 0 {
			return Verdict{Action: Continue}, nil
		}
		observe := p.mode == ModeObserve
		c.Hit("sensitive_word", "sensitive_request", Medium, observe, "configured request content policy matched")
		if deny {
			return Verdict{Action: Deny, Status: 403, Observe: observe, Reason: "sensitive_request"}, nil
		}
		if observe || c.DryRun {
			return Verdict{Action: Continue, Observe: observe, Reason: "sensitive_observed"}, nil
		}
		copyURL := *c.Request.URL
		changedPath, ok := sensitiveReplace(copyURL.Path, words, p.replacement, 8192)
		if !ok {
			return Verdict{}, errors.New("sensitive path replacement exceeded limit")
		}
		copyURL.Path = changedPath
		if changedPath != c.Request.URL.Path {
			copyURL.RawPath = ""
		}
		pairs := strings.Split(copyURL.RawQuery, "&")
		for i, pair := range pairs {
			key, value, has := strings.Cut(pair, "=")
			if !has {
				continue
			}
			decoded, e := url.QueryUnescape(value)
			if e != nil {
				continue
			}
			changed, ok := sensitiveReplace(decoded, words, p.replacement, 8192)
			if !ok {
				return Verdict{}, errors.New("sensitive query replacement exceeded limit")
			}
			if changed != decoded {
				pairs[i] = key + "=" + url.QueryEscape(changed)
			}
		}
		copyURL.RawQuery = strings.Join(pairs, "&")
		candidate := c.Request.Clone(c.Context)
		candidate.URL = &copyURL
		g := s.Config.Policy.Gate
		if changedPath != c.Request.URL.Path && (g.Enabled || g.RequireToken) {
			protected := len(g.Protect.Paths)+len(g.Protect.Methods)+len(g.Protect.Hosts) > 0
			if scopeMatches(g.Exempt, c.Request) != scopeMatches(g.Exempt, candidate) || protected && scopeMatches(g.Protect, c.Request) != scopeMatches(g.Protect, candidate) {
				return Verdict{Action: Deny, Status: 403, Reason: "sensitive_auth_scope_change"}, nil
			}
		}
		c.Request.URL = &copyURL
		c.Request.RequestURI = copyURL.RequestURI()
		if len(bodyWords) > 0 {
			if c.View.BodyTruncated || c.View.ParseError != "" || strings.HasPrefix(strings.ToLower(c.Request.Header.Get("Content-Type")), "multipart/") {
				c.Hit("sensitive_partial", "sensitive_request", Low, true, "request body replacement skipped: incomplete or multipart body")
			} else {
				body, ok := sensitiveReplace(string(c.View.Body), bodyWords, p.replacement, int(c.View.limits.BodyBytes))
				if !ok {
					return Verdict{}, errors.New("sensitive body replacement exceeded limit")
				}
				if body != string(c.View.Body) {
					_ = c.Request.Body.Close()
					c.Request.Body = io.NopCloser(bytes.NewBufferString(body))
					c.Request.ContentLength = int64(len(body))
					c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
					c.Request.Header.Del("Content-Encoding")
					c.Request.TransferEncoding = nil
					c.Request.GetBody = nil
					c.Request.Form = nil
					c.Request.PostForm = nil
				}
			}
		}
		deadline := c.View.deadline
		c.View = NewNormalizedView(c.Request, s.Config.Policy.RuleSet.View)
		c.View.deadline = deadline
		c.View.scanContext = c.Context
		return Verdict{Action: Continue, Reason: "sensitive_replaced"}, nil
	}}}
}

// Response processing always observes the original upstream bytes before edits.
func sensitiveResponse(s *Snapshot, c *RequestContext, body []byte) ([]byte, bool) {
	p := s.Compiled.Sensitive
	if p == nil || p.mode == ModeOff || c.Trusted || skipAttack(c, "sensitive") {
		return body, false
	}
	deny, words := p.out.match(string(body))
	if !deny && len(words) == 0 {
		return body, false
	}
	observe := p.mode == ModeObserve
	// An upstream leak is not evidence of a malicious visitor.
	c.Hit("sensitive_word", "sensitive_response", Medium, true, "configured response content policy matched")
	if observe {
		return body, false
	}
	if deny {
		return []byte("Response blocked by content policy"), true
	}
	out, ok := sensitiveReplace(string(body), words, p.replacement, 1<<20)
	if !ok {
		c.Hit("sensitive_partial", "sensitive_response", Low, true, "replacement quota exceeded")
		return body, false
	}
	return []byte(out), false
}

func sensitiveResponseActive(s *Snapshot, c *RequestContext) bool {
	p := s.Compiled.Sensitive
	return p != nil && p.mode != ModeOff && (p.out.deny != nil || p.out.replace != nil) && !c.Trusted && !skipAttack(c, "sensitive")
}
