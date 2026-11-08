// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"

	"github.com/gin-gonic/gin"
)

type EngineRouter struct {
}

func (receiver *EngineRouter) InitEngineRouter(group *gin.RouterGroup) {
	engineApi := api.APIGroupAPP.WafEngineApi
	wafEngineRouter := group.Group("")
	// N7：重启/重载 WAF 引擎属高危系统操作，仅系统管理员(或超管)可执行
	wafEngineRouter.GET("/api/v1/resetWAF", middleware.RequireRole(enums.ROLE_SYSTEM_ADMIN), engineApi.ResetWaf)

}
