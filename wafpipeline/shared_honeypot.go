package wafpipeline

import (
	"encoding/json"
	"sync/atomic"
)

type SharedHoneypotConfig struct {
	Revision   int64          `json:"revision"`
	Configured bool           `json:"configured"`
	Config     HoneypotConfig `json:"config"`
}

var sharedHoneypot atomic.Pointer[SharedHoneypotConfig]

func PublishSharedHoneypot(value *SharedHoneypotConfig) {
	if value == nil {
		sharedHoneypot.Store(nil)
		return
	}
	raw, _ := json.Marshal(value)
	var copy SharedHoneypotConfig
	_ = json.Unmarshal(raw, &copy)
	sharedHoneypot.Store(&copy)
}

func SharedHoneypotReady() bool { s := sharedHoneypot.Load(); return s != nil && s.Configured }

func HoneypotPolicy(s *Snapshot) HoneypotConfig {
	if d := s.Config.Policy.Defense; d != nil {
		if d.Enabled && d.Mode == DefenseHoney {
			if shared := sharedHoneypot.Load(); shared != nil && shared.Configured {
				return shared.Config
			}
		}
		return HoneypotConfig{}
	}
	return s.Config.Policy.Honeypot
}

// A shared honeypot owns presentation and resource limits, not port lockdowns.
func SharedHoneypotPolicy(c HoneypotConfig) HoneypotConfig {
	c.Enabled = true
	c.CaptureDenied = false
	c.DropAfterHits = 0
	c.DistributedDrop.Enabled = false
	return effectiveHoneypot(c)
}
