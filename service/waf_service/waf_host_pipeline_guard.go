package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"fmt"
)

// A fully handed-off site is retained as a source record, not a second active
// policy editor. A partially migrated multi-port site may still configure its
// remaining old listeners, but may not add a new native port to its bindings.
func guardNativePortOwnership(candidate model.Hosts, previous *model.Hosts) error {
	oldPorts := map[int]bool{}
	if previous != nil {
		allOwned := true
		listens := utils.ResolveHostListens(*previous)
		for _, listen := range listens {
			oldPorts[listen.Port] = true
			if listen.Port < 1 || listen.Port > 65535 || !wafpipeline.OwnsLegacyPort(uint16(listen.Port)) {
				allOwned = false
			}
		}
		if len(listens) > 0 && allOwned {
			return fmt.Errorf("该站点监听已由五步流水线接管，请在端口总览修改；此处为原站点源记录")
		}
	}
	var candidates []int
	for _, listen := range utils.ResolveHostListens(candidate) {
		if oldPorts[listen.Port] {
			continue
		}
		if listen.Port < 1 || listen.Port > 65535 {
			continue
		}
		if wafpipeline.OwnsLegacyPort(uint16(listen.Port)) {
			return fmt.Errorf("端口%d已归五步流水线管理，请在端口总览配置", listen.Port)
		}
		candidates = append(candidates, listen.Port)
	}
	if len(candidates) == 0 || global.GWAF_LOCAL_DB == nil || !global.GWAF_LOCAL_DB.Migrator().HasTable(&model.PortConfig{}) {
		return nil
	}
	var rows []model.PortConfig
	if err := global.GWAF_LOCAL_DB.Select("port").Where("port IN ?", candidates).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 0 {
		return fmt.Errorf("端口%d已有五步配置，不能从原站点入口重复绑定", rows[0].Port)
	}
	return nil
}
