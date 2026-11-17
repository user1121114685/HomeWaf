// HomeWaf modification (2026-11-17): explicit certificate ownership.
package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"sort"
	"strings"
)

type CertificateBinding struct {
	ValidTo     customtype.JsonTime `json:"valid_to"`
	Kind        string              `json:"kind"`
	ID          string              `json:"id"`
	Code        string              `json:"code"`
	Name        string              `json:"name"`
	Port        int                 `json:"port"`
	Protocol    string              `json:"protocol"`
	Listening   bool                `json:"listening"`
	Source      string              `json:"source"`
	Fingerprint string              `json:"fingerprint"`
}

func CertificateFingerprint(value string) string {
	block, _ := pem.Decode([]byte(value))
	if block == nil || block.Type != "CERTIFICATE" {
		return ""
	}
	sum := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(sum[:])
}

func CertificateBindings(db *gorm.DB, id string) ([]CertificateBinding, error) {
	result := []CertificateBinding{}
	hosts := []model.Hosts{}
	if db.Migrator().HasTable(&model.Hosts{}) {
		if err := db.Where("bind_ssl_id = ?", id).Find(&hosts).Error; err != nil {
			return nil, err
		}
		for _, h := range hosts {
			protocol := "http"
			if h.Ssl == 1 {
				protocol = "https"
			}
			result = append(result, CertificateBinding{Kind: "host", ID: h.Id, Code: h.Code, Name: h.DisplayName(), Port: h.Port, Protocol: protocol, Source: h.Code, ValidTo: CertificateExpiry(h.Certfile), Fingerprint: CertificateFingerprint(h.Certfile)})
		}
	}
	if db.Migrator().HasTable(&model.PortConfig{}) {
		var ports []model.PortConfig
		if err := db.Limit(wafpipeline.Ports.Limit() + 1).Find(&ports).Error; err != nil {
			return nil, err
		}
		for _, row := range ports {
			snapshot, err := decodePort(row)
			if err != nil {
				return nil, fmt.Errorf("端口 %d 证书绑定读取失败", row.Port)
			}
			for source, index := range snapshot.Secrets.TLSBindings {
				matches := source == certificateSourcePrefix+id
				if !matches {
					for _, h := range hosts {
						if source == h.Code {
							matches = true
						}
					}
				}
				if !matches {
					continue
				}
				pairs := append([]wafpipeline.TLSCertificate{{Certificate: snapshot.Secrets.TLSCert, PrivateKey: snapshot.Secrets.TLSKey}}, snapshot.Secrets.AdditionalTLS...)
				if index < 0 || index >= len(pairs) {
					return nil, errors.New("服务证书索引无效")
				}
				result = append(result, CertificateBinding{Kind: "pipeline", ID: row.Id, Code: pipelineCertificateCode(snapshot.Config), Name: snapshot.Config.Name, Port: row.Port, Protocol: snapshot.Config.Protocol, Listening: snapshot.Config.ListenEnabled, Source: source, ValidTo: CertificateExpiry(pairs[index].Certificate), Fingerprint: CertificateFingerprint(pairs[index].Certificate)})
			}
		}
	}
	if global.GWAF_SSL_BIND_CERT_ID == id {
		result = append(result, CertificateBinding{Kind: "manager", ID: id, Name: "管理控制台", Protocol: "https", Source: id})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Kind+result[i].ID+result[i].Source < result[j].Kind+result[j].ID+result[j].Source
	})
	return result, nil
}

func certificateTargetBinding(code string) (CertificateBinding, error) {
	target, err := GetSSLCertificateTarget(code)
	if err != nil {
		return CertificateBinding{}, err
	}
	b := CertificateBinding{Kind: target.Source, Code: code, Name: target.Label, Port: target.Port, Protocol: target.Protocol}
	if target.Source == "pipeline" {
		var row model.PortConfig
		if err := global.GWAF_LOCAL_DB.First(&row, "port = ?", target.Port).Error; err != nil {
			return b, err
		}
		snapshot, err := decodePort(row)
		if err != nil {
			return b, err
		}
		b.ID, b.Fingerprint = row.Id, CertificateFingerprint(snapshot.Secrets.TLSCert)
		b.ValidTo = CertificateExpiry(snapshot.Secrets.TLSCert)
		b.Source = certificateSourcePrefix + managedCertificateID(snapshot.Secrets)
	} else {
		var row model.Hosts
		if err := global.GWAF_LOCAL_DB.First(&row, "code = ?", code).Error; err != nil {
			return b, err
		}
		b.ID, b.Source, b.Fingerprint = row.Id, row.Code, CertificateFingerprint(row.Certfile)
		b.ValidTo = CertificateExpiry(row.Certfile)
	}
	return b, nil
}

func certificateSnapshot(bindings []CertificateBinding) string {
	raw, _ := json.Marshal(bindings)
	return string(raw)
}
func parseCertificateSnapshot(raw string) ([]CertificateBinding, error) {
	var result []CertificateBinding
	if err := json.Unmarshal([]byte(raw), &result); err != nil || result == nil {
		return nil, errors.New("证书绑定快照缺失，请重新确认使用范围")
	}
	return result, nil
}
func sameCertificateBinding(a, b CertificateBinding) bool {
	return a.Kind == b.Kind && a.ID == b.ID && a.Source == b.Source
}
func certificateHasBinding(bindings []CertificateBinding, b CertificateBinding) bool {
	for _, current := range bindings {
		if sameCertificateBinding(current, b) {
			return true
		}
	}
	return false
}
func certificateTargetID(bindings []CertificateBinding, code string) string {
	for _, b := range bindings {
		if b.Code == code && b.Kind == "pipeline" {
			return b.ID
		}
	}
	for _, b := range bindings {
		if b.Code == code && b.Kind != "manager" {
			return b.ID
		}
	}
	return ""
}
func certificateBoundToSource(bindings []CertificateBinding, code string) bool {
	return certificateTargetID(bindings, code) != ""
}
func certificateIdentityNames(value string) string {
	return strings.ToLower(canonicalSSLDomainSet(value))
}

type CertificateBindRequest struct {
	ID          string `json:"id" binding:"required,max=64"`
	HostCode    string `json:"host_code" binding:"required,max=64"`
	TargetID    string `json:"target_id" binding:"required,max=64"`
	Fingerprint string `json:"fingerprint" binding:"max=64"`
}

func CertificateBindingTargets() ([]CertificateBinding, error) {
	targets, err := GetSSLCertificateTargets()
	if err != nil {
		return nil, err
	}
	result := []CertificateBinding{}
	for _, target := range targets {
		b, err := certificateTargetBinding(target.Code)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}
func BindCertificateToService(req CertificateBindRequest) error {
	var certificate model.SslConfig
	if err := global.GWAF_LOCAL_DB.First(&certificate, "id = ?", req.ID).Error; err != nil {
		return err
	}
	target, err := GetSSLCertificateTarget(req.HostCode)
	if err != nil {
		return err
	}
	block, _ := pem.Decode([]byte(certificate.CertContent))
	if block == nil {
		return errors.New("证书内容无效")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	for _, domain := range target.Domains {
		if strings.HasPrefix(domain, "*.") {
			found := false
			for _, name := range parsed.DNSNames {
				if strings.EqualFold(name, domain) {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("证书未覆盖服务域名 %s", domain)
			}
		} else if err := parsed.VerifyHostname(domain); err != nil {
			return fmt.Errorf("证书未覆盖服务域名 %s", domain)
		}
	}
	current, err := certificateTargetBinding(req.HostCode)
	if err != nil {
		return err
	}
	if current.ID != req.TargetID || current.Fingerprint != req.Fingerprint {
		return errors.New("服务使用的证书已经变化，请重新读取后确认")
	}
	return deployCertificateBinding(current, certificate, true)
}

func CertificateExpiry(value string) customtype.JsonTime {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return customtype.JsonTime{}
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return customtype.JsonTime{}
	}
	return customtype.JsonTime(certificate.NotAfter)
}
