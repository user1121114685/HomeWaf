package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"testing"
)

func TestLegacyRequestAndResponsePoliciesAreCopiedAsPortSnapshots(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.Sensitive{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Sensitive{BaseOrm: pipelineORM(), Content: "private-content", Action: "replace", CheckDirection: "all"}).Error; err != nil {
		t.Fatal(err)
	}
	values := map[string]any{"csrf_json": `{"is_enable":1,"protect_methods":"POST","allowed_origins":"https://family.test","allow_empty_ref":0,"exclude_paths":"/callback"}`, "upload_security_json": `{"is_enable":1,"check_ext":1,"ext_blacklist":"private","over_limit_action":"pass","max_size_kb":256}`, "anti_leech_json": `{"is_enable_anti_leech":1,"file_types":"png","valid_referers":"server_names","action":"block"}`, "cookie_security_json": `{"is_enable":1,"http_only":1,"secure":2,"same_site":"Lax"}`, "response_compress_json": `{"is_enable":1,"prefer":"gzip_only"}`, "emergency_mode": 1, "emergency_until": int64(2000000000)}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "nas").Updates(values).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	r := p.Config.Policy.RuleSet
	if r.UploadOptions == nil || r.UploadOptions.ExtBlacklist != "private" || r.CSRFOptions == nil || r.CSRFOptions.ExcludePaths != "/callback" || r.AntiLeech == nil || r.Challenge.Until != 2000000000 || r.Sensitive == nil || len(r.Sensitive.Rules) != 1 || p.Config.Policy.Network.Response.Cookie.IsEnable != 1 || p.Config.Policy.Network.Response.Compression.Prefer != "gzip_only" {
		t.Fatal("source check options not copied")
	}
	config, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Sensitive{}).Where("content = ?", "private-content").Update("content", "changed-source").Error; err != nil {
		t.Fatal(err)
	}
	stored, err := s.Get(config.Port)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Policy.RuleSet.Sensitive.Rules[0].Content != "private-content" {
		t.Fatal("port still inherits mutable global sensitive source")
	}
	if stored.Policy.RuleSet.AntiLeech.Mode != wafpipeline.ModeBlock {
		t.Fatal("active source mode lost")
	}
}
