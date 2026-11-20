package wafpipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type PluginPolicy struct {
	Mode          Mode     `json:"mode"`
	IDs           []string `json:"ids"`
	Scope         Scope    `json:"scope"`
	TimeoutMillis int      `json:"timeout_ms"`
	Generation    uint64   `json:"generation"`
}
type PluginLaunch struct {
	Binary string         `json:"binary"`
	Params map[string]any `json:"params"`
}
type PluginRequest struct{ PortID, RequestID, IP, Path, UserAgent, Method, Host string }
type PluginCheck struct {
	ID    string
	Check func(context.Context, PluginRequest) (bool, error)
}
type PreparedPlugins struct {
	Digest string
	Checks []PluginCheck
	Stop   func()
	once   sync.Once
	closed atomic.Bool
}

func (p *PreparedPlugins) Close() {
	if p != nil {
		p.once.Do(func() {
			p.closed.Store(true)
			if p.Stop != nil {
				p.Stop()
			}
		})
	}
}
func (p *PreparedPlugins) Closed() bool { return p == nil || p.closed.Load() }

func ValidatePluginProfiles(profiles map[string]PluginLaunch) error {
	if len(profiles) > 4 {
		return errors.New("each port supports at most four plugins")
	}
	raw, err := json.Marshal(profiles)
	if err != nil {
		return errors.New("plugin profiles must contain JSON values")
	}
	if len(raw) > 256<<10 {
		return errors.New("plugin configuration exceeds quota")
	}
	for id, p := range profiles {
		if id == "" || len(id) > 128 || p.Binary == "" || len(p.Binary) > 4096 || strings.ContainsAny(p.Binary, "\x00\r\n") {
			return errors.New("invalid plugin profile")
		}
	}
	return nil
}
func validatePluginPolicy(p *PluginPolicy) error {
	if p == nil {
		return nil
	}
	if !ValidMode(p.Mode) || len(p.IDs) > 4 || p.TimeoutMillis < 0 || p.TimeoutMillis > 1000 || p.Mode != ModeOff && len(p.IDs) == 0 {
		return errors.New("invalid port plugin policy")
	}
	seen := map[string]bool{}
	for _, id := range p.IDs {
		if id == "" || len(id) > 128 || seen[id] {
			return errors.New("invalid or repeated plugin ID")
		}
		seen[id] = true
	}
	return ValidateScope(p.Scope)
}
func pluginStep(s *Snapshot) []Step {
	p := s.Config.Policy.RuleSet.Plugins
	if p == nil || p.Mode == ModeOff {
		return nil
	}
	return []Step{{Number: 3, Name: "plugins", Inspect: func(c *RequestContext) (Verdict, error) {
		if !scopeMatches(p.Scope, c.Request) || skipAttack(c, "plugin") {
			return Verdict{Action: Continue}, nil
		}
		if c.DryRun {
			return Verdict{Action: Continue, Observe: true, Reason: "plugin_not_executed_in_dry_run"}, nil
		}
		prepared := s.Compiled.Plugins
		if prepared.Closed() {
			return Verdict{}, errors.New("port plugin unavailable")
		}
		if c.View.deadline.IsZero() {
			c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
		}
		deadline := c.View.deadline
		if p.TimeoutMillis > 0 {
			candidate := time.Now().Add(time.Duration(p.TimeoutMillis) * time.Millisecond)
			if candidate.Before(deadline) {
				deadline = candidate
			}
		}
		ctx, cancel := context.WithDeadline(c.Context, deadline)
		defer cancel()
		request := PluginRequest{PortID: c.PortID, RequestID: c.ReqID, IP: c.ClientIP.String(), Path: c.View.Path, UserAgent: c.Request.UserAgent(), Method: c.Request.Method, Host: c.Request.Host}
		for _, checker := range prepared.Checks {
			matched, err := checker.Check(ctx, request)
			if err != nil || ctx.Err() != nil {
				c.Hit("plugin_error", checker.ID, Low, true, "local plugin unavailable or timed out")
				if ctx.Err() != nil {
					break
				}
				continue
			}
			if matched {
				observe := p.Mode == ModeObserve
				c.Hit("plugin_block", checker.ID, High, observe, "configured port plugin matched")
				return Verdict{Action: Deny, Status: 403, Reason: "plugin:" + checker.ID, RuleID: checker.ID, Observe: observe}, nil
			}
		}
		return Verdict{Action: Continue}, nil
	}}}
}
