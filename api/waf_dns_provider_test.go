package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/model/spec"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDNSHomeOrderAndRenewalWithoutPublicPorts(t *testing.T) {
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dns.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PrivateInfo{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslConfig{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldQueue := global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder
	global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = db, make(chan spec.ChanSslOrder, 5)
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = oldDB, oldQueue
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	service := createCertificateTestService(t, db)
	if err := waf_service.WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "household", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:test-ddns-secret"}); err != nil {
		t.Fatal(err)
	}
	call := func(handler gin.HandlerFunc, path string, data any) (int, string) {
		t.Helper()
		raw, _ := json.Marshal(data)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("is_openapi", true)
		c.Request = httptest.NewRequest("POST", path, bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		var result struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Code, w.Body.String()
	}
	api := &WafSslOrderApi{}
	if code, body := call(api.CertificateTargets, "/", nil); code != 0 || !strings.Contains(body, service.HostCode) {
		t.Fatalf("native selector failed: %s", body)
	}
	input := request.WafSslorderaddReq{HostCode: service.HostCode, ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", PrivateGroupName: "household", ApplyEmail: "owner@example.com", ApplyDomain: "example.com,*.example.com"}
	if code, body := call(api.AddApi, "/", input); code != 0 {
		t.Fatalf("DNS order wrongly requires public ports: %s", body)
	}
	queued := <-global.GWAF_CHAN_SSLOrder
	if queued.Content.(model.SslOrder).PrivateGroupName != "household" || queued.Content.(model.SslOrder).ApplyDomain != input.ApplyDomain {
		t.Fatal("order lost DNS group or identifiers")
	}
	// The console's initial and post-submit queries have empty optional filters.
	// Exercise the real JSON binding + database read, not a mocked list response.
	for _, tc := range []struct {
		name  string
		query map[string]any
		want  int
	}{
		{"default console query", map[string]any{"pageIndex": 1, "pageSize": 10, "host_code": "", "apply_domain": ""}, 1},
		{"omitted filters", map[string]any{"pageIndex": 1, "pageSize": 10}, 1},
		{"service only", map[string]any{"pageIndex": 1, "pageSize": 10, "host_code": service.HostCode}, 1},
		{"domain only", map[string]any{"pageIndex": 1, "pageSize": 10, "apply_domain": "example.com"}, 1},
		{"different service", map[string]any{"pageIndex": 1, "pageSize": 10, "host_code": "other"}, 0},
		{"different domain", map[string]any{"pageIndex": 1, "pageSize": 10, "apply_domain": "not-matching.example"}, 0},
	} {
		t.Run("accepted order appears in list/"+tc.name, func(t *testing.T) {
			code, body := call(api.GetListApi, "/api/v1/wafhost/sslorder/list", tc.query)
			if code != 0 {
				t.Fatalf("accepted order cannot be listed: %s", body)
			}
			var result struct {
				Data struct {
					List  []model.SslOrder `json:"list"`
					Total int              `json:"total"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Data.List) != tc.want || result.Data.Total != tc.want {
				t.Fatalf("list count %d/%d, want %d", len(result.Data.List), result.Data.Total, tc.want)
			}
			if tc.want == 1 && result.Data.List[0].Id != queued.Content.(model.SslOrder).Id {
				t.Fatal("list does not contain the accepted order")
			}
		})
	}
	input.ApplyMethod, input.ApplyDomain = "http01", "example.com"
	if code, _ := call(api.AddApi, "/", input); code == 0 {
		t.Fatal("HTTP order accepted without port 80")
	}

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.com"}, DNSNames: []string{"example.com", "*.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	order, claimed, err := waf_service.WafSSLOrderServiceApp.Claim(queued.Content.(model.SslOrder).Id)
	if err != nil || !claimed {
		t.Fatalf("claim issuance: %v", err)
	}
	order.ResultValidTo = customtype.JsonTime(cert.NotAfter)
	order.ResultCertificate = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	order.ResultPrivateKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := waf_service.RecordIssuedCertificate(&order); err != nil {
		t.Fatal(err)
	}
	if err := waf_service.DeployCertificateOrder(&order); err != nil {
		t.Fatal(err)
	}
	if err := waf_service.CompleteCertificateOrder(order, time.Now()); err != nil {
		t.Fatal(err)
	}
	renew := request.WafSslordereditReq{Id: order.Id, HostCode: service.HostCode, ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", PrivateGroupName: "household", ApplyEmail: "owner@example.com", ApplyDomain: "example.com,*.example.com", ApplyStatus: "renewed"}
	if code, body := call(api.ModifyApi, "/", renew); code != 0 {
		t.Fatalf("DNS renewal rejected: %s", body)
	}
	queued = <-global.GWAF_CHAN_SSLOrder
	if queued.Content.(model.SslOrder).PrivateGroupName != "household" || queued.Content.(model.SslOrder).ApplyMethod != "dns01" {
		t.Fatal("renewal lost its chosen provider/group")
	}
	code, body := call(api.DNSCredentialStatus, "/?provider=hurricane&group=household", nil)
	if code != 0 || !strings.Contains(body, "HURRICANE_TOKENS") || strings.Contains(body, "test-ddns-secret") || strings.Contains(body, "private_value") {
		t.Fatalf("credential status leaked or missing: %s", body)
	}
}

func createCertificateTestService(t *testing.T, db *gorm.DB) wafpipeline.PortConfig {
	t.Helper()
	if err := db.AutoMigrate(&model.PortConfig{}, &model.GateUser{}, &model.GateSession{}, &model.IPGroupItem{}); err != nil {
		t.Fatal(err)
	}
	oldPorts, oldPipeline, oldLog := wafpipeline.Ports, waf_service.WafPipelineServiceApp, global.GWAF_LOCAL_LOG_DB
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	waf_service.WafPipelineServiceApp = &waf_service.WafPipelineService{}
	global.GWAF_LOCAL_LOG_DB = nil
	t.Cleanup(func() {
		wafpipeline.Ports, waf_service.WafPipelineServiceApp, global.GWAF_LOCAL_LOG_DB = oldPorts, oldPipeline, oldLog
	})
	config := wafpipeline.PortConfig{Schema: 1, Port: 18443, Name: "NAS", Protocol: "http", HostNames: []string{"example.com", "*.example.com"}, Upstream: "http://127.0.0.1:5000", Policy: wafpipeline.DefaultPolicy()}
	config.Policy.Defense.Enabled = true
	saved, err := waf_service.WafPipelineServiceApp.Save(waf_service.PortSaveRequest{Config: config})
	if err != nil {
		t.Fatal(err)
	}
	return saved
}
