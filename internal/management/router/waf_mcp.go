package router

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/management/api"
	"HomeWaf/internal/management/mcp"
	"HomeWaf/internal/management/middleware"
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/platform/utils"
	"github.com/gin-gonic/gin"
)

func InitMCPRouter(engine *gin.Engine, admin *gin.RouterGroup) {
	operations := api.MCPOperations()
	handler, err := mcp.NewHandler(engine, operations)
	if err != nil {
		panic(err)
	}
	group := engine.Group("", middleware.MCPAuth(), middleware.IPWhitelist(), middleware.DomainWhitelist(), middleware.GinGlobalExceptionMiddleWare())
	for _, method := range []string{"GET", "POST", "DELETE"} {
		group.Handle(method, mcp.Endpoint, gin.WrapH(handler))
	}
	admin.GET("/api/v1/oplatform/mcp", func(c *gin.Context) {
		tools := make([]gin.H, 0, len(operations))
		for _, op := range operations {
			tools = append(tools, gin.H{"name": op.Name, "group": op.Group, "description": op.Description, "read_only": op.ReadOnly, "requires_confirmation": op.Confirm})
		}
		settings := global.CurrentSettings()
		response.OkWithData(gin.H{"endpoint": mcp.Endpoint, "transport": "Streamable HTTP", "enabled": settings.OpenPlatformEnabled(),
			"enabled_until": settings.OpenPlatformEnabledUntil, "suggested_source": utils.SuggestedMCPSource(c), "tools": tools}, c)
	})
}
