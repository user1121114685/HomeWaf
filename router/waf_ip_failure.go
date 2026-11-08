// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"

	"github.com/gin-gonic/gin"
)

type IPFailureRouter struct {
}

func (receiver *IPFailureRouter) InitIPFailureRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafIPFailureApi
	router := group.Group("")
	router.GET("/api/v1/wafhost/ipfailure/config", api.GetConfigApi)
	router.POST("/api/v1/wafhost/ipfailure/config", api.SetConfigApi)
	router.GET("/api/v1/wafhost/ipfailure/baniplist", api.GetBanIpListApi)
	router.POST("/api/v1/wafhost/ipfailure/removebanip", api.RemoveIPFailureBanIPApi)
}
