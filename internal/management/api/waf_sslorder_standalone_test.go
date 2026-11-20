package api

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/model/spec"
	"HomeWaf/internal/services/waf_service"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSSLOrderAPIAllowsIndependentDNSIssuance(t *testing.T) {
	setupNotifyTestEnv(t, &model.Hosts{}, &model.PrivateInfo{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{})
	db := global.GWAF_LOCAL_DB
	oldQueue := global.GWAF_CHAN_SSLOrder
	global.GWAF_CHAN_SSLOrder = make(chan spec.ChanSslOrder, 4)
	t.Cleanup(func() { global.GWAF_CHAN_SSLOrder = oldQueue })
	if err := waf_service.WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "standalone", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "independent.example.test:isolated-token"}); err != nil {
		t.Fatal(err)
	}
	call := func(method, host string) (int, string, string) {
		t.Helper()
		input := map[string]any{"request_id": "independent-" + method + host, "apply_method": method, "apply_dns": "hurricane", "private_group_name": "standalone", "apply_email": "owner@example.test", "apply_domain": "independent.example.test"}
		if host != "" {
			input["host_code"] = host
		}
		body, _ := json.Marshal(input)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Set("is_openapi", true)
		ctx.Request = httptest.NewRequest("POST", "/api/v1/wafhost/sslorder/add", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		new(WafSslOrderApi).AddApi(ctx)
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Code, result.Data.ID, result.Msg
	}
	code, id, message := call("dns01", "")
	if code != 0 || id == "" {
		t.Fatalf("independent request rejected: %d %s", code, message)
	}
	var stored model.SslOrder
	if err := db.First(&stored, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.HostCode != "" || stored.BindingSnapshot != "[]" || stored.ApplyStatus != "submitted" {
		t.Fatalf("unexpected service dependency: %+v", stored)
	}
	if code, _, message = call("dns01", "deleted"); code == 0 {
		t.Fatal("invalid optional service ignored")
	}
	if code, _, message = call("http01", ""); code == 0 || !strings.Contains(message, "DNS") {
		t.Fatalf("missing HTTP challenge prerequisite: %d %s", code, message)
	}
}
