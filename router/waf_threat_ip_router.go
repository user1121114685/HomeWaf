// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"
	"HomeWaf/enums"
	"HomeWaf/middleware"

	"github.com/gin-gonic/gin"
)

type ThreatIPRouter struct {
}

func (receiver *ThreatIPRouter) InitThreatIPRouter(group *gin.RouterGroup) {
	apiObj := api.APIGroupAPP.WafThreatIPApi
	router := group.Group("")

	router.POST("/api/v1/threatip/channel/list", apiObj.GetListApi)                                                   // 渠道列表
	router.GET("/api/v1/threatip/channel/detail", apiObj.GetDetailApi)                                                // 渠道详情
	router.POST("/api/v1/threatip/channel/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), apiObj.AddApi)     // 新增渠道
	router.POST("/api/v1/threatip/channel/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), apiObj.ModifyApi) // 修改渠道
	router.GET("/api/v1/threatip/channel/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), apiObj.DelApi)      // 删除渠道
	router.POST("/api/v1/threatip/channel/sync", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), apiObj.SyncApi)   // 手动同步渠道

	router.GET("/api/v1/threatip/landed/summary", apiObj.LandedSummaryApi) // 落地汇总(订阅来源Tab)
	router.POST("/api/v1/threatip/landed/ips", apiObj.LandedIPsApi)        // 某渠道落地IP分页浏览(只读)

	// 误报排除名单：订阅源误把正常 IP 列为恶意、且用户无法在上游订正时的本地补救
	exclude := api.APIGroupAPP.WafThreatIPExcludeApi
	router.POST("/api/v1/threatip/exclude/list", exclude.GetListApi)                                                   // 排除名单列表
	router.POST("/api/v1/threatip/exclude/add", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), exclude.AddApi)     // 新增排除
	router.POST("/api/v1/threatip/exclude/edit", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), exclude.ModifyApi) // 改备注/启停
	router.GET("/api/v1/threatip/exclude/del", middleware.RequireRole(enums.ROLE_SECURITY_ADMIN), exclude.DelApi)      // 删除排除
	router.POST("/api/v1/threatip/exclude/preview", exclude.PreviewApi)                                                // 试算影响(不落库)
	router.POST("/api/v1/threatip/exclude/audit", exclude.GetAuditListApi)                                             // 操作审计流水
	router.GET("/api/v1/threatip/exclude/builtin", exclude.EffectiveRulesApi)                                          // 内置排除规则(只读，不落库)
}
