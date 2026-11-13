// Package proxyidentity owns the identity boundary between WAF and upstream.
package proxyidentity

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
)

// Strip removes client-controlled identity and route-override headers. A
// backend must not reinterpret an alternate identity after WAF authorization.
// Iterate actual keys too: Header.Del alone misses non-canonical map keys.
func Strip(h http.Header, extra ...string) {
	for name := range h {
		remove := false
		switch strings.ToLower(name) {
		case "forwarded", "x-forwarded-for", "x-real-ip", "cf-connecting-ip", "cf-connecting-ipv6",
			"true-client-ip", "client-ip", "x-client-ip", "x-originating-ip", "x-remote-ip", "x-remote-addr",
			"proxy-client-ip", "wl-proxy-client-ip", "x-proxyuser-ip", "x-cluster-client-ip",
			"fastly-client-ip", "eo-connecting-ip", "eo-client-ip", "ali-cdn-real-ip", "cdn-src-ip",
			"cloudfront-viewer-address", "x-azure-clientip", "x-original-forwarded-for",
			"x-forwarded-host", "x-forwarded-proto", "x-forwarded-port", "x-forwarded-server",
			"x-original-url", "x-rewrite-url", "x-original-host", "x-host", "x-http-host-override",
			"x-waf-custom-xff-control":
			remove = true
		}
		for _, configured := range extra {
			if configured != "" && strings.EqualFold(name, strings.TrimSpace(configured)) {
				remove = true
			}
		}
		if remove {
			delete(h, name)
		}
	}
}

// ParseChain treats all field lines as one bounded list, rejecting the whole
// chain on any invalid hop instead of skipping attacker-controlled garbage.
func ParseChain(h http.Header, name string) ([]netip.Addr, bool) {
	values := h.Values(strings.TrimSpace(name))
	if len(values) == 0 {
		return nil, false
	}
	size := 0
	for _, value := range values {
		size += len(value) + 1
		if size > 8192 {
			return nil, false
		}
	}
	parts := strings.Split(strings.Join(values, ","), ",")
	if len(parts) > 64 {
		return nil, false
	}
	ips := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		ip, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() {
			return nil, false
		}
		ips = append(ips, ip.Unmap())
	}
	return ips, true
}

// UnsupportedUpgrade permits only the WebSocket upgrade that the WAF supports.
// h2c would turn an inspected HTTP request into an uninspected HTTP/2 tunnel.
func UnsupportedUpgrade(h http.Header) bool {
	values := h.Values("Upgrade")
	if len(values) == 0 {
		return false
	}
	return len(values) != 1 || !strings.EqualFold(strings.TrimSpace(values[0]), "websocket")
}

type Identity struct {
	IP, Proto, Host, Port string
	SourceHeaders         []string
}
type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	identity.SourceHeaders = append([]string(nil), identity.SourceHeaders...)
	return context.WithValue(ctx, identityKey{}, identity)
}

func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

// Apply runs after Director and hop-header removal, so Connection cannot
// nominate trusted headers for removal and custom headers cannot replace them.
func (i Identity) Apply(h http.Header) {
	Strip(h, i.SourceHeaders...)
	h.Set("X-Forwarded-For", i.IP)
	h.Set("X-Real-IP", i.IP)
	h.Set("X-Forwarded-Proto", i.Proto)
	h.Set("X-Forwarded-Host", i.Host)
	if i.Port != "" {
		h.Set("X-Forwarded-Port", i.Port)
	}
}
