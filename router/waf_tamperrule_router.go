// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafTamperRuleRouter struct {
}

func (receiver *WafTamperRuleRouter) InitWafTamperRuleRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafTamperRuleApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/tamperrule/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/tamperrule/list", api.GetListApi)
	router.GET("/api/v1/wafhost/tamperrule/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/tamperrule/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/tamperrule/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)
	router.GET("/api/v1/wafhost/tamperrule/relearn", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.RelearnApi)
	router.POST("/api/v1/wafhost/tamperrule/relearnbatch", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.RelearnBatchApi)
	router.POST("/api/v1/wafhost/tamperrule/extract", api.ExtractUrlsApi)
	router.POST("/api/v1/wafhost/tamperrule/addbatch", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddBatchApi)
	router.POST("/api/v1/wafhost/tamperrule/delbatch", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelBatchApi)
	router.GET("/api/v1/wafhost/tamperrule/baseline", api.GetBaselineApi)
}
