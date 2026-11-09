package api

import (
	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	"HomeWaf/wafreg"
	"HomeWaf/wafsec"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Covers the actual import boundary: preview must not activate; another
// authenticated session must not consume it; invalid replacements fail closed.
func TestSignedLicenseImportBoundToSession(t *testing.T) {
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	t.Setenv("HOMEWAF_DEV", "1")
	oldKeys, oldUser, oldTenant := licensing.TrustedKeysJSON, global.GWAF_USER_CODE, global.GWAF_TENANT_ID
	t.Cleanup(func() {
		_ = os.Remove("registration_data.bin")
		_, _, _ = wafreg.VerifyServerRegByDefaultFile()
		licensing.TrustedKeysJSON, global.GWAF_USER_CODE, global.GWAF_TENANT_ID = oldKeys, oldUser, oldTenant
	})
	global.GWAF_USER_CODE, global.GWAF_TENANT_ID = "test-instance", "test-tenant"
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]string{"test": base64.RawURLEncoding.EncodeToString(pub)})
	licensing.TrustedKeysJSON = string(keys)
	now := time.Now().UTC()
	document, err := licensing.Sign(licensing.Claims{
		Version: 2, Product: licensing.Product, LicenseID: "test-license", Subject: "Synthetic test fixture",
		InstanceID: wafreg.GenClientMachineInfo().MachineID, Edition: "commercial", Features: []string{"fixture"},
		IssuedAt: now.Add(-time.Minute), NotBefore: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}, "test", private)
	if err != nil {
		t.Fatal(err)
	}
	call := func(token string, upload []byte) int {
		t.Helper()
		var body bytes.Buffer
		contentType := "application/json"
		if upload != nil {
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "license.json")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = part.Write(upload)
			_ = writer.Close()
			contentType = writer.FormDataContentType()
		}
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest("POST", "/api/v1/license/import-test", &body)
		c.Request.Header.Set("Content-Type", contentType)
		c.Request.Header.Set("X-Token", token)
		c.Set("loginAccount", "fixture-admin")
		api := WafLicenseApi{}
		if upload != nil {
			api.CheckLicense(c)
		} else {
			api.ConfirmApi(c)
		}
		var result struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Code
	}
	if code := call("session-a", document); code != 0 {
		t.Fatalf("preview rejected: %d", code)
	}
	if wafreg.HasFeature("fixture") {
		t.Fatal("preview activated the license")
	}
	if code := call("session-b", nil); code == 0 {
		t.Fatal("another session consumed the preview")
	}
	if code := call("session-a", nil); code != 0 {
		t.Fatalf("confirmation failed: %d", code)
	}
	if !wafreg.HasFeature("fixture") {
		t.Fatal("confirmed signed feature was not activated")
	}
	persisted, err := os.ReadFile("registration_data.bin")
	if err != nil || !bytes.Equal(persisted, document) {
		t.Fatal("license was not saved in the application directory")
	}
	if code := call("session-a", []byte(`{"version":2,"signature":"forged"}`)); code == 0 {
		t.Fatal("forged document accepted")
	}
	if !wafreg.HasFeature("fixture") {
		t.Fatal("invalid replacement removed the valid license")
	}
}
