// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"

	"github.com/gin-gonic/gin"
)

type FirewallIPBlockRouter struct {
}

func (receiver *FirewallIPBlockRouter) InitFirewallIPBlockRouter(group *gin.RouterGroup) {
	api := api.APIGroupAPP.WafFirewallIPBlockApi
	router := group.Group("")

	// 基础CRUD
	router.POST("/api/v1/firewall/ipblock/list", api.GetListApi)                                                   // 获取列表
	router.GET("/api/v1/firewall/ipblock/detail", api.GetDetailApi)                                                // 获取详情
	router.POST("/api/v1/firewall/ipblock/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.AddApi)     // 添加
	router.GET("/api/v1/firewall/ipblock/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DelApi)      // 删除
	router.POST("/api/v1/firewall/ipblock/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ModifyApi) // 编辑

	// 批量操作
	router.POST("/api/v1/firewall/ipblock/batch/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.BatchAddApi) // 批量添加
	router.POST("/api/v1/firewall/ipblock/batch/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.BatchDelApi) // 批量删除

	// 启用/禁用
	router.POST("/api/v1/firewall/ipblock/enable", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.EnableApi)   // 启用
	router.POST("/api/v1/firewall/ipblock/disable", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.DisableApi) // 禁用

	// 高级功能
	router.POST("/api/v1/firewall/ipblock/sync", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.SyncApi)                  // 同步规则
	router.POST("/api/v1/firewall/ipblock/clear/expired", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), api.ClearExpiredApi) // 清理过期
	router.GET("/api/v1/firewall/ipblock/statistics", api.GetStatisticsApi)                                                       // 统计信息
	router.GET("/api/v1/firewall/ipblock/capability", api.GetCapabilityApi)                                                       // 当前环境防火墙能力
}
