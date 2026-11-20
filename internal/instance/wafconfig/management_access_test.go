package wafconfig

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/platform/utils/localca"
	"crypto/sha256"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func managementTestDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "conf"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf/config.yml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestManagementInitialTLSOptInAndRootRetention(t *testing.T) {
	dir := managementTestDir(t, "local_port: 26666\nsecurity:\n  ssl_enable: false\n")
	p := localca.DefaultPaths(dir)
	if err := EnsureInitialManagementTLS(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.CACert); !os.IsNotExist(err) {
		t.Fatal("existing HTTP-only instance changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "conf/config.yml"), []byte("local_port: 26666\nsecurity:\n  ssl_enable: true\n  ssl_auto_local: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialManagementTLS(dir); err != nil {
		t.Fatal(err)
	}
	if !localca.ServerCertCoversHost(p, "127.0.0.1") || !localca.ServerCertCoversHost(p, "::1") {
		t.Fatal("IP certificate SAN missing")
	}
	ca := localca.CASummary(p).Fingerprint
	if err := EnsureInitialManagementTLS(dir); err != nil {
		t.Fatal(err)
	}
	if localca.CASummary(p).Fingerprint != ca {
		t.Fatal("trusted root changed during restart")
	}
}

func TestManagementDomainPersistsStrictPolicyAndReusesRoot(t *testing.T) {
	beforeDomain, beforeList := GetManagementDomain(), global.GWAF_DOMAIN_WHITELIST
	beforeTLS, beforeForce := global.GWAF_SSL_ENABLE, global.GWAF_SSL_FORCE_HTTPS
	t.Cleanup(func() {
		managementDomain.Store(beforeDomain)
		global.GWAF_DOMAIN_WHITELIST = beforeList
		global.GWAF_SSL_ENABLE = beforeTLS
		global.GWAF_SSL_FORCE_HTTPS = beforeForce
	})
	dir := managementTestDir(t, "local_port: 27777\nui:\n  admin_title: retained\n")
	switchAddress := func(domain, host string) {
		t.Helper()
		if err := PrepareManagementChange(dir, "admin", "address", domain, "127.0.0.1"); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "https://"+host+"/", nil)
		sni := host
		if host == "127.0.0.1" {
			sni = ""
		}
		r.TLS.ServerName = sni
		cert := ManagementPreviewCertificate(sni, "127.0.0.1:27777")
		r = r.WithContext(WithManagementCertificate(r.Context(), sha256.Sum256(cert.Certificate[0])))
		id := ManagementLoginPreviewID(r, "admin")
		save := func(write func() error) error { return write() }
		if id == "" || !RecordManagementLogin(id, "new-session") {
			t.Fatal("preview login missing")
		}
		if err := FinishManagementChange(dir, "admin", id, "confirm", "old-session", r, save); err == nil {
			t.Fatal("old session confirmed change")
		}
		if err := FinishManagementChange(dir, "another", id, "confirm", "new-session", r, save); err == nil {
			t.Fatal("another account confirmed change")
		}
		if err := FinishManagementChange(dir, "admin", id, "confirm", "new-session", httptest.NewRequest("GET", "http://"+host+"/", nil), save); err == nil {
			t.Fatal("plaintext request confirmed change")
		}
		before := leafFingerprint(localca.DefaultPaths(dir))
		err := FinishManagementChange(dir, "admin", id, "confirm", "new-session", r, func(write func() error) error {
			if err := write(); err != nil {
				return err
			}
			return errors.New("session transaction failed")
		})
		if err == nil || leafFingerprint(localca.DefaultPaths(dir)) != before {
			t.Fatal("failed transaction replaced the active certificate")
		}
		if err := FinishManagementChange(dir, "admin", id, "confirm", "new-session", r, save); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(ResetManagementServingCertificate)
	switchAddress("", "127.0.0.1")
	p := localca.DefaultPaths(dir)
	root := localca.CASummary(p).Fingerprint
	switchAddress("WAF.LAN", "waf.lan")
	if GetManagementDomain() != "waf.lan" || global.GWAF_DOMAIN_WHITELIST != "waf.lan" || !global.GWAF_SSL_ENABLE || !global.GWAF_SSL_FORCE_HTTPS {
		t.Fatal("strict access policy not active")
	}
	if !localca.ServerCertCoversHost(p, "waf.lan") || localca.ServerCertCoversHost(p, "127.0.0.1") {
		t.Fatal("domain certificate must cover only the configured name")
	}
	if localca.CASummary(p).Fingerprint != root {
		t.Fatal("domain change rotated root")
	}
	c, err := managementConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GetString("security.management_domain") != "waf.lan" || !c.GetBool("security.ssl_force_https") || c.GetInt("local_port") != 27777 || c.GetString("ui.admin_title") != "retained" {
		t.Fatal("configuration persistence lost settings")
	}
	if UpdateSslEnable(false) == nil || UpdateSslForceHttps(false) == nil || UpdateDomainWhitelist("") == nil {
		t.Fatal("legacy writes bypass domain-only policy")
	}
	switchAddress("", "127.0.0.1")
	if GetManagementDomain() != "" || global.GWAF_SSL_FORCE_HTTPS || !localca.ServerCertCoversHost(p, "127.0.0.1") {
		t.Fatal("IP mode recovery failed")
	}
	if err := PrepareManagementChange(dir, "admin", "https_only", "127.0.0.1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	managementHandoff.Lock()
	managementHandoff.pending.Expires = time.Now().Add(-time.Second)
	managementHandoff.Unlock()
	if view := ManagementPreviewFor(httptest.NewRequest("GET", "/", nil), "admin", ""); view != nil || global.GWAF_SSL_FORCE_HTTPS {
		t.Fatal("expired preview changed the active policy")
	}
	data, err := os.ReadFile(filepath.Join(dir, "conf", "config.yml"))
	if err != nil || !strings.Contains(string(data), "ssl_force_https: false") {
		t.Fatal("expired preview persisted a restriction")
	}
}

func TestManagementDomainRejectsUnsafeNames(t *testing.T) {
	for _, domain := range []string{"https://waf.lan", "waf.lan:26666", "*.lan", "192.168.1.1", "waf.lan/other", "waf.lan,other.lan", "waf.lan\r\nHost: evil.lan", "-waf.lan"} {
		if _, err := NormalizeManagementDomain(domain); err == nil {
			t.Errorf("accepted invalid domain %q", domain)
		}
	}
}

func TestSecuritySettingsRejectInvalidControlData(t *testing.T) {
	for _, value := range []string{"a/b", "user's", "a\nb", ".."} {
		if err := UpdateSecurityEntry(true, value); err == nil {
			t.Fatalf("accepted entry %q", value)
		}
	}
	if err := UpdateNoticeTitle("line one\r\nline two"); err == nil {
		t.Fatal("accepted multiline title")
	}
}
