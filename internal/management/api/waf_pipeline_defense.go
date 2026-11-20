package api

import (
	_ "HomeWaf/internal/model" // Swagger response types.
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/protection/wafpipeline"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
	"strconv"
)

// @Summary Read a service's current defense status
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Param port query int true "Service port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=wafpipeline.DefenseStatus}
// @Router /api/v1/pipeline/defense [get]
func (w *WafPipelineApi) PipelineDefense(c *gin.Context) {
	port, ok := gatePort(c)
	if !ok {
		return
	}
	config, err := waf_service.WafPipelineServiceApp.Get(port)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	s := waf_service.WafPipelineDefenseServiceApp
	response.OkWithDetailed(s.Status(config, s.Ready()), "", c)
}
// @Summary Read the connection handling capabilities of this instance
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Param ip_version query string false "IP version to check"
// @Success 200 {object} response.Response
// @Router /api/v1/pipeline/defense/capabilities [get]
func (w *WafPipelineApi) PipelineDefenseCapabilities(c *gin.Context) {
	response.OkWithDetailed(waf_service.PipelineDefenseCapabilities(c.Query("ip_version")), "", c)
}
// @Summary Update a service's access state at the supplied revision
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body waf_service.DefenseAccessRequest true "Access state change"
// @Success 200 {object} response.Response{data=wafpipeline.PortConfig}
// @Router /api/v1/pipeline/ports/access [post]
func (w *WafPipelineApi) PipelineAccess(c *gin.Context) {
	var req waf_service.DefenseAccessRequest
	if !pipelineBind(c, &req) {
		return
	}
	config, err := waf_service.WafPipelineDefenseServiceApp.Access(req, pipelineOperator(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(config, "访问状态已更新", c)
}
// @Summary Read the shared honeypot settings and service bindings
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} response.Response
// @Router /api/v1/pipeline/honeypot [get]
func (w *WafPipelineApi) SharedPipelineHoneypot(c *gin.Context) {
	config, err := waf_service.WafPipelineHoneypotServiceApp.Get()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ports, err := waf_service.WafPipelineServiceApp.List()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	bound := make([]gin.H, 0)
	sources := make([]gin.H, 0, len(ports))
	for _, port := range ports {
		sources = append(sources, gin.H{"port": port.Port, "name": port.Name})
		if d := port.Policy.Defense; d.Enabled && d.Mode == wafpipeline.DefenseHoney {
			bound = append(bound, gin.H{"port": port.Port, "name": port.Name})
		}
	}
	response.OkWithDetailed(gin.H{"shared": config, "services": bound, "sources": sources}, "", c)
}
// @Summary Save the shared honeypot settings
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body wafpipeline.SharedHoneypotConfig true "Shared honeypot settings"
// @Success 200 {object} response.Response{data=wafpipeline.SharedHoneypotConfig}
// @Router /api/v1/pipeline/honeypot/save [post]
func (w *WafPipelineApi) SaveSharedPipelineHoneypot(c *gin.Context) {
	var req wafpipeline.SharedHoneypotConfig
	if !pipelineBind(c, &req) {
		return
	}
	value, err := waf_service.WafPipelineHoneypotServiceApp.Save(req, pipelineOperator(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(value, "统一蜜罐设置已保存", c)
}

// @Summary List recent honeypot events
// @Tags Pipeline
// @Security ApiKeyAuth
// @Produce json
// @Param port query int false "Filter by service port (zero means all ports)" minimum(0) maximum(65535)
// @Success 200 {object} response.Response{data=[]model.PipelineLog}
// @Router /api/v1/pipeline/honeypot/logs [get]
func (w *WafPipelineApi) SharedPipelineHoneypotLogs(c *gin.Context) {
	port, err := strconv.ParseUint(c.DefaultQuery("port", "0"), 10, 16)
	if err != nil {
		response.FailWithMessage("无效的服务端口", c)
		return
	}
	rows, err := waf_service.WafPipelineStateServiceApp.HoneypotLogs(uint16(port))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "", c)
}
