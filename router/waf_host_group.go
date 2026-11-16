// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"

	"github.com/gin-gonic/gin"
)

type HostGroupRouter struct {
}

func (receiver *HostGroupRouter) InitHostGroupRouter(group *gin.RouterGroup) {
	hostGroupApi := api.APIGroupAPP.WafHostGroupApi
	router := group.Group("")
	router.POST("/api/v1/wafhost/hostgroup/list", hostGroupApi.GetListApi)
	router.GET("/api/v1/wafhost/hostgroup/all", hostGroupApi.GetAllApi) //不分页，供网站列表左栏与表单下拉
	router.GET("/api/v1/wafhost/hostgroup/detail", hostGroupApi.GetDetailApi)
	router.POST("/api/v1/wafhost/hostgroup/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostGroupApi.AddApi)
	router.POST("/api/v1/wafhost/hostgroup/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostGroupApi.ModifyApi)
	router.GET("/api/v1/wafhost/hostgroup/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostGroupApi.DelApi)
	router.POST("/api/v1/wafhost/hostgroup/sort", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostGroupApi.SortApi)
	router.POST("/api/v1/wafhost/hostgroup/assign", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostGroupApi.AssignApi) //批量移动网站到分组
}
