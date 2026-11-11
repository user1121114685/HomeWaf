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
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

func unusedPort(t *testing.T) uint16 {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := uint16(l.Addr().(*net.TCPAddr).Port)
	l.Close()
	return p
}

func TestMigratedHTTPSListenerSelectsCertificateBySNI(t *testing.T) {
	old := Ports
	Ports = NewRegistry[Snapshot](200)
	defer func() { Ports = old }()
	certs := []TLSCertificate{}
	roots := x509.NewCertPool()
	for i, name := range []string{"nas.test", "camera.test"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		cert := &x509.Certificate{SerialNumber: big.NewInt(int64(i + 1)), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		pair := TLSCertificate{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))}
		certs = append(certs, pair)
		roots.AppendCertsFromPEM([]byte(pair.Certificate))
	}
	s := &Snapshot{Config: PortConfig{Port: unusedPort(t), Protocol: "https", ListenEnabled: true, Policy: DefaultPolicy()}, Secrets: PortSecrets{TLSCert: certs[0].Certificate, TLSKey: certs[0].PrivateKey, AdditionalTLS: certs[1:]}}
	s.Config.Policy.Network.ListenIPVersion = "ipv4"
	s.Config.Policy.Access.DrainSeconds = 1
	if err := Ports.Publish(s.Config.Port, func() (*Snapshot, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	m := &ListenerManager{}
	defer m.StopAll()
	m.SetHandler(func(s *Snapshot, w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	if err := m.Start(s); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nas.test", "camera.test"} {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", fmt.Sprintf("127.0.0.1:%d", s.Config.Port), &tls.Config{RootCAs: roots, ServerName: name, MinVersion: tls.VersionTLS12})
		if err != nil {
			t.Fatal(name, err)
		}
		if conn.ConnectionState().PeerCertificates[0].Subject.CommonName != name {
			t.Fatal("wrong virtual-host certificate")
		}
		conn.Close()
	}
	if conn, err := net.DialTimeout("tcp6", fmt.Sprintf("[::1]:%d", s.Config.Port), 100*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("IPv4-only migrated listener was exposed on IPv6")
	}
}
func TestIndependentPortListenersAndShutdown(t *testing.T) {
	old := Ports
	Ports = NewRegistry[Snapshot](200)
	t.Cleanup(func() { Ports = old })
	m := &ListenerManager{}
	t.Cleanup(m.StopAll)
	m.SetHandler(func(s *Snapshot, w http.ResponseWriter, r *http.Request) {
		p, e := LocalPort(r)
		if e != nil || p != s.Config.Port {
			t.Error("physical port identity mismatch")
		}
		_, _ = fmt.Fprintf(w, "%d", s.Config.Port)
	})
	a, b := unusedPort(t), unusedPort(t)
	for b == a {
		b = unusedPort(t)
	}
	for _, port := range []uint16{a, b} {
		snap := &Snapshot{Config: PortConfig{Port: port, Protocol: "http", ListenEnabled: true, Policy: DefaultPolicy()}}
		snap.Config.Policy.Access.DrainSeconds = 1
		_ = Ports.Publish(port, func() (*Snapshot, error) { return snap, nil })
		if err := m.Start(snap); err != nil {
			t.Fatal(err)
		}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	get := func(p uint16) {
		t.Helper()
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", p), nil)
		req.Host = fmt.Sprintf("forged:%d", a)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != fmt.Sprint(p) {
			t.Fatalf("port crossed: %s", body)
		}
	}
	get(a)
	get(b)
	if err := m.Stop(a); err != nil {
		t.Fatal(err)
	}
	get(b)
}
