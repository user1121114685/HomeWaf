// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/wafsec"
	"errors"
	"time"
)

type WafOtpService struct{}

var WafOtpServiceApp = new(WafOtpService)

// encManageOtpSecret 管理端 2FA 密钥加密落库（每实例 DEK，产出 swk1 密文；空值原样返回）。
// 加密失败必须拒绝保存，不能把 2FA 秘密退回明文。
func encManageOtpSecret(plain string) (string, error) {
	return encryptAccessSecret(plain)
}

// decManageOtpSecret rejects unauthenticated replacements, including plaintext.
func decManageOtpSecret(stored string) string {
	if stored == "" {
		return ""
	}
	if wafsec.IsDataKeyCiphertext(stored) {
		return decryptAccessSecret(stored)
	}
	return ""
}

// decryptOtpBean 就地把 Secret 还原为明文。所有 Get* 统一调用，
// 让调用方拿到的永远是可直接用于校验的明文，不必各自记得解密。
func decryptOtpBean(bean *model.Otp) {
	if bean == nil {
		return
	}
	bean.Secret = decManageOtpSecret(bean.Secret)
}

func (receiver *WafOtpService) AddApi(req request.WafOtpAddReq) error {
	secret, err := encManageOtpSecret(req.Secret)
	if err != nil {
		return err
	}
	var bean = &model.Otp{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},

		UserName: req.UserName,
		Url:      req.Url,
		Secret:   secret,
		Issuer:   req.Issuer,
		Remarks:  req.Remarks,
	}
	return global.GWAF_LOCAL_DB.Create(bean).Error
}

func (receiver *WafOtpService) CheckIsExistApi(req request.WafOtpAddReq) int {
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " user_name=? "
	}

	//where字段赋值

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereValues = append(whereValues, req.UserName)
		}
	}

	global.GWAF_LOCAL_DB.Model(&model.Otp{}).Where(whereField, whereValues...).Count(&total)
	return int(total)
}

func (receiver *WafOtpService) ModifyApi(req request.WafOtpEditReq) error {
	// 根据唯一字段生成查询条件（只有在UniFields不为空时才进行存在性检查）

	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " user_name=? "
	}

	//where字段赋值

	if len(req.UserName) > 0 {
		whereValues = append(whereValues, req.UserName)
	}

	global.GWAF_LOCAL_DB.Model(&model.Otp{}).Where(whereField, whereValues...).Count(&total)
	// 查询是否已存在记录
	var bean model.Otp
	global.GWAF_LOCAL_DB.Model(&model.Otp{}).Where(whereField, whereValues...).Limit(1).Find(&bean)

	if int(total) > 0 && bean.Id != "" && bean.Id != req.Id {
		return errors.New("当前记录已经存在")
	}

	secret, encryptErr := encManageOtpSecret(req.Secret)
	if encryptErr != nil {
		return encryptErr
	}
	beanMap := map[string]interface{}{

		"UserName": req.UserName,
		"Url":      req.Url,
		"Secret":   secret,
		"Issuer":   req.Issuer,
		"Remarks":  req.Remarks,

		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}
	err := global.GWAF_LOCAL_DB.Model(model.Otp{}).Where("id = ?", req.Id).Updates(beanMap).Error

	return err
}

func (receiver *WafOtpService) BindApi(req request.WafOtpBindReq) error {
	secret, err := encManageOtpSecret(req.Secret)
	if err != nil {
		return err
	}
	var bean = &model.Otp{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},

		UserName: req.UserName,
		Url:      req.Url,
		Secret:   secret,
		Issuer:   req.Issuer,
		Remarks:  req.Remarks,
	}
	return global.GWAF_LOCAL_DB.Create(bean).Error
}

// 以下 Get* 一律返回【明文】Secret：库里是 swk1 密文，读取时统一解密，
// 调用方拿到即可直接用于 TOTP 校验。
func (receiver *WafOtpService) GetDetailApi(req request.WafOtpDetailReq) model.Otp {
	var bean model.Otp
	global.GWAF_LOCAL_DB.Where("id=?", req.Id).Find(&bean)
	decryptOtpBean(&bean)
	return bean
}
func (receiver *WafOtpService) GetDetailByIdApi(id string) model.Otp {
	var bean model.Otp
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	decryptOtpBean(&bean)
	return bean
}
func (receiver *WafOtpService) GetDetailByUserNameApi(userName string) model.Otp {
	var bean model.Otp
	global.GWAF_LOCAL_DB.Where("user_name=?", userName).Find(&bean)
	decryptOtpBean(&bean)
	return bean
}

func (receiver *WafOtpService) GetListApi(req request.WafOtpSearchReq) ([]model.Otp, int64, error) {
	var list []model.Otp
	var total int64 = 0
	global.GWAF_LOCAL_DB.Model(&model.Otp{}).Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Find(&list)
	global.GWAF_LOCAL_DB.Model(&model.Otp{}).Count(&total)
	for i := range list {
		decryptOtpBean(&list[i])
	}

	return list, total, nil
}
func (receiver *WafOtpService) DelApi(req request.WafOtpDelReq) error {
	var bean model.Otp
	err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&bean).Error
	if err != nil {
		return err
	}
	err = global.GWAF_LOCAL_DB.Where("id = ?", req.Id).Delete(model.Otp{}).Error
	return err
}

func (receiver *WafOtpService) IsEmptyOtp(accountName string) bool {
	var total int64 = 0
	err := global.GWAF_LOCAL_DB.Model(&model.Otp{}).Where("user_name=?", accountName).Count(&total).Error
	if err == nil {
		if total == 0 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
