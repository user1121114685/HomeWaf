package localca

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestReplaceServerCertRestoresOnFailedCommit(t *testing.T) {
	p := DefaultPaths(t.TempDir())
	if _, err := IssueServerCert(p, []string{"127.0.0.1"}, 0); err != nil {
		t.Fatal(err)
	}
	oldCert, _ := os.ReadFile(p.SrvCert)
	oldKey, _ := os.ReadFile(p.SrvKey)
	root := CASummary(p).Fingerprint
	err := ReplaceServerCert(p, []string{"waf.lan"}, func() error { return errors.New("config is read-only") })
	if err == nil {
		t.Fatal("commit failure hidden")
	}
	cert, _ := os.ReadFile(p.SrvCert)
	key, _ := os.ReadFile(p.SrvKey)
	if !bytes.Equal(oldCert, cert) || !bytes.Equal(oldKey, key) || CASummary(p).Fingerprint != root {
		t.Fatal("certificate or root changed on failed commit")
	}
}

func TestEnsureCARejectsPartialRoot(t *testing.T) {
	p := DefaultPaths(t.TempDir())
	if _, _, err := EnsureCA(p); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(p.CACert)
	if err := os.Remove(p.CAKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureCA(p); err == nil {
		t.Fatal("partial trusted root silently replaced")
	}
	current, _ := os.ReadFile(p.CACert)
	if !bytes.Equal(old, current) {
		t.Fatal("root certificate changed")
	}
}

func TestPublicCAReturnsCertificateOnly(t *testing.T) {
	p := DefaultPaths(t.TempDir())
	if _, _, err := EnsureCA(p); err != nil {
		t.Fatal(err)
	}
	cert, _ := os.ReadFile(p.CACert)
	key, _ := os.ReadFile(p.CAKey)
	if err := os.WriteFile(p.CACert, append(cert, key...), 0600); err != nil {
		t.Fatal(err)
	}
	public, err := ReadCACert(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(public, []byte("PRIVATE KEY")) || !bytes.Equal(public, cert) {
		t.Fatal("public download leaked extra material")
	}
}
