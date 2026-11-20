// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/utils"
)

type WafHostService struct{}

var WafHostServiceApp = new(WafHostService)

func (receiver *WafHostService) GetDetailByCodeApi(code string) model.Hosts {
	var webHost model.Hosts
	global.GWAF_LOCAL_DB.Where("CODE=?", code).Find(&webHost)
	return webHost
}
func (receiver *WafHostService) CheckAvailablePortExistApi(port int) int64 {
	// 按声明的监听表检查仍在使用的端口。
	var hosts []model.Hosts
	global.GWAF_LOCAL_DB.Model(&model.Hosts{}).
		Where("start_status = 0 and global_host = 0").Find(&hosts)
	for _, h := range hosts {
		for _, p := range utils.HostListenPorts(h) {
			if p == port {
				return 1
			}
		}
	}
	return 0
}
