// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafTunnelRouter struct {
}

func (receiver *WafTunnelRouter) InitWafTunnelRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafTunnelApi
	router := group.Group("")
	router.POST("/api/v1/tunnel/tunnel/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.POST("/api/v1/tunnel/tunnel/list", api.GetListApi)
	router.GET("/api/v1/tunnel/tunnel/detail", api.GetDetailApi)
	router.POST("/api/v1/tunnel/tunnel/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.GET("/api/v1/tunnel/tunnel/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
	router.GET("/api/v1/tunnel/tunnel/connections", api.GetTunnelConnectionsApi)
}
