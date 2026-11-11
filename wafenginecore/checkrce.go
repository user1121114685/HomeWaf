// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafdefenserce"
	"HomeWaf/wafenginecore/wafhttpcore"
	"net/http"
	"net/url"
)

/*
*
检测Rce
*/
func (waf *WafEngine) CheckRce(r *http.Request, weblogbean *innerbean.WebLog, formValue url.Values, hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{
		JumpGuardResult: false,
		IsBlock:         false,
		Title:           "",
		Content:         "",
	}
	// URL and cookies retain their configured legacy checks. Every request-body
	// source below uses the same off/observe/block gate.
	isRce, RceName := wafdefenserce.DetermineRCE(weblogbean.RawQuery, weblogbean.URL, weblogbean.COOKIES)
	if isRce == true {
		weblogbean.RISK_LEVEL = 3
		result.IsBlock = true
		result.Title = "RCE:" + RceName
		result.Content = "请正确访问"
		return result
	}
	// 请求体深度检测（S1 新增：cookie/body 解码副本 + JSON 逐值）：受 body_detect_mode 门控
	if bodyDetectEnabled(r) {
		hit, name := wafdefenserce.DetermineRCE(weblogbean.CookiesDecoded)
		if !hit && !wafhttpcore.IsJSONBody(weblogbean.BODY) && len(formValue) == 0 {
			hit, name = wafdefenserce.DetermineRCE(weblogbean.BodyDecoded, weblogbean.BODY)
		}
		if !hit {
			for key, values := range formValue {
				if isBodyFieldExcluded(key, r) {
					continue
				}
				for _, v := range values {
					if ok, n := wafdefenserce.DetermineRCE(v); ok {
						hit, name = ok, n
						break
					}
				}
				if hit {
					break
				}
			}
		}
		if hit == false {
			for i, v := range weblogbean.BodyValues {
				if isBodyFieldExcluded(fieldAt(weblogbean.BodyFields, i), r) { // E
					continue
				}
				if ok, n := wafdefenserce.DetermineRCE(v); ok {
					hit, name = ok, n
					break
				}
			}
		}
		if hit == false { // 头位命令注入(S1-4)
			hit, name = headerRCEHit(r)
		}
		if hit {
			if bodyDetectBlocking(r) {
				weblogbean.RISK_LEVEL = 3
				result.IsBlock = true
				result.Title = "RCE:" + name
				result.Content = "请正确访问"
				return result
			}
			markBodyObserve(weblogbean, "RCE:"+name)
		}
	}
	return result
}
