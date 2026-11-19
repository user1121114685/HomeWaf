// HomeWaf modification (2026-11-17): resume deployment without reissuing.
package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type CertificateDeployment struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func DeployCertificateOrder(o *model.SslOrder) error {
	snapshot, err := parseCertificateSnapshot(o.BindingSnapshot)
	if err != nil {
		return err
	}
	if err := ValidateClaimedCertificateOrder(*o); err != nil {
		return err
	}
	db := global.GWAF_LOCAL_DB
	first := o.Operation == "issuance"
	if o.HostCode != "" && len(snapshot) == 0 {
		return errors.New("关联服务的证书绑定快照缺失")
	}
	newFingerprint := CertificateFingerprint(string(o.ResultCertificate))
	if newFingerprint == "" {
		return errors.New("签发结果不是有效 PEM 证书")
	}
	// Validate the original target before persisting the issued resource.
	if first && o.HostCode == "" && len(snapshot) != 0 {
		return errors.New("独立申请不能包含未选择的服务绑定")
	}
	if first && o.HostCode != "" {
		if len(snapshot) != 1 || snapshot[0].Code != o.HostCode {
			return errors.New("证书绑定快照缺失或与所选服务不一致")
		}
		current, err := certificateTargetBinding(o.HostCode)
		if err != nil {
			return err
		}
		if current.ID != snapshot[0].ID || (current.Fingerprint != snapshot[0].Fingerprint && current.Fingerprint != newFingerprint) {
			return errors.New("服务证书已由其他操作替换，请重新确认使用范围")
		}
	}
	var c model.SslConfig
	if o.CertificateID != "" {
		if err := db.First(&c, "id = ?", o.CertificateID).Error; err != nil {
			return err
		}
	}
	if c.Id == "" {
		c.BaseOrm = baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: o.USER_CODE, Tenant_ID: o.Tenant_ID, CREATE_TIME: customtype.JsonTime(time.Now()), UPDATE_TIME: customtype.JsonTime(time.Now())}
	}
	old := c
	if err := validateCertKeyPair(string(o.ResultCertificate), string(o.ResultPrivateKey)); err != nil {
		return err
	}
	if err := c.FillByCertAndKey(string(o.ResultCertificate), string(o.ResultPrivateKey)); err != nil {
		return err
	}
	c.AutoLoadPath = 0
	oldID, oldVersion := o.CertificateID, o.PolicyVersion
	err = db.Transaction(func(tx *gorm.DB) error {
		if o.CertificateID == "" {
			if err := tx.Create(&c).Error; err != nil {
				return err
			}
			targetID := ""
			if o.HostCode != "" {
				targetID = snapshot[0].ID
			}
			p := newCertificatePolicy(c, *o, targetID)
			p.Status, p.LastSuccessID, p.LastSuccessAt = "deploying", "", customtype.JsonTime{}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			o.CertificateID, o.PolicyVersion = c.Id, p.Version
		} else {
			if CertificateFingerprint(old.CertContent) != o.OriginalFingerprint && CertificateFingerprint(old.CertContent) != newFingerprint {
				return errors.New("证书内容已变更，取消更新")
			}
			result := tx.Model(&model.SslConfig{}).Where("id = ? AND cert_content = ? AND auto_load_path = ?", c.Id, old.CertContent, old.AutoLoadPath).
				Select("CertContent", "KeyContent", "SerialNo", "Subject", "Issuer", "Domains", "ValidFrom", "ValidTo", "AutoLoadPath").Updates(&c)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				var current model.SslConfig
				if err := tx.First(&current, "id = ?", c.Id).Error; err != nil {
					return err
				}
				if current.CertContent != c.CertContent || current.KeyContent != c.KeyContent || current.AutoLoadPath != c.AutoLoadPath {
					return errors.New("证书已被其他操作修改")
				}
			}
		}
		if err := tx.Model(&model.SslOrder{}).Where("id = ?", o.Id).Updates(map[string]any{"certificate_id": o.CertificateID, "policy_version": o.PolicyVersion, "stage": "deploy"}).Error; err != nil {
			return err
		}
		return tx.Model(&model.SslRenewalPolicy{}).Where("id = ?", o.CertificateID).Update("fingerprint", newFingerprint).Error
	})
	if err != nil {
		o.CertificateID, o.PolicyVersion = oldID, oldVersion
		return err
	}
	results := []CertificateDeployment{}
	var failures []error
	var currentBindings []CertificateBinding
	if !first {
		currentBindings, err = CertificateBindings(db, c.Id)
		if err != nil {
			return err
		}
	}
	for _, b := range snapshot {
		item := CertificateDeployment{Name: b.Name, Kind: b.Kind, Status: "updated"}
		if !first && !certificateHasBinding(currentBindings, b) {
			item.Status, item.Error = "detached", "此使用关系已解除，未覆盖新配置"
			results = append(results, item)
			continue
		}
		var currentCert model.SslConfig
		if err := db.Omit("KeyContent").First(&currentCert, "id = ?", c.Id).Error; err != nil {
			return err
		}
		if CertificateFingerprint(currentCert.CertContent) != newFingerprint {
			return errors.New("证书在更新服务前被手动替换，已停止")
		}
		err = deployCertificateBinding(b, c, first)
		if err != nil {
			item.Status, item.Error = "failed", err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", b.Name, err))
		}
		results = append(results, item)
	}
	raw, _ := json.Marshal(results)
	o.DeploymentResult = string(raw)
	if err := db.Model(&model.SslOrder{}).Where("id = ?", o.Id).Update("deployment_result", o.DeploymentResult).Error; err != nil {
		return err
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	// Export is a separate resumable stage. A failed export must not reissue.
	o.Stage = "export"
	if err := db.Model(&model.SslOrder{}).Where("id = ?", o.Id).Update("stage", "export").Error; err != nil {
		return err
	}
	return ExportCertificateOrder(o)
}
func ExportCertificateOrder(o *model.SslOrder) error {
	if err := ValidateClaimedCertificateOrder(*o); err != nil {
		return err
	}
	_, err := WafSslConfigServiceApp.ExportById(o.CertificateID)
	return err
}
func deployCertificateBinding(b CertificateBinding, c model.SslConfig, initial bool) error {
	db := global.GWAF_LOCAL_DB
	switch b.Kind {
	case "pipeline":
		var row model.PortConfig
		if err := db.First(&row, "id = ? AND port = ?", b.ID, b.Port).Error; err != nil {
			return errors.New("服务已删除或重建")
		}
		snap, err := decodePort(row)
		if err != nil {
			return err
		}
		update := pipelineTLSRefresh{HostCode: b.Source, Certificate: c.CertContent, PrivateKey: c.KeyContent}
		req := PortSaveRequest{Config: snap.Config, Operator: "certificate_lifecycle"}
		if initial {
			fp := CertificateFingerprint(snap.Secrets.TLSCert)
			if fp != b.Fingerprint && fp != CertificateFingerprint(c.CertContent) {
				return errors.New("服务默认 TLS 已由其他操作修改")
			}
			update.HostCode = certificateSourcePrefix + c.Id
			req.TLSBind = &update
		} else {
			index, ok := snap.Secrets.TLSBindings[b.Source]
			if !ok {
				return errors.New("证书绑定已解除")
			}
			pem := snap.Secrets.TLSCert
			if index > 0 {
				if index > len(snap.Secrets.AdditionalTLS) {
					return errors.New("证书索引无效")
				}
				pem = snap.Secrets.AdditionalTLS[index-1].Certificate
			}
			fp := CertificateFingerprint(pem)
			if fp != b.Fingerprint && fp != CertificateFingerprint(c.CertContent) {
				return errors.New("服务证书内容已经变更")
			}
			req.TLSRefresh = &update
		}
		_, err = WafPipelineServiceApp.Save(req)
		return err
	case "manager":
		return RefreshManagerCertificateResult(c.Id, c.CertContent, c.KeyContent)
	default:
		return errors.New("未知的证书绑定类型")
	}
}
