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

	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/model/spec"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSSLOrderActive          = errors.New("此服务已有等待中或申请中的证书订单，请等待完成后再提交")
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

// Submit persists the operation before notifying workers. Unique database keys,
// rather than a process-local mutex, protect retries across overlapping workers.
func (s *WafSSLOrderService) Submit(req request.WafSslorderaddReq, renewFrom string) (model.SslOrder, bool, error) {
	var order model.SslOrder
	req.HostCode, req.ApplyEmail = strings.TrimSpace(req.HostCode), strings.TrimSpace(req.ApplyEmail)
	if req.HostCode == "" || len(req.HostCode) > 64 || len(req.RequestID) > 128 {
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
	// Enabling the default has the same identity as submissions from older UIs.
	if *req.AutoRenew {
		fingerprintInput.AutoRenew = nil
	}
	fingerprintInput.RequestID = ""
	fingerprintInput.ApplyDomain = canonicalSSLDomainSet(req.ApplyDomain)
	fingerprint := sslSubmissionHash([]any{fingerprintInput, renewFrom})
	activeKey := sslSubmissionHash([]string{global.GWAF_USER_CODE, global.GWAF_TENANT_ID, req.HostCode})
	var requestKey *string
	if req.RequestID != "" {
		key := sslSubmissionHash([]string{global.GWAF_USER_CODE, global.GWAF_TENANT_ID, req.RequestID})
		requestKey = &key
	}
	created := false
	err = global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		// The first statement takes the write lock on SQLite. It also frees keys
		// from terminal rows written by older code during a rolling upgrade.
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
		// Include pre-migration pending orders; never delete the user's duplicates.
		var active model.SslOrder
		err := tx.Where("host_code = ? AND apply_status IN ?", req.HostCode, []string{"submitted", "applying"}).Order("create_time asc").First(&active).Error
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
	return s.Submit(request.WafSslorderaddReq{AutoRenew: req.AutoRenew, RequestID: req.RequestID, HostCode: req.HostCode, ApplyPlatform: req.ApplyPlatform, ApplyMethod: req.ApplyMethod, SkipDNSVerify: req.SkipDNSVerify, ApplyDns: req.ApplyDns, ApplyEmail: req.ApplyEmail, ApplyDomain: req.ApplyDomain, PrivateGroupName: req.PrivateGroupName}, req.Id)
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
		if order.SubmissionHash == "" {
			// Old UI double-clicks produced rows without submission keys. Keep
			// their history, but only run the first equivalent row in 5 seconds.
			var nearby []model.SslOrder
			if err := tx.Where("host_code = ? AND create_time >= ? AND create_time <= ? AND id <> ?", order.HostCode, customtype.JsonTime(time.Time(order.CREATE_TIME).Add(-5*time.Second)), order.CREATE_TIME, id).Order("create_time asc, id asc").Find(&nearby).Error; err != nil {
				return err
			}
			for _, other := range nearby {
				earlier := time.Time(other.CREATE_TIME).Before(time.Time(order.CREATE_TIME)) || (time.Time(other.CREATE_TIME).Equal(time.Time(order.CREATE_TIME)) && other.Id < order.Id)
				same := other.ApplyPlatform == order.ApplyPlatform && other.ApplyMethod == order.ApplyMethod && other.ApplyDns == order.ApplyDns && other.PrivateGroupName == order.PrivateGroupName && other.ApplyEmail == order.ApplyEmail && other.SkipDNSVerify == order.SkipDNSVerify && canonicalSSLDomainSet(other.ApplyDomain) == canonicalSSLDomainSet(order.ApplyDomain)
				if earlier && same {
					return tx.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ?", id, "submitted").Updates(map[string]any{"apply_status": "fail", "active_key": nil, "progress": "已跳过重复提交", "result_error": "检测到同一操作的重复订单，已跳过；请查看订单 " + other.Id}).Error
				}
			}
		}
		activeKey := sslSubmissionHash([]string{global.GWAF_USER_CODE, global.GWAF_TENANT_ID, order.HostCode})
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
		result := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("id = ? AND apply_status = ? AND update_time < ?", id, "applying", cutoff).Updates(map[string]any{"apply_status": "fail", "active_key": nil, "progress": "申请任务已中断", "result_error": message, "update_time": customtype.JsonTime(now)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
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
