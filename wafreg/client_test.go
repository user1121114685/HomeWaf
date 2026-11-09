package wafreg

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	"HomeWaf/model"
	"HomeWaf/wafsec"
)

func TestEntitlementAuthenticatesFileOnEveryDecision(t *testing.T) {
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	t.Setenv("HOMEWAF_DEV", "1")
	oldKeys, oldUser, oldTenant, oldInfo := licensing.TrustedKeysJSON, global.GWAF_USER_CODE, global.GWAF_TENANT_ID, global.GWAF_REG_INFO
	t.Cleanup(func() {
		licensing.TrustedKeysJSON, global.GWAF_USER_CODE, global.GWAF_TENANT_ID, global.GWAF_REG_INFO = oldKeys, oldUser, oldTenant, oldInfo
	})
	global.GWAF_USER_CODE, global.GWAF_TENANT_ID = "fixture-user", "fixture-tenant"
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]string{"fixture": base64.RawURLEncoding.EncodeToString(pub)})
	licensing.TrustedKeysJSON = string(keys)
	now := time.Now()
	claims := licensing.Claims{Version: 2, Product: licensing.Product, LicenseID: "fixture", Subject: "test", InstanceID: GenClientMachineInfo().MachineID, Edition: "commercial", Features: []string{"fixture.feature"}, IssuedAt: now.Add(-time.Minute), NotBefore: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	document, err := licensing.Sign(claims, "fixture", private)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(document); err != nil {
		t.Fatal(err)
	}
	if !HasFeature("fixture.feature") {
		t.Fatal("valid signed license rejected")
	}
	global.GWAF_REG_INFO = model.RegistrationInfo{MemberType: "commercial", Features: []string{"ungranted"}, ExpiryDate: now.AddDate(100, 0, 0)}
	if HasFeature("ungranted") {
		t.Fatal("display state granted a feature")
	}
	global.GWAF_USER_CODE = "other-instance"
	if HasFeature("fixture.feature") || CurrentInfo().MemberType != "noncommercial" {
		t.Fatal("instance change retained a grant")
	}
	global.GWAF_USER_CODE = "fixture-user"
	var envelope licensing.Envelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	claims.Features = []string{"ungranted"}
	payload, _ := json.Marshal(claims)
	envelope.Payload = base64.RawURLEncoding.EncodeToString(payload)
	forged, _ := json.Marshal(envelope)
	for _, invalid := range [][]byte{forged, []byte(`{"edition":"commercial","features":["fixture.feature"]}`), make([]byte, licensing.MaxDocumentSize+1)} {
		if err := os.WriteFile(licensePath(), invalid, 0600); err != nil {
			t.Fatal(err)
		}
		if HasFeature("fixture.feature") || HasFeature("ungranted") || CurrentInfo().MemberType != "noncommercial" {
			t.Fatal("file tampering retained entitlement")
		}
	}
	if err := Save(document); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(licensePath()); err != nil {
		t.Fatal(err)
	}
	if HasFeature("fixture.feature") {
		t.Fatal("deleted license remained active")
	}
}
