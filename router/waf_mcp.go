package router

import (
	"HomeWaf/api"
	"HomeWaf/global"
	"HomeWaf/middleware"
	"HomeWaf/model/common/response"
	"HomeWaf/wafmcp"
	"github.com/gin-gonic/gin"
)

func InitMCPRouter(engine *gin.Engine, admin *gin.RouterGroup) {
	operations := api.MCPOperations()
	handler, err := wafmcp.NewHandler(engine, operations)
	if err != nil {
		panic(err)
	}
	group := engine.Group("", middleware.MCPAuth(), middleware.IPWhitelist(), middleware.DomainWhitelist(), middleware.GinGlobalExceptionMiddleWare())
	for _, method := range []string{"GET", "POST", "DELETE"} {
		group.Handle(method, wafmcp.Endpoint, gin.WrapH(handler))
	}
	admin.GET("/api/v1/oplatform/mcp", func(c *gin.Context) {
		tools := make([]gin.H, 0, len(operations))
		for _, op := range operations {
			tools = append(tools, gin.H{"name": op.Name, "group": op.Group, "description": op.Description, "read_only": op.ReadOnly, "requires_confirmation": op.Confirm})
		}
		response.OkWithData(gin.H{"endpoint": wafmcp.Endpoint, "transport": "Streamable HTTP", "enabled": global.CurrentSettings().GCONFIG_OPEN_PLATFORM_ENABLED == 1, "tools": tools}, c)
	})
}
