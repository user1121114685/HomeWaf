package waf_service

import (
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/queue"
	"HomeWaf/internal/protection/wafpipeline"
	"net/http"
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

func TestDiagnosticMessageLinksReadFailureToItsRequest(t *testing.T) {
	old := global.GQEQUE_MESSAGE_DB
	global.GQEQUE_MESSAGE_DB = queue.NewQueue()
	t.Cleanup(func() { global.GQEQUE_MESSAGE_DB = old })
	c := wafpipeline.NewRequestContext(httptest.NewRequest("POST", "/private-path?token=SECRET", nil), "port:15000", netip.MustParseAddr("127.0.0.1"), wafpipeline.DefaultViewLimits())
	snap := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Name: "群晖", Port: 15000}, Compiled: &wafpipeline.CompiledPolicy{Metrics: &wafpipeline.PortMetrics{}}}
	occurred := time.Date(2026, 11, 22, 3, 58, 57, 0, time.UTC)
	verdict := wafpipeline.Verdict{Action: wafpipeline.Deny, Stage: 3, Status: http.StatusServiceUnavailable, Reason: "policy_unavailable"}
	idle := int64(130)
	result := wafpipeline.Result{
		Verdict:         verdict,
		Hits:            []wafpipeline.HitEvent{{Time: occurred, Stage: 3, Category: "detector_error", RuleID: "sqli", Evidence: "SECRET"}},
		Trace:           []wafpipeline.TraceEntry{{Check: "synology_login", Verdict: verdict, Error: "detector unavailable"}},
		BodyReadFailure: &bodyinspect.ReadFailure{Cause: bodyinspect.PrematureEOF, ElapsedMS: 230, IdleMS: &idle, BytesRead: 6412},
	}
	service := WafPipelineStateService{}
	service.diagnosticAlert(snap, c, result)
	pop, ok := global.GQEQUE_MESSAGE_DB.Dequeue()
	if !ok {
		t.Fatal("missing diagnostic alert")
	}
	msg := pop.(innerbean.SystemErrorMessageInfo)
	event := WafNotifySenderServiceApp.BuildNotifyEvent(msg)
	for _, want := range []string{occurred.Format(time.RFC3339), c.ReqID, "群晖", "port:15000", "sqli", "阶段 3", "synology_login", "转发前", "503", "正文提前结束", "6412", "130 ms"} {
		if !strings.Contains(event.DefaultContent, want) {
			t.Fatalf("notification missing %q: %s", want, event.DefaultContent)
		}
	}
	if event.Vars["RequestID"] != c.ReqID || event.Vars["Time"] != occurred.Format(time.RFC3339) ||
		strings.Contains(event.DefaultContent, "SECRET") || strings.Contains(event.DefaultContent, "private-path") {
		t.Fatal("custom variables or notification privacy changed")
	}
}

func TestNativeRuleAndBanNotificationSources(t *testing.T) {
	oldMessages, oldLogs := global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB
	global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB = queue.NewQueue(), nil
	t.Cleanup(func() { global.GQEQUE_MESSAGE_DB, global.GQEQUE_LOG_DB = oldMessages, oldLogs })
	service := WafPipelineStateService{}
	snap := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Name: "photos", Port: 15000}}
	c := wafpipeline.NewRequestContext(httptest.NewRequest("GET", "http://home.test/?token=PRIVATE", nil), "port:15000", netip.MustParseAddr("192.0.2.1"), wafpipeline.DefaultViewLimits())
	at := time.Now()
	result := wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Deny, RuleID: "test-rule", Status: 403},
		Hits: []wafpipeline.HitEvent{{Category: "scanner", RuleID: "test-rule", Time: at, Evidence: "PRIVATE"}}}
	c.DryRun = true
	service.LogResult(snap, c, result)
	c.DryRun = false
	result.Hits[0].Observe = true
	service.LogResult(snap, c, result)
	if global.GQEQUE_MESSAGE_DB.Size() != 0 {
		t.Fatal("preview or observation produced a block notice")
	}
	result.Hits[0].Observe = false
	service.LogResult(snap, c, result)
	value, ok := global.GQEQUE_MESSAGE_DB.Dequeue()
	if !ok {
		t.Fatal("missing rule notice")
	}
	ev := WafNotifySenderServiceApp.BuildNotifyEvent(value)
	if ev.Vars["RequestID"] != c.ReqID || ev.Vars["Domain"] != "home.test" || !strings.Contains(ev.DefaultContent, "403") || strings.Contains(ev.DefaultContent, "PRIVATE") {
		t.Fatal(ev.DefaultContent)
	}
	change := wafpipeline.PunishmentChange{Event: "ban_created", Record: wafpipeline.PunishmentRecord{PortID: "port:15000", Kind: "ban", Prefix: "192.0.2.1/32", Reason: "scanner", CreatedAt: at.Unix(), ExpiresAt: at.Unix() + 25}}
	service.recordChange(change)
	value, ok = global.GQEQUE_MESSAGE_DB.Dequeue()
	if !ok {
		t.Fatal("missing port ban")
	}
	ev = WafNotifySenderServiceApp.BuildNotifyEvent(value)
	if ev.MessageType != "ip_ban" || ev.Vars["Scope"] != "port:15000" || ev.Vars["DurationSeconds"] != "25" || !strings.Contains(ev.DefaultContent, "25秒") {
		t.Fatal(ev.DefaultContent)
	}
	change.Event = "release"
	service.recordChange(change)
	if global.GQEQUE_MESSAGE_DB.Size() != 0 {
		t.Fatal("release produced another ban notice")
	}
}
