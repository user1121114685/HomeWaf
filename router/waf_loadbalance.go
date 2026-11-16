// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type LoadBalanceRouter struct {
}

func (receiver *LoadBalanceRouter) InitLoadBalanceRouter(group *gin.RouterGroup) {
	loadBalanceApi := api.APIGroupAPP.WafLoadBalanceApi
	loadBalanceRouter := group.Group("")
	loadBalanceRouter.POST("/api/v1/wafhost/loadbalance/list", loadBalanceApi.GetListApi)
	loadBalanceRouter.GET("/api/v1/wafhost/loadbalance/detail", loadBalanceApi.GetDetailApi)
	loadBalanceRouter.POST("/api/v1/wafhost/loadbalance/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), loadBalanceApi.AddApi)
	loadBalanceRouter.GET("/api/v1/wafhost/loadbalance/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), loadBalanceApi.DelLoadBalanceApi)
	loadBalanceRouter.POST("/api/v1/wafhost/loadbalance/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), loadBalanceApi.ModifyApi)
}
