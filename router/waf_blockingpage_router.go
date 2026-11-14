// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafBlockingPageRouter struct {
}

func (receiver *WafBlockingPageRouter) InitWafBlockingPageRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafBlockingPageApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/blockingpage/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/blockingpage/list", api.GetListApi)
	router.GET("/api/v1/wafhost/blockingpage/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/blockingpage/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/blockingpage/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)
	router.GET("/api/v1/wafhost/blockingpage/unmatchedbuiltin", api.GetUnmatchedBuiltinApi)
}
