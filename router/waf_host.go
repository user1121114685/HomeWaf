// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"github.com/gin-gonic/gin"
)

type HostRouter struct {
}

func (receiver *HostRouter) InitHostRouter(group *gin.RouterGroup) {
	hostApi := api.APIGroupAPP.WafHostAPi
	hostRouter := group.Group("")
	hostRouter.POST("/api/v1/wafhost/host/list", hostApi.GetListApi)
	hostRouter.GET("/api/v1/wafhost/host/detail", hostApi.GetDetailApi)
	hostRouter.POST("/api/v1/wafhost/host/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.AddApi)
	hostRouter.GET("/api/v1/wafhost/host/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.DelHostApi)
	hostRouter.POST("/api/v1/wafhost/host/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.ModifyHostApi)
	hostRouter.GET("/api/v1/wafhost/host/guardstatus", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.ModifyGuardStatusApi)
	hostRouter.GET("/api/v1/wafhost/host/startstatus", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.ModifyStartStatusApi)
	hostRouter.GET("/api/v1/wafhost/host/allhost", hostApi.GetAllListApi)
	hostRouter.GET("/api/v1/wafhost/host/alldomainbyhostcode", hostApi.GetDomainsByHostCodeApi)
	hostRouter.POST("/api/v1/wafhost/host/modfiyallstatus", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.ModifyAllGuardStatusApi)
	hostRouter.POST("/api/v1/wafhost/host/batchcopyconfig", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.BatchCopyConfigApi)
	hostRouter.GET("/api/v1/wafhost/host/ipsource/probe", hostApi.IPSourceProbeApi)                                                               //查看最近到达的真实请求头(排查真实IP来源)
	hostRouter.GET("/api/v1/wafhost/host/ipsource/probe/clear", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.IPSourceProbeClearApi) //清空采样
	hostRouter.POST("/api/v1/wafhost/host/health/test", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.TestHealthApi)
	hostRouter.POST("/api/v1/wafhost/host/diagnostic", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), hostApi.DiagnosticApi)
	hostRouter.POST("/api/v1/wafhost/host/checkports", hostApi.CheckPortsApi)   //保存前预检端口监听表(issue #955)
	hostRouter.GET("/api/v1/wafhost/port/overview", hostApi.GetPortOverviewApi) //端口占用总览

}
