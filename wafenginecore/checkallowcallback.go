// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafbot"
	"net/http"
	"net/url"
)

func (waf *WafEngine) CheckAllowCallBackIP(r *http.Request, weblogbean *innerbean.WebLog, formValue url.Values, hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{
		JumpGuardResult: false,
		IsBlock:         false,
		Title:           "",
		Content:         "",
	}
	clientIp := weblogbean.SRC_IP
	if global.GCONFIG_RECORD_PROXY_HEADER == "" {
		clientIp = weblogbean.NetSrcIp
	}
	backIpResult := wafbot.CheckCallBackIp(clientIp)
	if backIpResult.IsCallBack {
		result.JumpGuardResult = true
		weblogbean.GUEST_IDENTIFICATION = backIpResult.CallBackName
	}
	return result
}
