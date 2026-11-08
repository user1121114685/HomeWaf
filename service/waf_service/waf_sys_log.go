// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
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
	global.GWAF_LOCAL_LOG_DB.Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Order("create_time desc").Find(&bean)
	global.GWAF_LOCAL_LOG_DB.Model(&model.WafSysLog{}).Count(&total)
	return bean, total, nil
}
