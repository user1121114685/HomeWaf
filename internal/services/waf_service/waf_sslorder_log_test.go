package waf_service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/storage/wafdb"
)

func TestSSLOrderLogsAreBoundedIsolatedAndDeletedWithOrder(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := wafdb.RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	order := model.SslOrder{ApplyStatus: "fail", ApplyDomain: "example.com"}
	order.Id = "log-order"
	order.Tenant_ID, order.USER_CODE = global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	order.CREATE_TIME = customtype.JsonTime(time.Now())
	order.UPDATE_TIME = order.CREATE_TIME
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < sslOrderLogLimit+5; i++ {
		if err := WafSSLOrderServiceApp.AppendLog(order.Id, "info", fmt.Sprintf("step %d token=fixture-secret", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := WafSSLOrderServiceApp.AppendLog("different-order", "info", "must not appear"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SslOrderLog{OrderID: order.Id, TenantID: "other-tenant", UserCode: "other-user", CreatedAt: customtype.JsonTime(time.Now()), Level: "info", Message: "foreign tenant must not appear"}).Error; err != nil {
		t.Fatal(err)
	}
	logs, err := WafSSLOrderServiceApp.Logs(order.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs.Entries) != sslOrderLogLimit || !strings.HasPrefix(logs.Entries[0].Message, "step 5 ") {
		t.Fatal("retention did not keep the latest bounded events")
	}
	for _, entry := range logs.Entries {
		if strings.Contains(entry.Message, "fixture-secret") || strings.Contains(entry.Message, "must not appear") {
			t.Fatal("log leaked secret or other order")
		}
	}
	if _, err := WafSSLOrderServiceApp.Logs("../../logs/log.log"); err == nil {
		t.Fatal("nonexistent order accepted")
	}
	if err := WafSSLOrderServiceApp.DelApi(request.WafSslorderdeleteReq{Id: order.Id}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.SslOrderLog{}).Where("order_id = ?", order.Id).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("orphan logs: %d %v", count, err)
	}
}

func TestSSLOrderHeartbeatStateVisibleInListAndLogs(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := wafdb.RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	order := model.SslOrder{ApplyStatus: "applying", ApplyDomain: "example.com"}
	order.Id = "stale-order"
	order.Tenant_ID, order.USER_CODE = global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	order.CREATE_TIME = customtype.JsonTime(time.Now())
	order.UPDATE_TIME = customtype.JsonTime(time.Now().Add(-2 * time.Minute))
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	if err := WafSSLOrderServiceApp.AppendLog(order.Id, "info", "fixture progress"); err != nil {
		t.Fatal(err)
	}
	rows, _, err := WafSSLOrderServiceApp.GetListApi(request.WafSslordersearchReq{})
	if err != nil || len(rows) != 1 || !rows[0].HeartbeatStale {
		t.Fatalf("stale heartbeat hidden in list: %v", err)
	}
	logs, err := WafSSLOrderServiceApp.Logs(order.Id)
	if err != nil || !logs.HeartbeatStale {
		t.Fatalf("stale heartbeat hidden in logs: %v", err)
	}
	if err := db.Model(&model.SslOrder{}).Where("id = ?", order.Id).Update("update_time", customtype.JsonTime(time.Now())).Error; err != nil {
		t.Fatal(err)
	}
	if got := WafSSLOrderServiceApp.GetDetailById(order.Id); got.HeartbeatStale {
		t.Fatal("live order marked as stale")
	}
}
