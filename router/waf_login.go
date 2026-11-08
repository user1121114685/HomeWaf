// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"github.com/gin-gonic/gin"
)

type LoginRouter struct {
}
type LoginOutRouter struct {
}

func (receiver *LoginRouter) InitLoginRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafLoginApi
	router := group.Group("")
	router.POST("/api/v1/public/login", api.LoginApi)
}
func (receiver *LoginOutRouter) InitLoginOutRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafLoginApi
	router := group.Group("")
	router.POST("/api/v1/logout", api.LoginOutApi)
}
