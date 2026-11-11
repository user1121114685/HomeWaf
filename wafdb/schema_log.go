// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"gorm.io/gorm"
)

var logSchemaModels = []any{
	&innerbean.WebLog{},
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
	{"web_logs", "idx_web_logs_ai_score_day", "CREATE INDEX IF NOT EXISTS idx_web_logs_ai_score_day ON web_logs (ai_score, day)"},
	{"host_login_event", "idx_hle_ip_time", "CREATE INDEX IF NOT EXISTS idx_hle_ip_time ON host_login_event(ip, event_time)"},
	{"host_login_event", "idx_hle_src_time", "CREATE INDEX IF NOT EXISTS idx_hle_src_time ON host_login_event(source, event_time)"},
	{"host_login_event", "idx_hle_event_time", "CREATE INDEX IF NOT EXISTS idx_hle_event_time ON host_login_event(event_time)"},
	{"threat_ip_exclude_audit", "idx_tiea_create_time", "CREATE INDEX IF NOT EXISTS idx_tiea_create_time ON threat_ip_exclude_audit(create_time)"},
	{"security_audit_log", "idx_security_audit_day_event", "CREATE INDEX IF NOT EXISTS idx_security_audit_day_event ON security_audit_log (day, event)"},
	{"security_audit_log", "idx_security_audit_account", "CREATE INDEX IF NOT EXISTS idx_security_audit_account ON security_audit_log (account_name)"},
	{"security_audit_log", "idx_security_audit_ip", "CREATE INDEX IF NOT EXISTS idx_security_audit_ip ON security_audit_log (client_ip)"},
	{"security_audit_log", "idx_security_audit_category", "CREATE INDEX IF NOT EXISTS idx_security_audit_category ON security_audit_log (category)"},
	{"web_logs", "idx_web_ip_time", "CREATE INDEX IF NOT EXISTS idx_web_ip_time ON web_logs (tenant_id, user_code, src_ip, unix_add_time desc)"},
	{"web_logs", "idx_web_rule_time", "CREATE INDEX IF NOT EXISTS idx_web_rule_time ON web_logs (tenant_id, user_code, rule, unix_add_time desc)"},
	{"web_logs", "idx_web_day_isbot", "CREATE INDEX IF NOT EXISTS idx_web_day_isbot ON web_logs (day, is_bot)"},
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
	{"web_logs", "idx_web_logs_task_flag_time", "CREATE INDEX IF NOT EXISTS idx_web_logs_task_flag_time ON web_logs (task_flag, unix_add_time)"},
	{"web_logs", "idx_web_time_tenant_user_code", "CREATE INDEX IF NOT EXISTS idx_web_time_tenant_user_code ON web_logs (unix_add_time, tenant_id, user_code)"},
	{"web_logs", "idx_req_uuid_web_logs", "CREATE INDEX IF NOT EXISTS idx_req_uuid_web_logs ON web_logs (REQ_UUID, tenant_id, user_code)"},
	{"web_logs", "idx_tenant_usercode_web_logs", "CREATE INDEX IF NOT EXISTS idx_tenant_usercode_web_logs ON web_logs (tenant_id, user_code)"},
	{"web_logs", "idx_web_time_desc_tenant_user_code", "CREATE INDEX IF NOT EXISTS idx_web_time_desc_tenant_user_code ON web_logs (unix_add_time desc, tenant_id, user_code)"},
	{"web_logs", "idx_web_time_desc_tenant_user_code_ip", "CREATE INDEX IF NOT EXISTS idx_web_time_desc_tenant_user_code_ip ON web_logs (unix_add_time desc, tenant_id, user_code, src_ip)"},
	{"web_logs", "idx_web_guest_id_entification", "CREATE INDEX IF NOT EXISTS idx_web_guest_id_entification ON web_logs (guest_id_entification, day, is_bot, host_code)"},
}

// InitializeLogSchema creates the current tables and indexes without replaying historical upgrades.
func InitializeLogSchema(db *gorm.DB) error {
	return initializeSchema(db, logSchemaModels, logSchemaIndexes)
}
