// HomeWaf modification (2026-11-17): certificate management API.
package api

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/model/enums"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
	"time"
)

func (s *WafSslConfigApi) LifecycleList(c *gin.Context) {
	var req waf_service.CertificateListQuery
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("查询条件无效", c)
		return
	}
	list, total, err := waf_service.CertificateList(req)
	if err != nil {
		response.FailWithMessage("证书读取失败，请重试", c)
		return
	}
	response.OkWithDetailed(gin.H{"list": list, "total": total}, "获取成功", c)
}
func (s *WafSslConfigApi) LifecycleDetail(c *gin.Context) {
	var req request.SslConfigDetailReq
	if c.ShouldBindQuery(&req) != nil {
		response.FailWithMessage("证书 ID 无效", c)
		return
	}
	value, err := waf_service.CertificateDetail(req.Id)
	if err != nil {
		response.FailWithMessage("证书不存在或读取失败", c)
		return
	}
	response.OkWithDetailed(value, "获取成功", c)
}
func (s *WafSslConfigApi) LifecyclePolicy(c *gin.Context) {
	var req waf_service.CertificatePolicyEdit
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("续期设置参数无效", c)
		return
	}
	value, err := waf_service.SaveCertificatePolicy(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(value, "续期设置已保存，未发起签发任务", c)
}
func (s *WafSslConfigApi) LifecycleRenew(c *gin.Context) {
	var req waf_service.CertificateRenewRequest
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("证书或提交标识无效", c)
		return
	}
	order, created, err := waf_service.RequestCertificateRenewal(req.ID, req.RequestID, false, time.Now())
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if created {
		waf_service.WakeSSLOrderWorker(order)
	}
	response.OkWithDetailed(gin.H{"id": order.Id, "duplicate": !created, "apply_status": order.ApplyStatus}, "任务已受理，请查看处理结果", c)
}
func (s *WafSslConfigApi) LifecycleNotifications(c *gin.Context) {
	var req request.SslConfigDetailReq
	if c.ShouldBindQuery(&req) != nil {
		response.FailWithMessage("证书 ID 无效", c)
		return
	}
	if _, err := waf_service.CertificateDetail(req.Id); err != nil {
		response.FailWithMessage("证书不存在或无权访问", c)
		return
	}
	value, err := waf_service.CertificateNotifications(req.Id)
	if err != nil {
		response.FailWithMessage("通知状态读取失败", c)
		return
	}
	response.OkWithDetailed(value, "获取成功", c)
}
func (s *WafSslConfigApi) LifecycleNotificationRetry(c *gin.Context) {
	var req struct {
		ID string `json:"id" binding:"required,max=64"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("投递 ID 无效", c)
		return
	}
	if err := waf_service.RetryCertificateNotification(req.ID); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("通知已重新排队，不会重新申请证书", c)
}
func (s *WafSslConfigApi) LifecycleSettings(c *gin.Context) {
	var tasks []model.Task
	if err := global.GWAF_LOCAL_DB.Where("task_method IN ?", []string{enums.TASK_SSL_ORDER_RENEW, enums.TASK_SSL_PATH_LOAD, enums.TASK_SSL_EXPIRE_CHECK}).Find(&tasks).Error; err != nil {
		response.FailWithMessage("计划任务读取失败", c)
		return
	}
	settings := global.CurrentSettings()
	var usages []model.SslRenewalPolicy
	if err := global.GWAF_LOCAL_DB.Select("Id", "HostCode", "ApplyDomain", "ApplyPlatform", "ApplyDNS", "CredentialGroup", "Enabled").Find(&usages).Error; err != nil {
		response.FailWithMessage("凭据使用关系读取失败", c)
		return
	}
	role, _ := c.Get("userRole")
	canManage := role == enums.ROLE_SYSTEM_ADMIN || role == enums.ROLE_SUPER_ADMIN

	response.OkWithDetailed(gin.H{"can_manage": canManage, "credential_usage": usages, "tasks": tasks, "timezone": "Asia/Shanghai", "renew_days": settings.GCONFIG_RECORD_SSLOrder_EXPIRE_DAY, "ip_renew_days": settings.GCONFIG_RECORD_SSL_IP_EXPIRE_DAY,
		"file_sync_enabled": settings.GCONFIG_RECORD_AUTO_LOAD_SSL == 1, "retry_intervals_minutes": []int{15, 60, 240, 360}, "retry_limit_per_day": 6}, "获取成功", c)
}

func (s *WafSslConfigApi) LifecycleBindingTargets(c *gin.Context) {
	targets, err := waf_service.CertificateBindingTargets()
	if err != nil {
		response.FailWithMessage("服务使用状态读取失败", c)
		return
	}
	response.OkWithDetailed(targets, "获取成功", c)
}
func (s *WafSslConfigApi) LifecycleBind(c *gin.Context) {
	var req waf_service.CertificateBindRequest
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("证书或服务参数无效", c)
		return
	}
	if err := waf_service.BindCertificateToService(req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("服务证书关联已保存，协议与监听开关保持不变", c)
}
