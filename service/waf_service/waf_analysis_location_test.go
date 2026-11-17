package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"fmt"
	"testing"
)

func TestDashboardRegionsRetainCitiesAndCountryAggregation(t *testing.T) {
	_, db, _ := setupLogLookupTest(t)
	old := global.GWAF_LOCAL_STATS_DB
	global.GWAF_LOCAL_STATS_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_STATS_DB = old })
	if err := db.AutoMigrate(&model.StatsIPCityDay{}); err != nil {
		t.Fatal(err)
	}
	for i, row := range []model.StatsIPCityDay{
		{Day: 20261117, Country: "中国", Province: "重庆市", City: "重庆市", Type: "放行", Count: 10},
		{Day: 20261118, Country: "中国", Province: "重庆市", City: "重庆市", Type: "阻止", Count: 2},
		{Day: 20261118, Country: "中国", Province: "浙江省", City: "杭州市", Type: "放行", Count: 4},
		{Day: 20261118, Country: "United States", Province: "Florida", City: "Miami", Type: "阻止", Count: 3},
		{Day: 20261118, Country: "United States", Type: "放行", Count: 1},
		{Day: 20261118, Country: "内网", Type: "放行", Count: 100},
		{Day: 20261116, Country: "中国", City: "武汉市", Type: "阻止", Count: 100},
	} {
		row.BaseOrm = baseorm.BaseOrm{Id: fmt.Sprint(i), Tenant_ID: global.GWAF_TENANT_ID, USER_CODE: global.GWAF_USER_CODE}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	req := request.WafStatsAnalysisDayRangeCountryReq{StartDay: "20261117", EndDay: "20261118", GroupBy: "city"}
	rows, err := WafAnalysisServiceApp.StatAnalysisDayCountryRangeApi(req)
	if err != nil || len(rows) != 4 || rows[0].Country != "中国" || rows[0].City != "重庆市" || rows[0].Value != 12 {
		t.Fatalf("cities=%+v err=%v", rows, err)
	}
	if rows[3].City != "" || rows[3].Value != 1 {
		t.Fatalf("missing city was fabricated or discarded: %+v", rows[3])
	}
	req.AttackType = "阻止"
	rows, err = WafAnalysisServiceApp.StatAnalysisDayCountryRangeApi(req)
	if err != nil || len(rows) != 2 || rows[0].City != "Miami" || rows[0].Value != 3 {
		t.Fatalf("blocked=%+v err=%v", rows, err)
	}
	req.GroupBy, req.AttackType = "", ""
	rows, err = WafAnalysisServiceApp.StatAnalysisDayCountryRangeApi(req)
	if err != nil || len(rows) != 2 || rows[0].Name != "中国" || rows[0].Value != 16 || rows[0].City != "" {
		t.Fatalf("country compatibility=%+v err=%v", rows, err)
	}
	req.GroupBy = "city; DROP TABLE stats_ip_city_days"
	if _, err := WafAnalysisServiceApp.StatAnalysisDayCountryRangeApi(req); err == nil {
		t.Fatal("invalid dimension accepted")
	}
	req.GroupBy = "city"
	if err := db.Migrator().DropTable(&model.StatsIPCityDay{}); err != nil {
		t.Fatal(err)
	}
	if _, err := WafAnalysisServiceApp.StatAnalysisDayCountryRangeApi(req); err == nil {
		t.Fatal("read failure became a successful empty result")
	}
}
