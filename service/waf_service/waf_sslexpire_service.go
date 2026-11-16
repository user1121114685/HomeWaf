// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/model/response"
	"errors"
	"time"
)

type WafSslExpireService struct{}

var WafSslExpireServiceApp = new(WafSslExpireService)

func (receiver *WafSslExpireService) AddApi(req request.WafSslExpireAddReq) error {
	return addCertificateMonitor(req.Domain, req.Port)
}
func (receiver *WafSslExpireService) Add(domain string, port int) error {
	return addCertificateMonitor(domain, port)
}

func (receiver *WafSslExpireService) CheckIsExistApi(req request.WafSslExpireAddReq) int {
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(req.Domain) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " domain=? "
	}

	if req.Port > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " port=? "
	}

	//where字段赋值
	if len(req.Domain) > 0 {
		whereValues = append(whereValues, req.Domain)
	}
	if req.Port > 0 {
		whereValues = append(whereValues, req.Port)
	}
	global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Where(whereField, whereValues...).Count(&total)
	return int(total)
}

func (receiver *WafSslExpireService) CheckIsExist(domain string, port int) int {
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(domain) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " domain=? "
	}

	if port > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " port=? "
	}

	//where字段赋值
	if len(domain) > 0 {
		whereValues = append(whereValues, domain)
	}
	if port > 0 {
		whereValues = append(whereValues, port)
	}
	global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Where(whereField, whereValues...).Count(&total)
	return int(total)
}

func (receiver *WafSslExpireService) ModifyApi(req request.WafSslExpireEditReq) error {
	domain, err := validateCertificateMonitor(req.Domain, req.Port)
	if err != nil {
		return err
	}
	var current model.SslExpire
	if err := global.GWAF_LOCAL_DB.First(&current, "id = ?", req.Id).Error; err != nil {
		return err
	}
	if current.Status == "checking" && time.Since(time.Time(current.UPDATE_TIME)) < 2*time.Minute {
		return errors.New("目标正在检测，请稍后修改")
	}
	var duplicate int64
	if err := global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Where("id <> ? AND domain = ? AND port = ?", req.Id, domain, req.Port).Count(&duplicate).Error; err != nil {
		return err
	}
	if duplicate > 0 {
		return errors.New("此域名与端口已在监测列表")
	}
	return global.GWAF_LOCAL_DB.Model(&current).Updates(map[string]any{"domain": domain, "port": req.Port, "valid_to": nil, "last_detect": nil, "last_success": nil, "visit_log": "", "status": "unverified", "update_time": customtype.JsonTime(time.Now())}).Error
}

func (receiver *WafSslExpireService) Modify(req model.SslExpire) error {

	beanMap := map[string]interface{}{
		"Domain":      req.Domain,
		"Port":        req.Port,
		"VisitLog":    req.VisitLog,
		"Status":      req.Status,
		"LastDetect":  req.LastDetect,
		"LastSuccess": req.LastSuccess,
		"ValidTo":     req.ValidTo,
		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}
	err := global.GWAF_LOCAL_DB.Model(model.SslExpire{}).Where("id = ?", req.Id).Updates(beanMap).Error

	return err
}
func (receiver *WafSslExpireService) GetDetailApi(req request.WafSslExpireDetailReq) model.SslExpire {
	var bean model.SslExpire
	global.GWAF_LOCAL_DB.Where("id=?", req.Id).Find(&bean)
	return bean
}
func (receiver *WafSslExpireService) GetDetailByIdApi(id string) model.SslExpire {
	var bean model.SslExpire
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	return bean
}
func (receiver *WafSslExpireService) GetListApi(req request.WafSslExpireSearchReq) ([]response.WafSslCheckRep, int64, error) {
	if req.PageIndex <= 0 {
		req.PageIndex = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 1000 {
		return nil, 0, errors.New("每页最多1000条")
	}
	var list []model.SslExpire
	var total int64
	query := global.GWAF_LOCAL_DB.Model(&model.SslExpire{})
	if req.Domain != "" {
		query = query.Where("domain LIKE ?", "%"+req.Domain+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("valid_to asc").Limit(req.PageSize).Offset((req.PageIndex - 1) * req.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	result := []response.WafSslCheckRep{}
	for _, row := range list {
		row.Status = certificateMonitorState(row, time.Now())
		result = append(result, response.WafSslCheckRep{SslExpire: row, ExpirationInfo: row.ExpirationMessage(), ExpirationDay: row.ExpirationDay()})
	}
	return result, total, nil
}

func (receiver *WafSslExpireService) GetAllList() ([]model.SslExpire, int64, error) {
	var list []model.SslExpire
	var total int64 = 0
	global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Find(&list)
	global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Count(&total)

	return list, total, nil
}
func (receiver *WafSslExpireService) DelApi(req request.WafSslExpireDelReq) error {
	var bean model.SslExpire
	err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&bean).Error
	if err != nil {
		return err
	}
	err = global.GWAF_LOCAL_DB.Where("id = ?", req.Id).Delete(model.SslExpire{}).Error
	return err
}
