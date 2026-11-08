// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"github.com/gin-gonic/gin"
)

type WebSocketRouter struct {
}

func (receiver *WebSocketRouter) InitWebSocketRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafWebSocketApi
	router := group.Group("")
	router.GET("/api/v1/ws", api.WebSocketMessageApi)
}
