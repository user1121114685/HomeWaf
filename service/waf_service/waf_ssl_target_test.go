package waf_service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"HomeWaf/model"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafpipeline"
)

func TestSSLCertificateTargetsAndNativeBinding(t *testing.T) {
	oldDialect := func() (d dialect.DBDialect) { defer func() { _ = recover() }(); return dialect.Get() }()
	dialect.Register(&dialect.SQLiteDialect{})
	t.Cleanup(func() { dialect.Register(oldDialect) })
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.SslConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&model.Hosts{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Hosts{BaseOrm: pipelineORM(), Code: "global-only", Host: "*", GLOBAL_HOST: 1}).Error; err != nil {
		t.Fatal(err)
	}
	config := wafpipeline.PortConfig{Schema: 1, Port: 15000, Name: "家庭 NAS", Protocol: "http", HostNames: []string{"nas.example.com", "*.example.com"}, Upstream: "http://127.0.0.1:5000", Policy: newServicePolicyFixture()}
	config.Policy.Gate.Enabled = false
	config, err := s.Save(PortSaveRequest{Config: config})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := GetSSLCertificateTargets()
	if err != nil || len(targets) != 1 {
		t.Fatalf("native-only setup must have one selectable target: %+v %v", targets, err)
	}
	target := targets[0]
	if target.Code != "pipeline-15000" || target.Source != "pipeline" || target.HTTP01Ready || len(target.Domains) != 2 {
		t.Fatalf("wrong target: %+v", target)
	}
	if err := ValidateSSLCertificateTarget(target.Code, "dns01", "nas.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSSLCertificateTarget(target.Code, "http01", "nas.example.com"); err == nil {
		t.Fatal("HTTP-01 accepted on 15000")
	}
	if _, err := GetSSLCertificateTarget("global-only"); err == nil {
		t.Fatal("global rules selected as certificate target")
	}
	if _, err := GetSSLCertificateTarget("pipeline-15001"); err == nil {
		t.Fatal("unknown service accepted")
	}

	makeCertificate := func() model.SslConfig {
		cert, key := genExportTestCert(t, "nas.example.com")
		result := model.SslConfig{BaseOrm: pipelineORM()}
		if err := result.FillByCertAndKey(cert, key); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := makeCertificate()
	id, err := s.ApplySSLCertificate(target, first, false)
	if err != nil {
		t.Fatal(err)
	}
	var stored model.PortConfig
	if err := db.First(&stored, "port = ?", 15000).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodePort(stored)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Secrets.TLSCert != first.CertContent || managedCertificateID(snapshot.Secrets) != id {
		t.Fatal("issued certificate was not persistently bound to the service")
	}
	unchanged := snapshot.Config
	unchanged.Revision = config.Revision
	if !reflect.DeepEqual(unchanged, config) {
		t.Fatal("issuing certificate changed protocol, listening state or protection policy")
	}
	var raw string
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", 15000).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "PRIVATE KEY") {
		t.Fatal("certificate key stored without encryption")
	}
	eligible, err := GetSSLRenewalHosts()
	if err != nil || len(eligible) != 1 || eligible[0].Code != target.Code {
		t.Fatalf("native service missing from auto-renewal: %+v %v", eligible, err)
	}

	second := makeCertificate()
	renewedID, err := s.ApplySSLCertificate(target, second, true)
	if err != nil || renewedID != id {
		t.Fatalf("renewal lost stable certificate binding: %s %v", renewedID, err)
	}
	third := makeCertificate()
	third.Id = id
	if err := WafSslConfigServiceApp.ModifyInner(third); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshSourceCertificates(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&stored, "port = ?", 15000).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = decodePort(stored)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Secrets.TLSCert != third.CertContent {
		t.Fatal("certificate library refresh did not reach native service")
	}
	public, _ := json.Marshal(target)
	if strings.Contains(string(public), "private_key") || strings.Contains(string(public), "policy") {
		t.Fatal("selector exposed private service configuration")
	}
	if _, err := s.Save(PortSaveRequest{Config: snapshot.Config, TLSCert: first.CertContent, TLSKey: first.KeyContent}); err != nil {
		t.Fatal(err)
	}
	eligible, err = GetSSLRenewalHosts()
	if err != nil || len(eligible) != 0 {
		t.Fatal("manual certificate replacement must stop managed auto-renewal")
	}
	if _, err := s.ApplySSLCertificate(target, second, true); err == nil {
		t.Fatal("in-flight renewal overwrote a manual certificate replacement")
	}
}

func TestSSLCertificateBindingPreservesOtherSNISources(t *testing.T) {
	oldCert, oldKey := genExportTestCert(t, "camera.example.com")
	newCert, newKey := genExportTestCert(t, "nas.example.com")
	secrets := wafpipeline.PortSecrets{TLSCert: oldCert, TLSKey: oldKey, TLSBindings: map[string]int{"legacy-nas": 0, "legacy-camera": 0}}
	if err := bindCertificateSource(&secrets, pipelineTLSRefresh{certificateSourcePrefix + "new", newCert, newKey}); err != nil {
		t.Fatal(err)
	}
	if secrets.TLSCert != newCert || secrets.TLSBindings["legacy-camera"] != 1 || secrets.TLSBindings["legacy-nas"] != 1 || secrets.AdditionalTLS[0].Certificate != oldCert {
		t.Fatal("native issuance removed another SNI certificate")
	}
	if err := refreshManagedTLS(&secrets, pipelineTLSRefresh{"legacy-nas", newCert, newKey}); err != nil {
		t.Fatal(err)
	}
	pairs := append([]wafpipeline.TLSCertificate{{Certificate: secrets.TLSCert, PrivateKey: secrets.TLSKey}}, secrets.AdditionalTLS...)
	if pairs[secrets.TLSBindings["legacy-camera"]].Certificate != oldCert || secrets.TLSCert != newCert {
		t.Fatal("legacy renewal overwrote another source or the default")
	}
}
