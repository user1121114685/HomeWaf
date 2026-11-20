package wafpipeline

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"
)

func rotatingTestCertificate(t *testing.T, serial int64) TLSCertificate {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	c := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "renew.test"}, DNSNames: []string{"renew.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, c, c, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	private, e := x509.MarshalECPrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return TLSCertificate{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private}))}
}
func TestTLSRenewalUpdatesExistingListenerAndRejectsBadCandidate(t *testing.T) {
	a, b := rotatingTestCertificate(t, 101), rotatingTestCertificate(t, 102)
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(a.Certificate))
	roots.AppendCertsFromPEM([]byte(b.Certificate))
	s := &Snapshot{Config: PortConfig{Port: unusedPort(t), Protocol: "https", ListenEnabled: true, Policy: DefaultPolicy()}, Secrets: PortSecrets{TLSCert: a.Certificate, TLSKey: a.PrivateKey}}
	s.Config.Policy.Network.ListenIPVersion = "ipv4"
	s.Config.Policy.Network.DisableHTTP2 = true
	m := &ListenerManager{}
	defer m.StopAll()
	if err := m.Start(s); err != nil {
		t.Fatal(err)
	}
	listener := m.servers[s.Config.Port]
	first := listener.tls.Load()
	check := func(serial int64) {
		t.Helper()
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", fmt.Sprintf("127.0.0.1:%d", s.Config.Port), &tls.Config{RootCAs: roots, ServerName: "renew.test", NextProtos: []string{"h2", "http/1.1"}})
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		state := conn.ConnectionState()
		if state.PeerCertificates[0].SerialNumber.Int64() != serial || state.NegotiatedProtocol != "http/1.1" {
			t.Fatal("certificate or ALPN mismatch")
		}
	}
	check(101)
	next := &Snapshot{Config: CloneConfig(s.Config), Secrets: PortSecrets{TLSCert: b.Certificate, TLSKey: b.PrivateKey}}
	if err := m.Apply(s, next); err != nil {
		t.Fatal(err)
	}
	if m.servers[s.Config.Port] != listener || first == listener.tls.Load() {
		t.Fatal("renewal restarted listener or reused mutable TLS config")
	}
	check(102)
	bad := &Snapshot{Config: CloneConfig(s.Config), Secrets: PortSecrets{TLSCert: "invalid", TLSKey: b.PrivateKey}}
	if m.Apply(next, bad) == nil {
		t.Fatal("invalid certificate accepted")
	}
	check(102)
	if first.Certificates[0].Leaf.SerialNumber.Int64() != 101 {
		t.Fatal("old TLS snapshot mutated")
	}
	m.StopAll()
	if m.Start(next) == nil {
		t.Fatal("background refresh resurrected a drained listener")
	}
}
