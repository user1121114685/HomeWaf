package api

import (
	"HomeWaf/internal/protection/wafpipeline"
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAssessmentJobsOwnershipCancellationAndCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := wafpipeline.PortConfig{Schema: 1, Port: 16346, Protocol: "http", Upstream: "http://127.0.0.1:18080", Policy: wafpipeline.DefaultPolicy(), Revision: 4}
	if err := wafpipeline.Ports.Publish(config.Port, func() (*wafpipeline.Snapshot, error) { return &wafpipeline.Snapshot{Config: config}, nil }); err != nil {
		t.Fatal(err)
	}
	defer wafpipeline.Ports.Remove(config.Port)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("is_openapi", true); c.Set("openapi_key_id", c.GetHeader("X-Test-Owner")) })
	api := new(WafPipelineApi)
	r.POST("/start", api.AssessPort)
	r.GET("/job", api.AssessmentJob)
	r.POST("/cancel", api.CancelAssessment)
	call := func(method, path, owner string, input any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(input)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("X-Test-Owner", owner)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	input := PipelineAssessmentRequest{Port: config.Port, Background: true, AssessmentInput: wafpipeline.AssessmentInput{Builtins: true, Sources: []wafpipeline.AssessmentSource{{IP: "198.51.100.25", ExpectAccess: true}}}}
	if call("POST", "/start", "", input)["code"].(float64) == 0 {
		t.Fatal("ownerless task accepted")
	}
	started := call("POST", "/start", "a", input)
	if started["code"].(float64) != 0 {
		t.Fatal(started)
	}
	id := started["data"].(map[string]any)["job"].(map[string]any)["id"].(string)
	if call("GET", "/job?id="+id, "b", nil)["code"].(float64) == 0 {
		t.Fatal("cross-owner read")
	}
	if call("POST", "/cancel", "b", map[string]string{"id": id})["code"].(float64) == 0 {
		t.Fatal("cross-owner cancellation")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		result := call("GET", "/job?id="+id, "a", nil)["data"].(map[string]any)
		if result["state"] == "complete" {
			if result["result"] == nil {
				t.Fatal("missing result")
			}
			break
		}
		if result["state"] == "failed" || time.Now().After(deadline) {
			t.Fatal(result)
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	assessmentJobs.Lock()
	assessmentJobs.items["cancel-fixture"] = &assessmentJob{ID: "cancel-fixture", owner: "key:a", State: "running", cancel: cancel}
	assessmentJobs.Unlock()
	if call("POST", "/cancel", "a", map[string]string{"id": "cancel-fixture"})["code"].(float64) != 0 {
		t.Fatal("owner cannot cancel")
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("cancel did not reach execution")
	}
	assessmentJobs.Lock()
	delete(assessmentJobs.items, id)
	delete(assessmentJobs.items, "cancel-fixture")
	assessmentJobs.Unlock()
}
