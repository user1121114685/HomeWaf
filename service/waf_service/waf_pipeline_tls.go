package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TLS previews return certificate identities and validation results, never key material.
type PipelineTLSRequest struct {
	Port          uint16   `json:"port"`
	Revision      int64    `json:"revision"`
	HostNames     []string `json:"host_names"`
	CertificateID string   `json:"tls_certificate_id"`
	Certificate   string   `json:"tls_certificate"`
	PrivateKey    string   `json:"tls_private_key"`
}

type PipelineTLSCertificate struct {
	ID        string   `json:"id"`
	Domains   []string `json:"domains"`
	ExpiresAt string   `json:"expires_at"`
	Valid     bool     `json:"valid"`
	Reason    string   `json:"reason,omitempty"`
}

type PipelineTLSOptions struct {
	Certificates []PipelineTLSCertificate `json:"certificates"`
	Current      *PipelineTLSCertificate  `json:"current,omitempty"`
}

func certificateCoversHost(cert *x509.Certificate, host string) bool {
	if strings.HasPrefix(host, "*.") {
		for _, name := range cert.DNSNames {
			if strings.EqualFold(name, host) {
				return true
			}
		}
		return false
	}
	return cert.VerifyHostname(host) == nil
}

func checkedTLSLeaf(certificate, privateKey string) (*x509.Certificate, error) {
	if certificate == "" || privateKey == "" {
		return nil, errors.New("HTTPS 需要证书和私钥，请选择已有证书或填写 PEM")
	}
	pair, err := tls.X509KeyPair([]byte(certificate), []byte(privateKey))
	if err != nil {
		return nil, errors.New("证书或私钥不合法，或两者不匹配")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, errors.New("无法解析 TLS 证书")
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return leaf, errors.New("证书尚未生效")
	}
	if !now.Before(leaf.NotAfter) {
		return leaf, errors.New("证书已过期，请更新证书")
	}
	return leaf, nil
}

func validatePipelineTLS(secrets wafpipeline.PortSecrets, hosts []string) error {
	if err := wafpipeline.ValidateHostNames(hosts); err != nil {
		return err
	}
	leaf, err := checkedTLSLeaf(secrets.TLSCert, secrets.TLSKey)
	if err != nil {
		return err
	}
	leaves := []*x509.Certificate{leaf}
	for _, pair := range secrets.AdditionalTLS {
		if additional, err := checkedTLSLeaf(pair.Certificate, pair.PrivateKey); err == nil {
			leaves = append(leaves, additional)
		}
	}
	for _, host := range hosts {
		covered := false
		for _, cert := range leaves {
			if certificateCoversHost(cert, host) {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("证书未覆盖域名或 IP：%s", host)
		}
	}
	return nil
}

func pipelineTLSMetadata(id string, secrets wafpipeline.PortSecrets, hosts []string) PipelineTLSCertificate {
	result := PipelineTLSCertificate{ID: id, Domains: []string{}}
	leaf, _ := checkedTLSLeaf(secrets.TLSCert, secrets.TLSKey)
	if leaf != nil {
		result.Domains = append(result.Domains, leaf.DNSNames...)
		for _, ip := range leaf.IPAddresses {
			result.Domains = append(result.Domains, ip.String())
		}
		result.ExpiresAt = leaf.NotAfter.UTC().Format(time.RFC3339)
	}
	if err := validatePipelineTLS(secrets, hosts); err != nil {
		result.Reason = err.Error()
	} else {
		result.Valid = true
	}
	return result
}

func selectedPipelineCertificate(id string) (pipelineTLSRefresh, error) {
	db, err := pipelineDB()
	if err != nil {
		return pipelineTLSRefresh{}, err
	}
	if id == "" || len(id) > 64 {
		return pipelineTLSRefresh{}, errors.New("请选择证书")
	}
	var record model.SslConfig
	if err := db.Where("id = ? AND user_code = ? AND tenant_id = ?", id, global.GWAF_USER_CODE, global.GWAF_TENANT_ID).First(&record).Error; err != nil {
		return pipelineTLSRefresh{}, errors.New("所选证书不可用，请重新读取证书列表")
	}
	return pipelineTLSRefresh{HostCode: certificateSourcePrefix + record.Id, Certificate: record.CertContent, PrivateKey: record.KeyContent}, nil
}

func pipelineTLSCurrent(req PipelineTLSRequest) (wafpipeline.PortSecrets, error) {
	if req.Revision == 0 {
		return wafpipeline.PortSecrets{}, nil
	}
	db, err := pipelineDB()
	if err != nil {
		return wafpipeline.PortSecrets{}, err
	}
	var row model.PortConfig
	if err := db.Where("port = ? AND user_code = ? AND tenant_id = ?", req.Port, global.GWAF_USER_CODE, global.GWAF_TENANT_ID).First(&row).Error; err != nil {
		return wafpipeline.PortSecrets{}, errors.New("无法读取当前服务的证书")
	}
	if row.Revision != req.Revision {
		return wafpipeline.PortSecrets{}, errors.New("服务配置已变化，请重新打开后配置证书")
	}
	snapshot, err := decodePort(row)
	if err != nil {
		return wafpipeline.PortSecrets{}, err
	}
	return snapshot.Secrets, nil
}

func (s *WafPipelineService) TLSOptions(req PipelineTLSRequest) (PipelineTLSOptions, error) {
	result := PipelineTLSOptions{Certificates: []PipelineTLSCertificate{}}
	if err := wafpipeline.ValidateHostNames(req.HostNames); err != nil {
		return result, err
	}
	current, err := pipelineTLSCurrent(req)
	if err != nil {
		return result, err
	}
	if current.TLSCert != "" {
		metadata := pipelineTLSMetadata(managedCertificateID(current), current, req.HostNames)
		result.Current = &metadata
	}
	db, err := pipelineDB()
	if err != nil {
		return result, err
	}
	var records []model.SslConfig
	if err := db.Where("user_code = ? AND tenant_id = ?", global.GWAF_USER_CODE, global.GWAF_TENANT_ID).Order("valid_to desc, id asc").Find(&records).Error; err != nil {
		return result, errors.New("读取证书列表失败，请重试")
	}
	for _, record := range records {
		result.Certificates = append(result.Certificates, pipelineTLSMetadata(record.Id, wafpipeline.PortSecrets{TLSCert: record.CertContent, TLSKey: record.KeyContent}, req.HostNames))
	}
	return result, nil
}

func (s *WafPipelineService) CheckTLS(req PipelineTLSRequest) (PipelineTLSCertificate, error) {
	var empty PipelineTLSCertificate
	if len(req.Certificate) > 1<<20 || len(req.PrivateKey) > 1<<20 {
		return empty, errors.New("证书或私钥内容过大")
	}
	secrets, err := pipelineTLSCurrent(req)
	if err != nil {
		return empty, err
	}
	if req.CertificateID != "" {
		if req.Certificate != "" || req.PrivateKey != "" {
			return empty, errors.New("已有证书和手动 PEM 只能选择一种")
		}
		selected, err := selectedPipelineCertificate(req.CertificateID)
		if err != nil {
			return empty, err
		}
		// A newly selected certificate must cover the requested names itself.
		if err := validatePipelineTLS(wafpipeline.PortSecrets{TLSCert: selected.Certificate, TLSKey: selected.PrivateKey}, req.HostNames); err != nil {
			return empty, err
		}
		if err := bindCertificateSource(&secrets, selected); err != nil {
			return empty, err
		}
	} else if req.Certificate != "" || req.PrivateKey != "" {
		secrets.TLSCert, secrets.TLSKey = req.Certificate, req.PrivateKey
	}
	if err := validatePipelineTLS(secrets, req.HostNames); err != nil {
		return empty, err
	}
	return pipelineTLSMetadata(req.CertificateID, secrets, req.HostNames), nil
}
