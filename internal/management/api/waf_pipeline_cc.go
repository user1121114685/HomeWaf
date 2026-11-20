package api

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
)

// RecommendPipelineCC estimates thresholds for a native service without saving.
// @Summary Estimate a service CC threshold
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body waf_service.PipelineCCRecommendRequest true "Service and draft counting options"
// @Success 200 {object} response.Response
// @Router /api/v1/pipeline/cc/recommend [post]
func (w *WafPipelineApi) RecommendPipelineCC(c *gin.Context) {
	var req waf_service.PipelineCCRecommendRequest
	if !pipelineBind(c, &req) {
		return
	}
	result, err := waf_service.WafPipelineServiceApp.RecommendCC(c.Request.Context(), req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(result, "估算完成，未保存配置", c)
}

// PipelineCCHits returns a bounded recent history, never a lifetime total.
// @Summary Recent service CC hits
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Param port query int true "Service port"
// @Success 200 {object} response.Response
// @Router /api/v1/pipeline/cc/hits [get]
func (w *WafPipelineApi) PipelineCCHits(c *gin.Context) {
	port, ok := gatePort(c)
	if !ok {
		return
	}
	rows, err := waf_service.WafPipelineStateServiceApp.CCHits(port)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "当前日志分区最近 200 条 CC 命中，受日志保留和队列状态影响", c)
}
