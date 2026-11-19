// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package router

import (
	"HomeWaf/api"

	"github.com/gin-gonic/gin"
)

type RuleRouter struct {
}

func (receiver *RuleRouter) InitRuleRouter(group *gin.RouterGroup) {
	ruleApi := api.APIGroupAPP.WafRuleAPi
	wafRuleRouter := group.Group("")
	wafRuleRouter.POST("/api/v1/wafhost/rule/format", ruleApi.FormatRuleApi)
	wafRuleRouter.POST("/api/v1/wafhost/rule/test", ruleApi.TestRuleApi)
}
