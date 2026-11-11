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
		cfg.GCONFIG_AI_ENABLE = 0
		cfg.GCONFIG_AI_MODE = "observe"
		cfg.GCONFIG_BODY_DETECT_FIELD_EXCLUDE = "notes"
	})
	pinned := WithSettings(context.Background())
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < 500; i++ {
			UpdateSettings(func(cfg *RuntimeSettings) {
				cfg.GCONFIG_AI_ENABLE = int64(i % 2)
				if i%2 == 0 {
					cfg.GCONFIG_AI_MODE = "observe"
				} else {
					cfg.GCONFIG_AI_MODE = "block"
				}
				cfg.GCONFIG_BODY_DETECT_FIELD_EXCLUDE = "content"
			})
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < 1000; i++ {
			cfg := CurrentSettings()
			if (cfg.GCONFIG_AI_ENABLE == 0) != (cfg.GCONFIG_AI_MODE == "observe") {
				t.Error("reader saw partial publication")
				return
			}
			request := SettingsForContext(pinned)
			if request.GCONFIG_AI_ENABLE != 0 || request.GCONFIG_AI_MODE != "observe" || !BodyFieldExcluded(pinned, "Notes") || BodyFieldExcluded(pinned, "content") {
				t.Error("request snapshot changed during reload")
				return
			}
		}
	}()
	group.Wait()
	var retained *RuntimeSettings
	UpdateSettings(func(cfg *RuntimeSettings) { retained = cfg; cfg.GCONFIG_AI_MODE = "observe" })
	retained.GCONFIG_AI_MODE = "mutated"
	copy := CurrentSettings()
	copy.GCONFIG_AI_MODE = "other"
	if CurrentSettings().GCONFIG_AI_MODE != "observe" {
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
