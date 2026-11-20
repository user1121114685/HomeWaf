// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package response

import (
	"HomeWaf/internal/platform/wafsec"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

const (
	ERROR             = -1
	SUCCESS           = 0
	INPUT_SECRET_CODE = -2
	NEED_BIND_2FA     = -3
	NEED_CHANGE_PWD   = -4
	// NEED_REHANDSHAKE 告诉客户端本次没有可用的会话密钥，请重新握手后重试。
	NEED_REHANDSHAKE = -5
	// BACKEND_UNAVAILABLE 依赖的存储/缓存后端本次不可用，请求未能完成。
	// 与 AUTHFAIL 的区别：登录状态没有问题，客户端应保留登录态并稍后重试。
	BACKEND_UNAVAILABLE = -6
	// EXPORT_DISABLED 日志导出被配置关闭（conf/config.yml 的 export_download 未开启）。
	// 前端据此给出开启方法的指引，而不是一句「当前不允许导出」。
	EXPORT_DISABLED        = -7
	NEED_CONFIRM_BLOCK_ALL = -8 // WAF全网黑名单需要显式确认
	FORBIDDEN              = -403
	AUTHFAIL               = -999
)

// HeaderKeyID 是客户端声明本次会话标识的请求头。
const HeaderKeyID = "X-Key-Id"

func Result(code int, data interface{}, msg string, c *gin.Context) {
	// API Key 客户端使用 JSON 响应。
	if isOpenApi, exists := c.Get("is_openapi"); exists && isOpenApi == true {
		c.JSON(http.StatusOK, Response{
			code,
			data,
			msg,
		})
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var session *wafsec.TransportSession
	if value, ok := c.Get(TransportSessionKey); ok {
		session, _ = value.(*wafsec.TransportSession)
	} else if c.Request != nil {
		session, _ = wafsec.OpenTransportSession(c.GetHeader(HeaderKeyID))
	}
	if session == nil {
		if code == SUCCESS {
			Rehandshake(c)
			return
		}
		c.JSON(http.StatusOK, Response{code, "", msg})
		return
	}
	encrypted, err := session.Encrypt(raw)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, Response{code, encrypted, msg})
}

const TransportSessionKey = "management_transport_session"

func Rehandshake(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusOK, Response{NEED_REHANDSHAKE, "", "会话密钥不可用，请重新握手"})
}

// OkWithPlainData 返回不加密的 data，供握手类接口使用——此时双方还没有共享密钥。
func OkWithPlainData(data interface{}, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		SUCCESS,
		data,
		"查询成功",
	})
}

// FailWithPlainMessage 与 OkWithPlainData 配套的失败返回（同样不加密）。
func FailWithPlainMessage(message string, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		ERROR,
		"",
		message,
	})
}

func Ok(c *gin.Context) {
	Result(SUCCESS, map[string]interface{}{}, "操作成功", c)
}

func OkWithMessage(message string, c *gin.Context) {
	Result(SUCCESS, map[string]interface{}{}, message, c)
}

func OkWithData(data interface{}, c *gin.Context) {
	Result(SUCCESS, data, "查询成功", c)
}

func OkWithDetailed(data interface{}, message string, c *gin.Context) {
	Result(SUCCESS, data, message, c)
}

func FailWithMessage(message string, c *gin.Context) {
	Result(ERROR, map[string]interface{}{}, message, c)
}

func FailWithDetailed(data interface{}, message string, c *gin.Context) {
	Result(ERROR, data, message, c)
}
func AuthFailWithMessage(message string, c *gin.Context) {
	Result(AUTHFAIL, map[string]interface{}{}, message, c)
}

// ForbiddenWithMessage 已认证但无权限（角色不满足），不触发重新登录
func ForbiddenWithMessage(message string, c *gin.Context) {
	Result(FORBIDDEN, map[string]interface{}{}, message, c)
}

// BackendUnavailableWithMessage 后端存储本次不可用，登录状态不受影响，客户端保留登录态稍后重试
func BackendUnavailableWithMessage(message string, c *gin.Context) {
	Result(BACKEND_UNAVAILABLE, map[string]interface{}{}, message, c)
}
func SecretCodeFailWithMessage(message string, c *gin.Context) {
	Result(INPUT_SECRET_CODE, map[string]interface{}{}, message, c)
}
func NeedBind2FAWithMessage(message string, c *gin.Context) {
	Result(NEED_BIND_2FA, map[string]interface{}{}, message, c)
}

// NeedChangePwdWithMessage 令牌需强制改密：前端据此码引导用户先改密后再操作
func NeedChangePwdWithMessage(message string, c *gin.Context) {
	Result(NEED_CHANGE_PWD, map[string]interface{}{}, message, c)
}
