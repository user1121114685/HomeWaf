package api

import (
	"HomeWaf/internal/protection/wafpipeline"
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestSingleAssessmentRouteBindsOneRequestAndRevision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := wafpipeline.PortConfig{Schema: 1, Port: 16348, Protocol: "http", Upstream: "http://127.0.0.1:18080", Revision: 7, Policy: wafpipeline.DefaultPolicy()}
	s := &wafpipeline.Snapshot{Config: cfg}
	if err := wafpipeline.Ports.Publish(cfg.Port, func() (*wafpipeline.Snapshot, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	defer wafpipeline.Ports.Remove(cfg.Port)
	route := gin.New()
	route.POST("/assessment/single", func(c *gin.Context) { c.Set("is_openapi", true); new(WafPipelineApi).AssessSingle(c) })
	in := map[string]any{"port": cfg.Port, "expected_revision": cfg.Revision, "request": wafpipeline.SingleRequest{SourceIP: "192.0.2.25", SourcePort: 5555, Method: "GET", Path: "/", Version: "HTTP/1.1"}}
	call := func() map[string]any {
		raw, _ := json.Marshal(in)
		w := httptest.NewRecorder()
		route.ServeHTTP(w, httptest.NewRequest("POST", "/assessment/single", bytes.NewReader(raw)))
		var reply map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
			t.Fatal(err)
		}
		return reply
	}
	ok := call()
	if ok["code"] != float64(0) {
		t.Fatal(ok)
	}
	data := ok["data"].(map[string]any)
	decision := data["decision"].(map[string]any)
	if data["job"] != nil || decision["peer"] != "192.0.2.25:5555" || decision["target"] != "http://127.0.0.1:16348" {
		t.Fatal(data)
	}
	in["pressure"] = 100
	if call()["code"] == float64(0) {
		t.Fatal("bulk fields accepted")
	}
	delete(in, "pressure")
	in["expected_revision"] = 6
	if call()["code"] == float64(0) {
		t.Fatal("stale draft accepted")
	}
}
