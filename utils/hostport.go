// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package utils

import (
	"HomeWaf/common/zlog"
	"HomeWaf/model"
	"encoding/json"
	"strings"
)

// 端口监听协议与 IP 版本的合法取值。
// IP 版本词汇与隧道 Tunnel.IpVersion 保持同一套（ipv4/ipv6/both）。
const (
	ListenProtoHTTP  = "http"
	ListenProtoHTTPS = "https"
	ListenIPVBoth    = "both"
	ListenIPV4       = "ipv4"
	ListenIPV6       = "ipv6"
)

// PortListenItem Hosts.PortListensJSON 的元素结构。
type PortListenItem struct {
	Port  int    `json:"port"`
	Proto string `json:"proto"`
	Ipv   string `json:"ipv,omitempty"`
	Addr  string `json:"addr,omitempty"` // 指定监听地址暂不支持
}

// HostListen 站点的一条监听配置（解析归一后的结果）。
// json 标签与 PortListenItem 对齐，前端把 resolved_listens 原样映射回端口行即可。
type HostListen struct {
	Port      int    `json:"port"`
	Protocol  string `json:"proto"` // http | https
	IPVersion string `json:"ipv"`   // both | ipv4 | ipv6
	Addr      string `json:"addr"`  // 恒为空，预留
	IsMain    bool   `json:"is_main"`
}

// ResolveHostListens 站点「端口→协议→IP版本」的唯一真源。
// 监听表必须显式声明主端口。无效配置不产生监听。
func ResolveHostListens(h model.Hosts) []HostListen {
	if h.GLOBAL_HOST == 1 {
		return nil
	}
	raw := strings.TrimSpace(h.PortListensJSON)
	if raw == "" {
		return nil
	}
	var items []PortListenItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		zlog.Warn("站点监听表 JSON 解析失败", "host", h.Host, "code", h.Code, "error", err.Error())
		return nil
	}
	out := make([]HostListen, 0, len(items))
	seen := map[int]bool{}
	for _, item := range items {
		if item.Port < 1 || item.Port > 65535 {
			zlog.Warn("站点监听表包含无效端口", "host", h.Host, "port", item.Port)
			return nil
		}
		protocol := strings.ToLower(strings.TrimSpace(item.Proto))
		if protocol != ListenProtoHTTP && protocol != ListenProtoHTTPS {
			zlog.Warn("站点监听表包含无效协议", "host", h.Host, "port", item.Port)
			return nil
		}
		ipVersion := strings.ToLower(strings.TrimSpace(item.Ipv))
		if ipVersion == "" {
			ipVersion = ListenIPVBoth
		}
		if ipVersion != ListenIPVBoth && ipVersion != ListenIPV4 && ipVersion != ListenIPV6 {
			zlog.Warn("站点监听表包含无效 IP 版本", "host", h.Host, "port", item.Port)
			return nil
		}
		if strings.TrimSpace(item.Addr) != "" {
			zlog.Warn("站点监听表指定了未支持的绑定地址", "host", h.Host, "port", item.Port)
			return nil
		}
		listen := HostListen{Port: item.Port, Protocol: protocol, IPVersion: ipVersion, IsMain: item.Port == h.Port}
		if seen[item.Port] {
			zlog.Warn("站点监听表包含重复端口", "host", h.Host, "port", item.Port)
			return nil
		}
		seen[item.Port] = true
		out = append(out, listen)
	}
	if !seen[h.Port] {
		zlog.Warn("站点监听表缺少主端口", "host", h.Host, "port", h.Port)
		return nil
	}
	moveMainFirst(out)
	return out
}

func moveMainFirst(out []HostListen) {
	for i := range out {
		if out[i].IsMain {
			if i != 0 {
				main := out[i]
				copy(out[1:i+1], out[0:i])
				out[0] = main
			}
			return
		}
	}
}

// HostListenPorts 返回站点声明的监听端口，供端口引用计数使用。
func HostListenPorts(h model.Hosts) []int {
	listens := ResolveHostListens(h)
	ports := make([]int, 0, len(listens))
	for _, l := range listens {
		ports = append(ports, l.Port)
	}
	return ports
}

// HostMainProtocol 返回监听表中主端口的协议。配置无效时返回空字符串。
func HostMainProtocol(h model.Hosts) string {
	for _, l := range ResolveHostListens(h) {
		if l.IsMain {
			return l.Protocol
		}
	}
	return ""
}
