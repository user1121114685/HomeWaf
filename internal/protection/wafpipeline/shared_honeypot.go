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

func HoneypotPolicy(s *Snapshot) (HoneypotConfig, bool) {
	d := s.Config.Policy.Defense
	shared := sharedHoneypot.Load()
	if d != nil && d.Enabled && d.Mode == DefenseHoney && shared != nil && shared.Configured {
		return shared.Config, true
	}
	return HoneypotConfig{}, false
}
func SharedHoneypotPolicy(c HoneypotConfig) HoneypotConfig { return c }
