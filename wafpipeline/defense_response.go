package wafpipeline

import (
	"net/http"
	"time"
)

func (r *Runtime) defenseResponse(s *Snapshot, c *RequestContext, w http.ResponseWriter, v Verdict) bool {
	d := s.Config.Policy.Defense
	if d == nil || !d.Enabled || c.DryRun || !DefenseFailure(c, v) {
		return false
	}
	if d.Mode == DefenseHoney {
		for key := range w.Header() {
			w.Header().Del(key)
		}
		c.Values["defense_response"] = true
		if !SharedHoneypotReady() {
			c.Values["response_verdict"] = Verdict{Action: Deny, Stage: v.Stage, Status: 503, Reason: "defense:honeypot_unavailable"}
			http.Error(w, "Protection unavailable", 503)
			return true
		}
		v.Action, v.Status = Honeypot, 200
		if r.Respond != nil {
			return r.Respond(s, c, w, v)
		}
		panic(http.ErrAbortHandler)
	}
	state := ActiveDefense(s.Config, time.Now().Unix())
	if state == nil || state.Mode != DefenseQuiet {
		return false
	}
	c.Values["defense_response"] = true
	v.Action, v.Status = Drop, 0
	c.Trace.Add(TraceEntry{Stage: 4, Check: "defense:quiet", Verdict: v})
	c.Values["response_verdict"] = v
	if d.TCPAction == "drop" {
		if r.SilentDrop == nil {
			c.Hit("defense_transport_error", "silent_drop", Low, true, "silent drop provider unavailable")
		} else if err := r.SilentDrop(c.Request); err != nil {
			c.Hit("defense_transport_error", "silent_drop", Low, true, "silent drop failed; connection aborted; check runtime status")
		}
	}
	AbortDefenseConnection(w, c.Request)
	return true
}
