package waf_service

import (
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/protection/wafpipeline"
	"HomeWaf/internal/storage/wafdb"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestLogDetailIncludesScopedSafeDetectionDiagnostics(t *testing.T) {
	_, db, _ := setupLogLookupTest(t)
	if err := db.AutoMigrate(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	policy := wafpipeline.DefaultPolicy()
	policy.RuleSet.Rules = []wafpipeline.Rule{{ID: "ssrf", Category: "ssrf", Mode: wafpipeline.ModeObserve, Severity: wafpipeline.High}}
	policy.RuleSet.BudgetMillis = 1000
	runtime := &wafpipeline.Runtime{RuleSteps: wafpipeline.DefaultRuleEngine().Steps}
	snapshot := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: 15000, Protocol: "http", Upstream: "http://upstream.test", Policy: policy}}
	r := httptest.NewRequest("GET", "http://example.test/fetch?url=http://127.0.0.1/?token=DO-NOT-EXPOSE", nil)
	result, err := runtime.Explain(snapshot, r, netip.MustParseAddr("192.0.2.1"))
	if err != nil || len(result.Hits) != 1 {
		t.Fatalf("runtime fixture: %+v %v", result, err)
	}
	raw, _ := json.Marshal(result)
	narrow := model.LogNarrow{ReqUUID: result.ReqID, TenantId: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, HostCode: "pipeline-15000", SRC_IP: "192.0.2.1", ACTION: "放行", RULE: "ssrf", LogOnlyMode: 1, STATUS_CODE: 200}
	if err := db.Create(&model.SecurityEvent{LogNarrow: narrow}).Error; err != nil {
		t.Fatal(err)
	}
	row := model.PipelineLog{ID: "correct", ReqID: result.ReqID, PortID: "port:15000", ClientIP: narrow.SRC_IP, Kind: "request", DetailJSON: string(raw)}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	foreign := row
	foreign.ID, foreign.PortID, foreign.TimeMillis, foreign.DetailJSON = "wrong-port", "port:15001", 100, `{"secret":"WRONG-PORT"}`
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	req := request.WafAttackLogDetailReq{REQ_UUID: result.ReqID, CurrrentDbName: wafdb.LiveLogName()}
	got, err := WafLogServiceApp.GetDetailApi(req)
	if err != nil || got.Diagnostics == nil || got.Diagnostics.State != "available" || len(got.Diagnostics.Hits) != 1 {
		t.Fatalf("missing persisted diagnostics: %+v %v", got.Diagnostics, err)
	}
	hit := got.Diagnostics.Hits[0]
	if hit.Category != "ssrf" || hit.Source != "query" || hit.Code != "internal_url" || !hit.Observe {
		t.Fatalf("wrong diagnostic metadata: %+v", hit)
	}
	encoded, _ := json.Marshal(got.Diagnostics)
	if strings.Contains(string(encoded), "DO-NOT-EXPOSE") || strings.Contains(string(encoded), "127.0.0.1") || strings.Contains(string(encoded), "WRONG-PORT") {
		t.Fatalf("request values leaked: %s", encoded)
	}
	// Arbitrary evidence and request values must never become shareable metadata.
	result.Hits[0].Evidence = "Cookie: session=PRIVATE-COOKIE; token=PRIVATE-TOKEN"
	idle := int64(900)
	result.BodyReadFailure = &bodyinspect.ReadFailure{Cause: bodyinspect.ReadTimeout, BytesRead: 1024, ElapsedMS: 1200, IdleMS: &idle}
	raw, _ = json.Marshal(result)
	if err := db.Exec("UPDATE pipeline_log SET detail_json = ? WHERE id = ?", string(raw), row.ID).Error; err != nil {
		t.Fatal(err)
	}
	got, err = WafLogServiceApp.GetDetailApi(req)
	encoded, _ = json.Marshal(got.Diagnostics)
	if err != nil || got.Diagnostics.Hits[0].Source != "" || strings.Contains(string(encoded), "PRIVATE-") ||
		got.Diagnostics.BodyReadFailure == nil || got.Diagnostics.BodyReadFailure.Cause != bodyinspect.ReadTimeout ||
		got.Diagnostics.BodyReadFailure.BytesRead != 1024 || *got.Diagnostics.BodyReadFailure.IdleMS != idle {
		t.Fatalf("unsafe detector evidence exposed: %s %v", encoded, err)
	}
	// A configured rule can have the same ID as the mandatory stage.
	result.Verdict = wafpipeline.Verdict{Action: wafpipeline.Deny, Stage: 3, Status: 503, Reason: "policy_unavailable"}
	result.Trace = []wafpipeline.TraceEntry{{Stage: 3, Check: "synology_login", Error: "detector unavailable", Verdict: result.Verdict}}
	failedHit := wafpipeline.HitEvent{ReqID: result.ReqID, PortID: row.PortID, Category: "detector_error", Check: "rule:synology_login", RuleID: "synology_login", Observe: true, Evidence: wafpipeline.BodyInspectionUnavailable}
	result.Hits = []wafpipeline.HitEvent{failedHit, failedHit}
	result.Hits[1].Check = "synology_login"
	raw, _ = json.Marshal(result)
	if err := db.Exec("UPDATE pipeline_log SET detail_json = ? WHERE id = ?", string(raw), row.ID).Error; err != nil {
		t.Fatal(err)
	}
	got, err = WafLogServiceApp.GetDetailApi(req)
	if err != nil || len(got.Diagnostics.Hits) != 2 || got.Diagnostics.Hits[0].StoppedRequest || !got.Diagnostics.Hits[1].StoppedRequest ||
		got.Diagnostics.Hits[1].Code != "body_read_timeout" || got.Diagnostics.Hits[1].Source != "body" {
		t.Fatalf("incorrect mandatory check attribution: %+v %v", got.Diagnostics, err)
	}
	if err := db.Exec("UPDATE security_event SET tenant_id = ? WHERE req_uuid = ?", "other-tenant", result.ReqID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := WafLogServiceApp.GetDetailApi(req); !errors.Is(err, ErrLogNotFound) {
		t.Fatalf("foreign request revealed diagnostics: %v", err)
	}
}

func TestRequestDiagnosticsDistinguishMissingInvalidAndReadFailure(t *testing.T) {
	_, db, _ := setupLogLookupTest(t)
	if err := db.AutoMigrate(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	log := &innerbean.WebLog{REQ_UUID: "diagnostic-request", HOST_CODE: "pipeline-15000", TenantId: global.GWAF_TENANT_ID, USER_CODE: global.GWAF_USER_CODE, SRC_IP: "192.0.2.1"}
	if got := requestDiagnostics(log); got.State != "missing" {
		t.Fatalf("missing != clean: %+v", got)
	}
	for _, tc := range []struct{ raw, state string }{
		{`{"req_id":"diagnostic-request"}`, "available"},
		{`{"req_id":"diagnostic-request","body_read_failure":{"cause":"PRIVATE","bytes_read":0,"elapsed_ms":1}}`, "invalid"},
		{`{"req_id":"diagnostic-request","body_read_failure":{"cause":"timeout","bytes_read":8,"elapsed_ms":5,"idle_ms":9}}`, "invalid"},
		{`{"req_id":"other-request"}`, "invalid"},
		{`{"req_id":"diagnostic-request","hits":[{"req_id":"other-request","port_id":"port:15000"}]}`, "invalid"},
		{`{"req_id":"diagnostic-request","hits":[{"req_id":"diagnostic-request","port_id":"port:15001"}]}`, "invalid"},
		{`not-json`, "invalid"},
	} {
		if err := db.Exec("DELETE FROM pipeline_log").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.PipelineLog{ID: "fixture", ReqID: log.REQ_UUID, PortID: "port:15000", ClientIP: log.SRC_IP, Kind: "request", DetailJSON: tc.raw}).Error; err != nil {
			t.Fatal(err)
		}
		if got := requestDiagnostics(log); got.State != tc.state || len(got.Hits) != 0 {
			t.Fatalf("%s: %+v", tc.raw, got)
		}
	}
	if err := db.Migrator().DropTable(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	if got := requestDiagnostics(log); got.State != "unavailable" {
		t.Fatalf("failed read became a clean or missing result: %+v", got)
	}
}
