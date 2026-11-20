package api

import (
	"HomeWaf/global"
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"HomeWaf/utils"
	"HomeWaf/utils/localca"
	"HomeWaf/wafconfig"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func managementAddressInfo(c *gin.Context) gin.H {
	p := localca.DefaultPaths(utils.GetCurrentDir())
	caPEM, _ := localca.ReadCACert(p)
	domain := wafconfig.GetManagementDomain()
	certProblem := ""
	if err := localca.CheckServerCert(p); err != nil {
		certProblem = err.Error()
	}
	entry := "/"
	if global.GWAF_SECURITY_ENTRY_ENABLE {
		entry += global.GWAF_SECURITY_ENTRY_PATH + "/"
	}
	urls := utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, true, entry, domain)
	if address, ok := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if host, _, err := net.SplitHostPort(address.String()); err == nil {
			preferred := "https://" + net.JoinHostPort(host, strconv.Itoa(global.GWAF_LOCAL_SERVER_PORT)) + entry
			for index, url := range urls {
				if url == preferred {
					urls[0], urls[index] = urls[index], urls[0]
					break
				}
			}
		}
	}
	return gin.H{"domain": domain, "https_enabled": global.GWAF_SSL_ENABLE, "force_https": global.GWAF_SSL_FORCE_HTTPS, "urls": urls, "cert": localca.CurrentServerCert(p),
		"pending": wafconfig.ManagementPreviewFor(c.Request, c.GetString("loginAccount"), c.GetHeader("X-Token")),
		"ca":      localca.CASummary(p), "ca_pem": string(caPEM), "is_local": localca.IsIssuedByLocalCA(p), "cert_usable": certProblem == "", "cert_problem": certProblem}
}

func (w *WafVpConfigApi) GetManagementAddressApi(c *gin.Context) {
	response.OkWithDetailed(managementAddressInfo(c), "获取管理地址成功", c)
}

func (w *WafVpConfigApi) ConfigureManagementAddressApi(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
		Mode   string `json:"mode"`
		ID     string `json:"id"`
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage("解析请求失败", c)
		return
	}
	account, token := c.GetString("loginAccount"), c.GetHeader("X-Token")
	if account == "" || token == "" || c.GetBool("is_openapi") {
		response.ForbiddenWithMessage("请通过控制台使用系统管理账号验证新入口", c)
		return
	}
	dir := utils.GetCurrentDir()
	var err error
	switch req.Action {
	case "prepare":
		var localIP string
		if addr, ok := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			localIP, _, _ = net.SplitHostPort(addr.String())
		}
		err = wafconfig.PrepareManagementChange(dir, account, req.Mode, req.Domain, localIP)
	case "confirm", "cancel":
		err = wafconfig.FinishManagementChange(dir, account, req.ID, req.Action, token, c.Request, func(persist func() error) error {
			return waf_service.WafTokenInfoServiceApp.PromoteManagementLogin(account, token, wafconfig.ManagementPreviewLoginType, persist)
		})
	default:
		response.FailWithMessage("请刷新控制台，再准备并登录新入口", c)
		return
	}
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(managementAddressInfo(c), "操作成功", c)
}

// This unauthenticated bootstrap response contains public certificate data only.
// Domain/IP policy and the security-entry prefix still apply to this route.
func (w *WafVpConfigApi) GetPublicManagementTLSApi(c *gin.Context) {
	p := localca.DefaultPaths(utils.GetCurrentDir())
	host := c.Request.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	configured := global.GWAF_SSL_ENABLE && localca.CheckServerCert(p) == nil
	result := gin.H{"https_configured": configured, "host_covered": configured && localca.ServerCertCoversHost(p, host)}
	if configured && localca.IsIssuedByLocalCA(p) {
		if cert, err := localca.ReadCACert(p); err == nil {
			result["ca_pem"] = string(cert)
			result["ca"] = localca.CASummary(p)
		}
	}
	c.Header("Cache-Control", "no-store")
	response.OkWithDetailed(result, "获取 HTTPS 引导信息成功", c)
}
