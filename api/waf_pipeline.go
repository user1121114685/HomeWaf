package api

import (
	"HomeWaf/global"
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafenginecore"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type WafPipelineApi struct{}

// PipelineOverview returns port configuration and bounded runtime summaries.
// @Summary PipelineOverview returns port configuration and bounded runtime summaries.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/overview [get]
func (w *WafPipelineApi) PipelineOverview(c *gin.Context) {
	configs, err := waf_service.WafPipelineServiceApp.List()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	states := waf_service.WafPipelineStateServiceApp.Punisher.Records("")
	local := map[string]map[string]int{}
	globalBans := 0
	for _, r := range states {
		if r.Scope == "global" && r.Kind == "ban" {
			globalBans++
			continue
		}
		if local[r.PortID] == nil {
			local[r.PortID] = map[string]int{}
		}
		local[r.PortID][r.Kind]++
	}
	rows := make([]gin.H, 0, len(configs))
	for _, config := range configs {
		port := wafpipeline.PortID(config.Port)
		stats := map[string]uint64{}
		if s, ok := wafpipeline.Ports.Get(config.Port); ok && s.Compiled != nil && s.Compiled.Metrics != nil {
			stats = s.Compiled.Metrics.Stats()
		}
		counts := local[port]
		rows = append(rows, gin.H{"config": config, "stats": stats, "ban_count": globalBans + counts["ban"], "honeypot_count": counts["honeypot"], "locked": counts["lockdown"] > 0, "listener": wafpipeline.Listeners.Status(config.Port)})
	}
	data := gin.H{"ports": rows, "issues": waf_service.WafPipelineServiceApp.Issues(), "punishment": waf_service.WafPipelineStateServiceApp.Punisher.Stats()}
	if global.GQEQUE_LOG_DB != nil {
		data["log_queue"] = global.GQEQUE_LOG_DB.Stats()
	}
	response.OkWithDetailed(data, "", c)
}

// CheckPipelinePort checks new-port identity and socket conflicts without saving.
// @Summary CheckPipelinePort checks new-port identity and socket conflicts without saving.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body wafpipeline.PortConfig true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/ports/check [post]
func (w *WafPipelineApi) CheckPipelinePort(c *gin.Context) {
	var config wafpipeline.PortConfig
	if !pipelineBind(c, &config) {
		return
	}
	if err := waf_service.WafPipelineServiceApp.CheckNewPort(config); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("检查通过；创建时仍会再次校验", c)
}

// PreviewTemplateImport validates all template data without saving or applying it.
// @Summary PreviewTemplateImport validates all template data without saving or applying it.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body wafpipeline.Template true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.Template} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates/import-preview [post]
func (w *WafPipelineApi) PreviewTemplateImport(c *gin.Context) {
	var tpl wafpipeline.Template
	if !pipelineBind(c, &tpl) {
		return
	}
	tpl.Builtin = false
	if err := wafpipeline.ValidateTemplate(tpl); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(tpl, "模板有效，尚未导入", c)
}

// PipelineIssues lists isolated port configuration errors.
// @Summary PipelineIssues lists isolated port configuration errors.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/issues [get]
func (w *WafPipelineApi) PipelineIssues(c *gin.Context) {
	response.OkWithDetailed(waf_service.WafPipelineServiceApp.Issues(), "", c)
}

// PipelineStats returns runtime and pre-handler protocol counters.
// @Summary PipelineStats returns runtime and pre-handler protocol counters.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/stats [get]
func (w *WafPipelineApi) PipelineStats(c *gin.Context) {
	p, ok := gatePort(c)
	if !ok {
		return
	}
	data := gin.H{"runtime": wafpipeline.Live.Stats(), "protocol": wafpipeline.Listeners.ProtocolStats(p)}
	data["tamper_capture"] = waf_service.WafPipelineTamperServiceApp.Stats()
	data["legacy_tamper_jobs"] = wafenginecore.LegacyTamperTaskStats()
	if wafpipeline.Live.PluginStats != nil {
		data["plugin_pool"] = wafpipeline.Live.PluginStats()
	}
	data["daily_persist_errors"] = waf_service.WafPipelineStateServiceApp.MetricsPersistErrors.Load()
	data["punishment"] = waf_service.WafPipelineStateServiceApp.Punisher.Stats()
	if global.GQEQUE_LOG_DB != nil {
		data["log_queue"] = global.GQEQUE_LOG_DB.Stats()
	}
	if wafpipeline.Live.Admission != nil {
		data["admission"] = wafpipeline.Live.Admission.Stats()
	}
	if s, ok := wafpipeline.Ports.Get(p); ok && s.Compiled != nil {
		if s.Compiled.Metrics != nil {
			data["port"] = s.Compiled.Metrics.Stats()
			data["last_result"] = s.Compiled.Metrics.Last.Load()
		}
		if s.Compiled.Proxy != nil {
			total, failed := s.Compiled.Proxy.Health.Snapshot()
			data["upstream"] = gin.H{"requests_60s": total, "failures_60s": failed, "transport_errors": s.Compiled.Proxy.Errors.Load()}
		}
	}
	response.OkWithDetailed(data, "连接层统计不直接触发IP处罚", c)
}

func pipelineBind(c *gin.Context, out any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err == nil {
		err = wafpipeline.StrictDecode(data, out)
	}
	if err != nil {
		response.FailWithMessage("请求格式或字段不合法: "+err.Error(), c)
		return false
	}
	return true
}

// PipelineState lists active bans, honeypots and lockdowns for a port.
// @Summary PipelineState lists active bans, honeypots and lockdowns for a port.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=[]wafpipeline.PunishmentRecord} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/state [get]
func (w *WafPipelineApi) PipelineState(c *gin.Context) {
	p, ok := gatePort(c)
	if !ok {
		return
	}
	response.OkWithDetailed(waf_service.WafPipelineStateServiceApp.Punisher.Records(wafpipeline.PortID(p)), "", c)
}

// ManagePipelineState releases, makes permanent, or whitelists a captured IP.
// @Summary ManagePipelineState releases, makes permanent, or whitelists a captured IP.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineStateRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/state/manage [post]
func (w *WafPipelineApi) ManagePipelineState(c *gin.Context) {
	var req PipelineStateRequest
	if !pipelineBind(c, &req) {
		return
	}
	if req.Port == 0 || len(req.ID) > 200 {
		response.FailWithMessage("端口或记录不合法", c)
		return
	}
	if err := waf_service.WafPipelineStateServiceApp.Manage(req.Port, req.ID, req.Action, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("状态已更新", c)
}

// PipelineLogs returns bounded recent request traces and punishment changes.
// @Summary PipelineLogs returns bounded recent request traces and punishment changes.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=[]model.PipelineLog} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/logs [get]
func (w *WafPipelineApi) PipelineLogs(c *gin.Context) {
	p, ok := gatePort(c)
	if !ok {
		return
	}
	rows, err := waf_service.WafPipelineStateServiceApp.Logs(p)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "", c)
}

// ExplainPort evaluates the real policy without counters, sessions or punishments.
// @Summary ExplainPort evaluates the real policy without counters, sessions or punishments.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineExplainRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.Result} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/explain [post]
func (w *WafPipelineApi) ExplainPort(c *gin.Context) {
	var input PipelineExplainRequest
	if !pipelineBind(c, &input) {
		return
	}
	if len(input.Body) > 65536 || len(input.Headers) > 64 || len(input.Path) > 8192 || !strings.HasPrefix(input.Path, "/") {
		response.FailWithMessage("样例超过限制或路径不合法", c)
		return
	}
	ip, err := netip.ParseAddr(input.ClientIP)
	if err != nil {
		response.FailWithMessage("来源IP不合法", c)
		return
	}
	if input.Method == "" {
		input.Method = "GET"
	}
	req, err := http.NewRequest(input.Method, "http://preview.invalid"+input.Path, strings.NewReader(input.Body))
	if err != nil {
		response.FailWithMessage("请求样例不合法", c)
		return
	}
	if len(input.Config.HostNames) > 0 {
		host := input.Config.HostNames[0]
		if strings.HasPrefix(host, "*.") {
			host = "preview" + host[1:]
		}
		req.Host = net.JoinHostPort(host, strconv.Itoa(int(input.Config.Port)))
	}
	for k, v := range input.Headers {
		if strings.EqualFold(k, "Host") {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	req.RemoteAddr = net.JoinHostPort(ip.String(), "12345")
	snapshot := &wafpipeline.Snapshot{Config: input.Config}
	if current, ok := wafpipeline.Ports.Get(input.Config.Port); ok {
		snapshot.Secrets = current.Secrets
	}
	result, err := wafpipeline.Live.Explain(snapshot, req, ip)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(result, "模拟不会产生处罚", c)
}

// PipelineRegions enumerates values from the active local databases.
// @Summary PipelineRegions enumerates values from the active local databases.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=[]wafpipeline.Region} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/regions [get]
func (w *WafPipelineApi) PipelineRegions(c *gin.Context) {
	if global.GIPLOCATION_MANAGER == nil {
		response.OkWithDetailed([]wafpipeline.Region{}, "地区库未就绪", c)
		return
	}
	response.OkWithDetailed(global.GIPLOCATION_MANAGER.Regions(), "地区来自当前本地库的完整枚举值", c)
}

// FeedConfig returns the source configuration without its authentication token.
// @Summary FeedConfig returns the source configuration without its authentication token.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/feed [get]
func (w *WafPipelineApi) FeedConfig(c *gin.Context) {
	cfg, err := waf_service.WafPipelineFeedServiceApp.Get()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	authorized, trustedKeys := waf_service.WafPipelineFeedServiceApp.Availability()
	data := gin.H{"config": cfg, "authorized": authorized, "trusted_keys": trustedKeys}
	if f := waf_service.WafPipelineFeedServiceApp.Feed; f != nil {
		data["status"] = f.Status()
	}
	response.OkWithDetailed(data, "", c)
}

// SaveFeed explicitly configures the signed source and verifies authorization.
// @Summary SaveFeed explicitly configures the signed source and verifies authorization.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body waf_service.PipelineFeedConfig true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/feed/save [post]
func (w *WafPipelineApi) SaveFeed(c *gin.Context) {
	var cfg waf_service.PipelineFeedConfig
	if !pipelineBind(c, &cfg) {
		return
	}
	if err := waf_service.WafPipelineFeedServiceApp.Save(cfg, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("订阅配置已保存", c)
}

// SyncFeed performs one explicitly requested bounded refresh.
// @Summary SyncFeed performs one explicitly requested bounded refresh.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/feed/sync [post]
func (w *WafPipelineApi) SyncFeed(c *gin.Context) {
	if err := waf_service.WafPipelineFeedServiceApp.Sync(c.Request.Context()); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("订阅已检查", c)
}
func pipelineOperator(c *gin.Context) string {
	if s := c.GetString("loginAccount"); s != "" {
		return s
	}
	return c.GetString("login_account")
}

// ListPorts lists port policy without credentials.
// @Summary ListPorts lists port policy without credentials.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=[]wafpipeline.PortConfig} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/ports [get]
func (w *WafPipelineApi) ListPorts(c *gin.Context) {
	data, err := waf_service.WafPipelineServiceApp.List()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "", c)
}

// SavePort saves a revision-checked port configuration; new ports remain disabled.
// @Summary SavePort saves a revision-checked port configuration; new ports remain disabled.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body waf_service.PortSaveRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.PortConfig} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/ports/save [post]
func (w *WafPipelineApi) SavePort(c *gin.Context) {
	var req waf_service.PortSaveRequest
	if !pipelineBind(c, &req) {
		return
	}
	req.Operator = pipelineOperator(c)
	data, err := waf_service.WafPipelineServiceApp.Save(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "端口配置已保存", c)
}

// DeletePort requires the port to be stopped.
// @Summary DeletePort requires the port to be stopped.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelinePortRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/ports/delete [post]
func (w *WafPipelineApi) DeletePort(c *gin.Context) {
	var req PipelinePortRequest
	if !pipelineBind(c, &req) {
		return
	}
	if err := waf_service.WafPipelineServiceApp.Delete(req.Port, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("已删除", c)
}

// Templates returns read-only builtins and user templates.
// @Summary Templates returns read-only builtins and user templates.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Success 200 {object} response.Response{data=[]wafpipeline.Template} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates [get]
func (w *WafPipelineApi) Templates(c *gin.Context) {
	data, err := waf_service.WafPipelineServiceApp.Templates()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "", c)
}

// SaveTemplate validates all fields and rejects credentials and manual IPs.
// @Summary SaveTemplate validates all fields and rejects credentials and manual IPs.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body wafpipeline.Template true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.Template} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates/save [post]
func (w *WafPipelineApi) SaveTemplate(c *gin.Context) {
	var req wafpipeline.Template
	if !pipelineBind(c, &req) {
		return
	}
	data, err := waf_service.WafPipelineServiceApp.SaveTemplate(req, pipelineOperator(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "模板已保存", c)
}

// DeleteTemplate deletes only a user template.
// @Summary DeleteTemplate deletes only a user template.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineTemplateIDRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates/delete [post]
func (w *WafPipelineApi) DeleteTemplate(c *gin.Context) {
	var req PipelineTemplateIDRequest
	if !pipelineBind(c, &req) {
		return
	}
	if err := waf_service.WafPipelineServiceApp.DeleteTemplate(req.ID, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("已删除", c)
}

// PreviewTemplate returns a copied candidate and section differences without saving.
// @Summary PreviewTemplate returns a copied candidate and section differences without saving.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineTemplatePreviewRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates/preview [post]
func (w *WafPipelineApi) PreviewTemplate(c *gin.Context) {
	var req PipelineTemplatePreviewRequest
	if !pipelineBind(c, &req) {
		return
	}
	tpl, err := waf_service.WafPipelineServiceApp.Template(req.ID)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	candidate, err := wafpipeline.ApplyTemplate(req.Config, tpl, req.Sections)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{"config": candidate, "diff": wafpipeline.TemplateDiff(req.Config, tpl)}, "预览未保存", c)
}

// ExportTemplate produces a secret-free template from a port.
// @Summary ExportTemplate produces a secret-free template from a port.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=string} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/templates/export [get]
func (w *WafPipelineApi) ExportTemplate(c *gin.Context) {
	p, err := strconv.ParseUint(c.Query("port"), 10, 16)
	if err != nil {
		response.FailWithMessage("端口不合法", c)
		return
	}
	cfg, err := waf_service.WafPipelineServiceApp.Get(uint16(p))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	t := wafpipeline.TemplateFromPort("user-"+strconv.Itoa(int(p)), cfg.Name, cfg)
	if t.Name == "" {
		t.Name = t.ID
	}
	data, _ := json.MarshalIndent(t, "", "  ")
	response.OkWithDetailed(string(data), "", c)
}

// CopyPort returns a disabled candidate, never credentials.
// @Summary CopyPort returns a disabled candidate, never credentials.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineCopyRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=wafpipeline.PortConfig} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/ports/copy [post]
func (w *WafPipelineApi) CopyPort(c *gin.Context) {
	var req PipelineCopyRequest
	if !pipelineBind(c, &req) {
		return
	}
	data, err := waf_service.WafPipelineServiceApp.Copy(req.Source, req.Target, req.IncludeIPs)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "复制预览，请另设账号后保存", c)
}

// TestUpstream is an explicit administrator-triggered TCP reachability check.
// @Summary TestUpstream is an explicit administrator-triggered TCP reachability check.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineUpstreamRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=string} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/upstream/test [post]
func (w *WafPipelineApi) TestUpstream(c *gin.Context) {
	var req PipelineUpstreamRequest
	if !pipelineBind(c, &req) {
		return
	}
	data, err := waf_service.WafPipelineServiceApp.CheckUpstreamDial(req.Upstream, req.DialIP)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "", c)
}
