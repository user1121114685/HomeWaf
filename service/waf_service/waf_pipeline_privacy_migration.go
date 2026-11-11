package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"errors"
	"gorm.io/gorm"
)

func migrateLegacyPrivacy(db *gorm.DB, source model.Hosts, codes []string, c *wafpipeline.PortConfig, out *LegacyPortPreview) error {
	if !db.Migrator().HasTable(&model.LDPUrl{}) {
		return nil
	}
	var rows []model.LDPUrl
	if err := db.Where("host_code IN ? AND user_code = ? AND tenant_id = ?", codes, source.USER_CODE, source.Tenant_ID).Limit(257).Order("id").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 256 {
		return errors.New("隐私处理URL超过端口256条上限")
	}
	if len(rows) == 0 {
		return nil
	}
	policy := &wafpipeline.PrivacyPolicy{Mode: wafpipeline.ModeBlock}
	if source.LogOnlyMode == 1 {
		policy.Mode = wafpipeline.ModeObserve
	}
	names := map[string]string{"等于": "exact", "前缀匹配": "prefix", "后缀匹配": "suffix", "包含匹配": "contains"}
	for _, row := range rows {
		policy.URLs = append(policy.URLs, wafpipeline.PrivacyURL{Match: names[row.CompareType], Value: row.Url})
	}
	c.Policy.Network.Response.Privacy = policy
	out.secrets.PrivacyConfig = global.GWAF_DLP_CONFIG
	out.Warnings = append(out.Warnings, "响应隐私处理及引擎配置复制为端口独立快照；只处理有界完整响应，观察模式不改写，检测结果不会打印原始个人信息")
	return nil
}
