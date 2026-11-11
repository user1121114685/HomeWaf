package api

import (
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafpipeline"
	"github.com/gin-gonic/gin"
	"strconv"
)

func gatePort(c *gin.Context) (uint16, bool) {
	p, err := strconv.ParseUint(c.Query("port"), 10, 16)
	if err != nil || p == 0 {
		response.FailWithMessage("端口不合法", c)
		return 0, false
	}
	return uint16(p), true
}

// GateUsers lists accounts without password hashes.
// @Summary GateUsers lists accounts without password hashes.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=[]model.GateUser} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/users [get]
func (w *WafPipelineApi) GateUsers(c *gin.Context) {
	p, ok := gatePort(c)
	if !ok {
		return
	}
	rows, err := waf_service.WafPipelineGateServiceApp.Users(p)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "", c)
}

// SaveGateUser creates or updates an independent port account.
// @Summary SaveGateUser creates or updates an independent port account.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body waf_service.GateUserSaveRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/users/save [post]
func (w *WafPipelineApi) SaveGateUser(c *gin.Context) {
	var r waf_service.GateUserSaveRequest
	if !pipelineBind(c, &r) {
		return
	}
	if err := waf_service.WafPipelineGateServiceApp.Save(r, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("账号已保存，旧会话已撤销", c)
}

// DeleteGateUser deletes one account and revokes its sessions.
// @Summary DeleteGateUser deletes one account and revokes its sessions.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineGateUserRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/users/delete [post]
func (w *WafPipelineApi) DeleteGateUser(c *gin.Context) {
	var r PipelineGateUserRequest
	if !pipelineBind(c, &r) {
		return
	}
	if err := waf_service.WafPipelineGateServiceApp.Delete(r.Port, r.Username, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("账号已删除", c)
}

// GateSessions lists active sessions without bearer tokens.
// @Summary GateSessions lists active sessions without bearer tokens.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param port query int true "Listener port" minimum(1) maximum(65535)
// @Success 200 {object} response.Response{data=[]wafpipeline.GateSessionRecord} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/sessions [get]
func (w *WafPipelineApi) GateSessions(c *gin.Context) {
	p, ok := gatePort(c)
	if !ok {
		return
	}
	response.OkWithDetailed(waf_service.WafPipelineGateServiceApp.Store.Sessions(wafpipeline.PortID(p)), "", c)
}

// KickGateSession revokes one active session.
// @Summary KickGateSession revokes one active session.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineGateSessionRequest true "Decrypted request payload"
// @Success 200 {object} response.Response "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/sessions/kick [post]
func (w *WafPipelineApi) KickGateSession(c *gin.Context) {
	var r PipelineGateSessionRequest
	if !pipelineBind(c, &r) {
		return
	}
	if err := waf_service.WafPipelineGateServiceApp.Kick(r.Port, r.ID, pipelineOperator(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("会话已撤销", c)
}

// RotateGateToken replaces or revokes the port service token.
// @Summary RotateGateToken replaces or revokes the port service token.
// @Tags Pipeline
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Description Requires security-admin role. Management transport encrypts request and response data; schemas describe decrypted payloads. Check response.code (0 means success), including HTTP 200 validation errors.
// @Param request body PipelineGateTokenRequest true "Decrypted request payload"
// @Success 200 {object} response.Response{data=object} "Result envelope"
// @Failure 401 {object} response.Response "Authentication required"
// @Failure 403 {object} response.Response "Security-admin role required"
// @Router /api/v1/pipeline/gate/token/rotate [post]
func (w *WafPipelineApi) RotateGateToken(c *gin.Context) {
	var r PipelineGateTokenRequest
	if !pipelineBind(c, &r) {
		return
	}
	token, err := waf_service.WafPipelineServiceApp.RotateServiceToken(r.Port, r.Revoke, pipelineOperator(c), r.Revision)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	c.Header("Cache-Control", "no-store")
	data := gin.H{"token": token}
	if r.Revision > 0 {
		data["revision"] = r.Revision + 1
	}
	response.OkWithDetailed(data, "令牌仅显示一次，请妥善保存；旧令牌已失效", c)
}
