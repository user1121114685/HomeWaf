package wafpipeline

import (
	"HomeWaf/wafenginecore/ipset"
	"errors"
	"net"
)

type CallbackPolicy struct {
	Mode     Mode     `json:"mode"`
	Networks []string `json:"networks"`
	Scope    Scope    `json:"scope"`
}

func validateCallback(p *CallbackPolicy) error {
	if p == nil {
		return nil
	}
	if !ValidMode(p.Mode) || len(p.Networks) > 1024 {
		return errors.New("invalid callback policy")
	}
	if p.Mode != ModeOff && len(p.Scope.Paths) == 0 {
		return errors.New("callback exceptions require an explicit path scope")
	}
	for _, value := range p.Networks {
		if _, err := ipset.ParsePattern(value); err != nil {
			return errors.New("invalid callback address")
		}
	}
	return ValidateScope(p.Scope)
}
func callbackStep(s *Snapshot) []Step {
	p := s.Config.Policy.RuleSet.Callback
	if p == nil || p.Mode == ModeOff {
		return nil
	}
	matcher := ipset.BuildMatchSet(p.Networks)
	return []Step{{Number: 3, Name: "callback_exception", Inspect: func(c *RequestContext) (Verdict, error) {
		if !scopeMatches(p.Scope, c.Request) || !matcher.Contains(net.IP(c.ClientIP.AsSlice())) {
			return Verdict{Action: Continue}, nil
		}
		c.Hit("callback_exception", "callback_exception", Low, true, "explicit callback source and path matched")
		if p.Mode == ModeBlock {
			markRuleSkips(c, []string{"ALL"})
		}
		return Verdict{Action: Continue, Reason: "callback_exception", Observe: p.Mode == ModeObserve}, nil
	}}}
}
