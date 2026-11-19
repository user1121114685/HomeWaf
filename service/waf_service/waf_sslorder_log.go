package waf_service

import (
	"strings"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/internal/acmelog"
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"
)

const sslOrderLogLimit = 500

type SSLOrderLogs struct {
	Notifications    *CertificateNotificationView `json:"notifications,omitempty"`
	CertificateID    string                       `json:"certificate_id"`
	Stage            string                       `json:"stage"`
	Trigger          string                       `json:"trigger"`
	Operation        string                       `json:"operation"`
	RetryOf          string                       `json:"retry_of"`
	DeploymentResult string                       `json:"deployment_result"`
	Renewal          *model.SslRenewalPolicy      `json:"renewal,omitempty"`
	AutoRenew        bool                         `json:"auto_renew"`
	Platform         string                       `json:"apply_platform"`
	Method           string                       `json:"apply_method"`
	DNS              string                       `json:"apply_dns"`
	Email            string                       `json:"apply_email"`
	CredentialGroup  string                       `json:"private_group_name"`
	CreatedAt        customtype.JsonTime          `json:"create_time"`
	ValidTo          customtype.JsonTime          `json:"result_valid_to"`
	ID               string                       `json:"id"`
	Status           string                       `json:"apply_status"`
	Progress         string                       `json:"progress"`
	Domain           string                       `json:"apply_domain"`
	UpdatedAt        customtype.JsonTime          `json:"update_time"`
	ResultError      string                       `json:"result_error"`
	HeartbeatStale   bool                         `json:"heartbeat_stale"`

	Limit   int                 `json:"limit"`
	Entries []model.SslOrderLog `json:"entries"`
}

func (s *WafSSLOrderService) AppendLog(id, level, message string) error {
	if id == "" || strings.TrimSpace(message) == "" {
		return nil
	}
	settings := global.CurrentSettings()
	entry := model.SslOrderLog{OrderID: id, TenantID: global.GWAF_TENANT_ID, UserCode: global.GWAF_USER_CODE, CreatedAt: customtype.JsonTime(time.Now()), Level: level, Message: acmelog.SanitizeValues(message, settings.GCONFIG_ZEROSSL_ACCESS_KEY, settings.GCONFIG_ZEROSSL_EAB_KID, settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY)}
	if err := global.GWAF_LOCAL_DB.Create(&entry).Error; err != nil {
		return err
	}
	// Bound retention per order, including verbose DNS propagation messages.
	var old []uint64
	if err := global.GWAF_LOCAL_DB.Model(&model.SslOrderLog{}).Where("order_id = ?", id).Order("id desc").Offset(sslOrderLogLimit).Limit(100).Pluck("id", &old).Error; err != nil {
		return err
	}
	if len(old) > 0 {
		return global.GWAF_LOCAL_DB.Where("order_id = ? AND id IN ?", id, old).Delete(&model.SslOrderLog{}).Error
	}
	return nil
}

func (s *WafSSLOrderService) Logs(id string) (SSLOrderLogs, error) {
	var order model.SslOrder
	if err := global.GWAF_LOCAL_DB.First(&order, "id = ?", id).Error; err != nil {
		return SSLOrderLogs{}, err
	}
	order.RefreshHeartbeatState(time.Now())
	view := SSLOrderLogs{ID: order.Id, Status: order.ApplyStatus, Domain: order.ApplyDomain, Progress: order.Progress, UpdatedAt: order.UPDATE_TIME, ResultError: acmelog.Sanitize(order.ResultError), HeartbeatStale: order.HeartbeatStale, Limit: sslOrderLogLimit, Entries: []model.SslOrderLog{}}
	view.CertificateID, view.Stage, view.Trigger, view.Operation, view.RetryOf, view.DeploymentResult = order.CertificateID, order.Stage, order.Trigger, order.Operation, order.RetryOf, order.DeploymentResult
	if CertificateLifecycleAvailable() && order.CertificateID != "" {
		var p model.SslRenewalPolicy
		if global.GWAF_LOCAL_DB.First(&p, "id = ?", order.CertificateID).Error == nil {
			view.Renewal = &p
		}
	}
	if CertificateLifecycleAvailable() {
		notification, err := CertificateOrderNotifications(order.Id)
		if err != nil {
			return view, err
		}
		view.Notifications = &notification
	}
	view.AutoRenew, view.Platform, view.Method, view.DNS = order.AutoRenewEnabled(), order.ApplyPlatform, order.ApplyMethod, order.ApplyDns
	view.Email, view.CredentialGroup, view.CreatedAt, view.ValidTo = order.ApplyEmail, order.PrivateGroupName, order.CREATE_TIME, order.ResultValidTo
	if err := global.GWAF_LOCAL_DB.Where("order_id = ?", id).Order("id desc").Limit(sslOrderLogLimit).Find(&view.Entries).Error; err != nil {
		return view, err
	}
	for i, j := 0, len(view.Entries)-1; i < j; i, j = i+1, j-1 {
		view.Entries[i], view.Entries[j] = view.Entries[j], view.Entries[i]
	}
	settings := global.CurrentSettings()
	secrets := []string{settings.GCONFIG_ZEROSSL_ACCESS_KEY, settings.GCONFIG_ZEROSSL_EAB_KID, settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY}
	if order.ApplyMethod == "dns01" && order.PrivateGroupName != "" {
		if values, err := LoadDNSCredentials(order.ApplyDns, order.PrivateGroupName); err == nil {
			for _, value := range values {
				secrets = append(secrets, value)
			}
			if order.ApplyDns == "hurricane" {
				if tokens, err := dnsproviders.ParseHurricaneTokens(values["HURRICANE_TOKENS"]); err == nil {
					for _, token := range tokens {
						secrets = append(secrets, token)
					}
				}
			}
		}
	}
	view.ResultError = acmelog.SanitizeValues(order.ResultError, secrets...)
	for i := range view.Entries {
		view.Entries[i].Message = acmelog.SanitizeValues(view.Entries[i].Message, secrets...)
	}
	return view, nil
}
