package api

import (
	"HomeWaf/model/common/response"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

// Production handlers with an identifier-based request must reject empty input
// before reading a database, changing configuration, or sending a notification.
func TestBackendIdentifierEndpointsRejectEmptyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, method string
		handler      gin.HandlerFunc
	}{
		{"WafAccountApi.GetDetailApi", "GET", new(WafAccountApi).GetDetailApi},
		{"WafAccountApi.DelAccountApi", "GET", new(WafAccountApi).DelAccountApi},
		{"WafAccountApi.ModifyAccountApi", "POST", new(WafAccountApi).ModifyAccountApi},
		{"WafAccountLogApi.GetDetailApi", "GET", new(WafAccountLogApi).GetDetailApi},
		{"WafAllowIpApi.GetDetailApi", "GET", new(WafAllowIpApi).GetDetailApi},
		{"WafAllowIpApi.DelAllowIpApi", "GET", new(WafAllowIpApi).DelAllowIpApi},
		{"WafAllowIpApi.ModifyAllowIpApi", "POST", new(WafAllowIpApi).ModifyAllowIpApi},
		{"WafAllowUrlApi.GetDetailApi", "GET", new(WafAllowUrlApi).GetDetailApi},
		{"WafAllowUrlApi.DelAllowUrlApi", "GET", new(WafAllowUrlApi).DelAllowUrlApi},
		{"WafAllowUrlApi.ModifyAllowUrlApi", "POST", new(WafAllowUrlApi).ModifyAllowUrlApi},
		{"WafBatchTaskApi.GetBatchTaskDetailApi", "GET", new(WafBatchTaskApi).GetBatchTaskDetailApi},
		{"WafBatchTaskApi.DelBatchTaskApi", "GET", new(WafBatchTaskApi).DelBatchTaskApi},
		{"WafBatchTaskApi.ModifyBatchTaskApi", "POST", new(WafBatchTaskApi).ModifyBatchTaskApi},
		{"WafBlockIpApi.GetDetailApi", "GET", new(WafBlockIpApi).GetDetailApi},
		{"WafBlockIpApi.DelBlockIpApi", "GET", new(WafBlockIpApi).DelBlockIpApi},
		{"WafBlockIpApi.ModifyBlockIpApi", "POST", new(WafBlockIpApi).ModifyBlockIpApi},
		{"WafBlockUrlApi.GetDetailApi", "GET", new(WafBlockUrlApi).GetDetailApi},
		{"WafBlockUrlApi.DelBlockUrlApi", "GET", new(WafBlockUrlApi).DelBlockUrlApi},
		{"WafBlockUrlApi.ModifyBlockUrlApi", "POST", new(WafBlockUrlApi).ModifyBlockUrlApi},
		{"WafCacheRuleApi.GetDetailApi", "GET", new(WafCacheRuleApi).GetDetailApi},
		{"WafCacheRuleApi.DelApi", "GET", new(WafCacheRuleApi).DelApi},
		{"WafCacheRuleApi.ModifyApi", "POST", new(WafCacheRuleApi).ModifyApi},
		{"WafCaServerInfoApi.GetDetailApi", "GET", new(WafCaServerInfoApi).GetDetailApi},
		{"WafCaServerInfoApi.DelApi", "GET", new(WafCaServerInfoApi).DelApi},
		{"WafCaServerInfoApi.ModifyApi", "POST", new(WafCaServerInfoApi).ModifyApi},
		{"WafHttpAuthBaseApi.GetDetailApi", "GET", new(WafHttpAuthBaseApi).GetDetailApi},
		{"WafHttpAuthBaseApi.DelApi", "GET", new(WafHttpAuthBaseApi).DelApi},
		{"WafHttpAuthBaseApi.ModifyApi", "POST", new(WafHttpAuthBaseApi).ModifyApi},
		{"WafLdpUrlApi.GetDetailApi", "GET", new(WafLdpUrlApi).GetDetailApi},
		{"WafLdpUrlApi.DelLdpUrlApi", "GET", new(WafLdpUrlApi).DelLdpUrlApi},
		{"WafLdpUrlApi.ModifyLdpUrlApi", "POST", new(WafLdpUrlApi).ModifyLdpUrlApi},
		{"WafLoadBalanceApi.GetDetailApi", "GET", new(WafLoadBalanceApi).GetDetailApi},
		{"WafLoadBalanceApi.DelLoadBalanceApi", "GET", new(WafLoadBalanceApi).DelLoadBalanceApi},
		{"WafLoadBalanceApi.ModifyApi", "POST", new(WafLoadBalanceApi).ModifyApi},
		{"WafNotifyChannelApi.GetDetailApi", "GET", new(WafNotifyChannelApi).GetDetailApi},
		{"WafNotifyChannelApi.DelApi", "GET", new(WafNotifyChannelApi).DelApi},
		{"WafNotifyChannelApi.ModifyApi", "POST", new(WafNotifyChannelApi).ModifyApi},
		{"WafNotifyChannelApi.TestApi", "POST", new(WafNotifyChannelApi).TestApi},
		{"WafNotifyLogApi.GetDetailApi", "GET", new(WafNotifyLogApi).GetDetailApi},
		{"WafNotifyLogApi.DelApi", "GET", new(WafNotifyLogApi).DelApi},
		{"WafNotifySubscriptionApi.GetDetailApi", "GET", new(WafNotifySubscriptionApi).GetDetailApi},
		{"WafNotifySubscriptionApi.DelApi", "GET", new(WafNotifySubscriptionApi).DelApi},
		{"WafNotifySubscriptionApi.ModifyApi", "POST", new(WafNotifySubscriptionApi).ModifyApi},
		{"WafOneKeyModApi.GetDetailApi", "GET", new(WafOneKeyModApi).GetDetailApi},
		{"WafOneKeyModApi.DelApi", "GET", new(WafOneKeyModApi).DelApi},
		{"WafPrivateGroupApi.GetDetailApi", "GET", new(WafPrivateGroupApi).GetDetailApi},
		{"WafPrivateGroupApi.DelApi", "GET", new(WafPrivateGroupApi).DelApi},
		{"WafPrivateGroupApi.ModifyApi", "POST", new(WafPrivateGroupApi).ModifyApi},
		{"WafPrivateInfoApi.GetDetailApi", "GET", new(WafPrivateInfoApi).GetDetailApi},
		{"WafPrivateInfoApi.DelApi", "GET", new(WafPrivateInfoApi).DelApi},
		{"WafRuleAPi.GetDetailApi", "GET", new(WafRuleAPi).GetDetailApi},
		{"WafRuleAPi.DelRuleApi", "GET", new(WafRuleAPi).DelRuleApi},
		{"WafRuleAPi.ModifyRuleStatusApi", "GET", new(WafRuleAPi).ModifyRuleStatusApi},
		{"WafSensitiveApi.GetDetailApi", "GET", new(WafSensitiveApi).GetDetailApi},
		{"WafSensitiveApi.DelSensitiveApi", "GET", new(WafSensitiveApi).DelSensitiveApi},
		{"WafSslConfigApi.GetSslConfigDetailApi", "GET", new(WafSslConfigApi).GetSslConfigDetailApi},
		{"WafSslConfigApi.DelSslConfigApi", "GET", new(WafSslConfigApi).DelSslConfigApi},
		{"WafSslConfigApi.ModifySslConfigApi", "POST", new(WafSslConfigApi).ModifySslConfigApi},
		{"WafSslExpireApi.GetDetailApi", "GET", new(WafSslExpireApi).GetDetailApi},
		{"WafSslExpireApi.DelApi", "GET", new(WafSslExpireApi).DelApi},
		{"WafSslExpireApi.ModifyApi", "POST", new(WafSslExpireApi).ModifyApi},
		{"WafSslOrderApi.GetDetailApi", "GET", new(WafSslOrderApi).GetDetailApi},
		{"WafSslOrderApi.DelApi", "GET", new(WafSslOrderApi).DelApi},
		{"WafSslOrderApi.ModifyApi", "POST", new(WafSslOrderApi).ModifyApi},
		{"WafSystemConfigApi.GetDetailApi", "GET", new(WafSystemConfigApi).GetDetailApi},
		{"WafSystemConfigApi.DelApi", "GET", new(WafSystemConfigApi).DelApi},
		{"WafSystemConfigApi.ModifyApi", "POST", new(WafSystemConfigApi).ModifyApi},
		{"WafSysLogApi.GetDetailApi", "GET", new(WafSysLogApi).GetDetailApi},
		{"WafTamperRuleApi.GetDetailApi", "GET", new(WafTamperRuleApi).GetDetailApi},
		{"WafTamperRuleApi.DelApi", "GET", new(WafTamperRuleApi).DelApi},
		{"WafTamperRuleApi.ModifyApi", "POST", new(WafTamperRuleApi).ModifyApi},
		{"WafTaskApi.GetDetailApi", "GET", new(WafTaskApi).GetDetailApi},
		{"WafTaskApi.DelApi", "GET", new(WafTaskApi).DelApi},
		{"WafTaskApi.ModifyApi", "POST", new(WafTaskApi).ModifyApi},
		{"WafTaskApi.ManualExecuteApi", "GET", new(WafTaskApi).ManualExecuteApi},
		{"WafTunnelApi.GetDetailApi", "GET", new(WafTunnelApi).GetDetailApi},
		{"WafTunnelApi.DelApi", "GET", new(WafTunnelApi).DelApi},
		{"WafTunnelApi.ModifyApi", "POST", new(WafTunnelApi).ModifyApi},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set("is_openapi", true)
			c.Request = httptest.NewRequest(tc.method, "/", strings.NewReader("{}"))
			c.Request.Header.Set("Content-Type", "application/json")
			tc.handler(c)
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != response.ERROR {
				t.Fatalf("empty identifier did not fail before business processing: HTTP %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	t.Logf("verified %d production identifier handlers", len(cases))
}
