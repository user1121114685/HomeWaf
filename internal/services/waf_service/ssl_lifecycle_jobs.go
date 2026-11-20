// HomeWaf modification (2026-11-17): durable certificate attempts and backoff.
package waf_service

import (
	"HomeWaf/internal/acmelog"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/utils"
	"errors"
	"fmt"
	"github.com/go-acme/lego/v4/acme"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type CertificateRenewRequest struct {
	ID        string `json:"id" binding:"required,max=64"`
	RequestID string `json:"request_id" binding:"required,max=128"`
}

func RequestCertificateRenewal(id, requestID string, automatic bool, now time.Time) (model.SslOrder, bool, error) {
	var order model.SslOrder
	if id == "" || requestID == "" || len(requestID) > 128 {
		return order, false, errors.New("证书或提交标识无效")
	}
	db := global.GWAF_LOCAL_DB
	var p model.SslRenewalPolicy
	if err := db.First(&p, "id = ?", id).Error; err != nil {
		return order, false, err
	}
	if automatic && !p.Enabled {
		return order, false, nil
	}
	if p.Status == "manual" {
		return order, false, errors.New("请先确认此证书的自动申请来源")
	}
	var active model.SslOrder
	if err := db.Where("certificate_id = ? AND apply_status IN ?", id, []string{"submitted", "applying"}).First(&active).Error; err == nil {
		return active, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return order, false, err
	}
	if time.Time(p.RetryNotBefore).After(now) {
		return order, false, fmt.Errorf("证书机构限流，最早可重试时间：%s", time.Time(p.RetryNotBefore).Format(time.RFC3339))
	}
	var certificate model.SslConfig
	if err := db.First(&certificate, "id = ?", id).Error; err != nil {
		return order, false, err
	}
	if CertificateFingerprint(certificate.CertContent) != p.Fingerprint {
		return order, false, errors.New("证书内容已经变更，请重新确认更新来源")
	}
	bindings, err := CertificateBindings(db, id)
	if err != nil {
		return order, false, err
	}

	var last model.SslOrder
	if p.LastOrderID != "" {
		if err := db.First(&last, "id = ?", p.LastOrderID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return order, false, err
		}
	}
	resume := last.ApplyStatus == "fail" && (last.Stage == "deploy" || last.Stage == "export") && len(last.ResultCertificate) > 0
	if p.HostCode != "" && resume && last.Operation == "issuance" {
		target, err := certificateTargetBinding(p.HostCode)
		if err != nil {
			return order, false, err
		}
		if target.ID != p.TargetID {
			return order, false, errors.New("原服务已删除或重建")
		}
	} else if p.HostCode != "" {
		if certificateTargetID(bindings, p.HostCode) != p.TargetID {
			return order, false, errors.New("原服务已经删除、重建或解除绑定，请重新确认使用范围")
		}
	}
	if !resume {
		if _, err := ValidateSSLOrderChallenge(p.ApplyMethod, p.ApplyDNS, p.CredentialGroup, p.ApplyDomain, p.SkipDNSVerify); err != nil {
			return order, false, err
		}
		if err := ValidateSSLCertificateTarget(p.HostCode, p.ApplyMethod, p.ApplyDomain); err != nil {
			return order, false, err
		}
		if p.ApplyPlatform == "zerossl" && !GetZeroSSLAccountStatus().Configured {
			return order, false, errors.New(ZeroSSLAccountMissing)
		}
	}
	requestKey := sslSubmissionHash([]string{p.USER_CODE, p.Tenant_ID, "certificate", requestID})
	fingerprint := sslSubmissionHash([]any{id, p.Version, automatic, last.Id, resume})
	created := false
	err = db.Transaction(func(tx *gorm.DB) error {
		lock := tx.Model(&model.SslRenewalPolicy{}).Where("id = ? AND version = ?", id, p.Version).Update("update_time", customtype.JsonTime(now))
		if lock.Error != nil {
			return lock.Error
		}
		if lock.RowsAffected != 1 {
			return ErrCertificateConflict
		}
		err := tx.Where("request_id = ?", requestKey).First(&order).Error
		if err == nil {
			if order.CertificateID != id || order.PolicyVersion != p.Version {
				return ErrSSLOrderRequestConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := sslOrderPendingScope(tx, p.USER_CODE, p.Tenant_ID, p.HostCode, p.ApplyDomain).Where("apply_status IN ?", []string{"submitted", "applying"}).First(&order).Error; err == nil {
			if order.CertificateID == id {
				return nil
			}
			return ErrSSLOrderActive
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if automatic {
			var attempts []model.SslOrder
			if err := tx.Where("certificate_id = ? AND trigger = ? AND create_time >= ?", id, "automatic", customtype.JsonTime(now.Add(-24*time.Hour))).Order("create_time asc").Find(&attempts).Error; err != nil {
				return err
			}
			if len(attempts) >= 6 {
				next := time.Time(attempts[0].CREATE_TIME).Add(24*time.Hour + time.Minute)
				return tx.Model(&p).Updates(map[string]any{"next_attempt_at": customtype.JsonTime(next), "status": "retry_wait"}).Error
			}
		}
		candidate := certificatePolicyOrder(p)
		candidate.BaseOrm = baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: p.USER_CODE, Tenant_ID: p.Tenant_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}
		active := sslOrderActiveKey(p.USER_CODE, p.Tenant_ID, p.HostCode, p.ApplyDomain)
		candidate.RequestID, candidate.SubmissionHash, candidate.ActiveKey = &requestKey, fingerprint, &active
		candidate.ApplyStatus, candidate.Stage, candidate.Progress = "submitted", "issue", "等待证书任务处理"
		candidate.Operation = "renewal"
		candidate.Trigger = "manual"
		if automatic {
			candidate.Trigger = "automatic"
		}
		candidate.BindingSnapshot, candidate.OriginalFingerprint = certificateSnapshot(bindings), p.Fingerprint
		if err := restoreCertificateContext(p, certificate, &candidate); err != nil {
			return err
		}
		if last.ApplyStatus == "fail" {
			candidate.RetryOf = last.Id
		}
		if resume {
			candidate.Stage = last.Stage
			candidate.Operation = last.Operation
			candidate.ApplyKey = last.ApplyKey
			candidate.ResultDomain, candidate.ResultCertURL, candidate.ResultCertStableURL = last.ResultDomain, last.ResultCertURL, last.ResultCertStableURL
			candidate.ResultCertificate, candidate.ResultPrivateKey, candidate.ResultValidTo = last.ResultCertificate, last.ResultPrivateKey, last.ResultValidTo
			candidate.ResultIssuerCertificate, candidate.ResultCSR = last.ResultIssuerCertificate, last.ResultCSR
			candidate.BindingSnapshot = last.BindingSnapshot
			candidate.OriginalFingerprint = last.OriginalFingerprint
		} else if !time.Time(certificate.ValidTo).After(now) {
			candidate.ResultCertificate, candidate.ResultPrivateKey = nil, nil
			candidate.Progress = "证书已过期，使用已保存配置重新申请"
		}
		if err := tx.Create(&candidate).Error; err != nil {
			return err
		}
		if err := tx.Model(&p).Updates(map[string]any{"status": "queued", "last_order_id": candidate.Id, "last_attempt_at": customtype.JsonTime(now), "next_attempt_at": nil}).Error; err != nil {
			return err
		}
		order, created = candidate, true
		return nil
	})
	if err == nil && created {
		_ = WafSSLOrderServiceApp.AppendLog(order.Id, "info", order.Progress)
	}
	return order, created, err
}

// Classify typed rate limits and network errors. Unknown failures require
// investigation; they are not blindly replayed against the CA.
func ClassifyCertificateFailure(err error, stage string, now time.Time) (string, time.Time) {
	for _, term := range []string{"手动替换", "已经变更", "已由其他操作", "已经删除", "已删除", "已重建", "解除绑定", "绑定快照"} {
		if strings.Contains(err.Error(), term) {
			return "configuration", time.Time{}
		}
	}
	if stage == "deploy" || stage == "export" {
		return "deployment", time.Time{}
	}
	var limit *acme.RateLimitedError
	if errors.As(err, &limit) {
		next := now.Add(6 * time.Hour)
		if seconds, e := strconv.Atoi(limit.RetryAfter); e == nil && seconds >= 0 {
			next = now.Add(time.Duration(seconds) * time.Second)
		} else if t, e := http.ParseTime(limit.RetryAfter); e == nil {
			next = t
		}
		if next.Before(now.Add(time.Minute)) {
			next = now.Add(time.Minute)
		}
		return "rate_limit", next
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "temporary", time.Time{}
	}
	var problem *acme.ProblemDetails
	if errors.As(err, &problem) {
		if problem.HTTPStatus >= 500 || strings.HasSuffix(problem.Type, ":dns") {
			return "temporary", time.Time{}
		}
		if strings.HasSuffix(problem.Type, ":unauthorized") || problem.HTTPStatus == 401 || problem.HTTPStatus == 403 {
			return "configuration", time.Time{}
		}
	}
	text := strings.ToLower(err.Error())
	for _, term := range []string{"timeout", "timed out", "connection reset", "temporary failure", "propagation", "i/o timeout", "connection refused", "no such host"} {
		if strings.Contains(text, term) {
			return "temporary", time.Time{}
		}
	}
	for _, term := range []string{"凭据", "权限", "认证", "未配置", "不存在", "绑定", "http 文件验证", "invalid token", "unauthorized", "forbidden", "invalid credentials", "access denied"} {
		if strings.Contains(text, term) {
			return "configuration", time.Time{}
		}
	}
	return "unknown", time.Time{}
}
func certificateBackoff(count int, id string) time.Duration {
	delays := []time.Duration{15 * time.Minute, time.Hour, 4 * time.Hour}
	delay := 6 * time.Hour
	if count > 0 && count <= len(delays) {
		delay = delays[count-1]
	}
	// Stable positive jitter survives process restarts.
	hash := sslSubmissionHash([]string{id, fmt.Sprint(count)})
	jitter, _ := strconv.ParseInt(hash[:4], 16, 64)
	return delay + time.Duration(jitter%61)*time.Second
}

func RecordCertificateFailure(order model.SslOrder, cause error, now time.Time) error {
	kind, notBefore := ClassifyCertificateFailure(cause, order.Stage, now)
	if order.FailureKind != "" {
		kind = order.FailureKind
	}
	message := sanitizeCertificateMessage(order, cause.Error())
	noticeOrder := order
	if order.CertificateID != "" {
		if bindings, err := CertificateBindings(global.GWAF_LOCAL_DB, order.CertificateID); err == nil {
			for _, binding := range bindings {
				expires := time.Time(binding.ValidTo)
				if !expires.IsZero() && (time.Time(noticeOrder.ResultValidTo).IsZero() || expires.Before(time.Time(noticeOrder.ResultValidTo))) {
					noticeOrder.ResultValidTo = binding.ValidTo
				}
			}
		}
	}

	return global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status IN ?", order.Id, []string{"submitted", "applying"}).Updates(map[string]any{
			"apply_status": "fail", "active_key": nil, "stage": order.Stage, "failure_kind": kind, "result_error": message, "progress": "处理失败，请查看恢复安排",
			"retry_not_before": customtype.JsonTime(notBefore), "update_time": customtype.JsonTime(now)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		eventMessage := fmt.Sprintf("域名：%s\n服务：%s\n失败阶段：%s\n原因：%s", order.ApplyDomain, order.HostCode, order.Stage, message)
		if order.CertificateID != "" {
			var p model.SslRenewalPolicy
			if err := tx.First(&p, "id = ?", order.CertificateID).Error; err != nil {
				return err
			}
			count := p.FailureCount + 1
			state := "blocked"
			next := time.Time{}
			if p.Enabled && (kind == "temporary" || kind == "rate_limit" || kind == "deployment") {
				state = "retry_wait"
				next = now.Add(certificateBackoff(count, p.Id))
				if notBefore.After(next) {
					next = notBefore
				}
			}
			if p.Status == "manual" {
				state = "manual"
				next = time.Time{}
			}
			// Never let a stale queued job reverse a user's newer policy settings.
			if p.Version != order.PolicyVersion && order.PolicyVersion != 0 && p.Status != "manual" {
				state = "blocked"
				next = time.Time{}
			}
			if err := tx.Model(&p).Updates(map[string]any{"status": state, "last_error": message, "failure_kind": kind, "failure_count": count,
				"last_order_id": order.Id, "next_attempt_at": customtype.JsonTime(next), "retry_not_before": customtype.JsonTime(notBefore), "update_time": customtype.JsonTime(now)}).Error; err != nil {
				return err
			}
			if !next.IsZero() {
				eventMessage += "\n下次尝试：" + next.Format(time.RFC3339)
			} else {
				eventMessage += "\n自动恢复已暂停，请修复原因后重试"
			}
			var c model.SslConfig
			if err := tx.First(&c, "id = ?", p.Id).Error; err == nil {
				eventMessage += "\n当前证书有效期至：" + time.Time(c.ValidTo).Format(time.RFC3339)
			}
			// Consecutive identical failures are grouped, but a changed cause or daily
			// reminder gets a new durable event.
			if p.LastError == message && !time.Time(p.LastNoticeAt).IsZero() && now.Sub(time.Time(p.LastNoticeAt)) < 24*time.Hour {
				return nil
			}
			if err := tx.Model(&p).Update("last_notice_at", customtype.JsonTime(now)).Error; err != nil {
				return err
			}
		}
		if !time.Time(noticeOrder.ResultValidTo).IsZero() {
			eventMessage += "\n当前使用版本的最早有效期：" + time.Time(noticeOrder.ResultValidTo).Format(time.RFC3339)
		}
		return enqueueCertificateEvent(tx, noticeOrder, "failure", eventMessage, now)
	})
}

func RunCertificateRenewals(now time.Time, retriesOnly bool) error {
	if !CertificateLifecycleAvailable() {
		return nil
	}
	var policies []model.SslRenewalPolicy
	q := global.GWAF_LOCAL_DB.Where("enabled = ?", true)
	if retriesOnly {
		q = q.Where("status = ? AND next_attempt_at <= ?", "retry_wait", customtype.JsonTime(now))
	}
	if err := q.Find(&policies).Error; err != nil {
		return err
	}
	for _, p := range policies {
		if p.Status == "blocked" || p.Status == "manual" || p.Status == "queued" || p.Status == "running" || p.Status == "deploying" {
			continue
		}
		var c model.SslConfig
		if err := global.GWAF_LOCAL_DB.First(&c, "id = ?", p.Id).Error; err != nil {
			return err
		}
		if !time.Time(p.NextAttemptAt).IsZero() && time.Time(p.NextAttemptAt).After(now) {
			continue
		}
		if p.Status != "retry_wait" {
			days := global.CurrentSettings().GCONFIG_RECORD_SSLOrder_EXPIRE_DAY
			if utils.IsIP(p.ApplyDomain) {
				days = global.CurrentSettings().GCONFIG_RECORD_SSL_IP_EXPIRE_DAY
			}
			if time.Time(c.ValidTo).Sub(now) > time.Duration(days)*24*time.Hour {
				continue
			}
		}
		key := "scheduled:" + sslSubmissionHash([]any{p.Id, p.Version, p.LastOrderID, p.FailureCount})
		order, created, err := RequestCertificateRenewal(p.Id, key, true, now)
		if err != nil {
			// Preflight failures have no CA attempt, but still have a persisted alert
			// and actionable state instead of disappearing into the runtime log.
			if errors.Is(err, ErrSSLOrderActive) || errors.Is(err, ErrCertificateConflict) {
				continue
			}
			if err := blockCertificatePreflight(p, err, now); err != nil {
				return err
			}
			continue
		}
		if created {
			WakeSSLOrderWorker(order)
		}
	}
	return nil
}
func blockCertificatePreflight(p model.SslRenewalPolicy, cause error, now time.Time) error {
	message := acmelog.Sanitize(cause.Error())
	return global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&p).Where("version = ?", p.Version).Updates(map[string]any{"status": "blocked", "failure_kind": "configuration", "last_error": message, "next_attempt_at": nil, "update_time": customtype.JsonTime(now)})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		o := certificatePolicyOrder(p)
		o.BaseOrm = p.BaseOrm
		o.Id = sslSubmissionHash([]any{p.Id, p.Version, message, now.UTC().Format("2006-01-02")})
		return enqueueCertificateEvent(tx, o, "blocked", "续期无法开始："+message, now)
	})
}
func PrepareInitialCertificateOrder(o *model.SslOrder) error {
	if !CertificateLifecycleAvailable() {
		return nil
	}
	if o.RenewFrom != "" {
		return errors.New("续期需要使用证书的续期策略")
	}
	if o.HostCode == "" {
		if o.ApplyMethod != "dns01" {
			return errors.New("独立申请请使用 DNS 验证")
		}
		o.BindingSnapshot, o.OriginalFingerprint = certificateSnapshot([]CertificateBinding{}), ""
		o.Stage, o.Trigger, o.Operation = "issue", "initial", "issuance"
		return nil
	}
	b, err := certificateTargetBinding(o.HostCode)
	if err != nil {
		return err
	}
	o.BindingSnapshot = certificateSnapshot([]CertificateBinding{b})
	o.OriginalFingerprint = b.Fingerprint
	o.Stage, o.Trigger, o.Operation = "issue", "initial", "issuance"
	return nil
}
func ValidateClaimedCertificateOrder(o model.SslOrder) error {
	var active model.SslOrder
	if err := global.GWAF_LOCAL_DB.Select("Id", "ApplyStatus").First(&active, "id = ?", o.Id).Error; err != nil {
		return err
	}
	if active.ApplyStatus != "applying" {
		return errors.New("任务已结束或失去执行权")
	}

	if o.CertificateID == "" {
		return nil
	}
	var p model.SslRenewalPolicy
	if err := global.GWAF_LOCAL_DB.First(&p, "id = ?", o.CertificateID).Error; err != nil {
		return err
	}
	if p.Status == "manual" {
		return errors.New("证书更新方式已切换或来源尚未确认，已停止旧任务")
	}
	if o.Trigger == "automatic" && (!p.Enabled || p.Version != o.PolicyVersion) {
		return errors.New("自动续期设置已改变，任务不再执行")
	}
	var c model.SslConfig
	if err := global.GWAF_LOCAL_DB.First(&c, "id = ?", o.CertificateID).Error; err != nil {
		return err
	}
	fp := CertificateFingerprint(c.CertContent)
	if fp != o.OriginalFingerprint && fp != CertificateFingerprint(string(o.ResultCertificate)) {
		return errors.New("证书已手动替换，任务停止以保护新证书")
	}
	return nil
}
func RecordIssuedCertificate(o *model.SslOrder) error {
	o.Stage, o.Progress, o.UPDATE_TIME = "deploy", "证书已签发，正在保存证书并处理关联服务", customtype.JsonTime(time.Now())
	result := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", o.Id, "applying").Select("Stage", "ApplyKey", "ResultDomain", "ResultCertURL", "ResultCertStableURL", "ResultCertificate", "ResultPrivateKey", "ResultIssuerCertificate", "ResultCSR", "ResultValidTo", "Progress", "UPDATE_TIME").Updates(o)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("任务已失去执行权，未提交签发结果")
	}
	return nil
}

func CompleteCertificateOrder(o model.SslOrder, now time.Time) error {
	return global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", o.Id, "applying").Updates(map[string]any{
			"apply_status": "success", "active_key": nil, "stage": "complete", "result_error": "", "failure_kind": "", "progress": "证书已保存到证书管理",
			"deployment_result": o.DeploymentResult, "update_time": customtype.JsonTime(now)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("任务已结束，不能再次提交完成结果")
		}
		var p model.SslRenewalPolicy
		if err := tx.First(&p, "id = ?", o.CertificateID).Error; err != nil {
			return err
		}
		var current model.SslConfig
		if err := tx.Omit("KeyContent").First(&current, "id = ?", o.CertificateID).Error; err != nil {
			return err
		}
		if CertificateFingerprint(current.CertContent) != CertificateFingerprint(string(o.ResultCertificate)) {
			return errors.New("证书已被手动替换，未覆盖最新策略")
		}
		changes := map[string]any{"status": "idle", "last_order_id": o.Id, "last_success_id": o.Id, "last_success_at": customtype.JsonTime(now), "failure_count": 0,
			"last_error": "", "failure_kind": "", "next_attempt_at": nil, "retry_not_before": nil, "fingerprint": CertificateFingerprint(string(o.ResultCertificate)),
			"update_time": customtype.JsonTime(now)}
		if p.Status == "manual" {
			changes["status"] = "manual"
		}
		if err := tx.Model(&p).Updates(changes).Error; err != nil {
			return err
		}
		p.IssuanceContext = contextFromOrder(o)
		if err := tx.Model(&p).Select("IssuanceContext").Updates(&p).Error; err != nil {
			return err
		}
		kind := "success"
		if p.FailureCount > 0 {
			kind = "recovered"
		}
		return enqueueCertificateEvent(tx, o, kind, fmt.Sprintf("证书已更新。域名：%s\n有效期至：%s", o.ApplyDomain, time.Time(o.ResultValidTo).Format(time.RFC3339)), now)
	})
}
func certificateEventID(order model.SslOrder, kind string) string {
	return sslSubmissionHash([]string{order.USER_CODE, order.Tenant_ID, order.Id, kind})
}

// RetryIssuedCertificate preserves a successful CA result even when saving the
// first certificate resource failed. No policy/history synthesis is necessary.
func RetryIssuedCertificate(source model.SslOrder, requestID string) (model.SslOrder, bool, error) {
	var order model.SslOrder
	key := sslSubmissionHash([]string{source.USER_CODE, source.Tenant_ID, "issued-retry", source.Id, requestID})
	active := sslOrderActiveKey(source.USER_CODE, source.Tenant_ID, source.HostCode, source.ApplyDomain)
	created := false
	err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SslOrder{}).Where("id = ?", source.Id).Update("update_time", source.UPDATE_TIME).Error; err != nil {
			return err
		}
		if err := tx.Where("request_id = ?", key).First(&order).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := sslOrderPendingScope(tx.Model(&model.SslOrder{}), source.USER_CODE, source.Tenant_ID, source.HostCode, source.ApplyDomain).Where("apply_status IN ?", []string{"submitted", "applying"}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrSSLOrderActive
		}
		order = source
		now := customtype.JsonTime(time.Now())
		order.Id, order.CREATE_TIME, order.UPDATE_TIME = uuid.GenUUID(), now, now
		order.RequestID, order.ActiveKey = &key, &active
		order.SubmissionHash = sslSubmissionHash([]string{source.Id, requestID})
		order.RetryOf, order.Trigger, order.ApplyStatus, order.Progress = source.Id, "manual", "submitted", "沿用已签发材料，等待恢复更新"
		order.FailureKind, order.ResultError = "", ""
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return order, created, err
}

func enqueueCertificateEvent(tx *gorm.DB, o model.SslOrder, kind, message string, now time.Time) error {
	severity := "warn"
	if kind == "success" || kind == "recovered" {
		severity = "info"
	} else if !time.Time(o.ResultValidTo).IsZero() && time.Time(o.ResultValidTo).Sub(now) <= 72*time.Hour {
		severity = "critical"
	}
	e := model.CertificateEvent{BaseOrm: baseorm.BaseOrm{Id: certificateEventID(o, kind), USER_CODE: o.USER_CODE, Tenant_ID: o.Tenant_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)},
		CertificateID: o.CertificateID, OrderID: o.Id, Kind: kind, Severity: severity, Domain: o.ApplyDomain, Message: acmelog.Sanitize(message)}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&e).Error
}

func sanitizeCertificateMessage(order model.SslOrder, message string) string {
	settings := global.CurrentSettings()
	secrets := []string{settings.GCONFIG_ZEROSSL_ACCESS_KEY, settings.GCONFIG_ZEROSSL_EAB_KID, settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY}
	if order.ApplyMethod == "dns01" {
		if credentials, err := LoadDNSCredentials(order.ApplyDns, order.PrivateGroupName); err == nil {
			for _, value := range credentials {
				if value != "" {
					secrets = append(secrets, value)
				}
			}
		}
	}
	return acmelog.SanitizeValues(message, secrets...)
}
func SafeCertificateMessage(order model.SslOrder, message string) string {
	return sanitizeCertificateMessage(order, message)
}
