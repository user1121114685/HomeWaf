// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/model/request"
	"fmt"
	"time"
)

type WafDataRetentionService struct{}

var WafDataRetentionServiceApp = new(WafDataRetentionService)

// GetAllPolicies 获取所有数据保留策略
func (s *WafDataRetentionService) GetAllPolicies() ([]model.DataRetentionPolicy, error) {
	var policies []model.DataRetentionPolicy
	err := global.GWAF_LOCAL_DB.Find(&policies).Error
	return policies, err
}

// GetPolicyById 按 ID 获取策略
func (s *WafDataRetentionService) GetPolicyById(id string) (*model.DataRetentionPolicy, error) {
	var policy model.DataRetentionPolicy
	err := global.GWAF_LOCAL_DB.Where("id = ?", id).First(&policy).Error
	if err != nil {
		return nil, fmt.Errorf("策略不存在: %w", err)
	}
	return &policy, nil
}

// UpdatePolicyById 按 ID 更新策略（仅允许修改业务字段）
func (s *WafDataRetentionService) UpdatePolicyById(req request.WafDataRetentionEditReq) error {
	now := customtype.JsonTime(time.Now())
	return global.GWAF_LOCAL_DB.Model(&model.DataRetentionPolicy{}).
		Where("id = ?", req.Id).
		Updates(map[string]interface{}{
			"retain_days":   req.RetainDays,
			"retain_rows":   req.RetainRows,
			"clean_enabled": req.CleanEnabled,
			"remarks":       req.Remarks,
			"update_time":   now,
		}).Error
}
