// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafHostPathRuleRouter struct{}

func (r *WafHostPathRuleRouter) InitWafHostPathRuleRouter(group *gin.RouterGroup) {
	a := api.APIGroupAPP.WafHostPathRuleApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/pathrule/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), a.AddApi)
	router.POST("/api/v1/wafhost/pathrule/list", a.GetListApi)
	router.GET("/api/v1/wafhost/pathrule/detail", a.GetDetailApi)
	router.POST("/api/v1/wafhost/pathrule/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), a.ModifyApi)
	router.GET("/api/v1/wafhost/pathrule/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), a.DelApi)
}
