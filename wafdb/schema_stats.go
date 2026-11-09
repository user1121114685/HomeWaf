// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/model"
	"gorm.io/gorm"
)

var statsSchemaModels = []any{
	&model.StatsTotal{},
	&model.StatsDay{},
	&model.StatsIPDay{},
	&model.StatsIPCityDay{},
	&model.IPTag{},
	&model.StatsSiteDay{},
	&model.StatsSiteHour{},
	&model.StatsActorPathDay{},
	&model.StatsActorUaDay{},
	&model.StatsPathRuleDay{},
}

var statsSchemaIndexes = []schemaIndex{
	{"stats_site_days", "idx_stats_site_days_lookup", "CREATE INDEX IF NOT EXISTS idx_stats_site_days_lookup ON stats_site_days (host_code, day)"},
	{"stats_site_hours", "idx_stats_site_hours_lookup", "CREATE INDEX IF NOT EXISTS idx_stats_site_hours_lookup ON stats_site_hours (host_code, hour_time)"},
	{"stats_ip_days", "idx_stats_ip_days_day", "CREATE INDEX IF NOT EXISTS idx_stats_ip_days_day ON stats_ip_days(day)"},
	{"stats_ip_city_days", "idx_stats_ip_city_days_day", "CREATE INDEX IF NOT EXISTS idx_stats_ip_city_days_day ON stats_ip_city_days(day)"},
	{"ip_tags", "idx_ip_tags_create_time", "CREATE INDEX IF NOT EXISTS idx_ip_tags_create_time ON ip_tags(create_time)"},
	{"ip_tags", "idx_ip_tags_update_time", "CREATE INDEX IF NOT EXISTS idx_ip_tags_update_time ON ip_tags(update_time)"},
	{"stats_ip_days", "idx_stats_ip_days_day_type_ct", "CREATE INDEX IF NOT EXISTS idx_stats_ip_days_day_type_ct ON stats_ip_days (day, type, create_time, ip)"},
	{"stats_site_hours", "idx_stats_site_hours_time", "CREATE INDEX IF NOT EXISTS idx_stats_site_hours_time ON stats_site_hours (hour_time)"},
	{"stats_actor_path_days", "uni_stats_actor_path_days", "CREATE UNIQUE INDEX IF NOT EXISTS uni_stats_actor_path_days ON stats_actor_path_days (user_code, tenant_id, host_code, day, actor_key, path_norm)"},
	{"stats_actor_path_days", "idx_stats_actor_path_days_path", "CREATE INDEX IF NOT EXISTS idx_stats_actor_path_days_path ON stats_actor_path_days (host_code, day, path_norm)"},
	{"stats_actor_path_days", "idx_stats_actor_path_days_actor", "CREATE INDEX IF NOT EXISTS idx_stats_actor_path_days_actor ON stats_actor_path_days (host_code, day, actor_key)"},
	{"stats_actor_path_days", "idx_stats_actor_path_days_day", "CREATE INDEX IF NOT EXISTS idx_stats_actor_path_days_day ON stats_actor_path_days (day)"},
	{"stats_actor_ua_days", "uni_stats_actor_ua_days", "CREATE UNIQUE INDEX IF NOT EXISTS uni_stats_actor_ua_days ON stats_actor_ua_days (user_code, tenant_id, host_code, day, actor_key, ua_hash)"},
	{"stats_actor_ua_days", "idx_stats_actor_ua_days_day", "CREATE INDEX IF NOT EXISTS idx_stats_actor_ua_days_day ON stats_actor_ua_days (day)"},
	{"stats_path_rule_days", "uni_stats_path_rule_days", "CREATE UNIQUE INDEX IF NOT EXISTS uni_stats_path_rule_days ON stats_path_rule_days (user_code, tenant_id, host_code, day, path_norm, rule)"},
	{"stats_path_rule_days", "idx_stats_path_rule_days_day", "CREATE INDEX IF NOT EXISTS idx_stats_path_rule_days_day ON stats_path_rule_days (day)"},
	{"stats_days", "idx_stats_days_lookup", "CREATE INDEX IF NOT EXISTS idx_stats_days_lookup ON stats_days (tenant_id, user_code, host_code, type, day)"},
	{"stats_ip_days", "idx_stats_ip_days_lookup", "CREATE INDEX IF NOT EXISTS idx_stats_ip_days_lookup ON stats_ip_days (tenant_id, user_code, host_code, ip, type, day)"},
	{"stats_ip_city_days", "idx_stats_ip_city_days_lookup", "CREATE INDEX IF NOT EXISTS idx_stats_ip_city_days_lookup ON stats_ip_city_days (tenant_id, user_code, host_code, country, province, city, type, day)"},
	{"ip_tags", "uni_iptags_full", "CREATE UNIQUE INDEX IF NOT EXISTS uni_iptags_full ON ip_tags (user_code, tenant_id, ip, ip_tag)"},
	{"ip_tags", "idx_iptag_ip", "CREATE INDEX IF NOT EXISTS idx_iptag_ip ON ip_tags (user_code, tenant_id, ip)"},
}

// InitializeStatsSchema creates the current tables and indexes without replaying historical upgrades.
func InitializeStatsSchema(db *gorm.DB) error {
	return initializeSchema(db, statsSchemaModels, statsSchemaIndexes)
}
