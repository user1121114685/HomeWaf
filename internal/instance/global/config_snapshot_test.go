package global

import (
	"context"
	"sync"
	"testing"
)

func TestRuntimeSettingsAreCoherentAndRequestPinned(t *testing.T) {
	old := CurrentSettings()
	defer UpdateSettings(func(cfg *RuntimeSettings) { *cfg = old })
	UpdateSettings(func(cfg *RuntimeSettings) {
		cfg.GCONFIG_RECORD_ENABLE_OWASP = 0
		cfg.GCONFIG_OWASP_MODE = "DetectionOnly"
		cfg.GCONFIG_BODY_DETECT_FIELD_EXCLUDE = "notes"
	})
	pinned := WithSettings(context.Background())
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < 500; i++ {
			UpdateSettings(func(cfg *RuntimeSettings) {
				cfg.GCONFIG_RECORD_ENABLE_OWASP = int64(i % 2)
				if i%2 == 0 {
					cfg.GCONFIG_OWASP_MODE = "DetectionOnly"
				} else {
					cfg.GCONFIG_OWASP_MODE = "On"
				}
				cfg.GCONFIG_BODY_DETECT_FIELD_EXCLUDE = "content"
			})
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < 1000; i++ {
			cfg := CurrentSettings()
			if (cfg.GCONFIG_RECORD_ENABLE_OWASP == 0) != (cfg.GCONFIG_OWASP_MODE == "DetectionOnly") {
				t.Error("reader saw partial publication")
				return
			}
			request := SettingsForContext(pinned)
			if request.GCONFIG_RECORD_ENABLE_OWASP != 0 || request.GCONFIG_OWASP_MODE != "DetectionOnly" || !BodyFieldExcluded(pinned, "Notes") || BodyFieldExcluded(pinned, "content") {
				t.Error("request snapshot changed during reload")
				return
			}
		}
	}()
	group.Wait()
	var retained *RuntimeSettings
	UpdateSettings(func(cfg *RuntimeSettings) { retained = cfg; cfg.GCONFIG_OWASP_MODE = "DetectionOnly" })
	retained.GCONFIG_OWASP_MODE = "mutated"
	copy := CurrentSettings()
	copy.GCONFIG_OWASP_MODE = "other"
	if CurrentSettings().GCONFIG_OWASP_MODE != "DetectionOnly" {
		t.Fatal("caller mutated published settings")
	}
	before := CurrentSettings()
	candidate := before
	candidate.GCONFIG_RECORD_HIDE_SERVER_HEADER = 0
	UpdateSettings(func(cfg *RuntimeSettings) { cfg.GCONFIG_RECORD_PROXY_LOOP_MAX_HOP = 23 })
	CommitSettings(before, candidate)
	if CurrentSettings().GCONFIG_RECORD_PROXY_LOOP_MAX_HOP != 23 || CurrentSettings().GCONFIG_RECORD_HIDE_SERVER_HEADER != 0 {
		t.Fatal("prepared reload lost unrelated concurrent edit")
	}
}
