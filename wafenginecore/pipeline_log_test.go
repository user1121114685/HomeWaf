package wafenginecore

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func pipelineLogFixture(t *testing.T) (*WafEngine, *wafpipeline.Snapshot, *wafpipeline.RequestContext) {
	t.Helper()
	oldQueue, oldSettings, oldExclude := global.GQEQUE_LOG_DB, global.CurrentSettings(), globalIPLogExclude.Load()
	global.GQEQUE_LOG_DB = queue.NewQueue()
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "all"
		cfg.GCONFIG_RECORD_MAX_BODY_LENGTH = 2048
	})
	SetGlobalIPLogExclude("")
	t.Cleanup(func() {
		global.GQEQUE_LOG_DB = oldQueue
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { *cfg = oldSettings })
		globalIPLogExclude.Store(oldExclude)
	})
	req := httptest.NewRequest("POST", "http://family.test/login?api_key=SECRET_QUERY", nil)
	req.Header.Set("Authorization", "Bearer SECRET_AUTH")
	req.Header.Set("Cookie", "session=SECRET_COOKIE")
	req.Header.Set("Content-Type", "application/json")
	req.Body = unreadableLogBody{}
	c := wafpipeline.NewRequestContext(req, "port:15000", netip.MustParseAddr("203.0.113.7"), wafpipeline.DefaultViewLimits())
	c.View.Body = []byte(`{"password":"SECRET_BODY","name":"fixture"}`)
	s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: 15000, HostCode: "pipeline-15000"}}
	return &WafEngine{}, s, c
}

type unreadableLogBody struct{}

func (unreadableLogBody) Read([]byte) (int, error) { panic("logging consumed the request body") }
func (unreadableLogBody) Close() error             { return nil }

func TestPipelineWebLogKeepsIdentityVerdictAndRedactsSecrets(t *testing.T) {
	cases := []struct {
		name    string
		result  wafpipeline.Result
		event   bool
		observe int
	}{
		{"allowed", wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 201}}, false, 0},
		{"blocked", wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, RuleID: "sqli"}}, true, 0},
		{"host_rejected", wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, Reason: "host_not_allowed"}}, true, 0},
		{"observed", wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 200}, Hits: []wafpipeline.HitEvent{{Category: "xss", RuleID: "xss", Severity: wafpipeline.High, Observe: true}}}, true, 1},
		{"login_success", wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 302}, Hits: []wafpipeline.HitEvent{{Category: "auth_success", RuleID: "gate"}}}, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, s, c := pipelineLogFixture(t)
			engine.RecordPipelineWebLog(s, c, tc.result)
			item, ok := global.GQEQUE_LOG_DB.Dequeue()
			lg, typed := item.(*innerbean.WebLog)
			if !ok || !typed || !global.GQEQUE_LOG_DB.Empty() {
				t.Fatal("completed request must enqueue exactly one WebLog")
			}
			if lg.REQ_UUID != c.ReqID || lg.HOST_CODE != s.Config.HostCode || lg.SRC_IP != c.ClientIP.String() || lg.USER_CODE != global.GWAF_USER_CODE || lg.TenantId != global.GWAF_TENANT_ID || lg.UNIX_ADD_TIME != c.Started.UnixMilli() {
				t.Fatal("request correlation, site, client or ownership changed")
			}
			if lg.STATUS_CODE != tc.result.Verdict.Status || lg.IsSecurityEvent() != tc.event || lg.LogOnlyMode != tc.observe {
				t.Fatalf("incorrect classification: status=%d event=%v observe=%d", lg.STATUS_CODE, lg.IsSecurityEvent(), lg.LogOnlyMode)
			}
			if lg.GUEST_IDENTIFICATION != "未识别访客" {
				t.Fatalf("request outcome asserted visitor identity: %q", lg.GUEST_IDENTIFICATION)
			}
			raw, _ := json.Marshal(lg)
			if strings.Contains(string(raw), "SECRET_") {
				t.Fatal("queued request contains credentials")
			}
			if !strings.Contains(string(c.View.Body), "SECRET_BODY") || c.Request.Header.Get("Authorization") != "Bearer SECRET_AUTH" {
				t.Fatal("logging mutated the live request")
			}
		})
	}
}

func TestPipelineWebLogRespectsFiltersAndDryRun(t *testing.T) {
	engine, s, c := pipelineLogFixture(t)
	allow := wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Allow, Status: 200}}
	deny := wafpipeline.Result{Verdict: wafpipeline.Verdict{Action: wafpipeline.Deny, Status: 403, Reason: "access_disabled"}}
	check := func(result wafpipeline.Result, want bool) {
		t.Helper()
		engine.RecordPipelineWebLog(s, c, result)
		_, got := global.GQEQUE_LOG_DB.Dequeue()
		if got != want || !global.GQEQUE_LOG_DB.Empty() {
			t.Fatalf("queued=%v, want %v", got, want)
		}
	}
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "abnormal" })
	check(allow, false)
	check(deny, true)
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "all" })
	SetGlobalIPLogExclude("203.0.113.0/24")
	check(allow, false)
	check(deny, true)
	SetGlobalIPLogExclude("")
	engine.withWriteTable(func(table *routingTable) {
		table.HostCode[s.Config.HostCode] = "configured.test:15000"
		table.HostTarget["configured.test:15000"] = &wafenginmodel.HostSafe{Host: model.Hosts{EXCLUDE_URL_LOG: "/login"}}
	})
	check(allow, false)
	check(deny, false)
	engine.InitRouting()
	c.DryRun = true
	check(deny, false)
	c.DryRun = false
	s.Preview = true
	check(allow, false)
}
