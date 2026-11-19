// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package api

import (
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"github.com/gin-gonic/gin"
)

func LogServices(c *gin.Context) {
	options, err := waf_service.WafHostServiceApp.GetLogServiceOptions()
	if err != nil {
		response.FailWithMessage("读取服务列表失败", c)
		return
	}
	response.OkWithData(options, c)
}
