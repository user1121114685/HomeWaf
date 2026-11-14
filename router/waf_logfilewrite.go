// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type LogFileWriteRouter struct {
}

func (receiver *LogFileWriteRouter) InitLogFileWriteRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafLogFileWriteApi
	router := group.Group("")
	router.GET("/api/v1/logfilewrite/preview", api.GetPreviewApi)
	router.GET("/api/v1/logfilewrite/currentfile", api.GetCurrentFileInfoApi)
	router.GET("/api/v1/logfilewrite/backupfiles", api.GetBackupFilesApi)
	router.POST("/api/v1/logfilewrite/clear", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ClearLogFileApi)
	router.GET("/api/v1/logfilewrite/variables", api.GetTemplateVariablesApi)
}
