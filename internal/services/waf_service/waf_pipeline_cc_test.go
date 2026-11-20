package waf_service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/protection/wafpipeline"
)

func TestPipelineCCRecommendationUsesSelectedServiceAndRejectsSampledData(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.AccessLog{}); err != nil {
		t.Fatal(err)
	}
	oldLog, oldPorts, settings := global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports, global.CurrentSettings()
	global.GWAF_LOCAL_LOG_DB = db
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	t.Cleanup(func() {
		global.GWAF_LOCAL_LOG_DB = oldLog
		wafpipeline.Ports = oldPorts
		global.UpdateSettings(func(s *global.RuntimeSettings) { *s = settings })
	})
	// Get resolves this loaded, native service without consulting legacy Hosts.
	if err := wafpipeline.Ports.Publish(15000, func() (*wafpipeline.Snapshot, error) {
		return &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: 15000}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.GDATA_ACCESS_LOG_MODE = "db" })
	rows := make([]model.AccessLog, 0, 1100)
	for i := 0; i < 1100; i++ {
		host := "pipeline-15000"
		if i >= 1000 {
			host = "pipeline-15001"
		}
		rows = append(rows, model.AccessLog{LogNarrow: model.LogNarrow{ReqUUID: fmt.Sprint(i), HostCode: host, SRC_IP: fmt.Sprintf("192.0.2.%d", i%20+1), URL: "/service", UNIX_ADD_TIME: time.Now().UnixMilli()}})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	req := PipelineCCRecommendRequest{Port: 15000, Days: 1, WindowSeconds: 60}
	result, err := WafPipelineServiceApp.RecommendCC(context.Background(), req)
	if err != nil || !result.Supported || result.Requests != 1000 || result.Clients != 20 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.GDATA_ACCESS_LOG_MODE = "sample" })
	result, err = WafPipelineServiceApp.RecommendCC(context.Background(), req)
	if err != nil || result.Supported || result.Suggested != 0 {
		t.Fatalf("sampled logs generated recommendation: %+v %v", result, err)
	}
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.GDATA_ACCESS_LOG_MODE = "db" })
	if err := db.Migrator().DropTable(&model.AccessLog{}); err != nil {
		t.Fatal(err)
	}
	if _, err := WafPipelineServiceApp.RecommendCC(context.Background(), req); err == nil {
		t.Fatal("query failure became an empty successful estimate")
	}
}

func TestPipelineCCRecommendationPathAndStaticSemantics(t *testing.T) {
	request := PipelineCCRecommendRequest{WindowSeconds: 60, ByPath: true, ExcludeStatic: true}
	rows := []pipelineCCSample{{"192.0.2.1", "/asset.CSS?q=1", 60000}, {"192.0.2.1", "/x?a=1", 60001}, {"192.0.2.1", "/x?a=2", 60002}}
	result := summarizePipelineCC(rows, request, PipelineCCRecommendation{})
	if result.Requests != 2 || result.Supported {
		t.Fatalf("static/query semantics: %+v", result)
	}
}

func TestPipelineCCRecentHitsPreserveCCOutcomeAndPortScope(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.PipelineLog{}); err != nil {
		t.Fatal(err)
	}
	oldLog, oldPorts := global.GWAF_LOCAL_LOG_DB, wafpipeline.Ports
	global.GWAF_LOCAL_LOG_DB = db
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	t.Cleanup(func() { global.GWAF_LOCAL_LOG_DB = oldLog; wafpipeline.Ports = oldPorts })
	if err := wafpipeline.Ports.Publish(15000, func() (*wafpipeline.Snapshot, error) {
		return &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: 15000}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	detail, _ := json.Marshal(wafpipeline.Result{Hits: []wafpipeline.HitEvent{{Category: "malformed", Observe: true}, {Category: "cc", RuleID: "cc", Observe: false}}, Trace: []wafpipeline.TraceEntry{{Check: "cc", Verdict: wafpipeline.Verdict{Action: wafpipeline.Deny}}}})
	for _, port := range []uint16{15000, 15001} {
		row := model.PipelineLog{ID: fmt.Sprint(port), PortID: wafpipeline.PortID(port), Kind: "request", Category: "malformed", Observe: true, TimeMillis: time.Now().UnixMilli(), DetailJSON: string(detail)}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := WafPipelineStateServiceApp.CCHits(15000)
	if err != nil || len(rows) != 1 || rows[0].Observe || rows[0].Verdict != "deny" || rows[0].Category != "cc" {
		t.Fatalf("wrong CC rows/outcome: %+v %v", rows, err)
	}
}
