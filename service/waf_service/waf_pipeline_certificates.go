package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/tls"
	"errors"
	"sort"
)

type pipelineTLSRefresh struct{ HostCode, Certificate, PrivateKey string }

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
	if !db.Migrator().HasTable(&model.PortConfig{}) || !db.Migrator().HasTable(&model.Hosts{}) {
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
		if snapshot.Config.Protocol != "https" {
			continue
		}
		pairs := append([]wafpipeline.TLSCertificate{{Certificate: snapshot.Secrets.TLSCert, PrivateKey: snapshot.Secrets.TLSKey}}, snapshot.Secrets.AdditionalTLS...)
		for code, index := range snapshot.Secrets.TLSBindings {
			if index < 0 || index >= len(pairs) {
				failures = append(failures, errors.New("invalid persisted certificate binding"))
				continue
			}
			var host model.Hosts
			if err := db.Where("code = ? AND user_code = ? AND tenant_id = ?", code, row.USER_CODE, row.Tenant_ID).First(&host).Error; err != nil {
				failures = append(failures, err)
				continue
			}
			if pairs[index].Certificate == host.Certfile && pairs[index].PrivateKey == host.Keyfile {
				continue
			}
			current, err := s.Get(uint16(row.Port))
			if err == nil {
				_, err = s.Save(PortSaveRequest{Config: current, Operator: "certificate_refresh", TLSRefresh: &pipelineTLSRefresh{code, host.Certfile, host.Keyfile}})
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
