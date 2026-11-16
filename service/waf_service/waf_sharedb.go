// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafdb/dialect"
)

type WafShareDbService struct{}

var WafShareDbServiceApp = new(WafShareDbService)

func (receiver *WafShareDbService) AddApi(shareDb model.ShareDb) error {
	global.GWAF_LOCAL_DB.Create(shareDb)
	return nil
}

// DeleteById 删除一条分库归档记录（按主键 Id）。归档物理文件由调用方负责删除。
func (receiver *WafShareDbService) DeleteById(id string) error {
	return global.GWAF_LOCAL_DB.Where("id = ?", id).Delete(&model.ShareDb{}).Error
}

// 获取所有db（按当前驱动过滤分片类型）
// 历史遗留：share_dbs 表会同时存在 SQLite 文件分片(local_log*.db)与 MySQL 表分片(web_logs*)，
// 用户切换驱动后另一种分片对当前库无意义，这里按 ShareDb.IsTableShard() 与当前驱动是否文件型过滤：
//   - SQLite(文件型) 只保留文件分片(.db)
//   - MySQL(非文件型) 只保留表分片(无 .db 后缀)
func (receiver *WafShareDbService) GetAllShareDbApi() ([]model.ShareDb, error) {
	var all []model.ShareDb
	global.GWAF_LOCAL_DB.Model(&model.ShareDb{}).Find(&all)

	fileBased := dialect.Get().IsFileBased()
	list := make([]model.ShareDb, 0, len(all))
	for _, s := range all {
		// 文件型驱动保留非表分片(.db)，非文件型驱动保留表分片
		if s.IsTableShard() != fileBased {
			list = append(list, s)
		}
	}
	return list, nil
}
