// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/request"
	"errors"
)

type WafSysLogService struct{}

var WafSysLogServiceApp = new(WafSysLogService)

func (receiver *WafSysLogService) GetDetailApi(req request.WafSysLogDetailReq) model.WafSysLog {
	var bean model.WafSysLog
	global.GWAF_LOCAL_LOG_DB.Where("id=?", req.Id).Find(&bean)
	return bean
}
func (receiver *WafSysLogService) GetListApi(req request.WafSysLogSearchReq) ([]model.WafSysLog, int64, error) {
	var bean []model.WafSysLog
	var total int64 = 0
	if global.GWAF_LOCAL_LOG_DB == nil {
		return nil, 0, errors.New("日志数据库未就绪")
	}
	if req.PageIndex < 1 {
		req.PageIndex = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	query := global.GWAF_LOCAL_LOG_DB.Model(&model.WafSysLog{})
	if req.OpType != "" {
		query = query.Where("op_type = ?", req.OpType)
	}
	if req.OpContent != "" {
		query = query.Where("op_content LIKE ?", "%"+req.OpContent+"%")
	}
	if req.Keyword != "" {
		query = query.Where("(op_content LIKE ? OR op_type LIKE ?)", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Order("create_time desc").Find(&bean).Error; err != nil {
		return nil, 0, err
	}
	return bean, total, nil
}
