package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPipelineTLSSelectionPersistsBindingAndChecksNames(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.SslConfig{}); err != nil {
		t.Fatal(err)
	}
	certificate, key := genExportTestCert(t, "nas.example.test")
	row := model.SslConfig{BaseOrm: pipelineORM(), CertContent: certificate, KeyContent: key}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	foreign := model.SslConfig{BaseOrm: pipelineORM(), CertContent: certificate, KeyContent: key}
	foreign.Tenant_ID = "other-tenant"
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	options, err := s.TLSOptions(PipelineTLSRequest{HostNames: []string{"nas.example.test"}})
	if err != nil || len(options.Certificates) != 1 || !options.Certificates[0].Valid {
		t.Fatalf("options: %+v %v", options, err)
	}
	public, _ := json.Marshal(options)
	if strings.Contains(string(public), "PRIVATE KEY") || strings.Contains(string(public), "BEGIN CERTIFICATE") {
		t.Fatal("preview leaked certificate material")
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(reservation.Addr().(*net.TCPAddr).Port)
	reservation.Close()
	config := wafpipeline.PortConfig{Schema: 1, Port: port, Name: "custom port", Protocol: "https", Upstream: "http://127.0.0.1:8080", HostNames: []string{"nas.example.test"}, Policy: newServicePolicyFixture()}
	config.Policy.Network.ListenIPVersion = "ipv4"
	if _, err := s.Save(PortSaveRequest{Config: config}); err == nil {
		t.Fatal("accepted HTTPS draft without a certificate")
	}
	if _, err := s.Save(PortSaveRequest{Config: config, TLSCertificateID: foreign.Id}); err == nil {
		t.Fatal("bound another tenant's certificate")
	}
	config, err = s.Save(PortSaveRequest{Config: config, TLSCertificateID: row.Id})
	if err != nil {
		t.Fatal(err)
	}
	var stored model.PortConfig
	if err := db.First(&stored, "port = ?", config.Port).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodePort(stored)
	if err != nil || snapshot.Config.Port != port || snapshot.Config.ListenEnabled || managedCertificateID(snapshot.Secrets) != row.Id || snapshot.Secrets.TLSCert != certificate {
		t.Fatalf("binding not persisted: %v", err)
	}
	var encrypted string
	db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", config.Port).Scan(&encrypted)
	if strings.Contains(encrypted, "PRIVATE KEY") {
		t.Fatal("stored key in plaintext")
	}
	options, err = s.TLSOptions(PipelineTLSRequest{Port: config.Port, Revision: config.Revision, HostNames: config.HostNames})
	if err != nil || options.Current == nil || !options.Current.Valid {
		t.Fatalf("reopen: %+v %v", options, err)
	}
	config.HostNames = []string{"wrong.example.test"}
	if _, err := s.Save(PortSaveRequest{Config: config}); err == nil {
		t.Fatal("saved a certificate with mismatched hostname")
	}
	unchanged, err := s.Get(config.Port)
	if err != nil || unchanged.Revision != config.Revision || unchanged.HostNames[0] != "nas.example.test" {
		t.Fatal("failed validation changed persisted service")
	}
	if _, err := s.CheckTLS(PipelineTLSRequest{Port: config.Port, Revision: config.Revision + 1, HostNames: unchanged.HostNames}); err == nil {
		t.Fatal("accepted stale service revision")
	}
	listeners := &wafpipeline.ListenerManager{}
	defer listeners.StopAll()
	listeners.SetHandler(func(_ *wafpipeline.Snapshot, w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	s.OnChange = listeners.Apply
	unchanged.ListenEnabled = true
	if _, err := s.Save(PortSaveRequest{Config: unchanged}); err != nil {
		t.Fatal(err)
	}
	assertServed := func(cert string) {
		t.Helper()
		roots := x509.NewCertPool()
		roots.AppendCertsFromPEM([]byte(cert))
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", "127.0.0.1:"+strconv.Itoa(int(port)), &tls.Config{RootCAs: roots, ServerName: "nas.example.test", MinVersion: tls.VersionTLS12})
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		block, _ := pem.Decode([]byte(cert))
		if string(conn.ConnectionState().PeerCertificates[0].Raw) != string(block.Bytes) {
			t.Fatal("listener served the wrong certificate")
		}
	}
	assertServed(certificate)
	renewed, renewedKey := genExportTestCert(t, "nas.example.test")
	if err := db.Model(&row).Updates(map[string]any{"cert_content": renewed, "key_content": renewedKey}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshSourceCertificates(); err != nil {
		t.Fatal(err)
	}
	current, _ := wafpipeline.Ports.Get(config.Port)
	if current.Secrets.TLSCert != renewed || !current.Config.ListenEnabled {
		t.Fatal("managed renewal did not update the same running service")
	}
	assertServed(renewed)
	switchConfig := wafpipeline.CloneConfig(current.Config)
	switchConfig.Protocol = "http"
	if _, err := s.Save(PortSaveRequest{Config: switchConfig}); err == nil {
		t.Fatal("restarted a running listener without confirmation")
	}
	unchanged, _ = s.Get(port)
	if unchanged.Protocol != "https" || unchanged.Revision != switchConfig.Revision {
		t.Fatal("unconfirmed switch changed configuration")
	}
	switchConfig, err = s.Save(PortSaveRequest{Config: switchConfig, ConfirmListenerRestart: true})
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Timeout: time.Second}).Get("http://127.0.0.1:" + strconv.Itoa(int(port)))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatal("confirmed protocol did not serve")
	}
	switchConfig.Protocol = "https"
	if _, err := s.Save(PortSaveRequest{Config: switchConfig, ConfirmListenerRestart: true}); err != nil {
		t.Fatal(err)
	}
	assertServed(renewed)
}

func TestPipelineTLSValidationRejectsInvalidMaterialAndSupportsIPAndWildcard(t *testing.T) {
	certificate, key := genExportTestCert(t, "*.example.test")
	pair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(pair.Certificate[0])
	leaf.IPAddresses = []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("2001:db8::1")}
	issue := func() string {
		der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, leaf.PublicKey, pair.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	secrets := wafpipeline.PortSecrets{TLSCert: issue(), TLSKey: key}
	if err := validatePipelineTLS(secrets, []string{"nas.example.test", "*.example.test", "192.0.2.10", "2001:db8::1"}); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"example.test", "deep.nas.example.test", "192.0.2.11", "2001:db8::2"} {
		if validatePipelineTLS(secrets, []string{host}) == nil {
			t.Fatalf("accepted %s", host)
		}
	}
	_, wrongKey := genExportTestCert(t, "*.example.test")
	if validatePipelineTLS(wafpipeline.PortSecrets{TLSCert: secrets.TLSCert, TLSKey: wrongKey}, nil) == nil {
		t.Fatal("accepted mismatched private key")
	}
	leaf.NotBefore, leaf.NotAfter = time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour)
	if validatePipelineTLS(wafpipeline.PortSecrets{TLSCert: issue(), TLSKey: key}, nil) == nil {
		t.Fatal("accepted expired certificate")
	}
	leaf.NotBefore, leaf.NotAfter = time.Now().Add(time.Hour), time.Now().Add(48*time.Hour)
	if validatePipelineTLS(wafpipeline.PortSecrets{TLSCert: issue(), TLSKey: key}, nil) == nil {
		t.Fatal("accepted future certificate")
	}
}
