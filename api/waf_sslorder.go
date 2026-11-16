// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package api

import (
	"HomeWaf/common/zlog"
	"HomeWaf/internal/acmelog"
	"HomeWaf/model/common/response"
	"HomeWaf/model/request"
	"HomeWaf/service/waf_service"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WafSslOrderApi struct {
}

func (w *WafSslOrderApi) LogsApi(c *gin.Context) {
	var req request.WafSslorderdetailReq
	if err := c.ShouldBind(&req); err != nil {
		response.FailWithMessage("请提供有效的订单 ID", c)
		return
	}
	logs, err := wafSslOrderService.Logs(req.Id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.FailWithMessage("订单不存在，请刷新订单列表", c)
		} else {
			zlog.Error("证书申请日志读取失败", "order_id", req.Id, "error", acmelog.Sanitize(err.Error()))
			response.FailWithMessage("申请日志读取失败，具体原因已记录到服务运行日志", c)
		}
		return
	}
	response.OkWithDetailed(logs, "获取成功", c)
}

func (w *WafSslOrderApi) RetryApi(c *gin.Context) {
	var req request.WafSslorderretryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage("请提供订单 ID 和重试提交标识", c)
		return
	}
	order, created, err := wafSslOrderService.Retry(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if created {
		waf_service.WakeSSLOrderWorker(order)
	}
	response.OkWithDetailed(gin.H{"id": order.Id, "apply_status": order.ApplyStatus, "duplicate": !created}, "重试申请已受理，原失败记录已保留", c)
}

// AddApi 新建SSL证书申请订单
// @Summary      新建SSL证书申请订单
// @Description  向 Let's Encrypt 或 ZeroSSL 发起新的 SSL 证书申请
// @Tags         SSL证书订单
// @Accept       json
// @Produce      json
// @Param        data  body      request.WafSslorderaddReq  true  "订单配置"
// @Success      200   {object}  response.Response  "添加成功"
// @Security     ApiKeyAuth
// @Router       /wafhost/sslorder/add [post]
func (w *WafSslOrderApi) AddApi(c *gin.Context) {
	var req request.WafSslorderaddReq
	err := c.ShouldBindJSON(&req)
	if err == nil {
		if msg := checkApplyMethod(req.ApplyMethod); msg != "" {
			response.FailWithMessage(msg, c)
			return
		}
		if req.ApplyMethod != "dns01" {
			req.SkipDNSVerify = 0
		}
		req.ApplyDomain, err = waf_service.ValidateSSLOrderChallenge(req.ApplyMethod, req.ApplyDns, req.PrivateGroupName, req.ApplyDomain, req.SkipDNSVerify)
		if err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}
		if err := waf_service.ValidateSSLCertificateTarget(req.HostCode, req.ApplyMethod, req.ApplyDomain); err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}
		if req.ApplyPlatform == "zerossl" {
			if !waf_service.GetZeroSSLAccountStatus().Configured {
				response.FailWithMessage(waf_service.ZeroSSLAccountMissing, c)
				return
			}
		}
		addResult, created, err := wafSslOrderService.Submit(req, "")
		if err == nil {
			if created {
				waf_service.WakeSSLOrderWorker(addResult)
			}
			response.OkWithDetailed(gin.H{"id": addResult.Id, "apply_status": addResult.ApplyStatus, "duplicate": !created}, "申请已受理", c)
		} else {
			response.FailWithMessage(err.Error(), c)
		}
		return

	} else {
		response.FailWithMessage("解析失败", c)
	}
}

// GetDetailApi 获取SSL证书订单详情
// @Summary      获取SSL证书订单详情
// @Description  根据ID获取SSL证书申请订单详情
// @Tags         SSL证书订单
// @Accept       json
// @Produce      json
// @Param        id  query     string  true  "订单ID"
// @Success      200  {object}  response.Response  "获取成功"
// @Security     ApiKeyAuth
// @Router       /wafhost/sslorder/detail [get]
func (w *WafSslOrderApi) GetDetailApi(c *gin.Context) {
	var req request.WafSslorderdetailReq
	err := c.ShouldBind(&req)
	if err == nil {
		bean := wafSslOrderService.GetDetailApi(req)
		if bean.Id == "" {
			response.FailWithMessage("证书订单不存在或读取失败", c)
			return
		}
		response.OkWithDetailed(bean, "获取成功", c)
	} else {
		response.FailWithMessage("解析失败", c)
	}
}

// GetListApi 获取SSL证书订单列表
// @Summary      获取SSL证书订单列表
// @Description  分页查询SSL证书申请订单列表
// @Tags         SSL证书订单
// @Accept       json
// @Produce      json
// @Param        data  body      request.WafSslordersearchReq  true  "分页查询参数"
// @Success      200   {object}  response.Response{data=response.PageResult}  "获取成功"
// @Security     ApiKeyAuth
// @Router       /wafhost/sslorder/list [post]
func (w *WafSslOrderApi) GetListApi(c *gin.Context) {
	var req request.WafSslordersearchReq
	err := c.ShouldBindJSON(&req)
	if err == nil {
		beans, total, err := wafSslOrderService.GetListApi(req)
		if err != nil {
			response.FailWithMessage("读取证书订单失败，请稍后重试", c)
			return
		}
		response.OkWithDetailed(response.PageResult{
			List:      beans,
			Total:     total,
			PageIndex: req.PageIndex,
			PageSize:  req.PageSize,
		}, "获取成功", c)
	} else {
		response.FailWithMessage("解析失败", c)
	}
}

// DelApi 删除SSL证书订单
// @Summary      删除SSL证书订单
// @Description  根据ID删除SSL证书申请订单
// @Tags         SSL证书订单
// @Accept       json
// @Produce      json
// @Param        id  query     string  true  "订单ID"
// @Success      200  {object}  response.Response  "删除成功"
// @Security     ApiKeyAuth
// @Router       /wafhost/sslorder/del [get]
func (w *WafSslOrderApi) DelApi(c *gin.Context) {
	var req request.WafSslorderdeleteReq
	err := c.ShouldBind(&req)
	if err == nil {
		err = wafSslOrderService.DelApi(req)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			response.FailWithMessage("请检测参数", c)
		} else if err != nil {
			response.FailWithMessage("订单正在处理中或删除失败，请刷新后重试", c)
		} else {
			response.OkWithMessage("删除成功", c)
		}

	} else {
		response.FailWithMessage("解析失败", c)
	}
}

// ModifyApi 续期SSL证书订单
// @Summary      续期SSL证书订单
// @Description  对已成功申请且未过期的证书发起续期申请
// @Tags         SSL证书订单
// @Accept       json
// @Produce      json
// @Param        data  body      request.WafSslordereditReq  true  "续期配置"
// @Success      200   {object}  response.Response  "续期成功"
// @Security     ApiKeyAuth
// @Router       /wafhost/sslorder/edit [post]
func (w *WafSslOrderApi) ModifyApi(c *gin.Context) {
	var req request.WafSslordereditReq
	err := c.ShouldBindJSON(&req)
	if err == nil {
		if msg := checkApplyMethod(req.ApplyMethod); msg != "" {
			response.FailWithMessage(msg, c)
			return
		}
		if req.ApplyMethod != "dns01" {
			req.SkipDNSVerify = 0
		}
		req.ApplyDomain, err = waf_service.ValidateSSLOrderChallenge(req.ApplyMethod, req.ApplyDns, req.PrivateGroupName, req.ApplyDomain, req.SkipDNSVerify)
		if err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}
		if err := waf_service.ValidateSSLCertificateTarget(req.HostCode, req.ApplyMethod, req.ApplyDomain); err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}

		if req.ApplyPlatform == "zerossl" {
			if !waf_service.GetZeroSSLAccountStatus().Configured {
				response.FailWithMessage(waf_service.ZeroSSLAccountMissing, c)
				return
			}
		}

		order, created, err := wafSslOrderService.SubmitRenew(req)
		if err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}
		if created {
			waf_service.WakeSSLOrderWorker(order)
		}
		response.OkWithDetailed(gin.H{"id": order.Id, "apply_status": order.ApplyStatus, "duplicate": !created}, "续期申请已受理", c)
	} else {
		response.FailWithMessage("续期解析失败", c)
	}
}

// validApplyMethod 是订单里唯一被逐值判断的枚举字段，其余 apply_* 均为自由文本。
// 非法值不会在建单阶段被拦下，只会一路写进库里，到 ACME 侧才以
// "acme: could not determine solvers" 的形式失败，回指不到入参。
var validApplyMethod = map[string]bool{"http01": true, "dns01": true}

// checkApplyMethod 只认 ACME 挑战方式的取值，不认界面下拉里显示的标签文本
// （如「文件验证方式」）。返回空串表示通过。
func checkApplyMethod(method string) string {
	if validApplyMethod[method] {
		return ""
	}
	return "申请方式(apply_method)取值无效，只支持 http01（HTTP 文件验证）或 dns01（DNS 验证），请填写取值而不是界面显示的标签文本"
}
