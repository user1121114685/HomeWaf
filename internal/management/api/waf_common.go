// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package api

import (
	"HomeWaf/internal/model/common/response"
	"github.com/gin-gonic/gin"
)

type WafCommonApi struct {
}

// HeartbeatApi 心跳数据
func (w *WafCommonApi) HeartbeatApi(c *gin.Context) {
	if c.GetBool("is_openapi") {
		response.Ok(c)
		return
	}
	response.OkWithData(gin.H{"role": c.GetString("userRole"), "login_account": c.GetString("loginAccount")}, c)
}
