// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/internal/acmelog"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"errors"
	"time"

	"gorm.io/gorm"
)

type WafSSLOrderService struct{}

var WafSSLOrderServiceApp = new(WafSSLOrderService)

func (receiver *WafSSLOrderService) GetDetailApi(req request.WafSslorderdetailReq) model.SslOrder {
	return receiver.GetDetailById(req.Id)
}
func (receiver *WafSSLOrderService) GetListApi(req request.WafSslordersearchReq) ([]model.SslOrder, int64, error) {
	if req.PageIndex <= 0 {
		req.PageIndex = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 1000 {
		return nil, 0, errors.New("每页最多 1000 条")
	}
	var list []model.SslOrder
	var total int64 = 0

	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	//where字段
	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " host_code =? "
	}

	//where字段赋值
	if len(req.HostCode) > 0 {
		whereValues = append(whereValues, req.HostCode)
	}
	if req.ApplyDomain != "" {
		if whereField != "" {
			whereField += " and "
		}
		whereField += "apply_domain LIKE ?"
		whereValues = append(whereValues, "%"+req.ApplyDomain+"%")
	}
	if req.ApplyStatus != "" {
		if whereField != "" {
			whereField += " and "
		}
		whereField += "apply_status = ?"
		whereValues = append(whereValues, req.ApplyStatus)
	}

	if req.CertificateID != "" {
		if whereField != "" {
			whereField += " and "
		}
		whereField += "certificate_id = ?"
		whereValues = append(whereValues, req.CertificateID)
	}
	if req.Trigger != "" {
		if whereField != "" {
			whereField += " and "
		}
		whereField += "trigger = ?"
		whereValues = append(whereValues, req.Trigger)
	}
	/**排序*/
	orderInfo := "create_time desc"

	if err := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where(whereField, whereValues...).Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Order(orderInfo).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	if err := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where(whereField, whereValues...).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	for i := range list {
		list[i].RefreshHeartbeatState(time.Now())
	}

	return list, total, nil
}
func (receiver *WafSSLOrderService) DelApi(req request.WafSslorderdeleteReq) error {
	var bean model.SslOrder
	err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&bean).Error
	if err != nil {
		return err
	}
	if CertificateLifecycleAvailable() {
		var p model.SslRenewalPolicy
		if bean.CertificateID != "" && global.GWAF_LOCAL_DB.First(&p, "id = ?", bean.CertificateID).Error == nil {
			if p.Status == "unresolved" || (p.LastOrderID == bean.Id && bean.ApplyStatus == "fail") {
				return errors.New("此记录仍用于来源核对或故障恢复，请完成处理后再删除")
			}
		} else if bean.ApplyStatus == "success" {
			return errors.New("此成功记录尚未迁移签发上下文，暂不能删除")
		}
	}
	return global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND apply_status NOT IN ?", req.Id, []string{"submitted", "applying"}).Delete(&model.SslOrder{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("等待中或申请中的订单不能删除，请等待处理完成")
		}
		return tx.Where("order_id = ? AND tenant_id = ? AND user_code = ?", req.Id, bean.Tenant_ID, bean.USER_CODE).Delete(&model.SslOrderLog{}).Error
	})
}
func (receiver *WafSSLOrderService) GetDetailById(id string) model.SslOrder {
	var bean model.SslOrder
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	bean.RefreshHeartbeatState(time.Now())
	return bean
}
func (receiver *WafSSLOrderService) ModifyById(sslOrder model.SslOrder) error {
	sslOrder.ResultError = acmelog.Sanitize(sslOrder.ResultError)

	sslOrderMap := map[string]interface{}{
		"HostCode":                sslOrder.HostCode,
		"ApplyPlatform":           sslOrder.ApplyPlatform,
		"ApplyMethod":             sslOrder.ApplyMethod,
		"SkipDNSVerify":           sslOrder.SkipDNSVerify,
		"ApplyDns":                sslOrder.ApplyDns,
		"ApplyDomain":             sslOrder.ApplyDomain,
		"ApplyEmail":              sslOrder.ApplyEmail,
		"ApplyStatus":             sslOrder.ApplyStatus,
		"ApplyKey":                sslOrder.ApplyKey,
		"ResultDomain":            sslOrder.ResultDomain,
		"ResultCertURL":           sslOrder.ResultCertURL,
		"ResultCertStableURL":     sslOrder.ResultCertStableURL,
		"ResultPrivateKey":        sslOrder.ResultPrivateKey,
		"ResultCertificate":       sslOrder.ResultCertificate,
		"ResultIssuerCertificate": sslOrder.ResultIssuerCertificate,
		"ResultCSR":               sslOrder.ResultCSR,
		"ResultValidTo":           sslOrder.ResultValidTo,
		"ResultError":             sslOrder.ResultError,
		"PrivateGroupName":        sslOrder.PrivateGroupName,
		"UPDATE_TIME":             customtype.JsonTime(time.Now()),
	}
	if sslOrder.ApplyStatus == "success" {
		sslOrderMap["progress"] = "证书签发并保存成功"
	}
	if sslOrder.ApplyStatus == "fail" {
		sslOrderMap["progress"] = "申请失败，请查看错误详情"
	}
	if sslOrder.ApplyStatus != "submitted" && sslOrder.ApplyStatus != "applying" {
		sslOrderMap["active_key"] = nil
	}
	err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SslOrder{}).Where("id = ?", sslOrder.Id).Updates(sslOrderMap).Error; err != nil {
			return err
		}
		if sslOrder.ApplyStatus == "success" && sslOrder.RenewFrom != "" {
			return tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", sslOrder.RenewFrom, "success").Update("apply_status", "renewed").Error
		}
		return nil
	})
	if err == nil {
		if sslOrder.ApplyStatus == "success" {
			_ = receiver.AppendLog(sslOrder.Id, "info", "证书签发并保存成功")
		}
		if sslOrder.ApplyStatus == "fail" {
			_ = receiver.AppendLog(sslOrder.Id, "error", "申请失败："+sslOrder.ResultError)
		}
	}
	return err
}

// GetLastedInfo 查询最新的没有过期的
func (receiver *WafSSLOrderService) GetLastedInfo(hostCode string) (model.SslOrder, error) {
	var bean model.SslOrder

	err := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("host_code=? AND apply_status = ?", hostCode, "success").Where("result_certificate is not null").Order("create_time desc").Limit(1).First(&bean).Error
	return bean, err
}

// GetCAServerAddress 根据CA服务器名称获取地址
func GetCAServerAddress(caServerName string) string {
	// 如果没有指定CA服务器名称，默认使用letsencrypt
	if caServerName == "" {
		caServerName = "letsencrypt"
	}

	// 从数据库查询CA服务器信息
	var caServer model.CaServerInfo
	err := global.GWAF_LOCAL_DB.Where("ca_server_name = ?", caServerName).First(&caServer).Error

	// 如果查询失败或没有找到记录，返回默认的letsencrypt地址
	if err != nil {
		return "https://acme-v02.api.letsencrypt.org/directory"
	}

	// 如果找到记录但地址为空，也返回默认地址
	if caServer.CaServerAddress == "" {
		return "https://acme-v02.api.letsencrypt.org/directory"
	}

	return caServer.CaServerAddress
}
