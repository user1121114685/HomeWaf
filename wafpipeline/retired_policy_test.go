package wafpipeline

import (
	"encoding/json"
	"testing"
)

func TestRetiredAIPolicyIsRejected(t *testing.T) {
	config := PortConfig{Schema: 1, Port: 15555, Protocol: "http", ListenEnabled: true, Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}
	wire, _ := json.Marshal(config)
	var old map[string]any
	_ = json.Unmarshal(wire, &old)
	rules := old["policy"].(map[string]any)["ruleset"].(map[string]any)
	rules["ai"] = "block"
	rules["ai_scope"] = map[string]any{"paths": []string{"/"}}
	wire, _ = json.Marshal(old)
	var restored PortConfig
	if err := StrictDecode(wire, &restored); err == nil {
		t.Fatal("accepted retired AI detector settings")
	}
}
