package api

import (
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"github.com/gin-gonic/gin"
)

type PipelineMigrationPreviewRequest struct {
	Port       uint16 `json:"port"`
	PolicyHost string `json:"policy_host"`
}

// LegacyPipelinePorts previews existing site listeners without changing them.
// @Summary Preview legacy listeners available for migration
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} response.Response{data=[]waf_service.LegacyPortSummary}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /api/v1/pipeline/migration/legacy [get]
func (w *WafPipelineApi) LegacyPipelinePorts(c *gin.Context) {
	rows, err := waf_service.WafPipelineServiceApp.LegacyPorts()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "旧站点迁入预览", c)
}

// PreviewLegacyPipelinePort selects the shared policy for one legacy listener.
// @Summary Preview migration policy and source changes for one listener
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body PipelineMigrationPreviewRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=waf_service.LegacyPortPreview}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /api/v1/pipeline/migration/preview [post]
func (w *WafPipelineApi) PreviewLegacyPipelinePort(c *gin.Context) {
	var req PipelineMigrationPreviewRequest
	if !pipelineBind(c, &req) {
		return
	}
	row, err := waf_service.WafPipelineServiceApp.PreviewLegacyPort(req.Port, req.PolicyHost)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(row, "迁入尚未执行，监听保持原状", c)
}

// ImportLegacyPipelinePort persists copied policy, accounts and sealed secrets.
// @Summary Import a previewed legacy port without starting its new listener
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body waf_service.LegacyPortImportRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.PortConfig}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /api/v1/pipeline/migration/import [post]
func (w *WafPipelineApi) ImportLegacyPipelinePort(c *gin.Context) {
	var req waf_service.LegacyPortImportRequest
	if !pipelineBind(c, &req) {
		return
	}
	config, err := waf_service.WafPipelineServiceApp.ImportLegacyPort(req, pipelineOperator(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(config, "配置已迁入；新监听未启用，启用时将接管旧监听", c)
}
