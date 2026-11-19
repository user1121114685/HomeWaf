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

	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/service/waf_service"
	"github.com/gin-gonic/gin"
)

func certificateEditorTestPair(t *testing.T, serial int64) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "editor.example.test"}, DNSNames: []string{"editor.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

func TestCertificateEditorValidationAndSaveResult(t *testing.T) {
	setupNotifyTestEnv(t, &model.SslConfig{}, &model.Hosts{}, &model.SecurityAuditLog{})
	cert, key := certificateEditorTestPair(t, 1)
	_, otherKey := certificateEditorTestPair(t, 2)
	id, err := waf_service.WafSslConfigServiceApp.AddApi(request.SslConfigAddReq{CertContent: cert, KeyContent: key})
	if err != nil {
		t.Fatal(err)
	}
	otherCert, otherPairKey := certificateEditorTestPair(t, 3)
	otherID, err := waf_service.WafSslConfigServiceApp.AddApi(request.SslConfigAddReq{CertContent: otherCert, KeyContent: otherPairKey})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FieldErrors  map[string]string `json:"field_errors"`
			ExportFailed bool              `json:"export_failed"`
		} `json:"data"`
	}
	call := func(input request.SslConfigEditReq) result {
		t.Helper()
		payload, _ := json.Marshal(input)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Set("is_openapi", true)
		ctx.Request = httptest.NewRequest("POST", "/api/v1/sslconfig/edit", bytes.NewReader(payload))
		ctx.Request.Header.Set("Content-Type", "application/json")
		new(WafSslConfigApi).ModifySslConfigApi(ctx)
		if strings.Contains(recorder.Body.String(), "BEGIN PRIVATE KEY") || strings.Contains(recorder.Body.String(), "BEGIN CERTIFICATE") {
			t.Fatal("response exposed certificate material")
		}
		var got result
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, tc := range []struct{ name, cert, key, field string }{
		{"missing PEM", "invalid", key, "cert_content"},
		{"invalid certificate DER", "-----BEGIN CERTIFICATE-----\nZm9v\n-----END CERTIFICATE-----", key, "cert_content"},
		{"mismatched private key", cert, otherKey, "key_content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := call(request.SslConfigEditReq{Id: id, CertContent: tc.cert, KeyContent: tc.key, AutoLoadPath: new(0), ExportCertPath: new(string), ExportKeyPath: new(string)})
			if got.Code == 0 || got.Data.FieldErrors[tc.field] == "" || got.Msg != got.Data.FieldErrors[tc.field] {
				t.Fatalf("expected an actionable field error: %+v", got)
			}
			saved := waf_service.WafSslConfigServiceApp.GetDetailInner(id)
			if saved.CertContent != cert || saved.KeyContent != key || len(global.GWAF_CHAN_MSG) != 0 {
				t.Fatal("invalid edit changed the saved certificate or notified runtime")
			}
		})
	}
	disabled := 0
	crtPath, keyPath := filepath.Join(t.TempDir(), "read.crt"), filepath.Join(t.TempDir(), "read.key")
	got := call(request.SslConfigEditReq{Id: id, CertContent: cert, KeyContent: key, AutoLoadPath: &disabled, CertPath: crtPath, KeyPath: keyPath, ExportCertPath: new(string), ExportKeyPath: new(string)})
	if got.Code != 0 || got.Data.ExportFailed {
		t.Fatalf("save failed: %+v", got)
	}
	saved := waf_service.WafSslConfigServiceApp.GetDetailInner(id)
	if saved.AutoLoadPath != 0 || saved.CertPath != crtPath || saved.KeyPath != keyPath {
		t.Fatal("path settings did not persist")
	}
	// Export denial is a partial result: the valid certificate remains saved.
	oldAllowed := global.CurrentSettings().GCONFIG_SSL_EXPORT_ALLOWED_DIRS
	allowed := t.TempDir()
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_SSL_EXPORT_ALLOWED_DIRS = allowed })
	t.Cleanup(func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_SSL_EXPORT_ALLOWED_DIRS = oldAllowed })
	})
	exportPath := filepath.Join(t.TempDir(), "denied.crt")
	got = call(request.SslConfigEditReq{Id: id, CertContent: cert, KeyContent: key, AutoLoadPath: &disabled, CertPath: crtPath, KeyPath: keyPath, ExportCertPath: &exportPath, ExportKeyPath: new(string)})
	if got.Code != 0 || !got.Data.ExportFailed || !strings.Contains(got.Msg, "导出失败") {
		t.Fatalf("partial export failure was not distinguished: %+v", got)
	}
	saved = waf_service.WafSslConfigServiceApp.GetDetailInner(id)
	if saved.ExportCertPath != exportPath || saved.KeyContent != key {
		t.Fatal("export failure lost saved configuration")
	}
	if missing := call(request.SslConfigEditReq{Id: id, CertContent: cert, KeyContent: key}); missing.Code == 0 {
		t.Fatal("certificate edit accepted missing file settings")
	}
	if waf_service.WafSslConfigServiceApp.GetDetailInner(id).ExportCertPath != exportPath {
		t.Fatal("incomplete edit cleared the saved export path")
	}
	other := waf_service.WafSslConfigServiceApp.GetDetailInner(otherID)
	if other.CertContent != otherCert || other.KeyContent != otherPairKey || other.ExportCertPath != "" {
		t.Fatal("edit changed another certificate")
	}
}

func TestCertificateAddReturnsManagedResourceIdentity(t *testing.T) {
	setupNotifyTestEnv(t, &model.SslConfig{}, &model.Hosts{}, &model.SecurityAuditLog{})
	cert, key := certificateEditorTestPair(t, 11)
	payload, _ := json.Marshal(request.SslConfigAddReq{CertContent: cert, KeyContent: key})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("is_openapi", true)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/sslconfig/add", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	new(WafSslConfigApi).AddSslConfigApi(ctx)
	var result struct {
		Code int `json:"code"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || result.Code != 0 || result.Data.ID == "" {
		t.Fatalf("missing saved identity: %s %v", recorder.Body.String(), err)
	}
	if strings.Contains(recorder.Body.String(), "PRIVATE KEY") {
		t.Fatal("save leaked private material")
	}
}
