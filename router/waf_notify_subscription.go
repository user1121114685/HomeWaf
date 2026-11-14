// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type NotifySubscriptionRouter struct {
}

func (receiver *NotifySubscriptionRouter) InitNotifySubscriptionRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafNotifySubscriptionApi
	router := group.Group("")
	router.POST("/api/v1/notify/subscription/list", api.GetListApi)
	router.GET("/api/v1/notify/subscription/detail", api.GetDetailApi)
	router.POST("/api/v1/notify/subscription/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.GET("/api/v1/notify/subscription/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
	router.POST("/api/v1/notify/subscription/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)

	// 精细化配置与调试（issue #822）
	router.POST("/api/v1/notify/subscription/config", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.SaveConfigApi)
	router.POST("/api/v1/notify/subscription/batchconfig", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.BatchConfigApi)
	router.POST("/api/v1/notify/subscription/preview", api.PreviewApi)
	router.POST("/api/v1/notify/subscription/test", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.TestApi)
	router.POST("/api/v1/notify/subscription/dryrun", api.DryRunApi)
	router.GET("/api/v1/notify/subscription/templatevars", api.TemplateVarsApi)
	router.GET("/api/v1/notify/globalthrottle", api.GetGlobalThrottleApi)
	router.POST("/api/v1/notify/globalthrottle/update", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.UpdateGlobalThrottleApi)
}
