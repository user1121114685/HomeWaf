package waftask

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/protection/wafowasp"
	"testing"
)

func TestPreparedSettingsPublishBeforeReloadEffects(t *testing.T) {
	old := global.CurrentSettings()
	defer global.UpdateSettings(func(cfg *global.RuntimeSettings) { *cfg = old })
	oldMode := wafowasp.GetEngineMode()
	t.Cleanup(func() { wafowasp.SetEngineMode(oldMode) })
	wafowasp.SetEngineMode("On")
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GCONFIG_OWASP_MODE = "On"
		cfg.GCONFIG_BODY_DETECT_MODE = "observe"
	})
	before := global.CurrentSettings()
	candidate := before
	var effects []func()
	applyConfigStringValue(&candidate, &effects, "owasp_mode", "DetectionOnly", 1)
	applyConfigStringValue(&candidate, &effects, "body_detect_mode", "block", 1)
	applyConfigStringValue(&candidate, &effects, "body_detect_field_exclude", "notes", 1)
	if wafowasp.GetEngineMode() != "On" || global.CurrentSettings().GCONFIG_OWASP_MODE != "On" || global.CurrentSettings().GCONFIG_BODY_DETECT_MODE != "observe" {
		t.Fatal("preparation published partial values or signalled early")
	}
	global.CommitSettings(before, candidate)
	for _, effect := range effects {
		effect()
	}
	if wafowasp.GetEngineMode() != "DetectionOnly" || global.CurrentSettings().GCONFIG_OWASP_MODE != "DetectionOnly" || global.CurrentSettings().GCONFIG_BODY_DETECT_MODE != "block" || !global.BodyFieldExcluded(nil, "notes") {
		t.Fatal("consumer notified before complete snapshot/matcher publication")
	}
}
