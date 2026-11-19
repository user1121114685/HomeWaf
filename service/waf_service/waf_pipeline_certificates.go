package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/tls"
	"errors"
	"sort"
	"strings"
)

type pipelineTLSRefresh struct{ HostCode, Certificate, PrivateKey string }

const certificateSourcePrefix = "sslconfig:"

func managedCertificateID(secrets wafpipeline.PortSecrets) string {
	for source, index := range secrets.TLSBindings {
		if index == 0 && strings.HasPrefix(source, certificateSourcePrefix) {
			return strings.TrimPrefix(source, certificateSourcePrefix)
		}
	}
	return ""
}

// A newly issued certificate becomes the default for the selected service.
// Keep existing SNI sources and their certificates; never silently drop another domain.
func bindCertificateSource(secrets *wafpipeline.PortSecrets, update pipelineTLSRefresh) error {
	if !strings.HasPrefix(update.HostCode, certificateSourcePrefix) || len(update.HostCode) <= len(certificateSourcePrefix) {
		return errors.New("invalid certificate source")
	}
	if _, err := tls.X509KeyPair([]byte(update.Certificate), []byte(update.PrivateKey)); err != nil {
		return errors.New("invalid issued certificate")
	}
	if secrets.TLSBindings == nil {
		secrets.TLSBindings = map[string]int{}
	}
	if index, ok := secrets.TLSBindings[update.HostCode]; ok && index == 0 {
		secrets.TLSCert, secrets.TLSKey = update.Certificate, update.PrivateKey
		return nil
	}
	keepDefault := false
	for source, index := range secrets.TLSBindings {
		if index != 0 {
			continue
		}
		if strings.HasPrefix(source, certificateSourcePrefix) {
			delete(secrets.TLSBindings, source)
		} else {
			keepDefault = true
		}
	}
	if keepDefault {
		if len(secrets.AdditionalTLS) >= 100 {
			return errors.New("certificate quota exceeded")
		}
		secrets.AdditionalTLS = append(secrets.AdditionalTLS, wafpipeline.TLSCertificate{Certificate: secrets.TLSCert, PrivateKey: secrets.TLSKey})
		for source, index := range secrets.TLSBindings {
			if index == 0 {
				secrets.TLSBindings[source] = len(secrets.AdditionalTLS)
			}
		}
	}
	secrets.TLSCert, secrets.TLSKey = update.Certificate, update.PrivateKey
	secrets.TLSBindings[update.HostCode] = 0
	return nil
}

// ApplySSLCertificate uses the normal revision-checked save/hot-reload path.
// Obtaining a certificate never changes protocol, listening state or protection policy.
func (s *WafPipelineService) ApplySSLCertificate(target SSLCertificateTarget, certificate model.SslConfig, renewal bool) (string, error) {
	db, err := pipelineDB()
	if err != nil {
		return "", err
	}
	var row model.PortConfig
	if err := db.First(&row, "port = ?", target.Port).Error; err != nil {
		return "", err
	}
	snapshot, err := decodePort(row)
	if err != nil {
		return "", err
	}
	if pipelineCertificateCode(snapshot.Config) != target.Code {
		return "", errors.New("关联服务已变更，请重新申请")
	}
	id := managedCertificateID(snapshot.Secrets)
	if renewal && id == "" {
		return "", errors.New("该服务已解除自动证书绑定，本次续期更新已取消")
	}
	certificate.AutoLoadPath = 0
	if id == "" {
		id, err = WafSslConfigServiceApp.CreateInner(certificate)
	} else {
		var old model.SslConfig
		if err = db.First(&old, "id = ?", id).Error; err != nil {
			return "", err
		}
		certificate.Id, certificate.CertPath, certificate.KeyPath = old.Id, old.CertPath, old.KeyPath
		err = WafSslConfigServiceApp.ModifyInner(certificate)
	}
	if err != nil {
		return "", err
	}
	_, err = s.Save(PortSaveRequest{Config: snapshot.Config, Operator: "certificate_order", TLSBind: &pipelineTLSRefresh{HostCode: certificateSourcePrefix + id, Certificate: certificate.CertContent, PrivateKey: certificate.KeyContent}})
	return id, err
}

func refreshManagedTLS(secrets *wafpipeline.PortSecrets, update pipelineTLSRefresh) error {
	if _, ok := secrets.TLSBindings[update.HostCode]; !ok {
		return errors.New("certificate source is not bound to this port")
	}
	if _, err := tls.X509KeyPair([]byte(update.Certificate), []byte(update.PrivateKey)); err != nil {
		return errors.New("invalid renewed certificate")
	}
	old := append([]wafpipeline.TLSCertificate{{Certificate: secrets.TLSCert, PrivateKey: secrets.TLSKey}}, secrets.AdditionalTLS...)
	hosts := make([]string, 0, len(secrets.TLSBindings))
	for host := range secrets.TLSBindings {
		hosts = append(hosts, host)
	}
	sort.Slice(hosts, func(i, j int) bool {
		a, b := secrets.TLSBindings[hosts[i]], secrets.TLSBindings[hosts[j]]
		if a != b {
			return a < b
		}
		return hosts[i] < hosts[j]
	})
	var next []wafpipeline.TLSCertificate
	indices := map[string]int{}
	bindings := map[string]int{}
	for _, host := range hosts {
		index := secrets.TLSBindings[host]
		if index < 0 || index >= len(old) {
			return errors.New("invalid certificate source binding")
		}
		pair := old[index]
		if host == update.HostCode {
			pair = wafpipeline.TLSCertificate{Certificate: update.Certificate, PrivateKey: update.PrivateKey}
		}
		key := pair.Certificate + "\n" + pair.PrivateKey
		id, ok := indices[key]
		if !ok {
			id = len(next)
			indices[key] = id
			next = append(next, pair)
		}
		bindings[host] = id
	}
	if len(next) == 0 || len(next) > 101 {
		return errors.New("certificate quota exceeded")
	}
	secrets.TLSCert, secrets.TLSKey = next[0].Certificate, next[0].PrivateKey
	secrets.AdditionalTLS = next[1:]
	secrets.TLSBindings = bindings
	return nil
}

// Called by startup and a single background worker, never on request paths.
func (s *WafPipelineService) RefreshSourceCertificates() error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.PortConfig{}) {
		return nil
	}
	var rows []model.PortConfig
	if err := db.Order("port").Limit(wafpipeline.Ports.Limit() + 1).Find(&rows).Error; err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		snapshot, err := decodePort(row)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		pairs := append([]wafpipeline.TLSCertificate{{Certificate: snapshot.Secrets.TLSCert, PrivateKey: snapshot.Secrets.TLSKey}}, snapshot.Secrets.AdditionalTLS...)
		for code, index := range snapshot.Secrets.TLSBindings {
			if index < 0 || index >= len(pairs) {
				failures = append(failures, errors.New("invalid persisted certificate binding"))
				continue
			}
			if !strings.HasPrefix(code, certificateSourcePrefix) {
				failures = append(failures, errors.New("invalid certificate resource binding"))
				continue
			}
			var source model.SslConfig
			if err := db.Where("id = ? AND user_code = ? AND tenant_id = ?", strings.TrimPrefix(code, certificateSourcePrefix), row.USER_CODE, row.Tenant_ID).First(&source).Error; err != nil {
				failures = append(failures, err)
				continue
			}
			certificate, privateKey := source.CertContent, source.KeyContent
			if pairs[index].Certificate == certificate && pairs[index].PrivateKey == privateKey {
				continue
			}
			current, err := s.Get(uint16(row.Port))
			if err == nil {
				_, err = s.Save(PortSaveRequest{Config: current, Operator: "certificate_refresh", TLSRefresh: &pipelineTLSRefresh{code, certificate, privateKey}})
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
