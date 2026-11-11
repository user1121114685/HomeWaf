package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"container/list"
	"time"
)

const maxAccessActivity = 20000

type accessActivity struct {
	seen, queued time.Time
	node         *list.Element
}

// Activity never carries an authorization verdict. It only supplements the
// last-active timestamp of an independently validated, unexpired session.
func (s *WafAccessSessionService) sessionIdle(code string, persisted, now time.Time, idle time.Duration) bool {
	if idle <= 0 {
		return false
	}
	s.activityMu.Lock()
	if entry := s.activity["session_code:"+code]; entry != nil && entry.seen.After(persisted) {
		persisted = entry.seen
	}
	s.activityMu.Unlock()
	return !persisted.IsZero() && now.Sub(persisted) > idle
}

// Keep request writes in the existing bounded queue. No synchronous fallback:
// a missing/full consumer may lose activity telemetry, never block a request.
// Database predicates prevent stale ordering or a queued touch from reviving a
// revoked session. Positive authorization-cache lifetimes are not extended.
func (s *WafAccessSessionService) touchActivity(entity any, column, code string, persisted, now time.Time, interval time.Duration) {
	if code == "" || len(code) > 128 {
		return
	}
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	if s.activity == nil {
		s.activity = make(map[string]*accessActivity)
		s.activityOrder = list.New()
	}
	key := column + ":" + code
	entry := s.activity[key]
	if entry == nil {
		if len(s.activity) >= maxAccessActivity {
			old := s.activityOrder.Front()
			delete(s.activity, old.Value.(string))
			s.activityOrder.Remove(old)
		}
		entry = &accessActivity{seen: persisted, queued: persisted, node: s.activityOrder.PushBack(key)}
		s.activity[key] = entry
	} else {
		s.activityOrder.MoveToBack(entry.node)
	}
	if persisted.After(entry.queued) {
		entry.queued = persisted
	}
	if now.After(entry.seen) {
		entry.seen = now
	}
	if now.Sub(entry.queued) < interval || global.GQEQUE_UPDATE_DB == nil {
		return
	}
	stamp := customtype.JsonTime(now)
	update := innerbean.UpdateModel{
		Model:  entity,
		Query:  column + " = ? AND status = ? AND (last_active_time IS NULL OR last_active_time < ?)",
		Args:   []interface{}{code, model.AccessStatusValid, stamp},
		Update: map[string]interface{}{"last_active_time": stamp},
	}
	if global.GQEQUE_UPDATE_DB.TryEnqueue(update) {
		entry.queued = now
	}
}
