// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/storage/wafdb"
)

type WafShareDbService struct{}

var WafShareDbServiceApp = new(WafShareDbService)

func (receiver *WafShareDbService) AddApi(shareDb model.ShareDb) error {
	return global.GWAF_LOCAL_DB.Create(&shareDb).Error
}

// DeleteById 删除一条分库归档记录（按主键 Id）。归档物理文件由调用方负责删除。
func (receiver *WafShareDbService) DeleteById(id string) error {
	return global.GWAF_LOCAL_DB.Where("id = ?", id).Delete(&model.ShareDb{}).Error
}

// GetAllShareDbApi lists current log stores belonging to this driver.
func (receiver *WafShareDbService) GetAllShareDbApi() ([]model.ShareDb, error) {
	var all []model.ShareDb
	if err := global.GWAF_LOCAL_DB.Model(&model.ShareDb{}).Where("db_logic_type = ?", "log").Find(&all).Error; err != nil {
		return nil, err
	}
	list := make([]model.ShareDb, 0, len(all))
	for _, s := range all {
		if s.FileName == wafdb.LiveLogName() || wafdb.IsLogArchive(s.FileName) {
			list = append(list, s)
		}
	}
	return list, nil
}
