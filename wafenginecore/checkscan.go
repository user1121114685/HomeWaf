// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/libinjection-go"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"net/http"
	"net/url"
)

/*
*
检测扫描工具
*/
func (waf *WafEngine) CheckSan(r *http.Request, weblogbean *innerbean.WebLog, formValue url.Values, hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{
		JumpGuardResult: false,
		IsBlock:         false,
		Title:           "",
		Content:         "",
	}
	var scanFlag = false
	if libinjection.IsScan(weblogbean) {
		scanFlag = true
	}
	if scanFlag == true {
		weblogbean.RISK_LEVEL = 1

		result.IsBlock = true
		result.Title = "扫描工具"
		result.Content = "请正确访问"
		return result
	}
	return result
}
