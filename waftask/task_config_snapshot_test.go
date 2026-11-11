package waftask

import (
	"HomeWaf/global"
	"testing"
)

func TestPreparedSettingsPublishBeforeReloadEffects(t *testing.T) {
	old := global.CurrentSettings()
	defer global.UpdateSettings(func(cfg *global.RuntimeSettings) { *cfg = old })
	ch := withHTTP3Chan(t)
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GCONFIG_ENABLE_HTTP3 = 0
		cfg.GCONFIG_BODY_DETECT_MODE = "observe"
	})
	before := global.CurrentSettings()
	candidate := before
	var effects []func()
	applyConfigIntValue(&candidate, &effects, "http3", 1, 1)
	applyConfigStringValue(&candidate, &effects, "body_detect_mode", "block", 1)
	applyConfigStringValue(&candidate, &effects, "body_detect_field_exclude", "notes", 1)
	if len(ch) != 0 || global.CurrentSettings().GCONFIG_ENABLE_HTTP3 != 0 || global.CurrentSettings().GCONFIG_BODY_DETECT_MODE != "observe" {
		t.Fatal("preparation published partial values or signalled early")
	}
	global.CommitSettings(before, candidate)
	for _, effect := range effects {
		effect()
	}
	if len(ch) != 1 || global.CurrentSettings().GCONFIG_ENABLE_HTTP3 != 1 || global.CurrentSettings().GCONFIG_BODY_DETECT_MODE != "block" || !global.BodyFieldExcluded(nil, "notes") {
		t.Fatal("consumer notified before complete snapshot/matcher publication")
	}
}
