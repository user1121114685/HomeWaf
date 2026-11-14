// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafCacheRuleRouter struct {
}

func (receiver *WafCacheRuleRouter) InitWafCacheRuleRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafCacheRuleApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/cacherule/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/cacherule/list", api.GetListApi)
	router.GET("/api/v1/wafhost/cacherule/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/cacherule/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/cacherule/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)
}
