package wafenginecore

import (
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafenginecore/cccounter"
	"HomeWaf/wafpipeline"
	"net"
	"net/http"
	"strconv"
	"strings"
)

var legacyPasswordCache = accessgate.NewPasswordCache(10000)
var legacyAuthAttempts = cccounter.New(200000, 1000)
var legacyHashWork = wafpipeline.NewAdmission(32)

func CleanupLegacyAuthCounters() { legacyAuthAttempts.Cleanup(60) }
func httpAuthPort(host model.Hosts, r *http.Request) uint16 {
	if port, err := wafpipeline.LocalPort(r); err == nil {
		return port
	}
	if host.Port > 0 && host.Port <= 65535 {
		return uint16(host.Port)
	}
	return 0
}
func httpAuthCookieName(host model.Hosts, r *http.Request) string {
	return "samwaf_httpauth_token_" + strconv.Itoa(int(httpAuthPort(host, r)))
}
func legacyCredentialHash(host *wafenginmodel.HostSafe, user string) (string, bool) {
	for _, account := range host.HttpAuthBases {
		if account.UserName == user && account.PasswordHashed && utils.IsBcryptHash(account.Password) {
			return account.Password, true
		}
	}
	return accessgate.DummyPasswordHash, false
}
func legacyAllowAttempt(host model.Hosts, r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	result := legacyAuthAttempts.Incr("httpauth:"+strconv.Itoa(int(httpAuthPort(host, r))), peer, 60)
	return !result.Rejected && !result.Degraded && result.Count <= 10
}
func legacyLoginOrigin(r *http.Request) bool {
	source := r.Header.Get("Origin")
	if source == "" {
		source = r.Header.Get("Referer")
	}
	if source == "" {
		return false
	}
	return wafpipeline.SameOrigin(source, wafpipeline.RequestScheme(r), r.Host, r.Header.Get("Origin") == "")
}

func legacyRequestScheme(host model.Hosts, r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && strings.TrimSpace(host.IPTrustProxies) != "" && ipInCIDRList(peer, host.IPTrustProxies) && r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
