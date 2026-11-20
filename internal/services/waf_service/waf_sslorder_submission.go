package waf_service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/model/enums"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/model/spec"
	"HomeWaf/internal/platform/common/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSSLOrderActive          = errors.New("此服务或域名已有等待中或申请中的证书订单，请等待完成后再提交")
	ErrSSLOrderRequestConflict = errors.New("本次提交标识已用于另一份申请，请重新打开表单后提交")
)

func sslSubmissionHash(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func canonicalSSLDomainSet(domains string) string {
	parts := strings.FieldsFunc(strings.ToLower(domains), func(r rune) bool { return r == ',' || r == '\n' || r == '\r' })
	set := map[string]bool{}
	for _, p := range parts {
		set[strings.TrimSuffix(strings.TrimSpace(p), ".")] = true
	}
	parts = parts[:0]
	for p := range set {
		if p != "" {
			parts = append(parts, p)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// Bound requests retain their service lock. Independent requests lock only the
// canonical domain set, so unrelated certificates do not all share an empty host.
func sslOrderActiveKey(user, tenant, host, domains string) string {
	if host != "" {
		return sslSubmissionHash([]string{user, tenant, host})
	}
	return sslSubmissionHash([]string{user, tenant, "certificate-domains", canonicalSSLDomainSet(domains)})
}

func sslOrderPendingScope(db *gorm.DB, user, tenant, host, domains string) *gorm.DB {
	key := sslOrderActiveKey(user, tenant, host, domains)
	if host != "" {
		return db.Where("(active_key = ? OR host_code = ?)", key, host)
	}
	return db.Where("active_key = ?", key)
}

// Submit persists the operation before notifying workers. Unique database keys,
// rather than a process-local mutex, protect retries across overlapping workers.
func (s *WafSSLOrderService) Submit(req request.WafSslorderaddReq, renewFrom string) (model.SslOrder, bool, error) {
	var order model.SslOrder
	req.HostCode, req.ApplyEmail = strings.TrimSpace(req.HostCode), strings.TrimSpace(req.ApplyEmail)
	if len(req.HostCode) > 64 || len(req.RequestID) > 128 {
		return order, false, errors.New("关联服务或提交标识无效")
	}
	email, err := mail.ParseAddress(req.ApplyEmail)
	if err != nil || email.Address != req.ApplyEmail || len(req.ApplyEmail) > 255 {
		return order, false, errors.New("请填写有效的申请邮箱")
	}
	if req.ApplyPlatform == "" {
		req.ApplyPlatform = "letsencrypt"
	}
	if req.ApplyMethod != "dns01" && req.ApplyMethod != "http01" {
		return order, false, errors.New("申请方式无效")
	}
	if req.ApplyMethod == "http01" {
		if req.HostCode == "" {
			return order, false, errors.New("独立申请请使用 DNS 验证；HTTP 文件验证需要 HTTP 80 端口服务")
		}
		req.ApplyDns, req.PrivateGroupName, req.SkipDNSVerify = "", "", 0
	}
	if canonicalSSLDomainSet(req.ApplyDomain) == "" {
		return order, false, errors.New("申请域名不能为空")
	}
	if req.AutoRenew == nil {
		enabled := true
		if renewFrom != "" {
			var source model.SslOrder
			if err := global.GWAF_LOCAL_DB.First(&source, "id = ?", renewFrom).Error; err != nil {
				return order, false, err
			}
			enabled = source.AutoRenewEnabled()
		}
		req.AutoRenew = &enabled
	}
	fingerprintInput := req
	// Explicit and implicit default values have the same submission identity.
	if *req.AutoRenew {
		fingerprintInput.AutoRenew = nil
	}
	fingerprintInput.RequestID = ""
	fingerprintInput.ApplyDomain = canonicalSSLDomainSet(req.ApplyDomain)
	fingerprint := sslSubmissionHash([]any{fingerprintInput, renewFrom})
	activeKey := sslOrderActiveKey(global.GWAF_USER_CODE, global.GWAF_TENANT_ID, req.HostCode, req.ApplyDomain)
	var requestKey *string
	if req.RequestID != "" {
		key := sslSubmissionHash([]string{global.GWAF_USER_CODE, global.GWAF_TENANT_ID, req.RequestID})
		requestKey = &key
	}
	var prepared model.SslOrder
	if renewFrom == "" {
		prepared.HostCode = req.HostCode
		prepared.ApplyMethod = req.ApplyMethod
		if err := PrepareInitialCertificateOrder(&prepared); err != nil {
			return order, false, err
		}
	}
	created := false
	err = global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		// Release completed keys and take the SQLite write lock before reading.
		if err := tx.Model(&model.SslOrder{}).Where("active_key = ? AND apply_status NOT IN ?", activeKey, []string{"submitted", "applying"}).Update("active_key", nil).Error; err != nil {
			return err
		}
		if requestKey != nil {
			err := tx.Where("request_id = ?", *requestKey).First(&order).Error
			if err == nil {
				if order.SubmissionHash != fingerprint {
					return ErrSSLOrderRequestConflict
				}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		// An existing pending order owns the active service or domain key.
		var active model.SslOrder
		err := sslOrderPendingScope(tx, global.GWAF_USER_CODE, global.GWAF_TENANT_ID, req.HostCode, req.ApplyDomain).Where("apply_status IN ?", []string{"submitted", "applying"}).Order("create_time asc").First(&active).Error
		if err == nil {
			if active.SubmissionHash != fingerprint {
				return ErrSSLOrderActive
			}
			if requestKey != nil && (active.RequestID == nil || *active.RequestID != *requestKey) {
				return ErrSSLOrderActive
			}
			order = active
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := customtype.JsonTime(time.Now())
		candidate := model.SslOrder{
			BaseOrm:   baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: now, UPDATE_TIME: now},
			RequestID: requestKey, SubmissionHash: fingerprint, ActiveKey: &activeKey, RenewFrom: renewFrom, AutoRenew: req.AutoRenew,
			HostCode: req.HostCode, ApplyPlatform: req.ApplyPlatform, ApplyMethod: req.ApplyMethod, SkipDNSVerify: req.SkipDNSVerify,
			ApplyDns: req.ApplyDns, ApplyEmail: req.ApplyEmail, ApplyDomain: req.ApplyDomain, PrivateGroupName: req.PrivateGroupName, ApplyStatus: "submitted", Progress: "等待证书任务处理",
		}
		if renewFrom != "" {
			var source model.SslOrder
			if err := tx.First(&source, "id = ?", renewFrom).Error; err != nil {
				return err
			}
			if source.ApplyStatus != "success" || len(source.ResultCertificate) == 0 || len(source.ResultPrivateKey) == 0 || !time.Time(source.ResultValidTo).After(time.Now()) {
				return errors.New("原订单没有可续期的有效证书，请新建申请")
			}
			if source.HostCode != req.HostCode || canonicalSSLDomainSet(source.ApplyDomain) != canonicalSSLDomainSet(req.ApplyDomain) || source.ApplyPlatform != req.ApplyPlatform {
				return errors.New("续期不能变更关联服务、域名或 CA，请新建申请")
			}
			candidate.ApplyKey, candidate.ResultDomain = source.ApplyKey, source.ResultDomain
			candidate.ResultCertURL, candidate.ResultCertStableURL = source.ResultCertURL, source.ResultCertStableURL
			candidate.ResultPrivateKey, candidate.ResultCertificate = source.ResultPrivateKey, source.ResultCertificate
			candidate.ResultIssuerCertificate, candidate.ResultCSR, candidate.ResultValidTo = source.ResultIssuerCertificate, source.ResultCSR, source.ResultValidTo
		}
		if renewFrom == "" {
			candidate.Stage, candidate.Trigger, candidate.Operation, candidate.BindingSnapshot, candidate.OriginalFingerprint = prepared.Stage, prepared.Trigger, prepared.Operation, prepared.BindingSnapshot, prepared.OriginalFingerprint
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
		// MySQL's affected-row semantics differ; checking our generated ID works
		// with both dialects and distinguishes a conflicting unique key.
		if err := tx.First(&order, "id = ?", candidate.Id).Error; err == nil {
			created = true
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Where("active_key = ?", activeKey).First(&order).Error; err != nil {
			return err
		}
		if order.SubmissionHash != fingerprint {
			return ErrSSLOrderActive
		}
		return nil
	})
	if err == nil && created {
		_ = s.AppendLog(order.Id, "info", "申请已受理，等待后台领取任务")
	}
	return order, created, err
}

// Retry reads the saved configuration server-side. Clients cannot turn a retry
// into a different domain/credential operation or restart an active order.
func (s *WafSSLOrderService) Retry(req request.WafSslorderretryReq) (model.SslOrder, bool, error) {
	var source model.SslOrder
	if req.RequestID == "" || len(req.RequestID) > 128 {
		return source, false, errors.New("重试提交标识无效")
	}
	if err := global.GWAF_LOCAL_DB.First(&source, "id = ?", req.Id).Error; err != nil {
		return source, false, errors.New("原订单不存在或读取失败")
	}
	if CertificateLifecycleAvailable() && source.CertificateID == "" && source.ApplyStatus == "fail" && (source.Stage == "deploy" || source.Stage == "export") && len(source.ResultCertificate) > 0 {
		return RetryIssuedCertificate(source, req.RequestID)
	}
	if CertificateLifecycleAvailable() && source.CertificateID != "" {
		if source.ApplyStatus != "fail" {
			return model.SslOrder{}, false, errors.New("只能重试失败任务")
		}
		return RequestCertificateRenewal(source.CertificateID, "retry:"+sslSubmissionHash([]string{source.Id, req.RequestID}), false, time.Now())
	}
	if source.ApplyStatus != "fail" {
		return model.SslOrder{}, false, errors.New("只能重试已失败或已中断的订单；进行中的申请请先查看日志")
	}
	domains, err := ValidateSSLOrderChallenge(source.ApplyMethod, source.ApplyDns, source.PrivateGroupName, source.ApplyDomain, source.SkipDNSVerify)
	if err != nil {
		return model.SslOrder{}, false, err
	}
	if err := ValidateSSLCertificateTarget(source.HostCode, source.ApplyMethod, domains); err != nil {
		return model.SslOrder{}, false, err
	}
	if source.ApplyPlatform == "zerossl" && !GetZeroSSLAccountStatus().Configured {
		return model.SslOrder{}, false, errors.New(ZeroSSLAccountMissing)
	}
	input := request.WafSslorderaddReq{AutoRenew: source.AutoRenew, RequestID: "retry:" + sslSubmissionHash([]string{source.Id, req.RequestID}), HostCode: source.HostCode, ApplyPlatform: source.ApplyPlatform, ApplyMethod: source.ApplyMethod, SkipDNSVerify: source.SkipDNSVerify, ApplyDns: source.ApplyDns, ApplyEmail: source.ApplyEmail, ApplyDomain: domains, PrivateGroupName: source.PrivateGroupName}
	order, created, err := s.Submit(input, source.RenewFrom)
	if err == nil && created {
		_ = s.AppendLog(order.Id, "info", "复用失败订单配置重试，原订单："+source.Id)
		_ = s.AppendLog(source.Id, "info", "已创建重试申请，新订单："+order.Id)
	}
	return order, created, err
}

func (s *WafSSLOrderService) SubmitRenew(req request.WafSslordereditReq) (model.SslOrder, bool, error) {
	var source model.SslOrder
	if err := global.GWAF_LOCAL_DB.First(&source, "id = ?", req.Id).Error; err != nil {
		return model.SslOrder{}, false, err
	}
	if source.CertificateID == "" {
		return model.SslOrder{}, false, errors.New("此记录未关联证书，请从证书管理选择续期对象")
	}
	if source.HostCode != req.HostCode || canonicalSSLDomainSet(source.ApplyDomain) != canonicalSSLDomainSet(req.ApplyDomain) || source.ApplyPlatform != req.ApplyPlatform {
		return model.SslOrder{}, false, errors.New("续期不能更改服务、域名或 CA")
	}
	var p model.SslRenewalPolicy
	if err := global.GWAF_LOCAL_DB.First(&p, "id = ?", source.CertificateID).Error; err != nil {
		return model.SslOrder{}, false, err
	}
	enabled := p.Enabled
	if req.AutoRenew != nil {
		enabled = *req.AutoRenew
	}
	if enabled != p.Enabled || req.ApplyMethod != p.ApplyMethod || req.ApplyDns != p.ApplyDNS || req.ApplyEmail != p.ApplyEmail || req.PrivateGroupName != p.CredentialGroup || req.SkipDNSVerify != p.SkipDNSVerify {
		if _, err := SaveCertificatePolicy(CertificatePolicyEdit{ID: p.Id, Version: p.Version, Enabled: enabled, Method: req.ApplyMethod, DNS: req.ApplyDns, Email: req.ApplyEmail, Group: req.PrivateGroupName, SkipDNSVerify: req.SkipDNSVerify}); err != nil {
			return model.SslOrder{}, false, err
		}
	}
	if req.RequestID == "" {
		req.RequestID = uuid.GenUUID()
	}
	return RequestCertificateRenewal(source.CertificateID, req.RequestID, false, time.Now())
}

// Claim atomically rejects duplicate queue deliveries before any CA/DNS call.
func (s *WafSSLOrderService) Claim(id string) (model.SslOrder, bool, error) {
	var order model.SslOrder
	claimed := false
	err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		// Lock before reading, including SQLite's deferred transactions.
		if err := tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", id, "submitted").Update("update_time", customtype.JsonTime(time.Now())).Error; err != nil {
			return err
		}
		if err := tx.First(&order, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if order.ApplyStatus != "submitted" {
			return nil
		}
		activeKey := sslOrderActiveKey(global.GWAF_USER_CODE, global.GWAF_TENANT_ID, order.HostCode, order.ApplyDomain)
		var active model.SslOrder
		err := tx.Where("active_key = ? AND id <> ? AND apply_status IN ?", activeKey, id, []string{"submitted", "applying"}).First(&active).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", id, "submitted").Updates(map[string]any{"apply_status": "applying", "active_key": activeKey, "progress": "正在检查 CA 与 DNS 配置", "update_time": customtype.JsonTime(time.Now())})
		if result.Error != nil {
			return result.Error
		}
		claimed = result.RowsAffected == 1
		if claimed {
			return tx.First(&order, "id = ?", id).Error
		}
		return nil
	})
	return order, claimed, err
}

func (s *WafSSLOrderService) Progress(id, progress string) error {
	values := map[string]any{"update_time": customtype.JsonTime(time.Now())}
	if progress != "" {
		values["progress"] = progress
	}
	result := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", id, "applying").Updates(values)
	if result.Error == nil && result.RowsAffected == 1 && progress != "" {
		_ = s.AppendLog(id, "info", progress)
	}
	return result.Error
}

// Interrupted work is failed, never automatically replayed against the CA.
// Active workers refresh their heartbeat even while waiting for DNS propagation.
func (s *WafSSLOrderService) FailInterrupted(now time.Time) error {
	cutoff := customtype.JsonTime(now.Add(-10 * time.Minute))
	var ids []string
	if err := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("apply_status = ? AND update_time < ?", "applying", cutoff).Pluck("id", &ids).Error; err != nil {
		return err
	}
	const message = "证书任务超过 10 分钟未更新运行状态，可能因进程重启而中断。请查看申请日志，确认服务稳定运行后点击“重试”。"
	for _, id := range ids {
		result := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ? AND update_time < ?", id, "applying", cutoff).Updates(map[string]any{"apply_status": "fail", "active_key": nil, "progress": "申请任务已中断", "failure_kind": "interrupted", "result_error": message, "update_time": customtype.JsonTime(now)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			if CertificateLifecycleAvailable() {
				var o model.SslOrder
				if err := global.GWAF_LOCAL_DB.First(&o, "id = ?", id).Error; err != nil {
					return err
				}
				// This is a failure event, never an automatic replay of an uncertain CA call.
				if err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
					if o.CertificateID != "" {
						if err := tx.Model(&model.SslRenewalPolicy{}).Where("id = ?", o.CertificateID).Updates(map[string]any{"status": "blocked", "failure_kind": "interrupted", "last_error": message, "next_attempt_at": nil}).Error; err != nil {
							return err
						}
					}
					return enqueueCertificateEvent(tx, o, "interrupted", message, now)
				}); err != nil {
					return err
				}
			}
			_ = s.AppendLog(id, "error", message)
		}
	}
	return nil
}

// WakeSSLOrderWorker is only a wake-up hint. The durable pending-row scan also
// handles a full queue, a lost HTTP response, or a restart before notification.
func WakeSSLOrderWorker(order model.SslOrder) {
	kind := enums.ChanSslOrderSubmitted
	if order.RenewFrom != "" {
		kind = enums.ChanSslOrderrenew
	}
	select {
	case global.GWAF_CHAN_SSLOrder <- spec.ChanSslOrder{Type: kind, Content: order}:
	default:
	}
}

func (s *WafSSLOrderService) Pending() ([]model.SslOrder, error) {
	var orders []model.SslOrder
	err := global.GWAF_LOCAL_DB.Where("apply_status = ?", "submitted").Order("create_time asc").Limit(20).Find(&orders).Error
	return orders, err
}

func (s *WafSSLOrderService) AutomaticRenew(order model.SslOrder) (model.SslOrder, bool, error) {
	if !order.AutoRenewEnabled() {
		return model.SslOrder{}, false, nil
	}
	return s.Submit(request.WafSslorderaddReq{AutoRenew: order.AutoRenew, RequestID: fmt.Sprintf("auto:%s:%s", order.Id, time.Now().UTC().Format("2006-01-02")), HostCode: order.HostCode, ApplyPlatform: order.ApplyPlatform, ApplyMethod: order.ApplyMethod, SkipDNSVerify: order.SkipDNSVerify, ApplyDns: order.ApplyDns, ApplyEmail: order.ApplyEmail, ApplyDomain: order.ApplyDomain, PrivateGroupName: order.PrivateGroupName}, order.Id)
}
