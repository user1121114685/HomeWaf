// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/innerbean"
)

func shouldRecordWebLog(weblog *innerbean.WebLog) bool {
	if weblog == nil {
		return false
	}
	if !weblog.IsSecurityEvent() && globalIPLogExclude.Load().match(weblog.SRC_IP) {
		return false
	}
	switch global.CurrentSettings().GWAF_RUNTIME_RECORD_LOG_TYPE {
	case "all":
		return true
	case "abnormal":
		return weblog.IsSecurityEvent()
	}
	return false
}
