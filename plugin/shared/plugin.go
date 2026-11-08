// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package shared

import (
	"HomeWaf/plugin/process"
	"HomeWaf/plugin/proto"
)

// Serve starts the signed HomeWaf plugin protocol; legacy go-plugin handshakes are not accepted.
func Serve(server proto.IPFilterPluginServer) error { return process.Serve(server) }
