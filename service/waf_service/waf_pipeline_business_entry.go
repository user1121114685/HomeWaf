package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/utils"
)

func legacyBusinessEntry(host model.Hosts, protocol string) (string, uint16, int) {
	if host.START_STATUS != 0 {
		return "unavailable", 0, 0
	}
	if protocol != "http" || host.AutoJumpHTTPS != 1 {
		return "", 0, 0
	}
	for _, listen := range utils.ResolveHostListens(host) {
		if listen.Protocol == "https" && listen.Port > 0 && listen.Port < 65536 {
			code := int(global.CurrentSettings().GCONFIG_RECORD_REDIRECT_HTTPS_CODE)
			if code != 301 && code != 302 && code != 307 && code != 308 {
				code = 301
			}
			return "https_redirect", uint16(listen.Port), code
		}
	}
	return "", 0, 0
}
