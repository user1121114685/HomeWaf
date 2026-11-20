package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/wafdb"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestLogServicesIncludePipelineAndPreserveLogIdentity(t *testing.T) {
	core, _, _ := setupLogLookupTest(t)
	if err := core.AutoMigrate(&model.PortConfig{}); err != nil {
		t.Fatal(err)
	}
	config := wafpipeline.PortConfig{Schema: 1, Port: 15000, Name: "群晖", Protocol: "http", Upstream: "http://127.0.0.1:5000", HostCode: "pipeline-15000", Policy: wafpipeline.DefaultPolicy()}
	encoded, _ := json.Marshal(config)
	// Deliberately unusable secrets: a log selector must never read/decrypt them.
	err := core.Table("port_config").Create(map[string]interface{}{"id": "service", "port": 15000, "config_json": string(encoded), "secrets_json": "not-readable-by-log-selector", "tenant_id": global.GWAF_TENANT_ID, "user_code": global.GWAF_USER_CODE, "revision": 1}).Error
	if err != nil {
		t.Fatal(err)
	}
	options, err := WafHostServiceApp.GetLogServiceOptions()
	if err != nil || len(options) != 1 {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	if options[0].Code != config.HostCode || options[0].Nickname != "群晖" || options[0].Source != "pipeline" || options[0].Port != 15000 {
		t.Fatalf("lost service identity: %+v", options[0])
	}

}

func TestLogRequestOutcomeFiltersObserveAndResponseFailures(t *testing.T) {
	_, live, _ := setupLogLookupTest(t)
	at := time.Now().UnixMilli()
	cases := []struct {
		action, rule    string
		observe, status int
	}{
		{"放行", "", 0, 200}, {"阻止", "ip_block", 0, 403},
		{"放行", "ssrf", 1, 200}, {"放行", "", 0, 500}, {"放行", "", 0, 404},
		{"放行", "csrf", 1, 200}, {"放行", "csrf", 1, 500},
		{"阻止", "sqli", 1, 403}, {"放行", "sqli", 0, 200}, {"", "", 1, 200},
		{"禁止", "blocked", 0, 403},
	}
	for i, c := range cases {
		row := model.LogNarrow{ReqUUID: fmt.Sprint(i), TenantId: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, HostCode: "pipeline-15000", ACTION: c.action, RULE: c.rule, LogOnlyMode: c.observe, STATUS_CODE: c.status, UNIX_ADD_TIME: at}
		if err := live.Create(&model.AccessLog{LogNarrow: row}).Error; err != nil {
			t.Fatal(err)
		}
	}
	req := request.WafAttackLogSearch{CurrrentDbName: wafdb.LiveLogName(), ViewType: "access", HostCode: "pipeline-15000", UnixAddTimeBegin: "0", UnixAddTimeEnd: fmt.Sprint(at + 1), SortBy: "unix_add_time", SortDescending: "desc"}
	req.PageIndex, req.PageSize = 1, 20
	for _, c := range []struct {
		value string
		total int64
	}{{"", 11}, {"normal", 1}, {"observed", 2}, {"abnormal", 8}, {"blocked", 3}} {
		req.RequestOutcome = c.value
		rows, count, _, err := WafLogServiceApp.GetListApiWithMeta(req)
		if err != nil || count != c.total || int64(len(rows)) != c.total {
			t.Fatalf("%q: rows=%d total=%d err=%v", c.value, len(rows), count, err)
		}
		if c.value == "observed" {
			for _, row := range rows {
				if row.ACTION != "放行" || row.LogOnlyMode != 1 || row.STATUS_CODE >= 400 || !row.IsSecurityEvent() {
					t.Fatalf("observation lost its outcome or security audit record: %+v", row)
				}
			}
		}
	}
	normal, abnormal := req, req
	normal.RequestOutcome, abnormal.RequestOutcome = "normal", "abnormal"
	if countCacheKey(normal, "archive") == countCacheKey(abnormal, "archive") {
		t.Fatal("outcome omitted from archive count cache")
	}
	req.RequestOutcome = "' or 1=1"
	if _, _, _, err := WafLogServiceApp.GetListApiWithMeta(req); err == nil {
		t.Fatal("invalid outcome accepted")
	}
}

func TestLogPipelineVisitorIdentityDetailAndFilter(t *testing.T) {
	_, live, _ := setupLogLookupTest(t)
	at := time.Now().UnixMilli()
	for _, tc := range []struct{ id, host, identity string }{
		{"blocked", "pipeline-15000", "未识别访客"},
		{"known", "pipeline-15000", "已验证爬虫"},
	} {
		row := model.LogNarrow{ReqUUID: tc.id, TenantId: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, HostCode: tc.host, ACTION: "阻止", RULE: "host_not_allowed", STATUS_CODE: 403, GUEST_IDENTIFICATION: tc.identity, UNIX_ADD_TIME: at}
		if err := live.Create(&model.AccessLog{LogNarrow: row}).Error; err != nil {
			t.Fatal(err)
		}
		detail, err := WafLogServiceApp.GetDetailApi(request.WafAttackLogDetailReq{REQ_UUID: tc.id, CurrrentDbName: wafdb.LiveLogName()})
		if err != nil || detail.GUEST_IDENTIFICATION != tc.identity || detail.ACTION != "阻止" || detail.RULE != "host_not_allowed" {
			t.Fatalf("detail=%+v err=%v", detail, err)
		}
		var stored model.AccessLog
		if err := live.Table(model.AccessLogTableName).Where("req_uuid = ?", tc.id).First(&stored).Error; err != nil {
			t.Fatal(err)
		}
		if stored.GUEST_IDENTIFICATION != tc.identity {
			t.Fatal("read changed stored identity")
		}
	}
	req := request.WafAttackLogSearch{CurrrentDbName: wafdb.LiveLogName(), UnixAddTimeBegin: "0", UnixAddTimeEnd: fmt.Sprint(at + 1), FilterBy: "guest_identification", FilterValue: "未识别访客", SortBy: "unix_add_time", SortDescending: "desc"}
	req.PageIndex, req.PageSize = 1, 20
	rows, count, _, err := WafLogServiceApp.GetListApiWithMeta(req)
	if err != nil || count != 1 || len(rows) != 1 || rows[0].REQ_UUID != "blocked" || rows[0].GUEST_IDENTIFICATION != "未识别访客" {
		t.Fatalf("rows=%+v count=%d err=%v", rows, count, err)
	}
}
