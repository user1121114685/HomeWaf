// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/internal/model"
	"gorm.io/gorm"
)

var logSchemaModels = []any{
	&model.AccountLog{},
	&model.WafSysLog{},
	&model.OneKeyMod{},
	&model.NotifyLog{},
	&model.OPlatformLog{},
	&model.SecurityAuditLog{},
	&model.PipelineLog{},
	&model.LoginHistory{},
	&model.HostLoginEvent{},
	&model.ThreatIPExcludeAudit{},
	&model.EventPayload{},
	&model.AccessLog{},
	&model.SecurityEvent{},
}

var logSchemaIndexes = []schemaIndex{
	{"host_login_event", "idx_hle_ip_time", "CREATE INDEX IF NOT EXISTS idx_hle_ip_time ON host_login_event(ip, event_time)"},
	{"host_login_event", "idx_hle_src_time", "CREATE INDEX IF NOT EXISTS idx_hle_src_time ON host_login_event(source, event_time)"},
	{"host_login_event", "idx_hle_event_time", "CREATE INDEX IF NOT EXISTS idx_hle_event_time ON host_login_event(event_time)"},
	{"threat_ip_exclude_audit", "idx_tiea_create_time", "CREATE INDEX IF NOT EXISTS idx_tiea_create_time ON threat_ip_exclude_audit(create_time)"},
	{"security_audit_log", "idx_security_audit_day_event", "CREATE INDEX IF NOT EXISTS idx_security_audit_day_event ON security_audit_log (day, event)"},
	{"security_audit_log", "idx_security_audit_account", "CREATE INDEX IF NOT EXISTS idx_security_audit_account ON security_audit_log (account_name)"},
	{"security_audit_log", "idx_security_audit_ip", "CREATE INDEX IF NOT EXISTS idx_security_audit_ip ON security_audit_log (client_ip)"},
	{"security_audit_log", "idx_security_audit_category", "CREATE INDEX IF NOT EXISTS idx_security_audit_category ON security_audit_log (category)"},
	{"access_log", "idx_al_time", "CREATE INDEX IF NOT EXISTS idx_al_time ON access_log (tenant_id, user_code, unix_add_time desc)"},
	{"access_log", "idx_al_ip_time", "CREATE INDEX IF NOT EXISTS idx_al_ip_time ON access_log (tenant_id, user_code, src_ip, unix_add_time desc)"},
	{"access_log", "idx_al_day_isbot", "CREATE INDEX IF NOT EXISTS idx_al_day_isbot ON access_log (day, is_bot)"},
	{"security_event", "idx_se_time", "CREATE INDEX IF NOT EXISTS idx_se_time ON security_event (tenant_id, user_code, unix_add_time desc)"},
	{"security_event", "idx_se_ip_time", "CREATE INDEX IF NOT EXISTS idx_se_ip_time ON security_event (tenant_id, user_code, src_ip, unix_add_time desc)"},
	{"security_event", "idx_se_rule_time", "CREATE INDEX IF NOT EXISTS idx_se_rule_time ON security_event (tenant_id, user_code, rule, unix_add_time desc)"},
	{"access_log", "idx_al_create_time", "CREATE INDEX IF NOT EXISTS idx_al_create_time ON access_log (create_time)"},
	{"security_event", "idx_se_create_time", "CREATE INDEX IF NOT EXISTS idx_se_create_time ON security_event (create_time)"},
	{"access_log", "idx_al_actor", "CREATE INDEX IF NOT EXISTS idx_al_actor ON access_log (tenant_id, user_code, actor_key, day)"},
	{"security_event", "idx_se_actor", "CREATE INDEX IF NOT EXISTS idx_se_actor ON security_event (tenant_id, user_code, actor_key, day)"},
}

// InitializeLogSchema creates the current tables and indexes without replaying historical upgrades.
func InitializeLogSchema(db *gorm.DB) error {
	return initializeSchema(db, logSchemaModels, logSchemaIndexes)
}
