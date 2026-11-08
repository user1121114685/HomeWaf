// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waftask

import (
	"HomeWaf/global"
)

func TaskCC() {
	global.GWAF_CHAN_CLEAR_CC_WINDOWS <- 1
}
