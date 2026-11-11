// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafbot"
	"net/http"
	"net/url"
	"strconv"
)

// DNS is opt-in and asynchronous. An unavailable/unverified identity continues
// ordinary checks; it is neither a trusted exemption nor evidence of spoofing.
func (waf *WafEngine) CheckBot(r *http.Request, weblogbean *innerbean.WebLog, formValue url.Values, hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{}
	weblogbean.BotVerifyStrong = 0
	canonical := wafbot.CanonicalCrawlerUA(weblogbean.USER_AGENT)
	if canonical == "" || hostTarget == nil {
		return result
	}
	listenerPort, _ := r.Context().Value(legacyListenerContextKey{}).(string)
	if listenerPort == "" {
		listenerPort = strconv.Itoa(hostTarget.Host.Port)
	}
	botResult := wafbot.BotResult{IsBot: true, IsNormalBot: true, BotName: canonical + "（身份未验证）"}
	if canonical == "360Spider" || canonical == "Bytespider" {
		botResult = wafbot.DetermineNormalSearch(canonical, weblogbean.SRC_IP)
	} else if waf.BotIdentity != nil {
		verified, spoofed := waf.BotIdentity("legacy:"+listenerPort, canonical, weblogbean.SRC_IP, false)
		botResult.StrongVerified = verified
		botResult.IsNormalBot = !spoofed
		if verified {
			botResult.BotName = canonical + "（已验证）"
		}
		if spoofed {
			botResult.BotName = "伪装" + canonical
		}
	}
	weblogbean.IsBot = 1
	weblogbean.GUEST_IDENTIFICATION = botResult.BotName
	if botResult.StrongVerified {
		weblogbean.BotVerifyStrong = 1
	}
	if !botResult.IsNormalBot {
		weblogbean.RISK_LEVEL = max(weblogbean.RISK_LEVEL, 1)
		result.IsBlock = requestSettings(r).GCONFIG_RECORD_SPIDER_DENY == 1
		result.Title, result.Content = botResult.BotName, "请正确访问"
	}
	return result
}
