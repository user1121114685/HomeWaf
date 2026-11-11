// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/model"
	"gorm.io/gorm"
)

var coreSchemaModels = []any{
	&model.PortConfig{},
	&model.PipelinePortMigration{},
	&model.PipelineFeed{},
	&model.ConfigTemplate{},
	&model.GateUser{},
	&model.GateSession{},
	&model.GateRuntime{},
	&model.PipelineState{},
	&model.PipelineDailyMetrics{},
	&model.Hosts{},
	&model.Rules{},
	&model.LDPUrl{},
	&model.IPAllowList{},
	&model.URLAllowList{},
	&model.IPBlockList{},
	&model.URLBlockList{},
	&model.AntiCC{},
	&model.TokenInfo{},
	&model.Account{},
	&model.SystemConfig{},
	&model.DelayMsg{},
	&model.ShareDb{},
	&model.Sensitive{},
	&model.LoadBalance{},
	&model.SslConfig{},
	&model.IPTag{},
	&model.BatchTask{},
	&model.SslOrder{},
	&model.SslExpire{},
	&model.HttpAuthBase{},
	&model.Task{},
	&model.BlockingPage{},
	&model.Otp{},
	&model.PrivateInfo{},
	&model.PrivateGroup{},
	&model.CacheRule{},
	&model.Tunnel{},
	&model.CaServerInfo{},
	&model.UIPreference{},
	&model.NotifyChannel{},
	&model.NotifySubscription{},
	&model.FirewallIPBlock{},
	&model.OPlatformKey{},
	&model.DataRetentionPolicy{},
	&model.HostPathRule{},
	&model.WafApp{},
	&model.WafAppChangeLog{},
	&model.WafLogLabelMark{},
	&model.TamperRule{},
	&model.AccountPwdHistory{},
	&model.ThreatIPChannel{},
	&model.ThreatIPSnapshot{},
	&model.CDNProvider{},
	&model.IPGroup{},
	&model.IPGroupItem{},
	&model.AccessAccount{},
	&model.AccessSession{},
	&model.AccessToken{},
	&model.AccessTicket{},
	&model.AccessConfig{},
	&model.HostGuardBan{},
	&model.HostGuardOffender{},
	&model.HostGuardBanLadder{},
	&model.ThreatIPExclude{},
	&model.UpgradeNoticeRecord{},
	&model.HostGroup{},
	&model.AntiCCRule{},
	&model.HttpAuthSession{},
	&model.IPWatchlist{},
	&model.LogExportTask{},
	&model.DashboardShare{},
	&model.DashboardTarget{},
}

var coreSchemaIndexes = []schemaIndex{
	{"firewall_ip_block", "idx_firewall_ip_block_ip", "CREATE INDEX IF NOT EXISTS idx_firewall_ip_block_ip ON firewall_ip_block(ip)"},
	{"firewall_ip_block", "idx_firewall_ip_block_status", "CREATE INDEX IF NOT EXISTS idx_firewall_ip_block_status ON firewall_ip_block(status)"},
	{"firewall_ip_block", "idx_firewall_ip_block_expire_time", "CREATE INDEX IF NOT EXISTS idx_firewall_ip_block_expire_time ON firewall_ip_block(expire_time)"},
	{"threat_ip_channel", "idx_threat_ip_channel_code", "CREATE INDEX IF NOT EXISTS idx_threat_ip_channel_code ON threat_ip_channel(code)"},
	{"threat_ip_snapshot", "idx_threat_ip_snapshot_channel", "CREATE INDEX IF NOT EXISTS idx_threat_ip_snapshot_channel ON threat_ip_snapshot(channel_code)"},
	{"ui_preferences", "uni_ui_pref", "CREATE UNIQUE INDEX IF NOT EXISTS uni_ui_pref ON ui_preferences (user_code, tenant_id, login_account, pref_name)"},
	{"firewall_ip_block", "idx_firewall_ip_block_status_expire", "CREATE INDEX IF NOT EXISTS idx_firewall_ip_block_status_expire ON firewall_ip_block(status, expire_time)"},
	{"cdn_provider", "idx_cdn_provider_code", "CREATE INDEX IF NOT EXISTS idx_cdn_provider_code ON cdn_provider(provider)"},
	{"ip_group", "uni_ip_group_code", "CREATE UNIQUE INDEX IF NOT EXISTS uni_ip_group_code ON ip_group (user_code, tenant_id, group_code)"},
	{"ip_group_item", "idx_ip_group_item_group", "CREATE INDEX IF NOT EXISTS idx_ip_group_item_group ON ip_group_item (group_code)"},
	{"ip_group_item", "uni_ip_group_item", "CREATE UNIQUE INDEX IF NOT EXISTS uni_ip_group_item ON ip_group_item (user_code, tenant_id, group_code, ip)"},
	{"access_account", "uni_access_account_name", "CREATE UNIQUE INDEX IF NOT EXISTS uni_access_account_name ON access_account (user_code, tenant_id, account_name)"},
	{"access_session", "uni_access_session_code", "CREATE UNIQUE INDEX IF NOT EXISTS uni_access_session_code ON access_session (session_code)"},
	{"access_session", "idx_access_session_account", "CREATE INDEX IF NOT EXISTS idx_access_session_account ON access_session (account_code, status)"},
	{"access_token", "uni_access_token_code", "CREATE UNIQUE INDEX IF NOT EXISTS uni_access_token_code ON access_token (token_code)"},
	{"access_token", "idx_access_token_session", "CREATE INDEX IF NOT EXISTS idx_access_token_session ON access_token (session_code)"},
	{"access_token", "idx_access_token_expire", "CREATE INDEX IF NOT EXISTS idx_access_token_expire ON access_token (status, expire_time)"},
	{"access_ticket", "uni_access_ticket_code", "CREATE UNIQUE INDEX IF NOT EXISTS uni_access_ticket_code ON access_ticket (ticket_code)"},
	{"access_ticket", "idx_access_ticket_expire", "CREATE INDEX IF NOT EXISTS idx_access_ticket_expire ON access_ticket (expire_time)"},
	{"host_guard_ban", "idx_hgb_status_expire", "CREATE INDEX IF NOT EXISTS idx_hgb_status_expire ON host_guard_ban(status, expire_time)"},
	{"host_guard_ban", "idx_hgb_ip_status", "CREATE INDEX IF NOT EXISTS idx_hgb_ip_status ON host_guard_ban(ip, status)"},
	{"host_guard_ban", "idx_hgb_start_time", "CREATE INDEX IF NOT EXISTS idx_hgb_start_time ON host_guard_ban(start_time)"},
	{"host_guard_offender", "idx_hgo_ip", "CREATE INDEX IF NOT EXISTS idx_hgo_ip ON host_guard_offender(ip)"},
	{"upgrade_notice_record", "uni_upgrade_notice_id", "CREATE UNIQUE INDEX IF NOT EXISTS uni_upgrade_notice_id ON upgrade_notice_record(notice_id, user_code, tenant_id)"},
	{"ip_tags", "uni_iptags_full", "CREATE UNIQUE INDEX IF NOT EXISTS uni_iptags_full ON ip_tags (user_code, tenant_id, ip, ip_tag)"},
	{"ip_tags", "idx_iptag_ip", "CREATE INDEX IF NOT EXISTS idx_iptag_ip ON ip_tags (user_code, tenant_id, ip)"},
}

// InitializeCoreSchema creates the current tables and indexes without replaying historical upgrades.
func InitializeCoreSchema(db *gorm.DB) error {
	if err := initializeSchema(db, coreSchemaModels, coreSchemaIndexes); err != nil {
		return err
	}
	// Explicit additive changes for the new pipeline tables, never a legacy
	// data rewrite or a blanket migration of unrelated application tables.
	for _, m := range []any{&model.GateUser{}, &model.GateSession{}} {
		if !db.Migrator().HasColumn(m, "AuthVersion") {
			if err := db.Migrator().AddColumn(m, "AuthVersion"); err != nil {
				return err
			}
		}
	}
	for _, change := range []struct {
		model any
		field string
	}{{&model.HttpAuthBase{}, "PasswordHashed"}, {&model.HttpAuthSession{}, "Port"}, {&model.GateRuntime{}, "Owner"}} {
		if !db.Migrator().HasColumn(change.model, change.field) {
			if err := db.Migrator().AddColumn(change.model, change.field); err != nil {
				return err
			}
		}
	}
	for _, field := range []string{"SignatureURL", "FeedID", "SigningKeyID", "SigningPublicKey", "SignedSequence", "SignedDigest", "SignatureExpiresAt"} {
		if !db.Migrator().HasColumn(&model.ThreatIPChannel{}, field) {
			if err := db.Migrator().AddColumn(&model.ThreatIPChannel{}, field); err != nil {
				return err
			}
		}
	}
	return seedCoreDefaults(db)
}
