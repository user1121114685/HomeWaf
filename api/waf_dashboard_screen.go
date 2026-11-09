package api

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/common/response"
	"HomeWaf/service/waf_service"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type WafDashboardScreenApi struct{}

func (a *WafDashboardScreenApi) Snapshot(c *gin.Context) {
	data, err := waf_service.GetScreenSnapshot(c.Request.Context())
	if err != nil {
		response.FailWithMessage("大屏数据暂不可用："+err.Error(), c)
		return
	}
	response.OkWithDetailed(data, "获取成功", c)
}

func (a *WafDashboardScreenApi) SaveTarget(c *gin.Context) {
	var req struct {
		Mode      string   `json:"mode"`
		Label     string   `json:"label"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage("节点设置格式错误", c)
		return
	}
	if req.Mode == "logical" {
		req.Mode = "auto"
	}
	if req.Mode != "auto" && req.Mode != "coordinates" {
		response.FailWithMessage("请选择自动公网 IP 定位或自定义坐标", c)
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" || len([]rune(label)) > 32 {
		response.FailWithMessage("节点名称须为 1 至 32 个字符", c)
		return
	}
	target := model.DashboardTarget{TenantID: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, Mode: req.Mode, Label: label}
	id := sha256.Sum256([]byte(global.GWAF_TENANT_ID + "\x00" + global.GWAF_USER_CODE))
	target.ID = hex.EncodeToString(id[:])
	if req.Mode == "coordinates" {
		if req.Latitude == nil || req.Longitude == nil || math.IsNaN(*req.Latitude) || math.IsNaN(*req.Longitude) || math.IsInf(*req.Latitude, 0) || math.IsInf(*req.Longitude, 0) || math.Abs(*req.Latitude) > 90 || math.Abs(*req.Longitude) > 180 {
			response.FailWithMessage("经度范围 -180 至 180，纬度范围 -90 至 90", c)
			return
		}
		target.Latitude, target.Longitude = *req.Latitude, *req.Longitude
	}
	if err := global.GWAF_LOCAL_DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"mode", "label", "latitude", "longitude"})}).Create(&target).Error; err != nil {
		response.FailWithMessage("保存节点失败", c)
		return
	}
	waf_service.InvalidateScreenSnapshot()
	if req.Mode == "auto" {
		waf_service.RefreshDashboardNodeLocation()
	}
	response.OkWithDetailed(target, "节点已保存", c)
}

func (a *WafDashboardScreenApi) Shares(c *gin.Context) {
	items, err := waf_service.ListDashboardShares()
	if err != nil {
		response.FailWithMessage("读取分享列表失败", c)
		return
	}
	response.OkWithDetailed(items, "获取成功", c)
}
func (a *WafDashboardScreenApi) CreateShare(c *gin.Context) {
	var req struct {
		Label string `json:"label"`
		Hours int    `json:"hours"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("分享设置格式错误", c)
		return
	}
	share, token, err := waf_service.CreateDashboardShare(req.Label, req.Hours)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OkWithDetailed(gin.H{"share": share, "token": token}, "分享已创建，令牌仅显示一次", c)
}
func (a *WafDashboardScreenApi) RevokeShare(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithMessage("无效的分享标识", c)
		return
	}
	if err := waf_service.RevokeDashboardShare(req.ID); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("分享已撤销", c)
}

// PublicShare has its own capability authentication. It never accepts an admin
// token through the URL, and the screen token is not accepted by any admin API.
func (a *WafDashboardScreenApi) PublicShare(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	share, err := waf_service.ValidateDashboardShare(c.GetHeader("X-Dashboard-Token"))
	if err != nil {
		if !errors.Is(err, waf_service.ErrDashboardShareInvalid) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "msg": "分享校验暂不可用，请稍后重试"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "msg": "分享链接无效、已过期或已撤销"})
		return
	}
	snapshot, err := waf_service.GetScreenSnapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "msg": "大屏数据暂不可用，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": waf_service.RedactScreenSnapshot(snapshot, share.ExpiresAt), "msg": "只读分享"})
}
