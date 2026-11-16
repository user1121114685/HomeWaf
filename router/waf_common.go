// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafCommonRouter struct {
}

func (receiver *WafCommonRouter) InitWafCommonRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafCommonApi
	router := group.Group("")
	router.GET("/api/v1/export", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ExportExcelApi)
	router.POST("/api/v1/import", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ImportExcelApi)
	//心跳数据
	router.GET("/api/v1/heartbeat", api.HeartbeatApi)
}
