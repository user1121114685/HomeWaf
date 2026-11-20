// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

type OneKeyModRouter struct {
}

func (receiver *OneKeyModRouter) InitOneKeyModRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafOneKeyModApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/onekeymod/list", api.GetListApi)
	router.GET("/api/v1/wafhost/onekeymod/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/onekeymod/doModify", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DoOneKeyModifyApi)
	router.GET("/api/v1/wafhost/onekeymod/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)
	router.GET("/api/v1/wafhost/onekeymod/restore", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.RestoreApi)
}
