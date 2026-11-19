package innerbean

import (
	"HomeWaf/wafnet"
	"net/http"
)

type ServerRunTime struct {
	//tcp http https
	ServerType string
	// IPVersion 监听 IP 版本：both(默认,双栈) / ipv4 / ipv6。空按 both 处理。
	IPVersion string
	Port      int
	Status    int // 0 是启动完成 ，1 是新增，2 是编辑 3，是删除
	Svr       *http.Server
	// Conns 当前该端口打开的连接数(原子计数)。由 http.Server.ConnState 维护：
	// StateNew +1 ；StateClosed / StateHijacked -1 。用于升级排空时观测"还剩多少连接"。
	// 为指针以便随 ServerRunTime 值在 SafeServerMap 间复制时仍共享同一计数。
	Conns *int64
	// Includes hijacked WebSocket connections until their underlying socket closes.
	Connections *wafnet.ConnTracker
}
