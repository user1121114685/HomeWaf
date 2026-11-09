package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	response "HomeWaf/model/response"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"net"
	"strings"
	"sync"
	"time"
)

const DashboardShareScope = "screen:read:redacted:all-sites"

var ErrDashboardShareInvalid = errors.New("分享链接无效、已过期或已撤销")

var createScreenShareMu sync.Mutex

func shareDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func CreateDashboardShare(label string, hours int) (model.DashboardShare, string, error) {
	if hours < 1 || hours > 168 {
		return model.DashboardShare{}, "", errors.New("有效期必须为 1 至 168 小时")
	}
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 64 {
		return model.DashboardShare{}, "", errors.New("名称最多 64 个字符")
	}
	if label == "" {
		label = "监控大屏只读分享"
	}
	createScreenShareMu.Lock()
	defer createScreenShareMu.Unlock()
	now := time.Now().UnixMilli()
	var count int64
	if err := global.GWAF_LOCAL_DB.Model(&model.DashboardShare{}).Where("tenant_id = ? AND user_code = ? AND expires_at > ? AND revoked_at = 0", global.GWAF_TENANT_ID, global.GWAF_USER_CODE, now).Count(&count).Error; err != nil {
		return model.DashboardShare{}, "", err
	}
	if count >= 20 {
		return model.DashboardShare{}, "", errors.New("最多保留 20 个有效分享，请先撤销不再使用的链接")
	}
	random := make([]byte, 48)
	if _, err := rand.Read(random); err != nil {
		return model.DashboardShare{}, "", err
	}
	token := "hws_" + base64.RawURLEncoding.EncodeToString(random[:32])
	share := model.DashboardShare{ID: hex.EncodeToString(random[32:]), TenantID: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, TokenHash: shareDigest(token), Label: label, Scope: DashboardShareScope, CreatedAt: now, ExpiresAt: time.Now().Add(time.Duration(hours) * time.Hour).UnixMilli()}
	if err := global.GWAF_LOCAL_DB.Create(&share).Error; err != nil {
		return model.DashboardShare{}, "", err
	}
	return share, token, nil
}

func ListDashboardShares() ([]model.DashboardShare, error) {
	items := []model.DashboardShare{}
	err := global.GWAF_LOCAL_DB.Where("tenant_id = ? AND user_code = ?", global.GWAF_TENANT_ID, global.GWAF_USER_CODE).Order("created_at DESC").Limit(50).Find(&items).Error
	return items, err
}

func RevokeDashboardShare(id string) error {
	if len(id) != 32 {
		return errors.New("无效的分享标识")
	}
	res := global.GWAF_LOCAL_DB.Model(&model.DashboardShare{}).Where("id = ? AND tenant_id = ? AND user_code = ?", id, global.GWAF_TENANT_ID, global.GWAF_USER_CODE).Update("revoked_at", time.Now().UnixMilli())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("分享不存在")
	}
	return nil
}

func ValidateDashboardShare(token string) (model.DashboardShare, error) {
	var share model.DashboardShare
	if len(token) != 47 || !strings.HasPrefix(token, "hws_") {
		return share, ErrDashboardShareInvalid
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token[4:])
	if err != nil || len(decoded) != 32 {
		return share, ErrDashboardShareInvalid
	}
	err = global.GWAF_LOCAL_DB.Where("token_hash = ? AND tenant_id = ? AND user_code = ? AND scope = ? AND revoked_at = 0 AND expires_at > ?", shareDigest(token), global.GWAF_TENANT_ID, global.GWAF_USER_CODE, DashboardShareScope, time.Now().UnixMilli()).First(&share).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.DashboardShare{}, ErrDashboardShareInvalid
	}
	if err != nil {
		return model.DashboardShare{}, err
	}
	return share, nil
}

func maskedScreenIP(value string) string {
	ip := net.ParseIP(value)
	if ip == nil {
		return "未识别"
	}
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.*.*", v4[0], v4[1])
	}
	return fmt.Sprintf("%x:%x:*:*", uint16(ip[0])<<8|uint16(ip[1]), uint16(ip[2])<<8|uint16(ip[3]))
}

func RedactScreenSnapshot(source *response.ScreenSnapshot, expires int64) *response.ScreenSnapshot {
	copy := *source
	copy.ShareExpiresAt = expires
	if copy.Target.PublicIP != "" {
		copy.Target.PublicIP = maskedScreenIP(copy.Target.PublicIP)
	}
	copy.IPs = append([]response.ScreenIP{}, source.IPs...)
	for i := range copy.IPs {
		copy.IPs[i].IP = maskedScreenIP(copy.IPs[i].IP)
	}
	copy.Events = append([]response.ScreenEvent{}, source.Events...)
	for i := range copy.Events {
		e := &copy.Events[i]
		e.IP = maskedScreenIP(e.IP)
		e.Host = "受保护站点"
		e.Rule = "攻击已拦截"
		e.Province = ""
		e.City = ""
		sum := sha256.Sum256([]byte(e.ID))
		e.ID = hex.EncodeToString(sum[:12])
	}
	return &copy
}
