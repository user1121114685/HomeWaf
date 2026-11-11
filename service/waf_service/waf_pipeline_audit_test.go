package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"strings"
	"testing"
)

func TestPortAuditRecordsChangedSectionsWithoutSecretValues(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.SecurityAuditLog{}, &model.ConfigTemplate{}); err != nil {
		t.Fatal(err)
	}
	oldLogs, oldQueue, oldMessages := global.GWAF_LOCAL_LOG_DB, global.GQEQUE_LOG_DB, global.GQEQUE_MESSAGE_DB
	global.GWAF_LOCAL_LOG_DB, global.GQEQUE_LOG_DB, global.GQEQUE_MESSAGE_DB = db, nil, nil
	defer func() {
		global.GWAF_LOCAL_LOG_DB = oldLogs
		global.GQEQUE_LOG_DB = oldQueue
		global.GQEQUE_MESSAGE_DB = oldMessages
	}()
	c := wafpipeline.PortConfig{Schema: 1, Port: 54321, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.Enabled = false
	c, err := s.Save(PortSaveRequest{Config: c, Operator: "fixture-reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	c.Policy.Access.Enabled = false
	c.Upstream = "http://private-origin.example:8080"
	c, err = s.Save(PortSaveRequest{Config: c, Operator: "fixture-reviewer", BusinessHeaders: map[string]wafpipeline.BusinessHeaders{"profile": {Request: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: "Bearer secret-value"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	var audit model.SecurityAuditLog
	if err = db.Where("event = ? AND host_code = ? AND message LIKE ?", "port_config_saved", wafpipeline.PortID(c.Port), "revision=2;%").First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.AccountName != "fixture-reviewer" || !strings.Contains(audit.Message, "revision=2") || !strings.Contains(audit.Message, "policy.access") || !strings.Contains(audit.Message, "upstream") || !strings.Contains(audit.Message, "credentials") {
		t.Fatalf("incomplete change audit: %+v", audit)
	}
	if strings.Contains(audit.Message, "secret-value") || strings.Contains(audit.Message, "private-origin") {
		t.Fatal("audit exposed configuration values")
	}
	template := wafpipeline.TemplateFromPort("audit-template", "审计夹具", c)
	if _, err = s.SaveTemplate(template, "fixture-reviewer"); err != nil {
		t.Fatal(err)
	}
	audit = model.SecurityAuditLog{}
	if err = db.Where("event = ?", "template_saved").First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.Message, "template=audit-template") || !strings.Contains(audit.Message, "version=1") {
		t.Fatal("template audit omitted identity")
	}
}
