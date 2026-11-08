// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafLicenseRouter struct {
}

func (receiver *WafLicenseRouter) InitLicenseRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafLicenseApi
	router := group.Group("").Use(middleware.RequireRole())
	router.GET("/api/v1/license/detail", api.GetDetailApi)
	router.POST("/api/v1/license/checklicense", api.CheckLicense)
	router.POST("/api/v1/license/confirm", api.ConfirmApi)
}
