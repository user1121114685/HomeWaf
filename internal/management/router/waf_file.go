// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

type WafFileRouter struct {
}

func (receiver *WafFileRouter) InitWafFileRouter(group *gin.RouterGroup) {
	fileApi := api.APIGroupAPP.WafFileApi
	router := group.Group("/api/v1/file")
	{
		router.GET("/data_files", fileApi.GetDataFilesApi)
		router.GET("/delete_by_id", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), fileApi.DeleteFileByIdApi)
	}
}
