// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waftunnelmodel

import (
	"HomeWaf/model"
	"sync"
)

// TunnelSafe 隧道安全配置
type TunnelSafe struct {
	Mux    sync.Mutex //互斥锁
	Tunnel model.Tunnel
}
