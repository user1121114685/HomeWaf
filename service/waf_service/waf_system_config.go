// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/common/zlog"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/wafsec"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WafSystemConfigService struct{}

var WafSystemConfigServiceApp = new(WafSystemConfigService)

// ConfigClearSentinel 前端要显式清空某个密钥类配置时提交的特殊值。
// 因为“留空=保持原值不变”（防止只改备注把密钥冲掉），清空必须走独立标记。
const ConfigClearSentinel = "__SAMWAF_CLEAR__"

// IsSensitiveConfigItem 判断某配置项是否为密钥类（需加密落库、脱敏回显、写保护）。
// 权威清单在 model 层（service 与 wafdb 迁移共用，避免两处漂移）。
func IsSensitiveConfigItem(item string) bool {
	return model.IsSensitiveConfigItem(item)
}

// encConfigValue / decConfigValue 密钥类配置值的落库加解密（每实例 DEK，swk1 前缀）。
// 非密钥项原样返回，行为不变。
//
// 加密失败时拒绝写入，绝不退回明文保存。
func encConfigValue(item, plain string) (string, error) {
	if plain == "" || !model.IsSensitiveConfigItem(item) {
		return plain, nil
	}
	// 防御：哨兵值代表“清空”，任何路径都不该把它当密钥本身加密落库。
	if plain == ConfigClearSentinel {
		return "", nil
	}
	enc, err := wafsec.DataEncrypt(plain)
	if err != nil || enc == "" {
		return "", errors.New("配置密钥加密失败，未保存；请检查实例数据密钥")
	}
	return enc, nil
}

// decConfigValue rejects unencrypted replacements of sensitive settings.
func decConfigValue(item, stored string) string {
	if stored == "" || !model.IsSensitiveConfigItem(item) {
		return stored
	}
	if !wafsec.IsDataKeyCiphertext(stored) {
		zlog.Error("拒绝未加密的敏感配置项", "item", item)
		return ""
	}
	plain, err := wafsec.DataDecrypt(stored)
	if err != nil {
		zlog.Error("配置项解密失败", "item", item, "err", err)
		return ""
	}
	return plain
}

// decryptConfigBean 就地把 bean 的 Value 还原为明文（读取路径统一入口）。
func decryptConfigBean(bean *model.SystemConfig) {
	if bean == nil {
		return
	}
	bean.Value = decConfigValue(bean.Item, bean.Value)
	if bean.Item == model.OpenPlatformSwitch && bean.EnabledUntil <= time.Now().Unix() {
		bean.Value = "0"
	}
}

// MaskSensitiveConfig 对密钥类配置项抹空 Value 并置 HasValue，供页面列表/详情接口使用。
// 只处理页面回显，绝不改动库内数据；内部读取真实值走 GetDetailByItemApi 等 getter，不经此。
func MaskSensitiveConfig(bean *model.SystemConfig) {
	if bean == nil || !IsSensitiveConfigItem(bean.Item) {
		return
	}
	bean.IsSensitive = true
	bean.HasValue = strings.TrimSpace(bean.Value) != ""
	bean.Value = ""
}

func (receiver *WafSystemConfigService) AddApi(wafSystemConfigAddReq request.WafSystemConfigAddReq) error {
	return receiver.addConfig(wafSystemConfigAddReq, "0")
}

// Program-owned settings are never exposed as deletable extension records.
func (receiver *WafSystemConfigService) AddManagedApi(req request.WafSystemConfigAddReq) error {
	return receiver.addConfig(req, "1")
}

func (receiver *WafSystemConfigService) addConfig(wafSystemConfigAddReq request.WafSystemConfigAddReq, managed string) error {
	// 幂等保护：先检查 item 是否已存在，存在则跳过，防止重复插入
	var existing model.SystemConfig
	result := global.GWAF_LOCAL_DB.Where("item = ?", wafSystemConfigAddReq.Item).First(&existing)
	if result.Error == nil && existing.Id != "" {
		// 已存在，跳过插入
		return nil
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	value, err := encConfigValue(wafSystemConfigAddReq.Item, wafSystemConfigAddReq.Value)
	if err != nil {
		return err
	}
	var bean = &model.SystemConfig{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		ItemClass: wafSystemConfigAddReq.ItemClass,
		Item:      wafSystemConfigAddReq.Item,
		Value:     value,
		IsSystem:  managed,
		ItemType:  wafSystemConfigAddReq.ItemType,
		Options:   wafSystemConfigAddReq.Options,
		Remarks:   wafSystemConfigAddReq.Remarks,
		HashInfo:  "",
	}
	if bean.Item == model.OpenPlatformSwitch {
		switch bean.Value {
		case "1":
			bean.EnabledUntil = time.Now().Add(model.OpenPlatformWindow).Unix()
		case "0":
		default:
			return errors.New("开放平台开关只接受 0 或 1")
		}
	}
	return global.GWAF_LOCAL_DB.Clauses(clause.OnConflict{DoNothing: true}).Create(bean).Error
}

func (receiver *WafSystemConfigService) CheckIsExistApi(wafSystemConfigAddReq request.WafSystemConfigAddReq) error {
	return global.GWAF_LOCAL_DB.First(&model.SystemConfig{}, "item = ? ", wafSystemConfigAddReq.Item).Error
}
func (receiver *WafSystemConfigService) ModifyApi(req request.WafSystemConfigEditReq) error {
	var sysConfig model.SystemConfig
	if err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&sysConfig).Error; err != nil {
		return errors.New("配置项不存在或读取失败")
	}
	if strings.TrimSpace(req.Item) == "" {
		return errors.New("配置项名称不能为空")
	}
	if req.Item != sysConfig.Item && (sysConfig.IsSystem == "1" || IsSensitiveConfigItem(sysConfig.Item)) {
		return errors.New("系统或密钥配置项不允许改名")
	}
	var count int64
	if err := global.GWAF_LOCAL_DB.Model(&model.SystemConfig{}).Where("item = ? AND id <> ?", req.Item, req.Id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("当前配置已经存在")
	}
	value, encryptErr := encConfigValue(req.Item, req.Value)
	if encryptErr != nil {
		return encryptErr
	}
	editMap := map[string]interface{}{
		"Item":        req.Item,
		"ItemClass":   req.ItemClass,
		"Value":       value,
		"Remarks":     req.Remarks,
		"ItemType":    req.ItemType,
		"Options":     req.Options,
		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}

	return updateSystemConfig(global.GWAF_LOCAL_DB.Model(model.SystemConfig{}).Where("id = ?", req.Id), req.Item, editMap)
}

// ModifyByItemApi 通过 item 修改系统配置的 value
func (receiver *WafSystemConfigService) ModifyByItemApi(req request.WafSystemConfigEditByItemReq) error {
	var sysConfig model.SystemConfig
	err := global.GWAF_LOCAL_DB.Where("item = ?", req.Item).First(&sysConfig).Error
	if err != nil {
		return errors.New("配置项不存在")
	}

	value, encryptErr := encConfigValue(req.Item, req.Value)
	if encryptErr != nil {
		return encryptErr
	}
	editMap := map[string]interface{}{
		"Value":       value,
		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}

	return updateSystemConfig(global.GWAF_LOCAL_DB.Model(model.SystemConfig{}).Where("item = ?", req.Item), req.Item, editMap)
}

func updateSystemConfig(query *gorm.DB, item string, updates map[string]interface{}) error {
	if item == model.OpenPlatformSwitch {
		switch updates["Value"] {
		case "1":
			now := time.Now()
			// A repeated save or concurrent enable must not renew an active window.
			updates["enabled_until"] = gorm.Expr("CASE WHEN enabled_until > ? THEN enabled_until ELSE ? END", now.Unix(), now.Add(model.OpenPlatformWindow).Unix())
		case "0":
			updates["enabled_until"] = 0
		default:
			return errors.New("开放平台开关只接受 0 或 1")
		}
	}
	return query.Updates(updates).Error
}

func (receiver *WafSystemConfigService) CloseExpiredOpenPlatform(now time.Time) error {
	return global.GWAF_LOCAL_DB.Model(&model.SystemConfig{}).
		Where("item = ? AND value = ? AND enabled_until <= ?", model.OpenPlatformSwitch, "1", now.Unix()).
		Updates(map[string]any{"value": "0", "enabled_until": 0, "update_time": customtype.JsonTime(now)}).Error
}

// SyncMetaByItemApi 只同步配置项的说明性字段（分类/类型/可选项/备注），不动 Value。
// 用途：配置项的说明文案会随版本演进，而存量库里的行是首次写入时的老文案，
// 页面上一直显示过时说明（例如令牌有效期的"默认5分钟"）。这里让描述跟着代码走，
// 用户设置过的值不受影响。
func (receiver *WafSystemConfigService) SyncMetaByItemApi(item string, itemClass string, itemType string, options string, remarks string) error {
	editMap := map[string]interface{}{
		"is_system":   "1",
		"item_class":  itemClass,
		"item_type":   itemType,
		"options":     options,
		"remarks":     remarks,
		"UPDATE_TIME": customtype.JsonTime(time.Now()),
	}
	return global.GWAF_LOCAL_DB.Model(model.SystemConfig{}).Where("item = ?", item).Updates(editMap).Error
}

// 以下 Get* 一律返回【明文】Value：密钥类配置在库里是 swk1 密文，读取时统一解密，
// 让所有调用方（内部逻辑与 API 层）拿到的都是可直接使用的值。
// 页面回显的脱敏由 API handler 的 MaskSensitiveConfig 单独负责，不在这里做。
func (receiver *WafSystemConfigService) GetDetailApi(req request.WafSystemConfigDetailReq) model.SystemConfig {
	var bean model.SystemConfig
	global.GWAF_LOCAL_DB.Where("id=?", req.Id).Find(&bean)
	decryptConfigBean(&bean)
	return bean
}
func (receiver *WafSystemConfigService) GetDetailByItemApi(req request.WafSystemConfigDetailByItemReq) model.SystemConfig {
	var bean model.SystemConfig
	global.GWAF_LOCAL_DB.Where("Item=?", req.Item).Find(&bean)
	decryptConfigBean(&bean)
	return bean
}
func (receiver *WafSystemConfigService) GetDetailByIdApi(id string) model.SystemConfig {
	var bean model.SystemConfig
	global.GWAF_LOCAL_DB.Where("id=?", id).Find(&bean)
	decryptConfigBean(&bean)
	return bean
}
func (receiver *WafSystemConfigService) GetDetailByItem(item string) model.SystemConfig {
	var bean model.SystemConfig
	global.GWAF_LOCAL_DB.Where("Item=?", item).Find(&bean)
	decryptConfigBean(&bean)
	return bean
}
func (receiver *WafSystemConfigService) GetListApi(req request.WafSystemConfigSearchReq) ([]model.SystemConfig, int64, error) {
	var list []model.SystemConfig
	var total int64 = 0
	/*where条件*/
	var whereField = ""
	var whereValues []interface{}
	//where字段
	whereField = ""
	if len(req.Item) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " item=? "
	}
	if len(req.Remarks) > 0 {
		if len(whereField) > 0 {
			whereField = whereField + " and "
		}
		whereField = whereField + " remarks like ? "
	}
	//where字段赋值
	if len(req.Item) > 0 {
		whereValues = append(whereValues, req.Item)
	}
	if len(req.Remarks) > 0 {
		whereValues = append(whereValues, "%"+req.Remarks+"%")
	}
	global.GWAF_LOCAL_DB.Model(&model.SystemConfig{}).Where("item NOT IN ?", []string{"gpt_url", "gpt_token", "gpt_model", "ai_enable", "ai_mode"}).Where(whereField, whereValues...).Limit(req.PageSize).Offset(req.PageSize * (req.PageIndex - 1)).Find(&list)
	global.GWAF_LOCAL_DB.Model(&model.SystemConfig{}).Where("item NOT IN ?", []string{"gpt_url", "gpt_token", "gpt_model", "ai_enable", "ai_mode"}).Where(whereField, whereValues...).Count(&total)
	for i := range list {
		decryptConfigBean(&list[i])
	}

	return list, total, nil
}
func (receiver *WafSystemConfigService) DelApi(req request.WafSystemConfigDelReq) error {
	var bean model.SystemConfig
	err := global.GWAF_LOCAL_DB.Where("id = ? and is_system='0'", req.Id).First(&bean).Error
	if err != nil {
		return err
	}
	if IsSensitiveConfigItem(bean.Item) || bean.Item == "last_run_version" || bean.Item == "max_run_version" {
		return errors.New("内置或密钥配置项不能删除，请在对应配置页修改")
	}
	result := global.GWAF_LOCAL_DB.Where("id = ? and is_system='0'", req.Id).Delete(model.SystemConfig{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("配置项已变化或受保护，请重新读取后操作")
	}
	return nil
}

// GetAllConfigs loads the instance settings, decrypting secret values for runtime use.
func (receiver *WafSystemConfigService) GetAllConfigs() map[string]model.SystemConfig {
	var configs []model.SystemConfig
	global.GWAF_LOCAL_DB.Find(&configs)
	result := make(map[string]model.SystemConfig, len(configs))
	for _, config := range configs {
		decryptConfigBean(&config)
		result[config.Item] = config
	}
	return result
}
