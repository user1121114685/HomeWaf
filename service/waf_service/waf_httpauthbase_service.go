// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/utils"
	"errors"
	"time"
)

type WafHttpAuthBaseService struct{}

var WafHttpAuthBaseServiceApp = new(WafHttpAuthBaseService)

func (receiver *WafHttpAuthBaseService) AddApi(req request.WafHttpAuthBaseAddReq) error {
	if err := validateGateUsername(req.UserName); err != nil {
		return err
	}
	if err := ValidateGatePassword(req.UserName, req.Password); err != nil {
		return err
	}
	var count int64
	if err := global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where("host_code = ?", req.HostCode).Count(&count).Error; err != nil {
		return err
	}
	if count >= 1000 {
		return errors.New("站点账号数量已达上限")
	}
	hash, err := utils.BcryptHash(req.Password)
	if err != nil {
		return err
	}
	var bean = &model.HttpAuthBase{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},

		HostCode:       req.HostCode,
		UserName:       req.UserName,
		Password:       hash,
		PasswordHashed: true,
	}
	return global.GWAF_LOCAL_DB.Create(bean).Error
}

func (receiver *WafHttpAuthBaseService) CheckIsExistApi(req request.WafHttpAuthBaseAddReq) int {
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " host_code=? "
	}

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " user_name=? "
	}

	//where字段赋值

	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereValues = append(whereValues, req.HostCode)
		}
	}

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereValues = append(whereValues, req.UserName)
		}
	}

	global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where(whereField, whereValues...).Count(&total)
	return int(total)
}

func (receiver *WafHttpAuthBaseService) ModifyApi(req request.WafHttpAuthBaseEditReq) error {
	// 根据唯一字段生成查询条件（只有在UniFields不为空时才进行存在性检查）
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""

	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " host_code=? "
	}

	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " user_name=? "
	}

	//where字段赋值

	if len(req.HostCode) > 0 {
		whereValues = append(whereValues, req.HostCode)
	}

	if len(req.UserName) > 0 {
		whereValues = append(whereValues, req.UserName)
	}

	global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where(whereField, whereValues...).Count(&total)
	// 查询是否已存在记录
	var bean model.HttpAuthBase
	global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where(whereField, whereValues...).Limit(1).Find(&bean)

	if int(total) > 0 && bean.Id != "" && bean.Id != req.Id {
		return errors.New("当前记录已经存在")
	}

	beanMap := map[string]interface{}{
		"HostCode":    req.HostCode,
		"UserName":    req.UserName,
		"Password":    req.Password,
		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}
	// 改之前先把老身份记下来：改密/改名之后，凭旧密码建立的会话必须一并作废，
	// 否则「改了密码」只挡住新登录，已经登进去的人照样在里面。
	var old model.HttpAuthBase
	if err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&old).Error; err != nil {
		return err
	}
	if err := validateGateUsername(req.UserName); err != nil {
		return err
	}
	password := old.Password
	if req.Password != "" {
		if err := ValidateGatePassword(req.UserName, req.Password); err != nil {
			return err
		}
		hash, err := utils.BcryptHash(req.Password)
		if err != nil {
			return err
		}
		password = hash
	} else if !old.PasswordHashed {
		hash, err := utils.BcryptHash(old.Password)
		if err != nil {
			return err
		}
		password = hash
	}
	beanMap["Password"] = password
	beanMap["PasswordHashed"] = true

	err := global.GWAF_LOCAL_DB.Model(model.HttpAuthBase{}).Where("id = ?", req.Id).Updates(beanMap).Error
	if err != nil {
		return err
	}
	if old.UserName != "" {
		WafHttpAuthSessionServiceApp.RevokeByUser(old.HostCode, old.UserName, model.HttpAuthRevokeByAccount)
	}
	if req.UserName != old.UserName || req.HostCode != old.HostCode {
		WafHttpAuthSessionServiceApp.RevokeByUser(req.HostCode, req.UserName, model.HttpAuthRevokeByAccount)
	}
	return nil
}
func (receiver *WafHttpAuthBaseService) GetDetailApi(req request.WafHttpAuthBaseDetailReq) model.HttpAuthBase {
	var bean model.HttpAuthBase
	global.GWAF_LOCAL_DB.Where("id=?", req.Id).Find(&bean)
	return bean
}

// LoadForHost performs a one-way conversion of old encrypted plaintext during
// configuration loading, never on a request. Account rows and keys are kept.
func (receiver *WafHttpAuthBaseService) LoadForHost(hostCode string) ([]model.HttpAuthBase, error) {
	var rows []model.HttpAuthBase
	if err := global.GWAF_LOCAL_DB.Where("host_code = ?", hostCode).Limit(1001).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 1000 {
		return nil, errors.New("HTTPAuth account quota exceeded")
	}
	for i := range rows {
		if rows[i].PasswordHashed {
			continue
		}
		hash, err := utils.BcryptHash(rows[i].Password)
		if err != nil {
			return nil, errors.New("legacy HTTPAuth password cannot be converted; reset this account")
		}
		if err = global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where("id = ? AND (password_hashed = ? OR password_hashed IS NULL)", rows[i].Id, false).Updates(map[string]any{"Password": hash, "PasswordHashed": true}).Error; err != nil {
			return nil, err
		}
		rows[i].Password = hash
		rows[i].PasswordHashed = true
	}
	return rows, nil
}
func (receiver *WafHttpAuthBaseService) GetDetailByIdApi(id string) model.HttpAuthBase {
	var bean model.HttpAuthBase
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	return bean
}
func (receiver *WafHttpAuthBaseService) GetListApi(req request.WafHttpAuthBaseSearchReq) ([]model.HttpAuthBase, int64, error) {
	var list []model.HttpAuthBase
	var total int64 = 0

	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""
	if len(req.HostCode) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " host_code=? "
	}
	if len(req.UserName) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " user_name =? "
	}
	//where字段赋值
	if len(req.HostCode) > 0 {
		whereValues = append(whereValues, req.HostCode)
	}
	if len(req.UserName) > 0 {
		whereValues = append(whereValues, req.UserName)
	}

	global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where(whereField, whereValues...).Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Find(&list)
	global.GWAF_LOCAL_DB.Model(&model.HttpAuthBase{}).Where(whereField, whereValues...).Count(&total)

	return list, total, nil
}
func (receiver *WafHttpAuthBaseService) DelApi(req request.WafHttpAuthBaseDelReq) error {
	var bean model.HttpAuthBase
	err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&bean).Error
	if err != nil {
		return err
	}
	err = global.GWAF_LOCAL_DB.Where("id = ?", req.Id).Delete(model.HttpAuthBase{}).Error
	if err != nil {
		return err
	}
	// 账号没了，他的在线会话也不该继续有效
	WafHttpAuthSessionServiceApp.RevokeByUser(bean.HostCode, bean.UserName, model.HttpAuthRevokeByAccount)
	return nil
}
