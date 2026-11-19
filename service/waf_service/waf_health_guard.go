// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/utils"
	"errors"
	"net"
	"strconv"
	"strings"
)

func newHealthDialGuard(runtime ...*wafenginmodel.SafeServerMap) (func(string) error, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, errors.New("读取本机源站检测地址信息失败")
	}
	localIPs := make(map[string]bool)
	for _, address := range addresses {
		if network, ok := address.(*net.IPNet); ok {
			localIPs[network.IP.String()] = true
		}
	}
	ports := map[int]bool{global.GWAF_LOCAL_SERVER_PORT: true}
	for _, listeners := range runtime {
		if listeners != nil {
			listeners.Range(func(port int, server innerbean.ServerRunTime) bool {
				if server.Status == 0 {
					ports[port] = true
				}
				return true
			})
		}
	}
	return func(address string) error {
		ipText, portText, err := net.SplitHostPort(address)
		if err != nil {
			return errors.New("实际源站地址格式不正确")
		}
		ip := net.ParseIP(strings.SplitN(ipText, "%", 2)[0])
		port, err := strconv.Atoi(portText)
		if ip == nil || err != nil {
			return errors.New("实际源站地址格式不正确")
		}
		if ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.IPv4bcast) {
			return errors.New("实际源站必须是可连接的单播地址")
		}
		if ip.IsLinkLocalUnicast() && !utils.IsOutboundHostAllowlisted(ip.String()) {
			return errors.New("链路本地源站需在 security.outbound_allowed_hosts 中声明其 IP")
		}
		if ports[port] && (ip.IsLoopback() || localIPs[ip.String()]) {
			return errors.New("检测目标为本机HomeWaf监听端口，请填写源站地址")
		}
		return nil
	}, nil
}
