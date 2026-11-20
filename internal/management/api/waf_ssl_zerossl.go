package api

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
)

func (w *WafSslOrderApi) ZeroSSLAccountStatus(c *gin.Context) {
	response.OkWithDetailed(waf_service.GetZeroSSLAccountStatus(), "获取成功", c)
}

func (w *WafSslOrderApi) SaveZeroSSLAccount(c *gin.Context) {
	var input waf_service.ZeroSSLAccountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.FailWithMessage("ZeroSSL 认证参数无效", c)
		return
	}
	if err := waf_service.SaveZeroSSLAccount(input); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(waf_service.GetZeroSSLAccountStatus(), "ZeroSSL 认证配置已保存，申请时校验有效性", c)
}
