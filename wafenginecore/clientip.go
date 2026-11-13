// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/common/zlog"
	"HomeWaf/internal/proxyidentity"
	"HomeWaf/model"
	"HomeWaf/wafenginecore/clientip"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// getBizClientIP only accepts identity from an explicitly trusted direct peer.
// Empty mode retains the configured global header names, not implicit trust.
// Unknown modes and missing trust configuration fail closed to RemoteAddr.
func (waf *WafEngine) getBizClientIP(r *http.Request, host model.Hosts) (error, string, string) {
	if host.IPMode == "nic" {
		return splitRemoteAddr(r.RemoteAddr)
	}
	switch host.IPSourceMode {
	case "nic":
		return splitRemoteAddr(r.RemoteAddr)
	case "header":
		if ip := trustedHeaderIP(r, host, host.IPRealHeader); ip != "" {
			return nil, ip, "0"
		}
		return splitRemoteAddr(r.RemoteAddr) // 头缺失/非法 → 回退网络层
	case "xff_depth":
		if ip := extractXFFDepth(r, host); ip != "" {
			return nil, ip, "0"
		}
		return splitRemoteAddr(r.RemoteAddr)
	case "cdn_preset":
		return waf.extractCDNPreset(r, host)
	case "":
		for _, header := range strings.Split(requestSettings(r).GCONFIG_RECORD_PROXY_HEADER, ",") {
			if ip := trustedHeaderIP(r, host, header); ip != "" {
				return nil, ip, "0"
			}
		}
		return splitRemoteAddr(r.RemoteAddr)
	default:
		return splitRemoteAddr(r.RemoteAddr)
	}
}

// splitRemoteAddr 从 r.RemoteAddr 拆出网络层 IP 与端口
func splitRemoteAddr(remoteAddr string) (error, string, string) {
	ip, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return err, "", ""
	}
	return nil, ip, port
}

// headerFirstValidIP accepts one unambiguous address from a trusted proxy.
func headerFirstValidIP(r *http.Request, header string) string {
	ips, ok := proxyidentity.ParseChain(r.Header, header)
	if ok && len(ips) == 1 {
		return ips[0].String()
	}
	return ""
}

func trustedHeaderIP(r *http.Request, host model.Hosts, header string) string {
	_, peer, _ := splitRemoteAddr(r.RemoteAddr)
	if !ipInCIDRList(peer, host.IPTrustProxies) {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(header), "X-Forwarded-For") || strings.EqualFold(strings.TrimSpace(header), "X-Original-Forwarded-For") {
		host.IPRealHeader = strings.TrimSpace(header)
		return extractXFFDepth(r, host)
	}
	return headerFirstValidIP(r, header)
}

// extractXFFDepth requires a trusted direct peer, then walks the complete
// valid chain from the right. A hop count alone is never proof of trust.
func extractXFFDepth(r *http.Request, host model.Hosts) string {
	header := host.IPRealHeader
	if strings.TrimSpace(header) == "" {
		header = "X-Forwarded-For"
	}
	trusted := strings.TrimSpace(host.IPTrustProxies)
	peerErr, peerIP, _ := splitRemoteAddr(r.RemoteAddr)
	if peerErr != nil || !ipInCIDRList(peerIP, trusted) {
		return ""
	}
	parts, ok := proxyidentity.ParseChain(r.Header, header)
	if !ok {
		return ""
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if !ipInCIDRList(parts[i].String(), trusted) {
			return parts[i].String()
		}
	}
	return ""
}

// extractCDNPreset cdn_preset 模式：仅当直连对端属于所选厂商回源段(或用户手填可信网段)时才信任其真实 IP 头。
func (waf *WafEngine) extractCDNPreset(r *http.Request, host model.Hosts) (error, string, string) {
	err, netIP, _ := splitRemoteAddr(r.RemoteAddr)
	if err != nil {
		return err, "", ""
	}
	// 来源可信判定：厂商官方回源段(Tier A 自动拉取) 或 用户手填网段(Tier B/兜底)
	sourceTrusted := clientip.IsProviderIP(host.CDNProvider, netIP)
	if !sourceTrusted && strings.TrimSpace(host.IPTrustProxies) != "" {
		sourceTrusted = ipInCIDRList(netIP, host.IPTrustProxies)
	}
	if sourceTrusted {
		header := host.IPRealHeader
		if strings.TrimSpace(header) == "" {
			header = clientip.DefaultHeader(host.CDNProvider)
		}
		if strings.EqualFold(header, "X-Forwarded-For") || strings.EqualFold(header, "X-Original-Forwarded-For") {
			if chain, ok := proxyidentity.ParseChain(r.Header, header); ok {
				for i := len(chain) - 1; i >= 0; i-- {
					if !IsIPSourceTrustedPeer(host, chain[i].String()) {
						return nil, chain[i].String(), "0"
					}
				}
			}
			return nil, netIP, "0"
		}
		if ip := headerFirstValidIP(r, header); ip != "" {
			return nil, ip, "0"
		}
	}
	// 保存时已拦住"两个可信来源都没有"的组合，但中心库可能事后被清空或拉取失败，
	// 那样这里会静默降级成一直取回源节点 IP。这属于配置问题(而非伪造攻击)，出声提醒。
	if clientip.GetProviderRanges(host.CDNProvider).Len() == 0 && strings.TrimSpace(host.IPTrustProxies) == "" {
		warnCDNPresetNoTrustSource(host)
	}
	// 来源不可信或头缺失 → 回退网络层(宁可少信任，不可采信伪造)
	return nil, netIP, "0"
}

// cdnPresetWarnAt 告警限频：hostCode -> 上次告警的 unix 秒
var cdnPresetWarnAt sync.Map

// warnCDNPresetNoTrustSource cdn_preset 但一个可信来源都没有时告警。
// 此时每个请求都会回退网络层 IP，用户往往毫无察觉，直到 CC/黑名单误封整个 CDN 回源节点、
// 全站访客一起被拦才发现。每站每 10 分钟最多一条，避免高频请求把日志刷爆。
func warnCDNPresetNoTrustSource(host model.Hosts) {
	now := time.Now().Unix()
	if v, ok := cdnPresetWarnAt.Load(host.Code); ok {
		if last, _ := v.(int64); now-last < 600 {
			return
		}
	}
	cdnPresetWarnAt.Store(host.Code, now)
	zlog.Warn("真实IP来源配置失效", "host", host.Host, "cdn_provider", host.CDNProvider,
		"detail", "中心库无该厂商回源段且未填写可信代理网段，来源校验全部失败，已回退网络层IP(CDN回源节点IP)")
}

// ipInCIDRList 判断 ip 是否落在逗号分隔的 CIDR/IP 列表内
func ipInCIDRList(ip, csv string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, entry := range strings.Split(csv, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if peer := net.ParseIP(entry); peer != nil && peer.Equal(parsed) {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			if ones, _ := network.Mask.Size(); ones > 0 && network.Contains(parsed) {
				return true
			}
		}
	}
	return false
}
