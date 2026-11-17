// HomeWaf modification (2026-11-17): one resumable certificate execution flow.
package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/service/waf_service"
	"HomeWaf/utils"
	"HomeWaf/utils/ssl"
	"HomeWaf/wafssl"
	"errors"
	"path/filepath"
	"time"
)

func (waf *WafEngine) applyLifecycleCertificate(order model.SslOrder) {
	fail := func(err error) {
		_ = waf_service.RecordCertificateFailure(order, err, time.Now())
		_ = wafSslOrderService.AppendLog(order.Id, "error", waf_service.SafeCertificateMessage(order, err.Error()))
	}
	if err := waf_service.ValidateClaimedCertificateOrder(order); err != nil {
		order.FailureKind = "configuration"
		fail(err)
		return
	}
	if order.Stage == "" {
		if err := waf_service.PrepareInitialCertificateOrder(&order); err != nil {
			fail(err)
			return
		}
		if err := global.GWAF_LOCAL_DB.Model(&model.SslOrder{}).Where("id = ?", order.Id).Updates(map[string]any{"certificate_id": order.CertificateID, "policy_version": order.PolicyVersion, "stage": order.Stage, "trigger": order.Trigger, "operation": order.Operation, "binding_snapshot": order.BindingSnapshot, "original_fingerprint": order.OriginalFingerprint}).Error; err != nil {
			fail(err)
			return
		}
	}
	if order.CertificateID != "" {
		_ = global.GWAF_LOCAL_DB.Model(&model.SslRenewalPolicy{}).Where("id = ?", order.CertificateID).Update("status", "running").Error
	}
	if order.Stage == "issue" {
		if err := waf_service.ValidateSSLCertificateTarget(order.HostCode, order.ApplyMethod, order.ApplyDomain); err != nil {
			order.FailureKind = "configuration"
			fail(err)
			return
		}
		issued := order
		err := wafssl.WithDNSCredentials(order, func() error {
			kid, hmac := "", ""
			if order.ApplyPlatform == "zerossl" {
				if err := waf_service.EnsureZeroSSLAccount(); err != nil {
					return err
				}
				settings := global.CurrentSettings()
				kid, hmac = settings.GCONFIG_ZEROSSL_EAB_KID, settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY
			}
			path := filepath.Join(utils.GetCurrentDir(), "data", "vhost", order.HostCode)
			if order.HostCode == "" {
				path = filepath.Join(utils.GetCurrentDir(), "data", "ssl", "orders", order.Id)
			}
			if err := utils.CheckPathAndCreate(path); err != nil {
				return err
			}
			_ = wafSslOrderService.Progress(order.Id, "正在请求 CA 并验证域名")
			var err error
			if len(order.ResultCertificate) > 0 {
				issued, err = ssl.ReNewSSL(order, path, waf_service.GetCAServerAddress(order.ApplyPlatform), order.ApplyPlatform, kid, hmac)
			} else {
				issued, err = ssl.RegistrationSSL(order, path, waf_service.GetCAServerAddress(order.ApplyPlatform), order.ApplyPlatform, kid, hmac)
			}
			return err
		})
		if err != nil {
			fail(err)
			return
		}
		order = issued
		if len(order.ResultCertificate) == 0 {
			fail(errors.New("证书机构没有返回签发材料"))
			return
		}
		if err := waf_service.RecordIssuedCertificate(&order); err != nil {
			// The CA succeeded but its result was not durably stored. Never schedule
			// blind reissuance when the response may already have consumed an order.
			order.Stage = "issue"
			order.FailureKind = "unknown"
			fail(err)
			return
		}
	}
	var err error
	if order.Stage == "export" {
		err = waf_service.ExportCertificateOrder(&order)
	} else {
		err = waf_service.DeployCertificateOrder(&order)
	}
	if err != nil {
		fail(err)
		return
	}
	if err := waf_service.CompleteCertificateOrder(order, time.Now()); err != nil {
		fail(err)
		return
	}
	_ = wafSslOrderService.AppendLog(order.Id, "info", "证书已更新，服务与导出结果已记录")
}
