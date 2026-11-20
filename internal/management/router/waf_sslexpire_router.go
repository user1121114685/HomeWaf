// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

type WafSslExpireRouter struct {
}

func (receiver *WafSslExpireRouter) InitWafSslExpireRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafSslExpireApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/sslexpire/add", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.AddApi)
	router.POST("/api/v1/wafhost/sslexpire/list", api.GetListApi)
	router.GET("/api/v1/wafhost/sslexpire/detail", api.GetDetailApi)
	router.POST("/api/v1/wafhost/sslexpire/edit", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.ModifyApi)
	router.GET("/api/v1/wafhost/sslexpire/del", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.DelApi)
	router.GET("/api/v1/wafhost/sslexpire/nowcheck", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.NowCheckExpireApi)
	router.GET("/api/v1/wafhost/sslexpire/sync_host", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), api.SyncHostApi)
}
