// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"

	"github.com/gin-gonic/gin"
)

type WafHttpAuthSessionRouter struct {
}

func (receiver *WafHttpAuthSessionRouter) InitWafHttpAuthSessionRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafHttpAuthSessionApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/httpauthsession/list", api.GetListApi)
	router.GET("/api/v1/wafhost/httpauthsession/kick", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.KickApi)
	router.POST("/api/v1/wafhost/httpauthsession/kickbyuser", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.KickByUserApi)
	router.POST("/api/v1/wafhost/httpauthsession/kickall", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.KickAllApi)
}
