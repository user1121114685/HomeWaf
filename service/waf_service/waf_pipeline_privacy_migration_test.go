package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"strings"
	"testing"
)

func TestPrivacyMigrationAndSourceGroupOwnership(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.LDPUrl{}, &model.Rules{}, &model.IPGroup{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.LDPUrl{BaseOrm: pipelineORM(), HostCode: "camera", CompareType: "前缀匹配", Url: "/private"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Rules{BaseOrm: pipelineORM(), HostCode: "camera", RuleCode: "source-group", RuleName: "SourceGroup", RuleStatus: 1, RuleContent: `rule SourceGroup "" salience 10 { when RF.IPInGroup(MF.SRC_IP,"family") then RF.Deny(); }`}).Error; err != nil {
		t.Fatal(err)
	}
	owned := model.IPGroup{BaseOrm: pipelineORM(), GroupCode: "family-owned", GroupName: "family"}
	foreign := model.IPGroup{BaseOrm: pipelineORM(), GroupCode: "family-foreign", GroupName: "foreign"}
	foreign.Tenant_ID += "other"
	for _, g := range []model.IPGroup{owned, foreign} {
		if err := db.Create(&g).Error; err != nil {
			t.Fatal(err)
		}
		item := model.IPGroupItem{BaseOrm: g.BaseOrm, GroupCode: g.GroupCode, Ip: "203.0.113.0/24"}
		item.Id += "-item"
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(5300, "camera")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	if p.Config.Policy.Network.Response.Privacy == nil || p.Config.Policy.Network.Response.Privacy.URLs[0].Value != "/private" || len(p.Config.ScriptGroups["family"]) != 1 || len(p.Config.ScriptGroups["foreign"]) != 0 {
		t.Fatal("privacy policy lost or foreign group copied")
	}
	if err := db.Create(&model.IPBlockList{BaseOrm: pipelineORM(), HostCode: "camera", IpType: model.IPEntryTypeGroup, GroupCode: "family-foreign", IsEnable: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewLegacyPort(5300, "camera"); err == nil || !strings.Contains(err.Error(), "用户/租户") {
		t.Fatal("unreadable blacklist group silently lost")
	}
	if p.Config.Policy.Network.Response.Privacy.Mode != wafpipeline.ModeBlock {
		t.Fatal("source privacy policy not active")
	}
}
