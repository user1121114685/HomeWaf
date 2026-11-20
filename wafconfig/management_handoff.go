package wafconfig

import (
	"HomeWaf/global"
	"HomeWaf/utils"
	"HomeWaf/utils/localca"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A preview belongs to this worker. A restart or timeout discards it without
// changing the certificate or access policy used by the next startup.
type ManagementPreview struct {
	ID         string    `json:"id"`
	URLs       []string  `json:"urls"`
	Expires    time.Time `json:"expires"`
	Mode       string    `json:"mode"`
	Domain     string    `json:"domain"`
	CanConfirm bool      `json:"can_confirm"`
	AtTarget   bool      `json:"at_target"`
}

type managementChange struct {
	ManagementPreview
	owner, domain, originalDomain, binding, whitelist string
	ssl, force                                        bool
	leafBefore, caBefore                              string
	active, staged                                    localca.Paths
	cert                                              *tls.Certificate
	fingerprint                                       [32]byte
	login                                             [32]byte
	cleanup                                           func()
	timer                                             *time.Timer
}

var managementHandoff struct {
	sync.Mutex
	pending *managementChange
	serving *tls.Certificate
}

const ManagementPreviewLoginType = "management-preview"

func activeChange() *managementChange {
	p := managementHandoff.pending
	if p != nil && !time.Now().Before(p.Expires) {
		discardChange(p)
		return nil
	}
	return p
}

func discardChange(p *managementChange) {
	managementHandoff.pending = nil
	p.timer.Stop()
	if p.cleanup != nil {
		p.cleanup()
	}
}

func leafFingerprint(p localca.Paths) string {
	if leaf := localca.CurrentServerCert(p); leaf != nil {
		return leaf.Fingerprint
	}
	return ""
}

func caFingerprint(p localca.Paths) string {
	if ca := localca.CASummary(p); ca != nil {
		return ca.Fingerprint
	}
	return ""
}

func PrepareManagementChange(baseDir, owner, mode, target, localAddress string) error {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	if owner == "" {
		return errors.New("请用管理账号登录后准备新入口")
	}
	if activeChange() != nil {
		return errors.New("已有管理入口等待验证，请先完成或取消")
	}
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	config, err := managementConfig(baseDir)
	if err != nil {
		return err
	}
	p := &managementChange{owner: owner, originalDomain: GetManagementDomain(), active: localca.DefaultPaths(baseDir),
		binding: config.GetString("security.ssl_bind_cert_id"), whitelist: config.GetString("security.domain_whitelist"),
		ssl: config.GetBool("security.ssl_enable"), force: config.GetBool("security.ssl_force_https")}
	p.Mode, p.ID, p.Expires = mode, rand.Text(), time.Now().Add(20*time.Minute)
	p.leafBefore = leafFingerprint(p.active)
	entry := ""
	if global.GWAF_SECURITY_ENTRY_ENABLE {
		entry = global.GWAF_SECURITY_ENTRY_PATH
	}
	switch mode {
	case "address":
		p.domain, err = NormalizeManagementDomain(target)
		if err != nil {
			return err
		}
		if p.binding != "" {
			return errors.New("请先解除管理端证书资源绑定，再签发本地证书")
		}
		sans := defaultManagementSANs()
		if p.domain != "" {
			sans = []string{p.domain}
		}
		p.URLs = utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, true, entry, p.domain)
		if p.domain == "" && net.ParseIP(localAddress) != nil {
			for _, san := range sans {
				if san == localAddress {
					primary := utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, true, entry, localAddress)[0]
					urls := []string{primary}
					for _, candidate := range p.URLs {
						if candidate != primary {
							urls = append(urls, candidate)
						}
					}
					p.URLs = urls
					break
				}
			}
		}
		if len(p.URLs) == 0 {
			return errors.New("未检测到可验证的服务器地址")
		}
		p.staged, p.cleanup, err = localca.StageServerCert(p.active, sans)
	case "https_only":
		p.domain = p.originalDomain
		host := strings.Trim(target, "[]")
		if p.domain != "" {
			host = p.domain
		}
		if net.ParseIP(host) == nil {
			if host != "localhost" {
				host, err = NormalizeManagementDomain(host)
				if err != nil || host == "" {
					return errors.New("请填写证书覆盖的域名或 IP")
				}
			}
		}
		if !localca.ServerCertCoversHost(p.active, host) {
			return errors.New("管理端证书没有覆盖此地址，请填写证书中的域名或 IP")
		}
		if p.whitelist != "" {
			permitted := false
			for _, item := range strings.Split(p.whitelist, ",") {
				if strings.EqualFold(strings.TrimSpace(item), host) {
					permitted = true
					break
				}
			}
			if !permitted {
				return errors.New("此地址不在当前管理域名白名单内")
			}
		}
		p.URLs = utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, true, entry, host)
		p.staged = p.active
	default:
		return errors.New("请选择管理地址或仅 HTTPS 模式")
	}
	if err == nil {
		err = localca.CheckServerCert(p.staged)
	}
	if err == nil {
		var cert tls.Certificate
		cert, err = tls.LoadX509KeyPair(p.staged.SrvCert, p.staged.SrvKey)
		p.cert = &cert
	}
	if err != nil {
		if p.cleanup != nil {
			p.cleanup()
		}
		return err
	}
	p.caBefore = caFingerprint(p.active)
	p.fingerprint = sha256.Sum256(p.cert.Certificate[0])
	p.ManagementPreview.Domain = p.domain
	p.timer = time.AfterFunc(time.Until(p.Expires), func() {
		managementHandoff.Lock()
		defer managementHandoff.Unlock()
		if managementHandoff.pending == p {
			discardChange(p)
		}
	})
	managementHandoff.pending = p
	return nil
}

func (p *managementChange) accepts(host string) bool {
	for _, raw := range p.URLs {
		u, _ := url.Parse(raw)
		if strings.EqualFold(u.Hostname(), host) {
			return true
		}
	}
	return false
}

// IP URLs omit SNI. Pick their preview certificate using the local listening
// address, and verify Host separately on every request on the connection.
func ManagementPreviewCertificate(serverName, localAddress string) *tls.Certificate {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	if serverName == "" {
		serverName, _, _ = net.SplitHostPort(localAddress)
	}
	if p := activeChange(); p != nil && p.accepts(serverName) {
		return p.cert
	}
	return managementHandoff.serving
}

// An explicit listener restart loads its current certificate from disk.
func ResetManagementServingCertificate() {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	managementHandoff.serving = nil
}

type previewConnectionKey struct{}

func WithManagementCertificate(ctx context.Context, fingerprint [32]byte) context.Context {
	return context.WithValue(ctx, previewConnectionKey{}, fingerprint)
}

func (p *managementChange) matches(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return r.TLS != nil && (strings.EqualFold(r.TLS.ServerName, host) || net.ParseIP(host) != nil && r.TLS.ServerName == "") &&
		r.Context().Value(previewConnectionKey{}) == p.fingerprint && p.accepts(host)
}

func IsManagementPreview(r *http.Request) bool {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	p := activeChange()
	return p != nil && p.matches(r)
}

func ManagementLoginPreviewID(r *http.Request, owner string) string {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	if p := activeChange(); p != nil && p.owner == owner && p.matches(r) {
		return p.ID
	}
	return ""
}

func RecordManagementLogin(id, token string) bool {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	if p := activeChange(); p != nil && p.ID == id && token != "" {
		p.login = sha256.Sum256([]byte(token))
		return true
	}
	return false
}

func ManagementPreviewFor(r *http.Request, owner, token string) *ManagementPreview {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	p := activeChange()
	if p == nil || p.owner != owner {
		return nil
	}
	view := p.ManagementPreview
	view.AtTarget = p.matches(r)
	view.CanConfirm = view.AtTarget && token != "" && sha256.Sum256([]byte(token)) == p.login
	return &view
}

func FinishManagementChange(baseDir, owner, id, action, token string, r *http.Request, promote func(func() error) error) error {
	managementHandoff.Lock()
	defer managementHandoff.Unlock()
	p := activeChange()
	if p == nil || p.ID != id || owner != p.owner {
		return errors.New("本次管理入口切换已失效，请重新准备")
	}
	if action == "cancel" {
		discardChange(p)
		return nil
	}
	if action != "confirm" || !p.matches(r) || token == "" || sha256.Sum256([]byte(token)) != p.login {
		return errors.New("请在待验证的 HTTPS 入口使用同一管理员账号重新登录并确认")
	}
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	config, err := managementConfig(baseDir)
	if err != nil {
		return err
	}
	if p.originalDomain != GetManagementDomain() || p.binding != config.GetString("security.ssl_bind_cert_id") ||
		p.whitelist != config.GetString("security.domain_whitelist") || p.ssl != config.GetBool("security.ssl_enable") ||
		p.force != config.GetBool("security.ssl_force_https") || p.leafBefore != leafFingerprint(p.active) || p.caBefore != caFingerprint(p.active) {
		return errors.New("验证期间管理配置或证书已更改，请取消并重新验证")
	}
	force := p.Mode == "https_only" || p.domain != ""
	config.Set("security.ssl_enable", true)
	config.Set("security.ssl_force_https", force)
	if p.Mode == "address" {
		config.Set("security.management_domain", p.domain)
		config.Set("security.domain_whitelist", p.domain)
		config.Set("security.ssl_auto_local", true)
	}
	previous, err := managementConfig(baseDir)
	if err != nil {
		return err
	}
	persist := func() error {
		written := false
		err := promote(func() error {
			if err := writeManagementConfig(config); err != nil {
				return err
			}
			written = true
			return nil
		})
		if err != nil && written {
			return errors.Join(err, writeManagementConfig(previous))
		}
		return err
	}
	if p.Mode == "address" {
		err = localca.InstallServerCert(p.active, p.staged, persist)
	} else {
		err = persist()
	}
	if err != nil {
		return err
	}
	if p.Mode == "address" {
		global.GWAF_DOMAIN_WHITELIST = p.domain
		managementDomain.Store(p.domain)
	}
	global.GWAF_SSL_ENABLE, global.GWAF_SSL_FORCE_HTTPS = true, force
	managementHandoff.serving = p.cert
	discardChange(p)
	return nil
}
