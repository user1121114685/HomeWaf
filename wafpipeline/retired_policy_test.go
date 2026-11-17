package wafpipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRetiredAIPolicyPreservesStoredServicesAndStrictValidation(t *testing.T) {
	config := PortConfig{Schema: 1, Port: 15555, Protocol: "http", ListenEnabled: true, Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}
	wire, _ := json.Marshal(config)
	var old map[string]any
	_ = json.Unmarshal(wire, &old)
	rules := old["policy"].(map[string]any)["ruleset"].(map[string]any)
	rules["ai"] = "block"
	rules["ai_scope"] = map[string]any{"paths": []string{"/"}}
	wire, _ = json.Marshal(old)
	var restored PortConfig
	if err := StrictDecode(wire, &restored); err != nil {
		t.Fatalf("persisted service failed to load: %v", err)
	}
	if !restored.ListenEnabled || restored.Port != config.Port || len(restored.Policy.RuleSet.Rules) != len(config.Policy.RuleSet.Rules) {
		t.Fatal("retirement changed retained protection")
	}
	output, _ := json.Marshal(restored)
	if strings.Contains(string(output), `"ai"`) || strings.Contains(string(output), `"ai_scope"`) {
		t.Fatal("retired settings were re-exposed")
	}
	rules["unknown_detector_option"] = true
	wire, _ = json.Marshal(old)
	if err := StrictDecode(wire, &restored); err == nil {
		t.Fatal("retirement weakened strict field validation")
	}
}
