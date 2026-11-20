package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/utils"
	"errors"
	"golang.org/x/net/idna"
	"net"
	"strconv"
	"strings"
	"time"
)

func validateCertificateMonitor(domain string, port int) (string, error) {
	domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
	if port < 1 || port > 65535 {
		return "", errors.New("端口必须在 1–65535 之间")
	}
	if ip := net.ParseIP(domain); ip != nil {
		return ip.String(), nil
	}
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil || len(ascii) == 0 || len(ascii) > 253 || strings.ContainsAny(ascii, "/:* @") {
		return "", errors.New("请填写域名或 IP，不包含协议、路径或泛域名")
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", errors.New("域名格式无效")
		}
	}
	return strings.ToLower(ascii), nil
}
func addCertificateMonitor(domain string, port int) error {
	domain, err := validateCertificateMonitor(domain, port)
	if err != nil {
		return err
	}
	var count int64
	if err := global.GWAF_LOCAL_DB.Model(&model.SslExpire{}).Where("domain = ? AND port = ?", domain, port).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("此域名与端口已在监测列表")
	}
	now := customtype.JsonTime(time.Now())
	row := model.SslExpire{BaseOrm: baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: now, UPDATE_TIME: now}, Domain: domain, Port: port, Status: "unverified"}
	return global.GWAF_LOCAL_DB.Create(&row).Error
}
func certificateMonitorState(row model.SslExpire, now time.Time) string {
	if row.Status == "checking" {
		if now.Sub(time.Time(row.UPDATE_TIME)) < 2*time.Minute {
			return "checking"
		}
		return "failed"
	}
	if row.VisitLog != "" {
		return "failed"
	}
	if time.Time(row.ValidTo).IsZero() {
		return "unverified"
	}
	if time.Time(row.LastDetect).IsZero() || now.Sub(time.Time(row.LastDetect)) > 48*time.Hour {
		return "stale"
	}
	if !time.Time(row.ValidTo).After(now) {
		return "expired"
	}
	days := global.CurrentSettings().GCONFIG_RECORD_SSLOrder_EXPIRE_DAY
	if net.ParseIP(row.Domain) != nil {
		days = global.CurrentSettings().GCONFIG_RECORD_SSL_IP_EXPIRE_DAY
	}
	if time.Time(row.ValidTo).Sub(now) <= time.Duration(days)*24*time.Hour {
		return "expiring"
	}
	return "valid"
}
func StartCertificateMonitor(id string) error {
	var row model.SslExpire
	db := global.GWAF_LOCAL_DB
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		return errors.New("目标不存在或无权访问")
	}
	now := time.Now()
	result := db.Model(&row).Where("status <> ? OR update_time <= ?", "checking", customtype.JsonTime(now.Add(-2*time.Minute))).Updates(map[string]any{"status": "checking", "update_time": customtype.JsonTime(now)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("此目标正在检测")
	}
	go func() {
		// Only update the exact target that was queued; editing a target while it
		// runs must not attach an old probe result to a different hostname.
		expires, err := utils.CheckSSLCertificateExpiry(net.JoinHostPort(row.Domain, strconv.Itoa(row.Port)))
		now := customtype.JsonTime(time.Now())
		values := map[string]any{"last_detect": now, "update_time": now, "status": "success", "visit_log": ""}
		if err != nil {
			values["status"], values["visit_log"] = "failed", err.Error()
		} else {
			values["last_success"], values["valid_to"] = now, customtype.JsonTime(expires)
		}
		_ = db.Model(&model.SslExpire{}).Where("id = ? AND domain = ? AND port = ?", row.Id, row.Domain, row.Port).Updates(values).Error
	}()
	return nil
}
