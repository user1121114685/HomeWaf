package api

import (
	"HomeWaf/global"
	"HomeWaf/model/common/response"
	"HomeWaf/utils"
	"HomeWaf/utils/localca"
	"HomeWaf/wafconfig"
	"net"
	"net/http"
	"strconv"
	"time"

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
	urls := utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, true, entry)
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
	if domain != "" {
		urls = []string{"https://" + net.JoinHostPort(domain, strconv.Itoa(global.GWAF_LOCAL_SERVER_PORT)) + entry}
	}
	return gin.H{"domain": domain, "https_enabled": global.GWAF_SSL_ENABLE, "force_https": global.GWAF_SSL_FORCE_HTTPS, "urls": urls, "cert": localca.CurrentServerCert(p),
		"ca": localca.CASummary(p), "ca_pem": string(caPEM), "is_local": localca.IsIssuedByLocalCA(p), "cert_usable": certProblem == "", "cert_problem": certProblem}
}

func (w *WafVpConfigApi) GetManagementAddressApi(c *gin.Context) {
	response.OkWithDetailed(managementAddressInfo(c), "获取管理地址成功", c)
}

func (w *WafVpConfigApi) ConfigureManagementAddressApi(c *gin.Context) {
	var req struct {
		Domain        string `json:"domain"`
		ConfirmDomain string `json:"confirm_domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage("解析请求失败", c)
		return
	}
	domain, err := wafconfig.NormalizeManagementDomain(req.Domain)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if domain != "" && req.ConfirmDomain != domain {
		response.FailWithMessage("请确认管理域名已解析到本服务器；应用后只接受该域名的 HTTPS 管理访问", c)
		return
	}
	if global.GWAF_CHAN_MANAGER_RESTART == nil {
		response.FailWithMessage("管理端重启通道尚未就绪，请稍后重试", c)
		return
	}
	if err := wafconfig.ConfigureManagementAddress(utils.GetCurrentDir(), domain); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(managementAddressInfo(c), "配置已保存，管理端将在 1 秒后重启；请导入根证书后验证新的 HTTPS 入口", c)
	go func() {
		time.Sleep(time.Second)
		select {
		case global.GWAF_CHAN_MANAGER_RESTART <- 1:
		default:
		}
	}()
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
