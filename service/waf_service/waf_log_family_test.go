package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
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
	if err := core.AutoMigrate(&model.Hosts{}, &model.PortConfig{}); err != nil {
		t.Fatal(err)
	}
	orm := baseorm.BaseOrm{Id: "global", Tenant_ID: global.GWAF_TENANT_ID, USER_CODE: global.GWAF_USER_CODE}
	if err := core.Create(&model.Hosts{BaseOrm: orm, Code: "global", Host: "全局网站", GLOBAL_HOST: 1}).Error; err != nil {
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
	if err != nil || len(options) != 2 {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	if options[0].Code != config.HostCode || options[0].Nickname != "群晖" || options[0].Source != "pipeline" || options[0].Port != 15000 {
		t.Fatalf("lost service identity: %+v", options[0])
	}
	// Site-only editors must not get synthetic pipeline sites.
	if sites := WafHostServiceApp.GetAllHostApi(); len(sites) != 1 || sites[0].Code != "global" {
		t.Fatalf("policy selector changed: %+v", sites)
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
	}{{"", 5}, {"normal", 1}, {"abnormal", 4}} {
		req.RequestOutcome = c.value
		rows, count, _, err := WafLogServiceApp.GetListApiWithMeta(req)
		if err != nil || count != c.total || int64(len(rows)) != c.total {
			t.Fatalf("%q: rows=%d total=%d err=%v", c.value, len(rows), count, err)
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
