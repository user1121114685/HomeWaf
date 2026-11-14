// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafCaServerInfoRouter struct {
}

func (receiver *WafCaServerInfoRouter) InitWafCaServerInfoRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafCaServerInfoApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/caserverinfo/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/caserverinfo/list", api.GetListApi)
	router.GET("/api/v1/wafhost/caserverinfo/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/caserverinfo/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/caserverinfo/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
}
