package wafconfig

import (
	"HomeWaf/global"
	"HomeWaf/utils"
	"HomeWaf/utils/localca"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"
)

var managementDomain atomic.Value
var configWriteMu sync.Mutex
var managementDNSLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func GetManagementDomain() string {
	value, _ := managementDomain.Load().(string)
	return value
}

func NormalizeManagementDomain(domain string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", nil
	}
	if len(domain) > 253 || net.ParseIP(domain) != nil || !strings.Contains(domain, ".") {
		return "", errors.New("请填写完整管理域名，例如 waf.lan；留空使用服务器 IP")
	}
	for _, label := range strings.Split(domain, ".") {
		if !managementDNSLabel.MatchString(label) {
			return "", errors.New("管理域名不能包含协议、端口、路径、通配符或多个域名")
		}
	}
	return domain, nil
}

func managementConfig(baseDir string) (*viper.Viper, error) {
	config := viper.New()
	config.SetConfigFile(filepath.Join(baseDir, "conf", "config.yml"))
	config.SetConfigType("yml")
	return config, config.ReadInConfig()
}

func defaultManagementSANs() []string {
	sans := append([]string{"localhost", "127.0.0.1", "::1"}, utils.ConsoleIPs()...)
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		sans = append(sans, hostname)
	}
	return sans
}

// EnsureInitialManagementTLS applies only to installations that opted into automatic local TLS.
// Existing certificate material is never replaced here, including certificates from other sources.
func EnsureInitialManagementTLS(baseDir string) error {
	config, err := managementConfig(baseDir)
	if err != nil {
		return err
	}
	if !config.GetBool("security.ssl_auto_local") || !config.GetBool("security.ssl_enable") {
		return nil
	}
	p := localca.DefaultPaths(baseDir)
	_, certErr := os.Stat(p.SrvCert)
	_, keyErr := os.Stat(p.SrvKey)
	if certErr == nil && keyErr == nil {
		if localca.IsIssuedByLocalCA(p) {
			current := localca.CurrentServerCert(p)
			if current != nil && time.Until(current.NotAfter) < 30*24*time.Hour {
				return localca.ReplaceServerCert(p, localca.SANsOf(current), func() error { return nil })
			}
		}
		return localca.CheckServerCert(p)
	}
	if !os.IsNotExist(certErr) || !os.IsNotExist(keyErr) {
		return errors.New("管理端证书材料不完整，请修复 domain.crt 与 domain.key；不会自动替换现有证书")
	}
	sans := defaultManagementSANs()
	if domain := config.GetString("security.management_domain"); domain != "" {
		sans = []string{domain}
	}
	_, err = localca.IssueServerCert(p, sans, 0)
	return err
}

// ConfigureManagementAddress commits the certificate and its access policy together.
// A failed config write restores the previous certificate, without rotating the trusted root.
func ConfigureManagementAddress(baseDir, domain string) error {
	domain, err := NormalizeManagementDomain(domain)
	if err != nil {
		return err
	}
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	config, err := managementConfig(baseDir)
	if err != nil {
		return err
	}
	if config.GetString("security.ssl_bind_cert_id") != "" {
		return errors.New("当前管理端绑定了证书资源，请先在证书来源中解除绑定，再使用本地管理证书")
	}
	sans := defaultManagementSANs()
	if domain != "" {
		sans = []string{domain}
	}
	p := localca.DefaultPaths(baseDir)
	return localca.ReplaceServerCert(p, sans, func() error {
		config.Set("security.management_domain", domain)
		config.Set("security.domain_whitelist", domain)
		config.Set("security.ssl_enable", true)
		config.Set("security.ssl_force_https", domain != "")
		config.Set("security.ssl_auto_local", true)
		if err := writeManagementConfig(config); err != nil {
			return fmt.Errorf("保存管理地址失败: %w", err)
		}
		global.GWAF_DOMAIN_WHITELIST = domain
		global.GWAF_SSL_ENABLE = true
		global.GWAF_SSL_FORCE_HTTPS = domain != ""
		managementDomain.Store(domain)
		return nil
	})
}

func writeManagementConfig(config *viper.Viper) error {
	path := config.ConfigFileUsed()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".management-config-*.yml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(name)
	if err := config.WriteConfigAs(name); err != nil {
		return err
	}
	if err := os.Chmod(name, 0600); err != nil {
		return err
	}
	return os.Rename(name, path)
}
