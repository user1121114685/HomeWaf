package wafpipeline

import (
	"HomeWaf/model"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type ssrfDetector struct{}

func SSRFDetector() Detector          { return ssrfDetector{} }
func (ssrfDetector) Name() string     { return "ssrf" }
func (ssrfDetector) Category() string { return "ssrf" }
func (ssrfDetector) Inspect(v *NormalizedView) []Match {
	for _, f := range v.Fields {
		if v.expired() {
			break
		}
		if v.excluded(f.Name) {
			continue
		}
		if ssrfURL(f.Value) || f.RawValue != f.Value && ssrfURL(f.RawValue) {
			return []Match{{"ssrf", f.Source, "internal_url"}}
		}
	}
	return nil
}

var carrierNAT = netip.MustParsePrefix("100.64.0.0/10")

func ssrfURL(value string) bool {
	value = strings.TrimSpace(value)
	scheme, _, ok := strings.Cut(value, ":")
	if !ok {
		return false
	}
	switch strings.ToLower(scheme) {
	case "http", "https", "file", "gopher", "dict":
	default:
		return false
	}
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "file", "gopher", "dict":
		return true
	case "http", "https":
	default:
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		var ok bool
		ip, ok = legacyIPv4(host)
		if !ok {
			return false
		}
	}
	ip = ip.Unmap()
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || carrierNAT.Contains(ip)
}

// Recognize inet_aton forms accepted by many URL clients: dotted decimal,
// shortened IPv4, hexadecimal, octal, and a single 32-bit integer. No DNS.
func legacyIPv4(host string) (netip.Addr, bool) {
	parts := strings.Split(host, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return netip.Addr{}, false
	}
	var total uint64
	for i, p := range parts {
		if p == "" || strings.HasPrefix(p, "+") || strings.HasPrefix(p, "-") {
			return netip.Addr{}, false
		}
		base := 10
		if len(p) > 1 && p[0] == '0' {
			base = 0
		}
		n, err := strconv.ParseUint(p, base, 32)
		if err != nil {
			return netip.Addr{}, false
		}
		if i < len(parts)-1 {
			if n > 255 {
				return netip.Addr{}, false
			}
			total = (total << 8) | n
		} else {
			bits := uint(8 * (5 - len(parts)))
			if bits < 32 && n >= 1<<bits {
				return netip.Addr{}, false
			}
			total = (total << bits) | n
		}
	}
	if total > 1<<32-1 {
		return netip.Addr{}, false
	}
	return netip.AddrFrom4([4]byte{byte(total >> 24), byte(total >> 16), byte(total >> 8), byte(total)}), true
}

type csrfDetector struct {
	scheme, missing string
	allowed         map[string]bool
	options         *model.CsrfConfig
}

func CSRFDetector(scheme, missing string, origins []string) Detector {
	d := csrfDetector{scheme: scheme, missing: missing, allowed: map[string]bool{}}
	for _, o := range origins {
		if normalized, ok := canonicalOrigin(o, false); ok {
			d.allowed[normalized] = true
		}
	}
	return d
}
func (csrfDetector) Name() string     { return "csrf" }
func (csrfDetector) Category() string { return "csrf" }
func (csrfDetector) NeedsBody() bool  { return false }

// CanonicalOrigin is shared with the legacy CSRF adapter so both compare
// scheme, hostname and effective port using the same parser.
func CanonicalOrigin(value string, referer bool) (string, bool) {
	return canonicalOrigin(value, referer)
}

func canonicalOrigin(value string, referer bool) (string, bool) {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Host == "" || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	if !referer && ((u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "") {
		return "", false
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.TrimSuffix(strings.ToLower(u.Hostname()), "."), port), true
}
func (d csrfDetector) Inspect(v *NormalizedView) []Match {
	r := v.Request
	if d.options != nil {
		cfg := d.options
		if cfg.IsEnable != 1 || !scopeMatches(Scope{Methods: splitOptionList(strings.ToUpper(cfg.ProtectMethods)), ExcludePaths: splitOptionList(cfg.ExcludePaths)}, r) {
			return nil
		}
	} else {
		switch r.Method {
		case "POST", "PUT", "PATCH", "DELETE":
		default:
			return nil
		}
		if len(r.Cookies()) == 0 {
			return nil
		}
	}
	scheme := d.scheme
	if value, ok := r.Context().Value(externalSchemeKey{}).(string); ok {
		scheme = value
	}
	expected, valid := canonicalOrigin(scheme+"://"+r.Host, false)
	if !valid {
		return []Match{{"csrf", "header", "invalid_host"}}
	}
	origin := r.Header.Get("Origin")
	referer := false
	if origin == "" {
		origin = r.Header.Get("Referer")
		referer = true
	}
	if origin == "" {
		switch d.missing {
		case "allow":
			return nil
		case "block":
			return []Match{{"csrf", "header", "missing_origin"}}
		default:
			return []Match{{"csrf", "header", "missing_origin_observe"}}
		}
	}
	actual, ok := canonicalOrigin(origin, referer)
	allowed := d.allowed[actual]
	if ok && !allowed {
		for pattern := range d.allowed {
			p, _ := url.Parse(pattern)
			a, _ := url.Parse(actual)
			if p != nil && a != nil && strings.HasPrefix(p.Hostname(), "*.") && p.Scheme == a.Scheme && p.Port() == a.Port() && (a.Hostname() == p.Hostname()[2:] || strings.HasSuffix(a.Hostname(), p.Hostname()[1:])) {
				allowed = true
				break
			}
		}
	}
	if !ok || len(r.Header.Values("Origin")) > 1 || actual != expected && !allowed {
		return []Match{{"csrf", "header", "origin_mismatch"}}
	}
	return nil
}
