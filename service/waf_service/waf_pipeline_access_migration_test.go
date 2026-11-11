package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"strings"
	"testing"
)

func TestSSOMigrationPreservesAuthorizedAccountsAndEncryptedTOTP(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.AccessConfig{}, &model.AccessAccount{}, &model.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Update("access_json", `{"mode":1}`).Error; err != nil {
		t.Fatal(err)
	}
	config := model.DefaultAccessConfig()
	config.BaseOrm = pipelineORM()
	config.RequireOtp = 1
	config.ServiceTokenHeader = "X-Source-Token"
	config.ServiceTokenHashes = strings.Repeat("a", 64)
	config.CenterOrigin = "https://auth.test:5443"
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.BcryptHash("sso-source-safe-password")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := encryptAccessSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []model.AccessAccount{{BaseOrm: pipelineORM(), AccountName: "family", Password: hash, Status: 1, AllowHostCodes: "camera", OtpBound: 1, OtpSecret: secret}, {BaseOrm: pipelineORM(), AccountName: "other-site", Password: hash, Status: 1, AllowHostCodes: "nas"}} {
		if err := db.Create(&account).Error; err != nil {
			t.Fatal(err)
		}
	}
	foreign := model.AccessAccount{BaseOrm: pipelineORM(), AccountName: "other-tenant", Password: hash, Status: 1}
	foreign.Tenant_ID += "-other"
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	if !p.Config.Policy.Gate.Enabled || p.Config.Policy.Gate.Mode != "form" || !p.RequiresTOTP || len(p.AccountNames) != 1 || p.AccountNames[0] != "family" {
		t.Fatal("SSO identity constraints lost")
	}
	if p.Config.Policy.Gate.TokenHeader != "X-Source-Token" || len(p.secrets.ServiceTokenHashes) != 1 {
		t.Fatal("legacy service-token policy lost")
	}
	public, _ := json.Marshal(p)
	if strings.Contains(string(public), "JBSWY3DPEHPK3PXP") || strings.Contains(string(public), hash) {
		t.Fatal("SSO preview leaked credentials")
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "camera", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err != nil {
		t.Fatal(err)
	}
	account, ok := WafPipelineGateServiceApp.Store.Account(wafpipeline.PortID(5300), "family")
	if !ok || account.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Fatal("required OTP was not loaded into native gate")
	}
	if _, ok := WafPipelineGateServiceApp.Store.Account(wafpipeline.PortID(5300), "other-site"); ok {
		t.Fatal("copied unauthorized SSO account")
	}
	if _, ok := WafPipelineGateServiceApp.Store.Account(wafpipeline.PortID(5300), "other-tenant"); ok {
		t.Fatal("copied cross-tenant account")
	}
	var stored string
	if err := db.Raw("SELECT totp_secret FROM gate_user WHERE username = ?", "family").Scan(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored == "" || strings.Contains(stored, "JBSWY3DPEHPK3PXP") {
		t.Fatal("OTP key persisted in plaintext")
	}
	users, err := WafPipelineGateServiceApp.Users(5300)
	if err != nil {
		t.Fatal(err)
	}
	public, _ = json.Marshal(users)
	if !users[0].TOTPConfigured || strings.Contains(string(public), "JBSWY3DPEHPK3PXP") {
		t.Fatal("account response exposed or omitted MFA status")
	}
}

func TestDualLegacyAuthenticationRequiresNewAccountWithoutBroaderExceptions(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.AccessConfig{}, &model.AccessAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "nas").Update("access_json", `{"mode":1,"exclude_paths":"/public"}`).Error; err != nil {
		t.Fatal(err)
	}
	cfg := model.DefaultAccessConfig()
	cfg.BaseOrm = pipelineORM()
	cfg.GlobalExcludePaths = "/webhook"
	cfg.ServiceTokenHeader = "X-Source-Token"
	cfg.ServiceTokenHashes = strings.Repeat("a", 64)
	cfg.RequireOtp = 1
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	hash, _ := utils.BcryptHash("sso-source-safe-password")
	secret, err := encryptAccessSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AccessAccount{BaseOrm: pipelineORM(), AccountName: "sso-owner", Password: hash, Status: 1, OtpBound: 1, OtpSecret: secret}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	if len(p.AccountNames) != 0 || !p.RequiresTOTP || len(p.Config.Policy.Gate.Exempt.Paths) > 0 || p.Config.Policy.Gate.TokenHeader != "" || len(p.secrets.ServiceTokenHashes) > 0 {
		t.Fatal("combined old authentication was weakened to either mechanism")
	}
	req := LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true, Username: "new-family", Password: "new-family-safe-password"}
	if _, err := s.ImportLegacyPort(req, "tester"); err == nil {
		t.Fatal("fresh replacement account discarded required OTP")
	}
	req.TOTPSecret = "JBSWY3DPEHPK3PXP"
	if _, err := s.ImportLegacyPort(req, "tester"); err != nil {
		t.Fatal(err)
	}
}

func TestSSOCenterCannotBeTakenBeforeItsLegacyConsumers(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.AccessConfig{}, &model.AccessAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Updates(map[string]any{"access_json": `{"mode":1}`, "remote_host": "", "remote_ip": ""}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := model.DefaultAccessConfig()
	cfg.BaseOrm = pipelineORM()
	cfg.CenterOrigin = "http://camera.test:5300"
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Hosts{BaseOrm: pipelineORM(), Code: "other-sso", Host: "other.test", Port: 5400, Remote_host: "http://127.0.0.1:8080", AccessJSON: `{"mode":1}`}).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(preview.Blockers, " "), "认证中心") {
		t.Fatal("migration could dismantle active old SSO center")
	}
	wafpipeline.SetLegacyPortOwner(5400, true)
	defer wafpipeline.SetLegacyPortOwner(5400, false)
	preview, err = s.PreviewLegacyPort(5300, "camera")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(preview.Blockers, " "), "认证中心") {
		t.Fatal("center remained blocked after its consumer was handed off")
	}
	if len(preview.Blockers) != 0 {
		t.Fatal("authentication-only site could not migrate", preview.Blockers)
	}
}

func TestLegacyAccountPreviewDefersHashAndLimitsSourceRows(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.Model(&model.HttpAuthBase{}).Where("host_code = ?", "nas").Updates(map[string]any{"password": "plaintext-migration-fixture", "password_hashed": false}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.accounts) != 1 {
		t.Fatal("preview failed", err)
	}
	if p.accounts[0].PasswordHash != "" {
		t.Fatal("preview performed bcrypt instead of deferring work")
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err != nil {
		t.Fatal(err)
	}
	u, ok := WafPipelineGateServiceApp.Store.Account(wafpipeline.PortID(5300), "owner")
	if !ok || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("plaintext-migration-fixture")) != nil {
		t.Fatal("plaintext source was not hashed on import")
	}
	var source model.HttpAuthBase
	if err := db.Where("host_code = ?", "nas").First(&source).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]model.HttpAuthBase, 1000)
	for i := 0; i < 1000; i++ {
		rows[i] = source
		rows[i].Id = "source-" + strconv.Itoa(i)
	}
	if err := db.CreateInBatches(rows, 50).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewLegacyPort(5300, "nas"); err == nil || !strings.Contains(err.Error(), "1000") {
		t.Fatal("account query was not bounded before processing")
	}
}

func TestNativeTokenDraftMustHaveCredentialBeforeActivation(t *testing.T) {
	s, _ := legacyMigrationFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 5401, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.RequireToken = true
	c.Policy.Gate.TokenHeader = "X-HomeWaf-Token"
	var err error
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	c.ListenEnabled = true
	if _, err := s.Save(PortSaveRequest{Config: c}); err == nil || !strings.Contains(err.Error(), "服务令牌") {
		t.Fatal("token-less draft could activate")
	}
}

func TestSSOHashCostMismatchStillRequiresMFAForReplacement(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.AccessConfig{}, &model.AccessAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "camera").Update("access_json", `{"mode":1}`).Error; err != nil {
		t.Fatal(err)
	}
	cfg := model.DefaultAccessConfig()
	cfg.BaseOrm = pipelineORM()
	cfg.RequireOtp = 1
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("old-safe-password"), bcrypt.MinCost)
	secret, err := encryptAccessSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AccessAccount{BaseOrm: pipelineORM(), AccountName: "family", Password: string(hash), Status: 1, OtpBound: 1, OtpSecret: secret}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || !p.RequiresTOTP || len(p.AccountNames) != 0 {
		t.Fatal("hash reset weakened MFA or retained mismatched bcrypt cost", err)
	}
}
