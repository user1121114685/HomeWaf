// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type WafHttpAuthBaseRouter struct {
}

func (receiver *WafHttpAuthBaseRouter) InitWafHttpAuthBaseRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafHttpAuthBaseApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/httpauthbase/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/httpauthbase/list", api.GetListApi)
	router.GET("/api/v1/wafhost/httpauthbase/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/httpauthbase/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/httpauthbase/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)
}
