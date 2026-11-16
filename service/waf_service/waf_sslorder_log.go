package waf_service

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/internal/acmelog"
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"
	"HomeWaf/utils"
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
	Legacy           bool                         `json:"legacy"`
	Limit            int                          `json:"limit"`
	Entries          []model.SslOrderLog          `json:"entries"`
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
	if len(view.Entries) == 0 || time.Time(view.Entries[0].CreatedAt).Sub(time.Time(order.CREATE_TIME)) > 10*time.Second {
		view.Legacy = true
		entries, err := readLegacySSLOrderLogs(filepath.Join(utils.GetCurrentDir(), "logs", "log.log"), order)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return view, err
		}
		view.Entries = append(entries, view.Entries...)
		if len(view.Entries) > sslOrderLogLimit {
			view.Entries = view.Entries[len(view.Entries)-sslOrderLogLimit:]
		}
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

var ansiLog = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Historical orders predate per-order logging. Read a bounded tail of the fixed
// local runtime log, matching an exact domain and this order's time window.
// Never accept a path from the client or return unrelated runtime log lines.
func readLegacySSLOrderLogs(path string, order model.SslOrder) ([]model.SslOrderLog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := info.Size() - (4 << 20)
	if start < 0 {
		start = 0
	}
	scanner := bufio.NewScanner(io.NewSectionReader(f, start, info.Size()-start))
	scanner.Buffer(make([]byte, 4096), (4<<20)+1)
	if start > 0 {
		scanner.Scan()
	}
	var domains []string
	for _, domain := range strings.Split(order.ApplyDomain, ",") {
		domains = append(domains, regexp.QuoteMeta(strings.TrimSpace(domain)))
	}
	if len(domains) == 0 || order.ApplyDomain == "" {
		return nil, nil
	}
	matchDomain := regexp.MustCompile(`(?i)(^|[\s=\[,])(?:` + strings.Join(domains, "|") + `)([\s,\]:]|$)`)
	from := time.Time(order.CREATE_TIME).Truncate(time.Second)
	until := time.Time(order.UPDATE_TIME).Add(time.Second)
	if order.ApplyStatus == "submitted" || order.ApplyStatus == "applying" {
		until = time.Now()
	}
	entries := []model.SslOrderLog{}
	for scanner.Scan() {
		line := ansiLog.ReplaceAllString(scanner.Text(), "")
		if !strings.Contains(line, "ACME证书:") && !strings.Contains(line, "正在进行首次证书申请") && !strings.Contains(line, "正在证书续期申请处理") {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 3 {
			continue
		}
		at, err := time.ParseInLocation("2006-01-02 15:04:05.000", parts[0], time.Local)
		if err != nil || at.Before(from) || at.After(until) || !matchDomain.MatchString(parts[2]) {
			continue
		}
		entries = append(entries, model.SslOrderLog{CreatedAt: customtype.JsonTime(at), Level: strings.ToLower(parts[1]), Message: acmelog.Sanitize(parts[2])})
		if len(entries) > sslOrderLogLimit {
			entries = entries[len(entries)-sslOrderLogLimit:]
		}
	}
	return entries, scanner.Err()
}
