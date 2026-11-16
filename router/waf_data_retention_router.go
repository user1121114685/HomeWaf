// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafDataRetentionRouter struct{}

func (receiver *WafDataRetentionRouter) InitWafDataRetentionRouter(group *gin.RouterGroup) {
	wafApi := api.APIGroupAPP.WafDataRetentionApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/dataretention/list", wafApi.GetListApi)
	router.GET("/api/v1/wafhost/dataretention/detail", wafApi.GetDetailApi)
	router.POST("/api/v1/wafhost/dataretention/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), wafApi.ModifyApi)
}
