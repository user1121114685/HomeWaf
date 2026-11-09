// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"gorm.io/gorm"
	"time"
)

func defaultBaseOrm() baseorm.BaseOrm {
	now := customtype.JsonTime(time.Now())
	return baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: now, UPDATE_TIME: now}
}

func defaultRetentionPolicies() []model.DataRetentionPolicy {
	return []model.DataRetentionPolicy{
		{TableName: "stats_ip_days", DbType: "stats", RetainDays: 90, RetainRows: 100000, DayField: "day", DayFieldType: "int_day", RowOrderField: "day", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "IP日统计-按day字段判断天数,保留day值最大(最新)的行"},
		{TableName: "stats_ip_city_days", DbType: "stats", RetainDays: 90, RetainRows: 100000, DayField: "day", DayFieldType: "int_day", RowOrderField: "day", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "IP城市日统计-按day字段判断天数,保留day值最大(最新)的行"},
		{TableName: "ip_tags", DbType: "stats", RetainDays: 90, RetainRows: 50000, DayField: "create_time", DayFieldType: "datetime", RowOrderField: "update_time", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "IP标签-按create_time判断天数,按update_time排序保留最近活跃的行"},
		{TableName: "stats_actor_path_days", DbType: "stats", RetainDays: 90, RetainRows: 1000000, DayField: "day", DayFieldType: "int_day", RowOrderField: "day", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "行为×目标日汇总-按day字段判断天数,保留day值最大(最新)的行"},
		{TableName: "stats_actor_ua_days", DbType: "stats", RetainDays: 90, RetainRows: 200000, DayField: "day", DayFieldType: "int_day", RowOrderField: "day", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "行为×UA日汇总-按day字段判断天数,保留day值最大(最新)的行"},
		{TableName: "stats_path_rule_days", DbType: "stats", RetainDays: 90, RetainRows: 200000, DayField: "day", DayFieldType: "int_day", RowOrderField: "day", RowOrderDir: "DESC", CleanEnabled: 0, Remarks: "目标×规则日汇总-按day字段判断天数,保留day值最大(最新)的行"},
	}
}

// seedCoreDefaults only inserts missing defaults; it never rewrites existing user data.
func seedCoreDefaults(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, policy := range defaultRetentionPolicies() {
			policy.BaseOrm = defaultBaseOrm()
			if err := tx.Where("table_name = ?", policy.TableName).Attrs(policy).FirstOrCreate(&model.DataRetentionPolicy{}).Error; err != nil {
				return err
			}
		}
		for _, provider := range []string{"alidns", "huaweicloud", "tencentcloud", "cloudflare", "baiducloud"} {
			group := model.PrivateGroup{BaseOrm: defaultBaseOrm(), PrivateGroupName: "default", PrivateGroupBelongCloud: provider}
			if err := tx.Where("private_group_belong_cloud = ?", provider).Attrs(group).FirstOrCreate(&model.PrivateGroup{}).Error; err != nil {
				return err
			}
		}
		for _, server := range []model.CaServerInfo{
			{CaServerName: "zerossl", CaServerAddress: "https://acme.zerossl.com/v2/DV90", Remarks: "ZeroSSL"},
			{CaServerName: "letsencrypt", CaServerAddress: "https://acme-v02.api.letsencrypt.org/directory", Remarks: "Let's Encrypt"},
		} {
			server.BaseOrm = defaultBaseOrm()
			if err := tx.Where("ca_server_name = ? OR ca_server_address = ?", server.CaServerName, server.CaServerAddress).Attrs(server).FirstOrCreate(&model.CaServerInfo{}).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&model.HostGuardBanLadder{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			for _, ladder := range model.DefaultBanLadders() {
				ladder.BaseOrm = defaultBaseOrm()
				if err := tx.Create(&ladder).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
