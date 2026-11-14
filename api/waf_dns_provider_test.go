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
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/model/spec"
	"HomeWaf/service/waf_service"
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
	if err := db.AutoMigrate(&model.PrivateInfo{}, &model.Hosts{}, &model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldQueue := global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder
	global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = db, make(chan spec.ChanSslOrder, 5)
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = oldDB, oldQueue
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := db.Create(&model.Hosts{BaseOrm: baseorm.BaseOrm{Id: "nas-host"}, Code: "nas", Host: "192.168.1.20", Port: 8443, Ssl: 1}).Error; err != nil {
		t.Fatal(err)
	}
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
	input := request.WafSslorderaddReq{HostCode: "nas", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", PrivateGroupName: "household", ApplyEmail: "owner@example.com", ApplyDomain: "example.com,*.example.com"}
	if code, body := call(api.AddApi, "/", input); code != 0 {
		t.Fatalf("DNS order wrongly requires public ports: %s", body)
	}
	queued := <-global.GWAF_CHAN_SSLOrder
	if queued.Content.(model.SslOrder).PrivateGroupName != "household" || queued.Content.(model.SslOrder).ApplyDomain != input.ApplyDomain {
		t.Fatal("order lost DNS group or identifiers")
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
	order := queued.Content.(model.SslOrder)
	order.ApplyMethod = "http01" // Migration to DNS at renewal must check the new method.
	order.ApplyStatus = "success"
	order.ResultValidTo = customtype.JsonTime(cert.NotAfter)
	order.ResultCertificate = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	order.ResultPrivateKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	renew := request.WafSslordereditReq{Id: order.Id, HostCode: "nas", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", PrivateGroupName: "household", ApplyEmail: "owner@example.com", ApplyDomain: "example.com,*.example.com", ApplyStatus: "renewed"}
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
