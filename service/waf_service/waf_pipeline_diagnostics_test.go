package waf_service

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/wafpipeline"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPipelineDiagnosticNotificationsAreBoundedIsolatedAndDryRunSafe(t *testing.T) {
	oldMessages, oldLogs := global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB
	global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB = queue.NewQueue(), queue.NewQueue()
	defer func() { global.GQEQUE_MESSAGE_DB = oldMessages; global.GQEQUE_LOG_DB = oldLogs }()
	service := &WafPipelineStateService{}
	a := &wafpipeline.Snapshot{Compiled: &wafpipeline.CompiledPolicy{Metrics: &wafpipeline.PortMetrics{}}}
	b := &wafpipeline.Snapshot{Compiled: &wafpipeline.CompiledPolicy{Metrics: &wafpipeline.PortMetrics{}}}
	newRequest := func(id string) *wafpipeline.RequestContext {
		return wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://family.test/", nil), id, netip.MustParseAddr("127.0.0.1"), wafpipeline.DefaultViewLimits())
	}
	result := wafpipeline.Result{Hits: []wafpipeline.HitEvent{{Category: "detector_error", Observe: true, Evidence: "SECRET must not enter notifications"}}}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); service.LogResult(a, newRequest("port:5000"), result) }()
	}
	workers.Wait()
	if global.GQEQUE_MESSAGE_DB.Size() != 1 || global.GQEQUE_LOG_DB.Size() != 16 {
		t.Fatal("diagnostic throttle lost logs or flooded notifications")
	}
	dry := newRequest("port:5001")
	dry.DryRun = true
	service.diagnosticAlert(b, dry, result)
	if global.GQEQUE_MESSAGE_DB.Size() != 1 {
		t.Fatal("dry-run sent notification")
	}
	service.LogResult(b, newRequest("port:5001"), result)
	if global.GQEQUE_MESSAGE_DB.Size() != 2 {
		t.Fatal("one port suppressed another port's warning")
	}
	a.Compiled.Metrics.DiagnosticAlertAt.Store(time.Now().Add(-61 * time.Second).Unix())
	service.LogResult(a, newRequest("port:5000"), result)
	if global.GQEQUE_MESSAGE_DB.Size() != 3 {
		t.Fatal("warning did not recover after cooldown")
	}
	for !global.GQEQUE_MESSAGE_DB.Empty() {
		value, _ := global.GQEQUE_MESSAGE_DB.Dequeue()
		message, ok := value.(innerbean.SystemErrorMessageInfo)
		if !ok || strings.Contains(message.ErrorMsg, "SECRET") {
			t.Fatal("notification leaked evidence or wrong message type")
		}
	}
}
