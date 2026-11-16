// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type SslOrderRouter struct {
}

func (receiver *SslOrderRouter) InitSslOrderRouter(group *gin.RouterGroup) {
	sslOrderApi := api.APIGroupAPP.WafSslOrderApi
	router := group.Group("")
	router.GET("/api/v1/wafhost/sslorder/zerossl/account", sslOrderApi.ZeroSSLAccountStatus)
	router.POST("/api/v1/wafhost/sslorder/zerossl/account", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.SaveZeroSSLAccount)
	router.GET("/api/v1/wafhost/sslorder/targets", sslOrderApi.CertificateTargets)
	router.GET("/api/v1/wafhost/sslorder/dns/providers", sslOrderApi.DNSProviders)
	router.GET("/api/v1/wafhost/sslorder/dns/credentials", sslOrderApi.DNSCredentialStatus)
	router.POST("/api/v1/wafhost/sslorder/list", sslOrderApi.GetListApi)
	router.GET("/api/v1/wafhost/sslorder/detail", sslOrderApi.GetDetailApi)
	router.GET("/api/v1/wafhost/sslorder/logs", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.LogsApi)
	router.POST("/api/v1/wafhost/sslorder/retry", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.RetryApi)
	router.POST("/api/v1/wafhost/sslorder/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.AddApi)
	router.GET("/api/v1/wafhost/sslorder/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.DelApi)
	router.POST("/api/v1/wafhost/sslorder/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), sslOrderApi.ModifyApi)
}
