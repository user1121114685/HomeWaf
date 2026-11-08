// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafinterface

import "HomeWaf/innerbean"

type WafNotify interface {
	NotifyBatch(logs []*innerbean.WebLog) error // 处理多个日志
	NotifySingle(log *innerbean.WebLog) error   // 处理单个日志
}
