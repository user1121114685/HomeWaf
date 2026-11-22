// HomeWaf modification (2026-11-17): certificate lifecycle and durable recovery.
package model

import (
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"time"
)

// CertificateDaysLeft keeps an already expired certificate distinct from one due today.
func CertificateDaysLeft(at, expires time.Time) int {
	delta := expires.Sub(at)
	days := int(delta / (24 * time.Hour))
	if delta < 0 && delta%(24*time.Hour) != 0 {
		days--
	}
	return days
}

// SslRenewalPolicy owns mutable renewal configuration independently of history.
// Id is the certificate resource ID. IssuanceContext contains ACME account/resource
// material and is encrypted with the same serializer as other stored secrets.
type SslRenewalPolicy struct {
	baseorm.BaseOrm
	Version         uint64              `json:"version"`
	Enabled         bool                `json:"enabled"`
	HostCode        string              `gorm:"size:64" json:"host_code"`
	TargetID        string              `gorm:"size:64" json:"-"`
	ApplyDomain     string              `gorm:"type:text" json:"apply_domain"`
	ApplyPlatform   string              `gorm:"size:100" json:"apply_platform"`
	ApplyMethod     string              `gorm:"size:50" json:"apply_method"`
	ApplyDNS        string              `gorm:"size:100" json:"apply_dns"`
	ApplyEmail      string              `gorm:"size:255" json:"apply_email"`
	CredentialGroup string              `gorm:"size:255" json:"private_group_name"`
	SkipDNSVerify   int64               `json:"skip_dns_verify"`
	IssuanceContext string              `gorm:"type:text;serializer:homewaf_secret" json:"-"`
	Fingerprint     string              `gorm:"size:64" json:"fingerprint"`
	Status          string              `gorm:"size:40" json:"status"`
	LastOrderID     string              `gorm:"size:64" json:"last_order_id"`
	LastSuccessID   string              `gorm:"size:64" json:"last_success_id"`
	LastError       string              `gorm:"type:text" json:"last_error"`
	FailureKind     string              `gorm:"size:40" json:"failure_kind"`
	FailureCount    int                 `json:"failure_count"`
	NextAttemptAt   customtype.JsonTime `gorm:"index" json:"next_attempt_at"`
	RetryNotBefore  customtype.JsonTime `json:"retry_not_before"`
	LastAttemptAt   customtype.JsonTime `json:"last_attempt_at"`
	LastSuccessAt   customtype.JsonTime `json:"last_success_at"`
	LastNoticeAt    customtype.JsonTime `json:"last_notice_at"`
}

type CertificateEvent struct {
	baseorm.BaseOrm
	ExpiresAt     customtype.JsonTime `json:"expires_at,omitempty"`
	CertificateID string              `gorm:"size:64;index" json:"certificate_id"`
	OrderID       string              `gorm:"size:64;index" json:"order_id"`
	Kind          string              `gorm:"size:40" json:"kind"`
	Severity      string              `gorm:"size:20" json:"severity"`
	Domain        string              `gorm:"type:text" json:"domain"`
	Message       string              `gorm:"type:text" json:"message"`
	Expanded      bool                `gorm:"index" json:"-"`
	Coverage      string              `gorm:"size:100" json:"coverage"`
}

type CertificateDelivery struct {
	RetryNotBefore customtype.JsonTime `json:"retry_not_before"`
	baseorm.BaseOrm
	EventID        string              `gorm:"size:64;index" json:"event_id"`
	SubscriptionID string              `gorm:"size:64" json:"subscription_id"`
	ChannelID      string              `gorm:"size:64" json:"channel_id"`
	ChannelName    string              `gorm:"size:255" json:"channel_name"`
	Status         string              `gorm:"size:30;index" json:"status"`
	Attempts       int                 `json:"attempts"`
	NextAttemptAt  customtype.JsonTime `gorm:"index" json:"next_attempt_at"`
	LeaseUntil     customtype.JsonTime `json:"-"`
	Error          string              `gorm:"type:text" json:"error"`
}
