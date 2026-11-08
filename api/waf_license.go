// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package api

import (
	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	"HomeWaf/model/common/response"
	response2 "HomeWaf/model/response"
	"HomeWaf/wafreg"
	"crypto/sha256"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"sync"
	"time"
)

type WafLicenseApi struct{}
type pendingLicense struct {
	document []byte
	expires  time.Time
}

var licensePreviews = struct {
	sync.Mutex
	items map[string]pendingLicense
}{items: make(map[string]pendingLicense)}

func licenseSession(c *gin.Context) string {
	account := c.GetString("loginAccount")
	if account == "" {
		return ""
	}
	token := c.GetHeader("X-Token")
	if token == "" {
		token = c.GetHeader("X-Mobile-Token")
	}
	if token == "" {
		token = c.GetHeader("X-API-Key")
	}
	if token == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(account+"\x00"+token)))
}
func (w *WafLicenseApi) GetDetailApi(c *gin.Context) {
	info := wafreg.CurrentInfo()
	response.OkWithDetailed(response2.LicenseRep{License: info, MachineId: wafreg.GenClientMachineInfo().MachineID, Version: global.GWAF_REG_VERSION}, "获取成功", c)
}
func (w *WafLicenseApi) CheckLicense(c *gin.Context) {
	session := licenseSession(c)
	if session == "" {
		response.FailWithMessage("授权导入需要已登录的管理会话", c)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, licensing.MaxDocumentSize+16384)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.FailWithMessage("授权文件读取失败或文件过大", c)
		return
	}
	defer file.Close()
	document, err := io.ReadAll(io.LimitReader(file, licensing.MaxDocumentSize+1))
	if err != nil || len(document) > licensing.MaxDocumentSize {
		response.FailWithMessage("授权文件读取失败或文件过大", c)
		return
	}
	if ok, _, err := wafreg.VerifyServerReg(document); !ok || err != nil {
		response.FailWithMessage("授权验证失败：请检查签名、实例及有效期", c)
		return
	}
	licensePreviews.Lock()
	defer licensePreviews.Unlock()
	now := time.Now()
	for key, value := range licensePreviews.items {
		if now.After(value.expires) {
			delete(licensePreviews.items, key)
		}
	}
	if len(licensePreviews.items) >= 128 {
		response.FailWithMessage("待确认授权过多，请稍后重试", c)
		return
	}
	licensePreviews.items[session] = pendingLicense{append([]byte{}, document...), now.Add(10 * time.Minute)}
	response.OkWithMessage("授权验证成功，请在十分钟内确认保存", c)
}
func (w *WafLicenseApi) ConfirmApi(c *gin.Context) {
	session := licenseSession(c)
	licensePreviews.Lock()
	pending, ok := licensePreviews.items[session]
	licensePreviews.Unlock()
	if session == "" || !ok || !time.Now().Before(pending.expires) {
		response.FailWithMessage("尚未上传授权或确认已过期，请重新上传", c)
		return
	}
	if err := wafreg.Save(pending.document); err != nil {
		response.FailWithMessage("授权重新验证或保存失败", c)
		return
	}
	licensePreviews.Lock()
	delete(licensePreviews.items, session)
	licensePreviews.Unlock()
	w.GetDetailApi(c)
}
