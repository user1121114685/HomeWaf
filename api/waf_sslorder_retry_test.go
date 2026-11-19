package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/model/spec"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafdb"
	"HomeWaf/wafsec"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSSLOrderRetryReusesSavedConfigurationAndDeduplicates(t *testing.T) {
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "retry.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PrivateInfo{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}, &model.CertificateDelivery{}, &model.NotifySubscription{}, &model.NotifyChannel{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldQueue := global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder
	oldTenant, oldUser := global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	global.GWAF_TENANT_ID, global.GWAF_USER_CODE = "retry-tenant", "retry-user"
	global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = db, make(chan spec.ChanSslOrder, 20)
	t.Cleanup(func() {
		global.GWAF_TENANT_ID, global.GWAF_USER_CODE = oldTenant, oldUser
		global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = oldDB, oldQueue
		conn, _ := db.DB()
		_ = conn.Close()
	})
	if err := wafdb.RegisterTenantQueryScope(db); err != nil {
		t.Fatal(err)
	}
	service := createCertificateTestService(t, db)
	if err := waf_service.WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "household", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:fixture-ddns-secret"}); err != nil {
		t.Fatal(err)
	}
	source := model.SslOrder{HostCode: service.HostCode, ApplyStatus: "fail", Progress: "申请任务已中断", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyDomain: "example.com", ApplyEmail: "owner@example.com", PrivateGroupName: "household", ResultError: "interrupted", ApplyKey: "fixture-account-secret"}
	source.Id = "failed-source"
	disabled := false
	source.AutoRenew = &disabled
	source.Tenant_ID, source.USER_CODE = global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	source.CREATE_TIME = customtype.JsonTime(time.Now())
	source.UPDATE_TIME = source.CREATE_TIME
	if err := db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	api := &WafSslOrderApi{}
	call := func(handler gin.HandlerFunc, path string, input any) (int, string, string) {
		raw, _ := json.Marshal(input)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("is_openapi", true)
		method := "POST"
		if input == nil {
			method = "GET"
		}
		c.Request = httptest.NewRequest(method, path, bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		var result struct {
			Code int `json:"code"`
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Error(err)
		}
		return result.Code, result.Data.ID, rec.Body.String()
	}
	input := map[string]any{"id": source.Id, "request_id": "same-retry", "host_code": "client-must-not-override", "private_group_name": "other"}
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, id, body := call(api.RetryApi, "/", input)
			if code != 0 {
				t.Errorf("retry rejected: %s", body)
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("retries returned different orders")
		}
		id = got
	}
	if len(global.GWAF_CHAN_SSLOrder) != 1 {
		t.Fatalf("duplicate issuance messages: %d", len(global.GWAF_CHAN_SSLOrder))
	}
	queued := (<-global.GWAF_CHAN_SSLOrder).Content.(model.SslOrder)
	if queued.Id != id || queued.HostCode != source.HostCode || queued.ApplyDomain != source.ApplyDomain || queued.PrivateGroupName != source.PrivateGroupName || queued.ApplyDns != source.ApplyDns || queued.AutoRenewEnabled() {
		t.Fatal("retry did not reuse server-side configuration")
	}
	var count int64
	db.Model(&model.SslOrder{}).Count(&count)
	if count != 2 {
		t.Fatalf("wrong order count %d", count)
	}
	if waf_service.WafSSLOrderServiceApp.GetDetailById(source.Id).ApplyStatus != "fail" {
		t.Fatal("retry overwrote failure history")
	}
	if code, _, _ := call(api.RetryApi, "/", map[string]any{"id": id, "request_id": "active-retry"}); code == 0 {
		t.Fatal("active order retried")
	}
	// Read-side redaction also protects historical entries recorded before the
	// scoped logger was installed, including bare credential values.
	if err := db.Create(&model.SslOrderLog{OrderID: id, TenantID: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, CreatedAt: customtype.JsonTime(time.Now()), Level: "error", Message: "provider echoed fixture-ddns-secret"}).Error; err != nil {
		t.Fatal(err)
	}
	if code, _, body := call(api.LogsApi, "/?id="+id, nil); code != 0 || !strings.Contains(body, "entries") || strings.Contains(body, "fixture-account-secret") || strings.Contains(body, "fixture-ddns-secret") {
		t.Fatalf("unsafe or unreadable log response: %s", body)
	}
	if code, _, _ := call(api.RetryApi, "/", map[string]any{"id": source.Id}); code == 0 {
		t.Fatal("retry accepted without idempotency key")
	}
}
