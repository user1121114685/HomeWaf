// HomeWaf modification (2026-11-17): certificate-owned renewal configuration.
package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/mail"
	"strings"
	"time"
)

func CertificateLifecycleAvailable() bool {
	return global.GWAF_LOCAL_DB != nil && global.GWAF_LOCAL_DB.Migrator().HasTable(&model.SslRenewalPolicy{})
}

type certificateContext struct {
	AccountKey string
	Domain     string
	URL        string
	StableURL  string
	Issuer     []byte
	CSR        []byte
}

func contextFromOrder(o model.SslOrder) string {
	raw, _ := json.Marshal(certificateContext{o.ApplyKey, o.ResultDomain, o.ResultCertURL, o.ResultCertStableURL, o.ResultIssuerCertificate, o.ResultCSR})
	return string(raw)
}
func restoreCertificateContext(p model.SslRenewalPolicy, c model.SslConfig, o *model.SslOrder) error {
	var saved certificateContext
	if err := json.Unmarshal([]byte(p.IssuanceContext), &saved); err != nil {
		return errors.New("签发上下文缺失，请重新申请")
	}
	o.ApplyKey, o.ResultDomain, o.ResultCertURL, o.ResultCertStableURL = saved.AccountKey, saved.Domain, saved.URL, saved.StableURL
	o.ResultIssuerCertificate, o.ResultCSR = saved.Issuer, saved.CSR
	o.ResultCertificate, o.ResultPrivateKey, o.ResultValidTo = []byte(c.CertContent), []byte(c.KeyContent), c.ValidTo
	return nil
}
func newCertificatePolicy(c model.SslConfig, o model.SslOrder, targetID string) model.SslRenewalPolicy {
	now := customtype.JsonTime(time.Now())
	return model.SslRenewalPolicy{BaseOrm: baseorm.BaseOrm{Id: c.Id, USER_CODE: c.USER_CODE, Tenant_ID: c.Tenant_ID, CREATE_TIME: now, UPDATE_TIME: now},
		Version: 1, Enabled: o.AutoRenewEnabled(), HostCode: o.HostCode, TargetID: targetID, ApplyDomain: o.ApplyDomain, ApplyPlatform: o.ApplyPlatform,
		ApplyMethod: o.ApplyMethod, ApplyDNS: o.ApplyDns, ApplyEmail: o.ApplyEmail, CredentialGroup: o.PrivateGroupName, SkipDNSVerify: o.SkipDNSVerify,
		IssuanceContext: contextFromOrder(o), Fingerprint: CertificateFingerprint(c.CertContent), Status: "idle", LastOrderID: o.Id, LastSuccessID: o.Id, LastSuccessAt: o.UPDATE_TIME}
}

// Migration only adopts exact certificate material and an existing service
// binding. Conflicting policies are never resolved by choosing a random order.
func MigrateCertificatePolicies() error {
	if !CertificateLifecycleAvailable() {
		return nil
	}
	db := global.GWAF_LOCAL_DB
	var certs []model.SslConfig
	if err := db.Find(&certs).Error; err != nil {
		return err
	}
	for _, c := range certs {
		var existing model.SslRenewalPolicy
		err := db.First(&existing, "id = ?", c.Id).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if c.AutoLoadPath != 0 {
			continue
		}
		fp := CertificateFingerprint(c.CertContent)
		if fp == "" {
			continue
		}
		bindings, err := CertificateBindings(db, c.Id)
		if err != nil {
			return err
		}
		codes := []string{}
		for _, b := range bindings {
			if b.Code != "" {
				codes = append(codes, b.Code)
			}
		}
		if len(codes) == 0 {
			continue
		}
		var orders []model.SslOrder
		if err := db.Where("host_code IN ? AND apply_status = ?", codes, "success").Order("create_time desc").Find(&orders).Error; err != nil {
			return err
		}
		candidates := []model.SslOrder{}
		seen := map[string]bool{}
		for _, o := range orders {
			if seen[o.HostCode] {
				continue
			}
			seen[o.HostCode] = true
			if CertificateFingerprint(string(o.ResultCertificate)) == fp && certificateBoundToSource(bindings, o.HostCode) {
				candidates = append(candidates, o)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		p := newCertificatePolicy(c, candidates[0], certificateTargetID(bindings, candidates[0].HostCode))
		for _, o := range candidates[1:] {
			q := newCertificatePolicy(c, o, "")
			if q.ApplyPlatform != p.ApplyPlatform || q.ApplyMethod != p.ApplyMethod || q.ApplyDNS != p.ApplyDNS || q.CredentialGroup != p.CredentialGroup || q.Enabled != p.Enabled || q.ApplyEmail != p.ApplyEmail || q.SkipDNSVerify != p.SkipDNSVerify || certificateIdentityNames(q.ApplyDomain) != certificateIdentityNames(p.ApplyDomain) {
				p.Status, p.Enabled, p.LastError = "unresolved", false, "相同证书存在不同续期配置，请在续期设置中选择来源后确认"
			}
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
			return err
		}
		for _, o := range candidates {
			if err := db.Model(&model.SslOrder{}).Where("id = ? AND (certificate_id = ? OR certificate_id IS NULL)", o.Id, "").Update("certificate_id", c.Id).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

type CertificatePolicyEdit struct {
	ID            string `json:"id" binding:"required,max=64"`
	Version       uint64 `json:"version" binding:"required"`
	Enabled       bool   `json:"enabled"`
	Method        string `json:"apply_method" binding:"required,oneof=dns01 http01"`
	DNS           string `json:"apply_dns" binding:"max=100"`
	Email         string `json:"apply_email" binding:"required,email,max=255"`
	Group         string `json:"private_group_name" binding:"max=255"`
	SkipDNSVerify int64  `json:"skip_dns_verify"`
	SourceOrderID string `json:"source_order_id" binding:"omitempty,max=64"`
	ConfirmSource bool   `json:"confirm_source"`
}

var ErrCertificateConflict = errors.New("续期设置已被其他操作修改，请重新读取后再保存")

func SaveCertificatePolicy(req CertificatePolicyEdit) (model.SslRenewalPolicy, error) {
	var p model.SslRenewalPolicy
	if err := global.GWAF_LOCAL_DB.First(&p, "id = ?", req.ID).Error; err != nil {
		return p, err
	}
	if req.Version != p.Version {
		return p, ErrCertificateConflict
	}
	if p.Status == "unresolved" && !req.ConfirmSource {
		return p, errors.New("请核对当前显示的服务、CA 和凭据并确认采用此来源")
	}
	if p.Status == "unresolved" {
		var c model.SslConfig
		if err := global.GWAF_LOCAL_DB.First(&c, "id = ?", p.Id).Error; err != nil {
			return p, err
		}
		sources, err := certificatePolicySources(c)
		if err != nil {
			return p, err
		}
		matched := false
		for _, source := range sources {
			if source.ID == req.SourceOrderID {
				var order model.SslOrder
				if err := global.GWAF_LOCAL_DB.First(&order, "id = ?", source.ID).Error; err != nil {
					return p, err
				}
				bindings, err := CertificateBindings(global.GWAF_LOCAL_DB, c.Id)
				if err != nil {
					return p, err
				}
				chosen := newCertificatePolicy(c, order, certificateTargetID(bindings, order.HostCode))
				p.HostCode, p.TargetID, p.ApplyPlatform, p.ApplyDomain, p.IssuanceContext = chosen.HostCode, chosen.TargetID, chosen.ApplyPlatform, chosen.ApplyDomain, chosen.IssuanceContext
				matched = true
				break
			}
		}
		if !matched {
			return p, errors.New("请选择与当前证书及服务绑定一致的签发来源")
		}
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return p, errors.New("申请邮箱无效")
	}
	if req.SkipDNSVerify != 0 && req.SkipDNSVerify != 1 {
		return p, errors.New("DNS 传播检查选项无效")
	}
	if req.Enabled {
		if _, err := ValidateSSLOrderChallenge(req.Method, req.DNS, req.Group, p.ApplyDomain, req.SkipDNSVerify); err != nil {
			return p, err
		}
		if err := ValidateSSLCertificateTarget(p.HostCode, req.Method, p.ApplyDomain); err != nil {
			return p, err
		}
		if p.ApplyPlatform == "zerossl" && !GetZeroSSLAccountStatus().Configured {
			return p, errors.New(ZeroSSLAccountMissing)
		}
		var c model.SslConfig
		if err := global.GWAF_LOCAL_DB.First(&c, "id = ?", p.Id).Error; err != nil {
			return p, err
		}
		if CertificateFingerprint(c.CertContent) != p.Fingerprint {
			return p, errors.New("证书已被手动替换，请重新配置自动申请")
		}
		if c.AutoLoadPath != 0 {
			return p, errors.New("请先关闭此证书的文件自动加载，避免两个更新来源互相覆盖")
		}
	}
	if req.Method == "http01" {
		req.DNS, req.Group, req.SkipDNSVerify = "", "", 0
	}
	err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.SslRenewalPolicy{}).Where("id = ? AND version = ?", p.Id, req.Version).Updates(map[string]any{
			"enabled": req.Enabled, "version": p.Version + 1, "apply_method": req.Method, "apply_dns": req.DNS, "credential_group": req.Group, "apply_email": strings.TrimSpace(req.Email),
			"skip_dns_verify": req.SkipDNSVerify, "status": "idle", "last_error": "", "failure_kind": "", "next_attempt_at": nil, "update_time": customtype.JsonTime(time.Now())})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCertificateConflict
		}
		if req.SourceOrderID != "" {
			if err := tx.Model(&p).Select("HostCode", "TargetID", "ApplyPlatform", "ApplyDomain", "IssuanceContext").Updates(&p).Error; err != nil {
				return err
			}
		}
		// Only queued automatic work can be cancelled. An already-claimed order
		// keeps its immutable snapshot; a strategy update does not misrepresent undo.
		if err := tx.Model(&model.SslOrder{}).Where("certificate_id = ? AND apply_status = ? AND trigger = ?", p.Id, "submitted", "automatic").Updates(map[string]any{
			"apply_status": "fail", "failure_kind": "cancelled", "result_error": "续期策略已变更，未开始的自动任务已取消", "progress": "已取消等待中的自动任务", "active_key": nil}).Error; err != nil {
			return err
		}
		var active model.SslOrder
		if err := tx.Where("certificate_id = ? AND apply_status IN ?", p.Id, []string{"submitted", "applying"}).First(&active).Error; err == nil {
			status := "queued"
			if active.ApplyStatus == "applying" {
				status = "running"
			}
			if err := tx.Model(&p).Update("status", status).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.First(&p, "id = ?", p.Id).Error
	})
	return p, err
}

func certificateDisplayState(c model.SslConfig, now time.Time) string {
	if time.Time(c.ValidTo).IsZero() {
		return "invalid"
	}
	if now.Before(time.Time(c.ValidFrom)) {
		return "not_yet_valid"
	}
	if !now.Before(time.Time(c.ValidTo)) {
		return "expired"
	}
	days := global.CurrentSettings().GCONFIG_RECORD_SSLOrder_EXPIRE_DAY
	if time.Time(c.ValidTo).Sub(now) <= time.Duration(days)*24*time.Hour {
		return "expiring"
	}
	return "valid"
}

type CertificateView struct {
	Fingerprint string `json:"fingerprint"`
	model.SslConfig
	Sources      []CertificatePolicySource `json:"sources,omitempty"`
	Bindings     []CertificateBinding      `json:"bindings"`
	Policy       *model.SslRenewalPolicy   `json:"renewal"`
	Validity     string                    `json:"validity"`
	UpdateMethod string                    `json:"update_method"`
}

func CertificateDetail(id string) (CertificateView, error) {
	var c model.SslConfig
	db := global.GWAF_LOCAL_DB
	if err := db.Omit("KeyContent").First(&c, "id = ?", id).Error; err != nil {
		return CertificateView{}, err
	}
	bindings, err := CertificateBindings(db, id)
	if err != nil {
		return CertificateView{}, err
	}
	v := CertificateView{Fingerprint: CertificateFingerprint(c.CertContent), SslConfig: c, Bindings: bindings, Validity: certificateDisplayState(c, time.Now()), UpdateMethod: "manual"}
	if c.AutoLoadPath != 0 {
		v.UpdateMethod = "file"
	}
	var p model.SslRenewalPolicy
	if err := db.First(&p, "id = ?", id).Error; err == nil {
		v.Policy = &p
		if p.Status == "unresolved" {
			sources, err := certificatePolicySources(c)
			if err != nil {
				return v, err
			}
			v.Sources = sources
		}
		if p.Status != "manual" {
			v.UpdateMethod = "managed"
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return v, err
	}
	return v, nil
}

type CertificateListQuery struct {
	Page      int    `json:"pageIndex"`
	Size      int    `json:"pageSize"`
	Search    string `json:"domains" binding:"max=8192"`
	State     string `json:"state"`
	Method    string `json:"method"`
	Service   string `json:"service"`
	Attention bool   `json:"attention"`
}

func CertificateList(req CertificateListQuery) ([]CertificateView, int, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Size > 200 {
		return nil, 0, errors.New("每页最多 200 条")
	}
	var certs []model.SslConfig
	query := global.GWAF_LOCAL_DB.Omit("KeyContent").Order("valid_to asc")
	if req.Search != "" {
		query = query.Where("domains LIKE ?", "%"+req.Search+"%")
	}
	if err := query.Find(&certs).Error; err != nil {
		return nil, 0, err
	}
	views := []CertificateView{}
	for _, c := range certs {
		v, err := CertificateDetail(c.Id)
		if err != nil {
			return nil, 0, err
		}
		if req.State != "" && v.Validity != req.State {
			continue
		}
		if req.Method != "" && v.UpdateMethod != req.Method {
			continue
		}
		if req.Service != "" {
			found := false
			for _, b := range v.Bindings {
				if b.Code == req.Service {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if req.Attention {
			attention := v.Validity != "valid" || strings.Contains(c.ExportStatus, "失败")
			if v.Policy != nil {
				attention = attention || v.Policy.Status == "blocked" || v.Policy.Status == "retry_wait" || v.Policy.Status == "unresolved"
			}
			if !attention {
				continue
			}
		}
		views = append(views, v)
	}
	total := len(views)
	start := (req.Page - 1) * req.Size
	if start >= total {
		return []CertificateView{}, total, nil
	}
	end := start + req.Size
	if end > total {
		end = total
	}
	return views[start:end], total, nil
}
func certificatePolicyOrder(p model.SslRenewalPolicy) model.SslOrder {
	enabled := p.Enabled
	return model.SslOrder{CertificateID: p.Id, PolicyVersion: p.Version, HostCode: p.HostCode, ApplyDomain: p.ApplyDomain, ApplyPlatform: p.ApplyPlatform,
		ApplyMethod: p.ApplyMethod, ApplyDns: p.ApplyDNS, ApplyEmail: p.ApplyEmail, PrivateGroupName: p.CredentialGroup, SkipDNSVerify: p.SkipDNSVerify, AutoRenew: &enabled}
}
func certificateError(err error) error { return fmt.Errorf("证书操作失败：%w", err) }

type CertificatePolicySource struct {
	ID            string `json:"id"`
	HostCode      string `json:"host_code"`
	Platform      string `json:"apply_platform"`
	Method        string `json:"apply_method"`
	DNS           string `json:"apply_dns"`
	Group         string `json:"private_group_name"`
	Email         string `json:"apply_email"`
	SkipDNSVerify int64  `json:"skip_dns_verify"`
	Enabled       bool   `json:"enabled"`
}

func certificatePolicySources(c model.SslConfig) ([]CertificatePolicySource, error) {
	bindings, err := CertificateBindings(global.GWAF_LOCAL_DB, c.Id)
	if err != nil {
		return nil, err
	}
	codes := []string{}
	for _, b := range bindings {
		if b.Code != "" {
			codes = append(codes, b.Code)
		}
	}
	result := []CertificatePolicySource{}
	if len(codes) == 0 {
		return result, nil
	}
	var orders []model.SslOrder
	if err := global.GWAF_LOCAL_DB.Where("host_code IN ? AND apply_status = ?", codes, "success").Order("create_time desc").Find(&orders).Error; err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, o := range orders {
		if seen[o.HostCode] {
			continue
		}
		seen[o.HostCode] = true
		if CertificateFingerprint(string(o.ResultCertificate)) != CertificateFingerprint(c.CertContent) {
			continue
		}
		result = append(result, CertificatePolicySource{o.Id, o.HostCode, o.ApplyPlatform, o.ApplyMethod, o.ApplyDns, o.PrivateGroupName, o.ApplyEmail, o.SkipDNSVerify, o.AutoRenewEnabled()})
	}
	return result, nil
}
