package wafconfig

import (
	"HomeWaf/global"
	"HomeWaf/utils/localca"
	"os"
	"path/filepath"
	"testing"
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
	if err := ConfigureManagementAddress(dir, ""); err != nil {
		t.Fatal(err)
	}
	p := localca.DefaultPaths(dir)
	root := localca.CASummary(p).Fingerprint
	if err := ConfigureManagementAddress(dir, "WAF.LAN"); err != nil {
		t.Fatal(err)
	}
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
	if err := ConfigureManagementAddress(dir, ""); err != nil {
		t.Fatal(err)
	}
	if GetManagementDomain() != "" || global.GWAF_SSL_FORCE_HTTPS || !localca.ServerCertCoversHost(p, "127.0.0.1") {
		t.Fatal("IP mode recovery failed")
	}
}

func TestManagementDomainRejectsUnsafeNames(t *testing.T) {
	for _, domain := range []string{"https://waf.lan", "waf.lan:26666", "*.lan", "192.168.1.1", "waf.lan/other", "waf.lan,other.lan", "waf.lan\r\nHost: evil.lan", "-waf.lan"} {
		if _, err := NormalizeManagementDomain(domain); err == nil {
			t.Errorf("accepted invalid domain %q", domain)
		}
	}
}
