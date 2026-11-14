package wafpipeline

import (
	"net/netip"
	"time"
)

func (p *Punisher) honeypotHit(s *Snapshot, e HitEvent, now time.Time) []PunishmentChange {
	cfg := effectiveHoneypot(s.Config.Policy.Honeypot)
	if !cfg.Enabled {
		return nil
	}
	var changes []PunishmentChange
	prefix := prefixFor(e.ClientIP, s.Config.Policy.HitPolicy.IPv6Prefix)
	if cfg.DropAfterHits > 0 {
		n := p.count(e.PortID, prefix.String(), "honeypot_visits", e.ReqID, "", cfg.DropWindowSeconds, false, s.Config.Policy.HitPolicy.MaxKeys, now)
		id := recordKey("drop", "port", e.PortID, prefix.String())
		if n >= cfg.DropAfterHits && !activeDrop(p.records[id], now) {
			r := PunishmentRecord{ID: id, Kind: "drop", Scope: "port", PortID: e.PortID, Prefix: prefix.String(), Reason: "honeypot_hit_threshold", RuleID: "honeypot", Hits: n, CreatedAt: now.Unix(), ExpiresAt: now.Add(time.Duration(cfg.DropSeconds) * time.Second).Unix()}
			if p.put(r) {
				changes = append(changes, PunishmentChange{r, "honeypot_drop_created"})
			}
		}
	}
	d := cfg.DistributedDrop
	if !d.Enabled {
		return changes
	}
	state := p.port(e.PortID)
	if now.Before(state.honeyCooldown) {
		return changes
	}
	if state.honeyDistinct == nil {
		state.honeyDistinct = map[netip.Addr]time.Time{}
	}
	for ip, seen := range state.honeyDistinct {
		if now.Sub(seen) >= time.Duration(d.WindowSeconds)*time.Second {
			delete(state.honeyDistinct, ip)
		}
	}
	key := prefix.Addr()
	if _, exists := state.honeyDistinct[key]; exists || len(state.honeyDistinct) < 1000 {
		state.honeyDistinct[key] = now
	}
	if len(state.honeyDistinct) < d.DistinctIPs {
		return changes
	}
	id := recordKey("honeypot_lockdown", "port", e.PortID, "")
	if activeDrop(p.records[id], now) {
		return changes
	}
	r := PunishmentRecord{ID: id, Kind: "honeypot_lockdown", Scope: "port", PortID: e.PortID, Reason: "honeypot_distinct_sources", RuleID: "honeypot", Hits: len(state.honeyDistinct), CreatedAt: now.Unix(), ExpiresAt: now.Add(time.Duration(d.Seconds) * time.Second).Unix(), CooldownUntil: now.Add(time.Duration(d.Seconds+60) * time.Second).Unix()}
	if p.put(r) {
		changes = append(changes, PunishmentChange{r, "honeypot_lockdown_started"})
	}
	return changes
}

func activeDrop(r PunishmentRecord, now time.Time) bool { return r.ID != "" && active(r, now.Unix()) }

// HoneypotDropReason is read before blacklist-to-honeypot response routing.
// These records are always port scoped; normal bans cannot weaken a drop.
func (p *Punisher) HoneypotDropReason(port string, ip netip.Addr) string {
	v := p.state.Load()
	if v == nil || len(v.HoneyLocks) == 0 && len(v.Drops) == 0 {
		return ""
	}
	now := p.Now().Unix()
	if r, ok := v.HoneyLocks[port]; ok && active(r, now) {
		return r.Reason
	}
	if r, ok := lookupState(v.Drops, v.Widths, port, ip, now); ok {
		return r.Reason
	}
	return ""
}

func (p *Punisher) releaseHoneyLock(r PunishmentRecord, now time.Time, cooldownUntil int64) {
	state := p.port(r.PortID)
	state.honeyDistinct = nil
	state.honeyCooldown = time.Unix(cooldownUntil, 0)
	if cooldownUntil > now.Unix() {
		r.Kind = "honeypot_cooldown"
		r.ID = recordKey(r.Kind, "port", r.PortID, "")
		r.ExpiresAt = cooldownUntil
		p.put(r)
	}
}
