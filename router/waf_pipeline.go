package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

func InitPipelineRouter(group *gin.RouterGroup) {
	r := group.Group("/api/v1/pipeline", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN))
	a := &api.WafPipelineApi{}
	r.GET("/ports", a.ListPorts)
	r.GET("/migration/legacy", a.LegacyPipelinePorts)
	r.POST("/migration/preview", a.PreviewLegacyPipelinePort)
	r.POST("/migration/import", a.ImportLegacyPipelinePort)
	r.GET("/overview", a.PipelineOverview)
	r.GET("/stats", a.PipelineStats)
	r.GET("/issues", a.PipelineIssues)
	r.GET("/state", a.PipelineState)
	r.POST("/state/manage", a.ManagePipelineState)
	r.GET("/logs", a.PipelineLogs)
	r.POST("/ports/save", a.SavePort)
	r.POST("/ports/check", a.CheckPipelinePort)
	r.POST("/ports/delete", a.DeletePort)
	r.POST("/ports/copy", a.CopyPort)
	r.GET("/templates", a.Templates)
	r.POST("/templates/save", a.SaveTemplate)
	r.POST("/templates/delete", a.DeleteTemplate)
	r.POST("/templates/preview", a.PreviewTemplate)
	r.POST("/templates/import-preview", a.PreviewTemplateImport)
	r.GET("/templates/export", a.ExportTemplate)
	r.POST("/upstream/test", a.TestUpstream)
	r.POST("/explain", a.ExplainPort)
	r.GET("/regions", a.PipelineRegions)
	r.GET("/feed", a.FeedConfig)
	r.POST("/feed/save", a.SaveFeed)
	r.POST("/feed/sync", a.SyncFeed)
	r.GET("/gate/users", a.GateUsers)
	r.POST("/gate/users/save", a.SaveGateUser)
	r.POST("/gate/users/delete", a.DeleteGateUser)
	r.GET("/gate/sessions", a.GateSessions)
	r.POST("/gate/sessions/kick", a.KickGateSession)
	r.POST("/gate/token/rotate", a.RotateGateToken)
}
