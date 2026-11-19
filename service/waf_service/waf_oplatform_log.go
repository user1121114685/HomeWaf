// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/wafsec"
	"errors"
	"time"
)

type WafOPlatformLogService struct{}

var WafOPlatformLogServiceApp = new(WafOPlatformLogService)

// AddLog 记录一条调用日志
func (receiver *WafOPlatformLogService) AddLog(logEntry model.OPlatformLog) error {
	logEntry.Id = uuid.GenUUID()
	logEntry.USER_CODE = global.GWAF_USER_CODE
	logEntry.Tenant_ID = global.GWAF_TENANT_ID
	logEntry.CREATE_TIME = customtype.JsonTime(time.Now())
	logEntry.UPDATE_TIME = customtype.JsonTime(time.Now())
	if logEntry.TimeStr == "" {
		logEntry.TimeStr = time.Now().Format("2006-01-02 15:04:05")
	}
	return global.GWAF_LOCAL_LOG_DB.Create(&logEntry).Error
}

func (receiver *WafOPlatformLogService) GetDetailApi(req request.WafOPlatformLogDetailReq) (model.OPlatformLog, error) {
	var bean model.OPlatformLog
	if err := global.GWAF_LOCAL_LOG_DB.Where("id = ?", req.Id).Take(&bean).Error; err != nil {
		return model.OPlatformLog{}, err
	}
	result, err := wafsec.DataDecrypt(bean.MCPResult)
	if err != nil {
		return model.OPlatformLog{}, errors.New("无法解密已记录的 MCP 返回内容，请检查实例数据密钥")
	}
	bean.MCPResult = result
	return bean, nil
}

func (receiver *WafOPlatformLogService) GetListApi(req request.WafOPlatformLogSearchReq) ([]model.OPlatformLog, int64, error) {
	var list []model.OPlatformLog
	var total int64 = 0

	whereField := ""
	var whereValues []interface{}

	if len(req.KeyName) > 0 {
		if len(whereField) > 0 {
			whereField += " and "
		}
		whereField += "key_name like ?"
		whereValues = append(whereValues, "%"+req.KeyName+"%")
	}
	if len(req.RequestPath) > 0 {
		if len(whereField) > 0 {
			whereField += " and "
		}
		whereField += "request_path like ?"
		whereValues = append(whereValues, "%"+req.RequestPath+"%")
	}
	if len(req.ClientIP) > 0 {
		if len(whereField) > 0 {
			whereField += " and "
		}
		whereField += "client_ip = ?"
		whereValues = append(whereValues, req.ClientIP)
	}
	if len(req.RequestMethod) > 0 {
		if len(whereField) > 0 {
			whereField += " and "
		}
		whereField += "request_method = ?"
		whereValues = append(whereValues, req.RequestMethod)
	}

	query := global.GWAF_LOCAL_LOG_DB.Model(&model.OPlatformLog{}).Where(whereField, whereValues...)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Omit("MCPResult").Order("create_time desc").
		Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Find(&list).Error
	return list, total, err
}

func (receiver *WafOPlatformLogService) DelApi(req request.WafOPlatformLogDelReq) error {
	return global.GWAF_LOCAL_LOG_DB.Where("id = ?", req.Id).Delete(model.OPlatformLog{}).Error
}

// AddLogAsync 异步记录日志，不阻塞主流程
func (receiver *WafOPlatformLogService) AddLogAsync(logEntry model.OPlatformLog) {
	go func() {
		_ = receiver.AddLog(logEntry)
	}()
}
