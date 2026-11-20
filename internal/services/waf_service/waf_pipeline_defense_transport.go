package waf_service

import (
	"HomeWaf/internal/protection/firewall"
	"HomeWaf/internal/protection/wafpipeline"
	"errors"
	"net/http"
	"net/netip"
)

var PipelineFlowDropper = firewall.NewFlowDropper()

func defenseDropCapability(version string) error { return PipelineFlowDropper.Probe(version) }
func PipelineDefenseCapabilities(version string) map[string]any {
	err := defenseDropCapability(version)
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return map[string]any{"reset_supported": true, "drop_supported": err == nil, "drop_reason": reason, "runtime_error": PipelineFlowDropper.Error(), "direct_only": true}
}
func DropPipelineConnection(r *http.Request) error {
	conn := wafpipeline.DefenseConnection(r)
	if conn == nil {
		return errors.New("未找到受管TCP连接，不能执行静默丢包")
	}
	local, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		return err
	}
	remote, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return err
	}
	return PipelineFlowDropper.Drop(local, remote)
}
