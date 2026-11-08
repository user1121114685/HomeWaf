// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package ssl

import (
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"testing"
)

func TestRegistrationSSL(t *testing.T) {
	order := model.SslOrder{
		BaseOrm:                 baseorm.BaseOrm{},
		ApplyPlatform:           "letsencrypt",
		ApplyMethod:             "http",
		ApplyDns:                "",
		ApplyEmail:              "samwafgo@gmail.com",
		ApplyDomain:             "ssl.samwaf.com",
		ApplyStatus:             "",
		ResultDomain:            "",
		ResultCertURL:           "",
		ResultCertStableURL:     "",
		ResultPrivateKey:        nil,
		ResultCertificate:       nil,
		ResultIssuerCertificate: nil,
		ResultCSR:               nil,
		Remarks:                 "",
	}
	RegistrationSSL(order, "C:\\huawei\\goproject\\SamWaf\\data\\vhost\\ssl#samwaf#com", "https://acme-v02.api.letsencrypt.org/directory", "letsencrypt", "", "")
}
