package waf_service

import (
	"HomeWaf/model"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"testing"
)

func TestMigratedSourceCertificateRefreshPersistsWithoutChangingOtherHost(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	fixture := httptest.NewTLSServer(nil)
	defer fixture.Close()
	key := fixture.TLS.Certificates[0].PrivateKey
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	private := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	original := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.TLS.Certificates[0].Certificate[0]}))
	cert := fixture.Certificate()
	cert.SerialNumber = big.NewInt(98765)
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, cert.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	renewed := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	var sources []model.Hosts
	if err := db.Find(&sources).Error; err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		source.Ssl = 1
		source.Certfile = original
		source.Keyfile = private
		if err := db.Save(&source).Error; err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err != nil {
		t.Fatal(err)
	}
	var nas model.Hosts
	if err := db.First(&nas, "code = ?", "nas").Error; err != nil {
		t.Fatal(err)
	}
	nas.Certfile = renewed
	if err := db.Save(&nas).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshSourceCertificates(); err != nil {
		t.Fatal(err)
	}
	var stored model.PortConfig
	if err := db.First(&stored, "port = ?", 5300).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodePort(stored)
	if err != nil {
		t.Fatal(err)
	}
	pairs := append([]string{snapshot.Secrets.TLSCert}, func() []string {
		var out []string
		for _, pair := range snapshot.Secrets.AdditionalTLS {
			out = append(out, pair.Certificate)
		}
		return out
	}()...)
	if pairs[snapshot.Secrets.TLSBindings["nas"]] != renewed || pairs[snapshot.Secrets.TLSBindings["camera"]] != original {
		t.Fatal("renewing one source changed the other SNI identity")
	}
	if snapshot.Config.Revision != 2 || snapshot.Config.ListenEnabled {
		t.Fatal("refresh lost revision or activated draft")
	}
	nas.Certfile = "invalid"
	if err := db.Save(&nas).Error; err != nil {
		t.Fatal(err)
	}
	if s.RefreshSourceCertificates() == nil {
		t.Fatal("bad source certificate accepted")
	}
	if err := db.First(&stored, "port = ?", 5300).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 2 {
		t.Fatal("invalid refresh persisted a new revision")
	}
}
