// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type IPLocationRouter struct {
}

func (receiver *IPLocationRouter) InitIPLocationRouter(group *gin.RouterGroup) {
	apiInstance := api.APIGroupAPP.WafIPLocationApi
	router := group.Group("/api/v1/iplocation")
	{
		router.GET("/status", apiInstance.GetIPDBStatusApi)
		router.GET("/config", apiInstance.GetIPDBConfigApi)
		router.POST("/config/save", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), apiInstance.SaveIPDBConfigApi)
		router.POST("/upload", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), apiInstance.UploadIPDBFileApi)
		router.POST("/reload", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), apiInstance.ReloadIPDBApi)
		router.POST("/test", apiInstance.TestIPLookupApi)
		router.GET("/upgrade/check", apiInstance.CheckIPDBUpgradeApi)
		router.POST("/upgrade/apply", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), apiInstance.ApplyIPDBUpgradeApi)
		router.GET("/upgrade/progress", apiInstance.GetIPDBUpgradeProgressApi)
		router.POST("/upgrade/cancel", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), apiInstance.CancelIPDBUpgradeApi)
	}
}
