// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafPrivateGroupRouter struct {
}

func (receiver *WafPrivateGroupRouter) InitWafPrivateGroupRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafPrivateGroupApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/privategroup/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/privategroup/list", api.GetListApi)
	router.POST("/api/v1/wafhost/privategroup/listbybelongcloud", api.GetListByBelongCloudApi)
	router.GET("/api/v1/wafhost/privategroup/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/privategroup/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/privategroup/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
}
