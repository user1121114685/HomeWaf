package wafpipeline

import (
	"HomeWaf/wafai"
	"time"
)

type AIPredictor interface {
	PredictRequest(method, path, query, body, userAgent string) wafai.PredictResult
}

func (r *Runtime) aiStep(s *Snapshot) Step {
	return Step{Number: 3, Name: "local_ai", Inspect: func(c *RequestContext) (Verdict, error) {
		mode := s.Config.Policy.RuleSet.AI
		if mode == "" || mode == ModeOff || !scopeMatches(s.Config.Policy.RuleSet.AIScope, c.Request) || skipAttack(c, "ai") {
			return Verdict{Action: Continue}, nil
		}
		if c.View.deadline.IsZero() {
			c.View.deadline = time.Now().Add(time.Duration(s.Config.Policy.RuleSet.BudgetMillis) * time.Millisecond)
		}
		c.View.scanContext = c.Context
		if c.View.expired() {
			c.Hit("detector_timeout", "local_ai", Low, true, "inspection budget exhausted")
			return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
		}
		if r.AI == nil {
			c.Hit("detector_error", "local_ai", Low, true, "local model unavailable")
			return Verdict{Action: Continue, Reason: "ai_unavailable"}, nil
		}
		if err := inspectRequestBody(c); err != nil {
			c.Hit("detector_error", "local_ai", Low, true, "body inspection unavailable")
			return Verdict{Action: Continue, Reason: "body_read_error"}, nil
		}
		// Inference is local, pure Go and bounded to an inspection prefix; no
		// network/DB operation or background prediction goroutine is started.
		body := string(c.View.Body)
		prediction := r.AI.PredictRequest(c.Request.Method, bound(c.Request.URL.Path, 8192), bound(c.Request.URL.RawQuery, 16384), bound(body, 32768), bound(c.Request.UserAgent(), 2048))
		if c.View.expired() {
			c.Hit("detector_timeout", "local_ai", Low, true, "inspection budget exhausted")
			return Verdict{Action: Continue, Reason: "detector_budget_exhausted"}, nil
		}
		if !prediction.Loaded {
			c.Hit("detector_error", "local_ai", Low, true, "local model unavailable")
			return Verdict{Action: Continue, Reason: "ai_unavailable"}, nil
		}
		if prediction.Score < prediction.ObserveThreshold {
			return Verdict{Action: Continue}, nil
		}
		observe := mode == ModeObserve || prediction.Score < prediction.BlockThreshold
		c.Hit("ai", "local_ai", High, observe, "local anomaly model matched")
		return Verdict{Action: Deny, Reason: "rule:local_ai", RuleID: "local_ai", Status: 403, Observe: observe}, nil
	}}
}
