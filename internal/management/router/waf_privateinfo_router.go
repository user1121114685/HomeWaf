// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

type WafPrivateInfoRouter struct {
}

func (receiver *WafPrivateInfoRouter) InitWafPrivateInfoRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafPrivateInfoApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/privateinfo/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/privateinfo/list", api.GetListApi)
	router.GET("/api/v1/wafhost/privateinfo/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/privateinfo/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/privateinfo/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
}
