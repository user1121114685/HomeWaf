package waf_service

import (
	"strings"
	"testing"

	"HomeWaf/model"
	"HomeWaf/model/request"
)

func TestDNSCredentialGroupIsolationAndPersistence(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.PrivateInfo{}); err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"default", "second"} {
		if err := WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: group, PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:original-token"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := WafPrivateInfoServiceApp.ModifyApi(request.WafPrivateInfoEditReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "second", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:replacement-token"}); err != nil {
		t.Fatal(err)
	}
	if err := WafPrivateInfoServiceApp.ModifyWithOutValueApi(request.WafPrivateInfoEditReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "second", PrivateKey: "HURRICANE_TOKENS", Remarks: "second only"}); err != nil {
		t.Fatal(err)
	}
	first, err := LoadDNSCredentials("hurricane", "default")
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadDNSCredentials("hurricane", "second")
	if err != nil {
		t.Fatal(err)
	}
	if first["HURRICANE_TOKENS"] != "example.com:original-token" || second["HURRICANE_TOKENS"] != "example.com:replacement-token" {
		t.Fatal("editing one group changed another or lost stored credentials")
	}
	var raw string
	if err := db.Raw("SELECT private_value FROM private_infos WHERE private_group_name = ?", "second").Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "replacement-token") {
		t.Fatal("credential persisted as plaintext")
	}
	if err := WafPrivateInfoServiceApp.ModifyApi(request.WafPrivateInfoEditReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "deleted-group", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:replacement-token"}); err == nil {
		t.Fatal("stale credential edit falsely reported success")
	}
	if _, err := LoadDNSCredentials("hurricane", "missing"); err == nil {
		t.Fatal("missing DDNS group accepted")
	}
	if err := db.Migrator().DropTable(&model.PrivateInfo{}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDNSCredentials("cloudflare", "default"); err == nil {
		t.Fatal("database error ignored")
	}
}

func TestDNSHomeNetworkChallengeValidation(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.PrivateInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: "default", PrivateKey: "HURRICANE_TOKENS", PrivateValue: "example.com:test-token"}); err != nil {
		t.Fatal(err)
	}
	// No listeners or public IP are present in this database.
	domains, err := ValidateSSLOrderChallenge("dns01", "hurricane", "default", "Example.COM\n*.example.com,example.com", 0)
	if err != nil || domains != "example.com,*.example.com" {
		t.Fatalf("home DNS order rejected: %s / %v", domains, err)
	}
	for _, tc := range []struct{ method, provider, domains string }{
		{"dns01", "hurricane", "192.0.2.1"}, {"http01", "", "*.example.com"},
		{"dns01", "bogus", "example.com"}, {"dns01", "manual", "example.com"},
		{"dns01", "hurricane", "https://example.com:8443"}, {"http01", "", "192.0.2.1,example.com"},
	} {
		if _, err := ValidateSSLOrderChallenge(tc.method, tc.provider, "default", tc.domains, 0); err == nil {
			t.Errorf("invalid challenge accepted: %+v", tc)
		}
	}
}
