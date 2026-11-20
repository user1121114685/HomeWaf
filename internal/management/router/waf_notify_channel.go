// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

type NotifyChannelRouter struct {
}

func (receiver *NotifyChannelRouter) InitNotifyChannelRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafNotifyChannelApi
	router := group.Group("")
	router.GET("/api/v1/notify/channel/types", api.GetTypesApi)
	router.POST("/api/v1/notify/channel/list", api.GetListApi)
	router.GET("/api/v1/notify/channel/detail", api.GetDetailApi)
	router.POST("/api/v1/notify/channel/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.GET("/api/v1/notify/channel/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
	router.POST("/api/v1/notify/channel/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.POST("/api/v1/notify/channel/test", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.TestApi)
}
