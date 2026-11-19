package api

import (
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafpipeline"
	"github.com/gin-gonic/gin"
	"strconv"
)

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
func (w *WafPipelineApi) PipelineDefenseCapabilities(c *gin.Context) {
	response.OkWithDetailed(waf_service.PipelineDefenseCapabilities(c.Query("ip_version")), "", c)
}
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
