package wafpipeline

import (
	"HomeWaf/wafenginecore/ipset"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type CompiledPolicy struct {
	Deny, Trusted, AllowOnly *ipset.TimedSet
	Proxies                  *ipset.MatchSet
	Pipeline                 *Pipeline
	Proxy                    *ProxyTarget
	Metrics                  *PortMetrics
	Extensions               *compiledExtensions
	Sensitive                *compiledSensitive
	Privacy                  *compiledPrivacy
	BusinessSelector         *ProxyTarget
	Plugins                  *PreparedPlugins
	GateLogo                 string
	GateExemptIPs            *ipset.MatchSet
}

func compileIP(c PortConfig) (*CompiledPolicy, error) {
	var deny, trusted, allow []ipset.TimedEntry
	wild := 0
	for _, e := range c.Policy.IPGroup.Entries {
		if e.ExpiresAt < 0 {
			return nil, fmt.Errorf("invalid IP expiry")
		}
		p, err := ipset.ParsePattern(e.Value)
		if err != nil {
			return nil, err
		}
		if p.Kind == ipset.KindWildcard && p.Prefix < 0 {
			wild++
			if wild > 512 {
				return nil, fmt.Errorf("non-contiguous wildcard limit is 512")
			}
		}
		entry := ipset.TimedEntry{Value: e.Value, ExpiresAt: e.ExpiresAt}
		if e.Type == "deny" {
			deny = append(deny, entry)
		} else if e.Trust == "trusted" {
			trusted = append(trusted, entry)
		} else {
			allow = append(allow, entry)
		}
	}
	d, err := ipset.BuildTimedSet(deny)
	if err != nil {
		return nil, err
	}
	t, err := ipset.BuildTimedSet(trusted)
	if err != nil {
		return nil, err
	}
	a, err := ipset.BuildTimedSet(allow)
	if err != nil {
		return nil, err
	}
	gateIPs := ipset.BuildMatchSet(c.Policy.Gate.ExemptIPs)
	if gateIPs.WildcardLen() > 512 {
		return nil, fmt.Errorf("gate wildcard exception quota exceeded")
	}
	return &CompiledPolicy{Deny: d, Trusted: t, AllowOnly: a, Proxies: ipset.BuildMatchSet(c.Policy.Network.TrustedProxies), GateExemptIPs: gateIPs}, nil
}
func ExtractClientIP(r *http.Request, cfg NetworkConfig, proxies *ipset.MatchSet) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid peer address")
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid peer IP")
	}
	peer = peer.Unmap()
	if proxies == nil || !proxies.Contains(net.IP(peer.AsSlice())) {
		return peer, nil
	}
	header := cfg.RealIPHeader
	if header == "" {
		header = "X-Forwarded-For"
	}
	chain := r.Header.Values(header)
	if len(chain) == 0 {
		return peer, nil
	}
	raw := strings.Join(chain, ",")
	if len(raw) > 8192 {
		return peer, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 64 {
		return peer, nil
	}
	// Any invalid hop makes this header unusable. Never skip attacker-controlled garbage.
	addresses := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		ip, e := netip.ParseAddr(strings.TrimSpace(part))
		if e != nil {
			return peer, nil
		}
		addresses = append(addresses, ip.Unmap())
	}
	for i := len(addresses) - 1; i >= 0; i-- {
		ip := addresses[i]
		if !proxies.Contains(net.IP(ip.AsSlice())) {
			return ip, nil
		}
	}
	return peer, nil
}

type IPStateReader interface {
	BanReason(string, netip.Addr) string
	Locked(string) bool
	InHoneypot(string, netip.Addr) bool
}

func protectedAddress(ip netip.Addr) bool {
	return !ip.IsValid() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
func (r *Runtime) ipStep(s *Snapshot) Step {
	return Step{Number: 2, Name: "ip_group", FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
		ip := net.IP(c.ClientIP.AsSlice())
		now := time.Now().Unix()
		p := s.Compiled
		if p.Deny.Contains(ip, now) {
			return Verdict{Action: Deny, Reason: "blacklist:custom", Status: 403}, nil
		}
		trusted := p.Trusted.Contains(ip, now)
		protected := protectedAddress(c.ClientIP)
		if r.Protected != nil && r.Protected(c.ClientIP) {
			protected = true
		}
		c.Values["protected_source"] = trusted || protected
		if trusted || protected {
			c.Trusted = trusted
			if r.State != nil {
				var matched bool
				if diagnostic, ok := r.State.(interface{ ProtectedRestriction(string, netip.Addr) bool }); ok {
					matched = diagnostic.ProtectedRestriction(c.PortID, c.ClientIP)
				} else {
					matched = r.State.BanReason(c.PortID, c.ClientIP) != "" || r.State.InHoneypot(c.PortID, c.ClientIP)
				}
				if matched {
					c.Hit("protected_match", "automatic_state", Low, true, "automatic restriction ignored for protected source")
				}
			}
			if s.Config.Policy.IPGroup.FeedEnabled && r.Feed != nil {
				matched := r.Feed.Contains(c.ClientIP, now)
				if diagnostic, ok := r.Feed.(interface{ ContainsForDiagnostics(netip.Addr, int64) bool }); ok {
					matched = diagnostic.ContainsForDiagnostics(c.ClientIP, now)
				}
				if matched {
					c.Hit("protected_match", "signed_feed", Low, true, "feed match ignored for protected source")
				}
			}
		}
		if trusted {
			return Verdict{Action: Bypass, Reason: "whitelist:trusted"}, nil
		}
		if !protected && r.State != nil {
			if reason := r.State.BanReason(c.PortID, c.ClientIP); reason != "" {
				return Verdict{Action: Deny, Reason: "blacklist:auto:" + reason, Status: 403}, nil
			}
		}
		if !protected && s.Config.Policy.IPGroup.FeedEnabled && r.Feed != nil && r.Feed.Contains(c.ClientIP, now) {
			return Verdict{Action: Deny, Reason: "blacklist:vip", Status: 403}, nil
		}
		if r.State != nil && r.State.Locked(c.PortID) {
			return Verdict{Action: s.Config.Policy.HitPolicy.Lockdown.Action, Reason: "lockdown", Status: 503}, nil
		}
		if !protected && s.Config.Policy.Honeypot.Enabled && r.State != nil && r.State.InHoneypot(c.PortID, c.ClientIP) {
			return Verdict{Action: Honeypot, Reason: "honeypot:state"}, nil
		}
		if p.AllowOnly.Contains(ip, now) {
			return Verdict{Action: Continue, Reason: "whitelist:allow_only"}, nil
		}
		if !protected && r.Geo != nil && len(s.Config.Policy.IPGroup.Regions) > 0 {
			geo := r.Geo.Lookup(c.ClientIP)
			if geo.OK {
				for _, allowed := range s.Config.Policy.IPGroup.Regions {
					if strings.EqualFold(allowed.Country, geo.Country) && (allowed.Province == "" || strings.EqualFold(allowed.Province, geo.Province)) {
						return Verdict{Action: Continue, Reason: "whitelist:region"}, nil
					}
				}
			}
		}
		action := s.Config.Policy.IPGroup.UnlistedAction
		status := 0
		if action == Deny {
			status = 403
		}
		return Verdict{Action: action, Reason: "ip:unlisted", Status: status}, nil
	}}
}
