// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package api

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
)

type WafEngineApi struct {
}

// ReloadServices reapplies the persisted policies of protected services.
func (w *WafEngineApi) ReloadServices(c *gin.Context) {
	if err := waf_service.WafPipelineServiceApp.Reload(); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("服务监听与防护配置已重新加载", c)
}
