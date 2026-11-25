package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"HomeWaf/internal/protection/wafpipeline"
	"github.com/gin-gonic/gin"
)

func TestAssessmentAPIBindsServiceRevisionAndRejectsUnboundedInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := wafpipeline.PortConfig{Schema: 1, Port: 16345, Name: "fixture", Protocol: "http", Upstream: "http://127.0.0.1:18080", Policy: wafpipeline.DefaultPolicy(), Revision: 7}
	snapshot := &wafpipeline.Snapshot{Config: config}
	if err := wafpipeline.Ports.Publish(config.Port, func() (*wafpipeline.Snapshot, error) { return snapshot, nil }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { wafpipeline.Ports.Remove(config.Port) })
	r := gin.New()
	r.POST("/assessment", func(c *gin.Context) { c.Set("is_openapi", true); new(WafPipelineApi).AssessPort(c) })
	input := PipelineAssessmentRequest{Port: config.Port, AssessmentInput: wafpipeline.AssessmentInput{Builtins: true, Sources: []wafpipeline.AssessmentSource{{IP: "198.51.100.25", ExpectAccess: true}}}}
	call := func(input PipelineAssessmentRequest) int {
		t.Helper()
		raw, _ := json.Marshal(input)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/assessment", bytes.NewReader(raw)))
		var result struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Code
	}
	if code := call(input); code != 0 {
		t.Fatalf("valid service failed: %d", code)
	}
	copy := config
	copy.Port++
	input.Config = &copy
	if call(input) == 0 {
		t.Fatal("cross-service draft accepted")
	}
	copy = config
	copy.Revision--
	input.Config = &copy
	if call(input) == 0 {
		t.Fatal("stale draft accepted")
	}
	input.Config = nil
	input.Pressure = 2001
	if call(input) == 0 {
		t.Fatal("unbounded pressure accepted")
	}
	input.Pressure = 0
	input.ProfileID = "missing"
	if call(input) == 0 {
		t.Fatal("missing profile silently ignored")
	}
	current, _ := wafpipeline.Ports.Get(config.Port)
	if current != snapshot || current.Config.Revision != 7 {
		t.Fatal("assessment published or saved configuration")
	}
}
