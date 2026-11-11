package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LegacyPortSource struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
type LegacyPortPreview struct {
	Port         uint16                 `json:"port"`
	Config       wafpipeline.PortConfig `json:"config"`
	Sources      []LegacyPortSource     `json:"sources"`
	PolicyHost   string                 `json:"policy_host"`
	PreviewToken string                 `json:"preview_token"`
	AccountNames []string               `json:"account_names"`
	Warnings     []string               `json:"warnings"`
	Blockers     []string               `json:"blockers"`
	Imported     bool                   `json:"imported"`
	State        string                 `json:"state"`
	RequiresTOTP bool                   `json:"requires_totp"`
	secrets      wafpipeline.PortSecrets
	accounts     []model.GateUser
	passwords    map[string]string
	hash         string
}
type LegacyPortImportRequest struct {
	Port          uint16 `json:"port"`
	PolicyHost    string `json:"policy_host"`
	PreviewToken  string `json:"preview_token"`
	AcceptChanges bool   `json:"accept_changes"`
	Username      string `json:"initial_username,omitempty"`
	Password      string `json:"initial_password,omitempty"`
	TOTPSecret    string `json:"initial_totp_secret,omitempty"`
}
type LegacyPortSummary struct {
	Port     uint16             `json:"port"`
	Sources  []LegacyPortSource `json:"sources"`
	Imported bool               `json:"imported"`
	State    string             `json:"state"`
}
type migrationToken struct {
	Hash    string
	Port    uint16
	Host    string
	Expires int64
}

func optionalLegacyRows(db *gorm.DB, dest any, query string, args ...any) error {
	if !db.Migrator().HasTable(dest) {
		return nil
	}
	return db.Where(query, args...).Order("id").Limit(100001).Find(dest).Error
}

func legacyAccountRows(db *gorm.DB, dest any, query string, args ...any) error {
	if !db.Migrator().HasTable(dest) {
		return nil
	}
	result := db.Where(query, args...).Order("id").Limit(1001).Find(dest)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 1000 {
		return errors.New("旧账号数量超过每端口1000上限，请先缩小账号范围")
	}
	return nil
}

func legacyOrigin(h model.Hosts) (string, string, error) {
	raw := strings.TrimSpace(h.Remote_host)
	if raw == "" {
		raw = h.Remote_ip
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", "", errors.New("旧站点上游地址不合法")
	}
	if h.Remote_port > 0 {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(h.Remote_port))
	}
	return u.String(), strings.TrimSpace(h.Remote_ip), nil
}
func splitLegacyList(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' })
}

func (s *WafPipelineService) LegacyPorts() ([]LegacyPortSummary, error) {
	db, err := pipelineDB()
	if err != nil {
		return nil, err
	}
	var hosts []model.Hosts
	if err = db.Where("global_host = ?", 0).Order("code").Limit(20001).Find(&hosts).Error; err != nil {
		return nil, err
	}
	if len(hosts) > 20000 {
		return nil, errors.New("旧站点数量超过迁入扫描上限")
	}
	ports := map[uint16][]LegacyPortSource{}
	for _, h := range hosts {
		for _, listen := range utils.ResolveHostListens(h) {
			if listen.Port > 0 && listen.Port < 65536 {
				ports[uint16(listen.Port)] = append(ports[uint16(listen.Port)], LegacyPortSource{h.Code, h.Host, h.START_STATUS == 0})
			}
		}
	}
	keys := make([]int, 0, len(ports))
	for p := range ports {
		keys = append(keys, int(p))
	}
	sort.Ints(keys)
	var markers []model.PipelinePortMigration
	if err := db.Find(&markers).Error; err != nil {
		return nil, err
	}
	states := map[uint16]string{}
	for _, marker := range markers {
		states[uint16(marker.Port)] = marker.State
	}
	var native []model.PortConfig
	if err := db.Select("port").Find(&native).Error; err != nil {
		return nil, err
	}
	imported := map[uint16]bool{}
	for _, row := range native {
		imported[uint16(row.Port)] = true
	}
	out := make([]LegacyPortSummary, 0, len(keys))
	for _, p := range keys {
		port := uint16(p)
		out = append(out, LegacyPortSummary{Port: port, Sources: ports[port], Imported: imported[port], State: states[port]})
	}
	return out, nil
}

func (s *WafPipelineService) PreviewLegacyPort(port uint16, policyHost string) (LegacyPortPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.previewLegacyPort(port, policyHost)
}

func (s *WafPipelineService) previewLegacyPort(port uint16, policyHost string) (LegacyPortPreview, error) {
	out := LegacyPortPreview{Port: port, Warnings: []string{}, Blockers: []string{}, Sources: []LegacyPortSource{}, AccountNames: []string{}}
	db, err := pipelineDB()
	if err != nil {
		return out, err
	}
	var all []model.Hosts
	if err = db.Order("code").Limit(20001).Find(&all).Error; err != nil {
		return out, err
	}
	if len(all) > 20000 {
		return out, errors.New("旧站点数量超过迁入扫描上限")
	}
	var hosts []model.Hosts
	var globalCodes []string
	protocol := ""
	ipVersion := ""
	for _, h := range all {
		if h.GLOBAL_HOST == 1 {
			globalCodes = append(globalCodes, h.Code)
			continue
		}
		for _, l := range utils.ResolveHostListens(h) {
			if l.Port != int(port) {
				continue
			}
			if protocol != "" && protocol != l.Protocol {
				out.Blockers = append(out.Blockers, "同一端口的旧站点声明了不同监听协议")
			}
			protocol = l.Protocol
			if ipVersion != "" && ipVersion != l.IPVersion {
				out.Blockers = append(out.Blockers, "同一端口的旧站点声明了不同IP监听范围，请先统一")
			}
			ipVersion = l.IPVersion
			hosts = append(hosts, h)
			out.Sources = append(out.Sources, LegacyPortSource{h.Code, h.Host, h.START_STATUS == 0})
			if l.Implied {
				out.Warnings = append(out.Warnings, "旧HTTP自动跳转入口迁为本地HTTPS重定向，保留路径与query；先经过访问总闸和IP策略，不回源或要求明文HTTP登录")
			}
			break
		}
	}
	if len(hosts) == 0 {
		return out, errors.New("未找到该端口的旧站点")
	}
	selected := hosts[0]
	if policyHost != "" {
		found := false
		for _, h := range hosts {
			if h.Code == policyHost {
				selected = h
				found = true
				break
			}
		}
		if !found {
			return out, errors.New("策略来源站点不属于该端口")
		}
	}
	out.PolicyHost = selected.Code
	globalCodes = nil
	for _, source := range all {
		if source.GLOBAL_HOST == 1 && source.USER_CODE == selected.USER_CODE && source.Tenant_ID == selected.Tenant_ID {
			globalCodes = append(globalCodes, source.Code)
		}
	}
	for _, source := range hosts {
		if source.USER_CODE != selected.USER_CODE || source.Tenant_ID != selected.Tenant_ID {
			out.Blockers = append(out.Blockers, "同一监听端口包含不同用户或租户，不能自动合并认证和策略，请先拆分监听")
		}
	}
	c := wafpipeline.PortConfig{Schema: wafpipeline.SchemaVersion, Port: port, Protocol: protocol, Name: selected.Nickname, ListenEnabled: false, Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Network.ListenIPVersion = ipVersion
	c.Policy.Network.DisableHTTP2 = selected.DisableHTTP2 == 1
	c.Policy.Network.MaxProxyHops = int(global.CurrentSettings().GCONFIG_RECORD_PROXY_LOOP_MAX_HOP)
	c.ACMEHosts = map[string]string{}
	if c.Name == "" {
		c.Name = selected.Host
	}
	c.Policy.Gate.Enabled = selected.IsEnableHttpAuthBase == 1
	c.Policy.Gate.BypassTrusted = !c.Policy.Gate.Enabled
	if c.Policy.Gate.Enabled && selected.HttpAuthBaseType != model.HttpAuthTypeCustom {
		c.Policy.Gate.Mode = "basic"
	}
	if c.Policy.Gate.Enabled {
		g := model.DecodeHttpAuthConfig(selected.HttpAuthJSON)
		c.Policy.Gate.AbsoluteSeconds = int(g.SessionTTL) * 60
		if g.IdleTimeout > 0 {
			c.Policy.Gate.IdleSeconds = int(g.IdleTimeout) * 60
		}
		c.Policy.Gate.BindIP = g.BindIP == 1
		if g.PathScope == model.HttpAuthScopeExclude {
			c.Policy.Gate.Exempt.Paths = splitLegacyList(g.Paths)
		}
		if g.PathScope == model.HttpAuthScopeInclude || g.BypassIPGroup != "" {
			if g.PathScope == model.HttpAuthScopeInclude {
				c.Policy.Gate.Protect.Paths = splitLegacyList(g.Paths)
			}
		}
	}
	defense := model.ParseHostsDefense(selected.DEFENSE_JSON)
	c.Policy.RuleSet.AI = wafpipeline.ModeOff
	if defense.DEFENSE_AI == 1 && selected.GUARD_STATUS == 1 && global.CurrentSettings().GCONFIG_AI_ENABLE == 1 {
		c.Policy.RuleSet.AI = wafpipeline.ModeObserve
		if global.CurrentSettings().GCONFIG_AI_MODE == "block" && selected.LogOnlyMode != 1 {
			c.Policy.RuleSet.AI = wafpipeline.ModeBlock
		}
	}
	flags := map[string]int{"scanner": defense.DEFENSE_SCAN, "sqli": defense.DEFENSE_SQLI, "xss": defense.DEFENSE_XSS, "rce": defense.DEFENSE_RCE, "lfi": defense.DEFENSE_DIR_TRAVERSAL}
	for i := range c.Policy.RuleSet.Rules {
		r := &c.Policy.RuleSet.Rules[i]
		if v, ok := flags[r.Category]; ok && v == 0 || selected.GUARD_STATUS == 0 {
			r.Mode = wafpipeline.ModeOff
		}
		if selected.LogOnlyMode == 1 && r.Mode != wafpipeline.ModeOff {
			r.Mode = wafpipeline.ModeObserve
		}
	}
	c.Policy.RuleSet.BOT = wafpipeline.ModeOff
	if defense.DEFENSE_BOT == 1 && selected.GUARD_STATUS == 1 {
		c.Policy.RuleSet.BOT = wafpipeline.ModeObserve
	}
	if defense.DEFENSE_OWASP_SET == 1 {
		c.Policy.RuleSet.OWASP = wafpipeline.ModeObserve
	}
	c.Policy.RuleSet.QPS.Mode = wafpipeline.ModeOff
	c.Policy.RuleSet.CC.Mode = wafpipeline.ModeOff
	var cc []model.AntiCC
	if err = optionalLegacyRows(db, &cc, "host_code = ?", selected.Code); err != nil {
		return out, err
	}
	if len(cc) > 0 && cc[0].Rate > 0 && cc[0].Limit > 0 && selected.GUARD_STATUS == 1 {
		if cc[0].LimitMode == "window" {
			c.Policy.RuleSet.CC.Mode = wafpipeline.ModeBlock
			c.Policy.RuleSet.CC.WindowSeconds = cc[0].Rate
			c.Policy.RuleSet.CC.Threshold = cc[0].Limit
		} else {
			c.Policy.RuleSet.QPS.Mode = wafpipeline.ModeBlock
			c.Policy.RuleSet.QPS.PerIP = float64(cc[0].Limit) / float64(cc[0].Rate)
			c.Policy.RuleSet.QPS.Burst = cc[0].Limit
		}
	}
	if selected.LogOnlyMode == 1 {
		if c.Policy.RuleSet.QPS.Mode != wafpipeline.ModeOff {
			c.Policy.RuleSet.QPS.Mode = wafpipeline.ModeObserve
		}
		if c.Policy.RuleSet.CC.Mode != wafpipeline.ModeOff {
			c.Policy.RuleSet.CC.Mode = wafpipeline.ModeObserve
		}
		c.Policy.HitPolicy.Enabled = false
		c.Policy.HitPolicy.Lockdown.Enabled = false
	}
	c.Policy.Network.TrustedProxies = splitLegacyList(selected.IPTrustProxies)
	c.Policy.Network.PreserveHost = selected.IsTransBackDomain != 1
	if selected.ResponseTimeOut > 0 {
		c.Policy.Network.ResponseHeaderSeconds = selected.ResponseTimeOut
	}
	if selected.InsecureSkipVerify == 1 {
		out.Warnings = append(out.Warnings, "新端口不继承跳过上游TLS校验；自签名上游请先在代理设置提供CA")
	}
	if len(hosts) > 1 {
		out.Warnings = append(out.Warnings, "同端口全部主机名共用选中来源站点的安全策略与账号，不保留隐式站点级防护覆盖")
	}
	seenHosts := map[string]bool{}
	allowAnyHost := false
	seenCert := map[string]bool{}
	allCodes := make([]string, 0, len(hosts))
	for _, h := range hosts {
		allCodes = append(allCodes, h.Code)
	}
	var pathRules []model.HostPathRule
	var balances []model.LoadBalance
	var cacheRules []model.CacheRule
	if err = optionalLegacyRows(db, &cacheRules, "host_code IN ?", allCodes); err != nil {
		return out, err
	}
	if err = optionalLegacyRows(db, &balances, "host_code IN ?", allCodes); err != nil {
		return out, err
	}
	if err = optionalLegacyRows(db, &pathRules, "host_code IN ?", allCodes); err != nil {
		return out, err
	}
	for _, h := range hosts {
		entryKind, redirectPort, redirectCode := legacyBusinessEntry(h, protocol)
		headerProfile := ""
		requestHeaders := model.ParseCustomHeadersConfig(h.CustomHeadersJSON)
		responseHeaders := model.ParseCustomResponseHeadersConfig(h.CustomResponseHeadersJSON)
		if h.CustomHeadersJSON != "" && !json.Valid([]byte(h.CustomHeadersJSON)) || h.CustomResponseHeadersJSON != "" && !json.Valid([]byte(h.CustomResponseHeadersJSON)) {
			out.Blockers = append(out.Blockers, h.Host+"：自定义头配置无效")
		}
		if requestHeaders.IsEnableCustomHeaders == 1 || responseHeaders.IsEnableCustomHeaders == 1 {
			headerProfile = "legacy-" + h.Code
			profile := wafpipeline.BusinessHeaders{}
			if requestHeaders.IsEnableCustomHeaders == 1 {
				profile.Request = requestHeaders.Headers
			}
			if responseHeaders.IsEnableCustomHeaders == 1 {
				profile.Response = responseHeaders.Rules
			}
			if out.secrets.BusinessHeaders == nil {
				out.secrets.BusinessHeaders = map[string]wafpipeline.BusinessHeaders{}
			}
			out.secrets.BusinessHeaders[headerProfile] = profile
			out.Warnings = append(out.Warnings, h.Host+"：自定义头值迁入加密配置 "+headerProfile+"；预览和导出不展示头值")
		}
		var static model.StaticSiteConfig
		if h.StaticSiteJSON != "" {
			if e := json.Unmarshal([]byte(h.StaticSiteJSON), &static); e != nil {
				out.Blockers = append(out.Blockers, h.Host+"：静态业务配置无效")
			}
		}
		upstream, dial, e := legacyOrigin(h)
		if e != nil {
			if entryKind == "" && static.IsEnableStaticSite != 1 && (strings.TrimSpace(h.Remote_host) != "" || strings.TrimSpace(h.Remote_ip) != "") {
				out.Blockers = append(out.Blockers, h.Host+"："+e.Error())
				continue
			}
			upstream, dial = "", ""
			out.Warnings = append(out.Warnings, h.Host+"：无默认上游，未匹配路径及认证入口时返回本地404，不会访问其他主机的上游")
		}
		names := append([]string{h.Host}, splitLegacyList(h.BindMoreHost)...)
		for i, name := range names {
			names[i] = strings.TrimSpace(strings.ToLower(name))
		}
		if entryKind != "" {
			c.Routes = append(c.Routes, wafpipeline.UpstreamRoute{Hosts: names, TargetType: entryKind, RedirectPort: redirectPort, RedirectCode: redirectCode, Upstream: upstream, DialIP: dial})
		} else if upstream != "" {
			route := wafpipeline.UpstreamRoute{Hosts: names, Upstream: upstream, DialIP: dial, HeaderProfile: headerProfile}
			var caching model.CacheConfig
			if h.CacheJSON != "" {
				if e := json.Unmarshal([]byte(h.CacheJSON), &caching); e != nil {
					out.Blockers = append(out.Blockers, h.Host+"：缓存配置无效")
				}
			}
			if caching.IsEnableCache == 1 {
				route.Cache = &wafpipeline.BusinessCacheConfig{}
				for _, rule := range cacheRules {
					if rule.HostCode == h.Code {
						if rule.ParamType == 0 {
							rule.ParamType = 2
						}
						route.Cache.Rules = append(route.Cache.Rules, rule)
					}
				}
				out.Warnings = append(out.Warnings, h.Host+"：缓存迁为端口隔离的有界内存缓存，仅GET/HEAD、无业务Cookie/Authorization且响应可缓存时生效；不搬移原磁盘缓存，不缓存POST/PUT或带自定义头的路由")
			}
			if h.IsEnableLoadBalance == 1 {
				route.Balance = "wrr"
				if h.LoadBalanceStage == 2 {
					route.Balance = "ip_hash"
				} else if h.LoadBalanceStage != 1 {
					out.Blockers = append(out.Blockers, h.Host+"：负载算法不受支持")
				}
				for _, backend := range balances {
					if backend.HostCode != h.Code {
						continue
					}
					url, ip, e := legacyOrigin(model.Hosts{Remote_host: upstream, Remote_ip: backend.Remote_ip, Remote_port: backend.Remote_port})
					if e != nil {
						out.Blockers = append(out.Blockers, h.Host+"：负载上游无效")
						continue
					}
					route.Backends = append(route.Backends, wafpipeline.Backend{Upstream: url, DialIP: ip, Weight: backend.Weight})
				}
				if len(route.Backends) == 0 {
					out.Blockers = append(out.Blockers, h.Host+"：启用负载均衡但没有后端")
				}
				out.Warnings = append(out.Warnings, h.Host+"：负载均衡使用端口独立的被动健康状态；连续3次5xx后避让5秒，不新增后台主动探测")
			}
			c.Routes = append(c.Routes, route)
		} else {
			c.Routes = append(c.Routes, wafpipeline.UpstreamRoute{Hosts: names, TargetType: "not_found"})
		}
		if entryKind == "" && static.IsEnableStaticSite == 1 {
			prefix := static.StaticSitePrefix
			if prefix == "" {
				prefix = "/"
				static.StaticSitePrefix = prefix
			}
			c.Routes = append(c.Routes, wafpipeline.UpstreamRoute{Hosts: names, PathPrefix: prefix, TargetType: "static", Priority: -100000, Static: &wafpipeline.StaticBusiness{Settings: static}, HeaderProfile: headerProfile})
		}
		for _, rule := range pathRules {
			if entryKind != "" || rule.HostCode != h.Code {
				continue
			}
			route := wafpipeline.UpstreamRoute{Hosts: append([]string{}, names...), PathPrefix: rule.Path, Priority: rule.Priority, StripPrefix: rule.StripPrefix == 1, HeaderProfile: headerProfile}
			switch rule.MatchType {
			case 1:
				route.MatchType = "prefix"
			case 2:
				route.MatchType = "exact"
			case 3:
				route.MatchType = "regex"
			default:
				out.Blockers = append(out.Blockers, h.Host+"：路径匹配类型无效")
			}
			switch rule.TargetType {
			case 1:
				scheme := rule.RemoteScheme
				if scheme == "" || scheme == "auto" {
					scheme = protocol
				}
				raw := scheme + "://" + rule.RemoteHost
				converted, ip, e := legacyOrigin(model.Hosts{Remote_host: raw, Remote_ip: rule.RemoteIP, Remote_port: rule.RemotePort})
				if e != nil {
					out.Blockers = append(out.Blockers, h.Host+"：路径上游无效")
				} else {
					route.Upstream, route.DialIP = converted, ip
				}
				if rule.ResponseTimeOut > 0 && rule.ResponseTimeOut != c.Policy.Network.ResponseHeaderSeconds {
					out.Warnings = append(out.Warnings, h.Host+"：路径超时统一使用所选端口的网络设置，请在第5步核对")
				}
			case 2:
				settings := static
				settings.StaticSitePath = rule.StaticRoot
				settings.StaticSitePrefix = ""
				if rule.StripPrefix == 1 {
					settings.StaticSitePrefix = rule.Path
				}
				settings.IsEnableStaticSite = 1
				route.TargetType = "static"
				route.Static = &wafpipeline.StaticBusiness{Settings: settings, SPAFallback: rule.SpaFallback == 1}
			case 3:
				route.TargetType = "redirect"
				route.RedirectURL = rule.RedirectURL
				route.RedirectCode = rule.RedirectCode
			default:
				out.Blockers = append(out.Blockers, h.Host+"：路径目标类型无效")
			}
			c.Routes = append(c.Routes, route)
		}
		if c.Upstream == "" || h.Code == selected.Code {
			c.Upstream, c.UpstreamDialIP = upstream, dial
			c.HeaderProfile = headerProfile
		}
		for _, name := range names {
			if name != "*" && !strings.HasPrefix(name, "*.") {
				c.ACMEHosts[name] = h.Code
			}
			if name != "*" && !seenHosts[name] {
				c.HostNames = append(c.HostNames, name)
				seenHosts[name] = true
			}
		}
		if h.Host == "*" {
			allowAnyHost = true
		}
		if protocol == "https" && h.Certfile != "" && !seenCert[h.Certfile] {
			if _, e := tls.X509KeyPair([]byte(h.Certfile), []byte(h.Keyfile)); e != nil {
				out.Blockers = append(out.Blockers, h.Host+"：监听证书无法转换")
			} else if out.secrets.TLSCert == "" {
				out.secrets.TLSCert, out.secrets.TLSKey = h.Certfile, h.Keyfile
			} else {
				out.secrets.AdditionalTLS = append(out.secrets.AdditionalTLS, wafpipeline.TLSCertificate{Certificate: h.Certfile, PrivateKey: h.Keyfile})
			}
			seenCert[h.Certfile] = true
		}
		if protocol == "https" && h.Certfile != "" {
			if out.secrets.TLSBindings == nil {
				out.secrets.TLSBindings = map[string]int{}
			}
			if h.Certfile == out.secrets.TLSCert {
				out.secrets.TLSBindings[h.Code] = 0
			} else {
				for i, pair := range out.secrets.AdditionalTLS {
					if pair.Certificate == h.Certfile {
						out.secrets.TLSBindings[h.Code] = i + 1
						break
					}
				}
			}
		}
	}
	if allowAnyHost {
		c.HostNames = nil
	}
	onlyLocal := len(c.Routes) > 0
	for _, route := range c.Routes {
		if route.TargetType == "" || route.TargetType == "proxy" {
			onlyLocal = false
			break
		}
	}
	if onlyLocal {
		c.Upstream = ""
		c.UpstreamDialIP = ""
	}
	if protocol == "https" && out.secrets.TLSCert == "" {
		out.Blockers = append(out.Blockers, "该HTTPS端口没有可迁入的有效证书")
	}
	codes := append(append([]string{}, globalCodes...), selected.Code)
	var allow []model.IPAllowList
	var deny []model.IPBlockList
	var credentials []model.HttpAuthBase
	var scripts []model.Rules
	var urlAllow []model.URLAllowList
	var urlDeny []model.URLBlockList
	var scriptGroups []model.IPGroup
	var scriptGroupItems []model.IPGroupItem
	if err = optionalLegacyRows(db, &scripts, "host_code IN ? AND rule_status = ?", codes, 1); err != nil {
		return out, err
	}
	if err = optionalLegacyRows(db, &urlAllow, "host_code IN ? AND is_enable = ?", codes, 1); err != nil {
		return out, err
	}
	if err = optionalLegacyRows(db, &urlDeny, "host_code IN ? AND is_enable = ?", codes, 1); err != nil {
		return out, err
	}
	mode := wafpipeline.ModeBlock
	if selected.GUARD_STATUS == 0 {
		mode = wafpipeline.ModeOff
	} else if selected.LogOnlyMode == 1 {
		mode = wafpipeline.ModeObserve
	}
	if err := migrateLegacyRequestOptions(db, selected, &c, &out, mode); err != nil {
		return out, err
	}
	if err := s.migrateLegacyTamper(db, hosts, &c, &out, mode); err != nil {
		return out, err
	}
	if err := migrateLegacyPrivacy(db, selected, codes, &c, &out); err != nil {
		return out, err
	}
	if s.LegacyPlugins != nil && selected.GUARD_STATUS == 1 {
		ids, profiles := s.LegacyPlugins()
		if len(ids) > 0 {
			c.Policy.RuleSet.Plugins = &wafpipeline.PluginPolicy{Mode: mode, IDs: ids}
			out.secrets.PluginProfiles = profiles
			out.Warnings = append(out.Warnings, "已启用的pre_check签名插件迁为端口独立进程，在登录后按预算检查；每端口最多4个，进程总额默认32。dry-run不调用插件，启动失败不会接管原监听")
		}
	}
	for _, rule := range scripts {
		c.Policy.RuleSet.Scripts = append(c.Policy.RuleSet.Scripts, wafpipeline.ScriptRule{ID: rule.RuleCode, Source: rule.RuleContent, Mode: mode, Severity: wafpipeline.High})
	}
	matchNames := map[string]string{"等于": "exact", "前缀匹配": "prefix", "后缀匹配": "suffix", "包含匹配": "contains"}
	for _, rule := range urlAllow {
		c.Policy.RuleSet.URLs = append(c.Policy.RuleSet.URLs, wafpipeline.URLRule{ID: "allow-" + rule.Id, Match: matchNames[rule.CompareType], Value: rule.Url, Action: wafpipeline.Allow, Mode: mode})
	}
	for _, rule := range urlDeny {
		c.Policy.RuleSet.URLs = append(c.Policy.RuleSet.URLs, wafpipeline.URLRule{ID: "deny-" + rule.Id, Match: matchNames[rule.CompareType], Value: rule.Url, Action: wafpipeline.Deny, Mode: mode})
	}
	if len(scripts) > 0 {
		if err = optionalLegacyRows(db, &scriptGroups, "user_code = ? AND tenant_id = ?", selected.USER_CODE, selected.Tenant_ID); err != nil {
			return out, err
		}
		if err = optionalLegacyRows(db, &scriptGroupItems, "user_code = ? AND tenant_id = ?", selected.USER_CODE, selected.Tenant_ID); err != nil {
			return out, err
		}
		c.ScriptGroups = map[string][]string{}
		for _, item := range scriptGroupItems {
			c.ScriptGroups[item.GroupCode] = append(c.ScriptGroups[item.GroupCode], item.Ip)
		}
		for _, group := range scriptGroups {
			c.ScriptGroups[group.GroupName] = append([]string{}, c.ScriptGroups[group.GroupCode]...)
		}
		out.Warnings = append(out.Warnings, "自定义规则改在登录门之后执行；Allow只跳过后续攻击检测，Ban通过本端口命中限制异步处理；IP组复制为端口独立快照，失败计数从接管后重新累计")
	}
	if len(urlAllow) > 0 {
		out.Warnings = append(out.Warnings, "URL白名单只跳过登录门之后的攻击检测，不能绕过黑名单、QPS/BOT或登录；URL黑名单优先")
	}
	if err = optionalLegacyRows(db, &allow, "host_code IN ? AND is_enable = ?", codes, 1); err != nil {
		return out, err
	}
	if err = optionalLegacyRows(db, &deny, "host_code IN ? AND is_enable = ?", codes, 1); err != nil {
		return out, err
	}
	groups := map[string][]model.IPGroupItem{}
	add := func(id, kind, value, group, remark string) error {
		values := []string{value}
		if group != "" {
			items, ok := groups[group]
			if !ok {
				if err := optionalLegacyRows(db, &items, "group_code = ? AND user_code = ? AND tenant_id = ?", group, selected.USER_CODE, selected.Tenant_ID); err != nil {
					return err
				}
				if len(items) == 0 {
					return errors.New("名单引用的IP组在源用户/租户中为空或不可读取，请先核对")
				}
				groups[group] = items
			}
			values = nil
			for _, item := range items {
				values = append(values, item.Ip)
			}
		}
		for n, ip := range values {
			entry := wafpipeline.IPEntry{ID: kind + "-" + id + "-" + strconv.Itoa(n), Type: kind, Value: ip, Remark: remark}
			if kind == "allow" {
				entry.Trust = "trusted"
			}
			c.Policy.IPGroup.Entries = append(c.Policy.IPGroup.Entries, entry)
		}
		return nil
	}
	for _, entry := range allow {
		group := ""
		if entry.IpType == model.IPEntryTypeGroup {
			group = entry.GroupCode
		}
		if err = add(entry.Id, "allow", entry.Ip, group, entry.Remarks); err != nil {
			return out, err
		}
	}
	for _, entry := range deny {
		group := ""
		if entry.IpType == model.IPEntryTypeGroup {
			group = entry.GroupCode
		}
		if err = add(entry.Id, "deny", entry.Ip, group, entry.Remarks); err != nil {
			return out, err
		}
	}
	if c.Policy.Gate.Enabled {
		if err = legacyAccountRows(db, &credentials, "host_code = ?", selected.Code); err != nil {
			return out, err
		}
		for _, user := range credentials {
			hash := user.Password
			if !user.PasswordHashed {
				if e := ValidateGatePassword(user.UserName, user.Password); e != nil {
					out.Warnings = append(out.Warnings, "旧账号不满足新口令要求，需设置新账号")
					continue
				}
				hash = ""
				if out.passwords == nil {
					out.passwords = map[string]string{}
				}
				out.passwords[user.UserName] = user.Password
			} else if cost, e := bcrypt.Cost([]byte(hash)); e != nil || cost != bcrypt.DefaultCost {
				out.Warnings = append(out.Warnings, "旧账号哈希无效或成本与当前登录门不同，需重设账号口令")
				continue
			}
			if err = validateGateUsername(user.UserName); err != nil {
				out.Warnings = append(out.Warnings, "旧用户名不满足新规则，需设置新账号")
				continue
			}
			out.accounts = append(out.accounts, model.GateUser{PortID: wafpipeline.PortID(port), Username: user.UserName, PasswordHash: hash, Enabled: true})
			out.AccountNames = append(out.AccountNames, user.UserName)
		}
	}
	accessEvidence, err := migrateLegacyAccess(db, selected, hosts, port, &c, &out)
	if err != nil {
		return out, err
	}
	if len(out.accounts) > 1000 {
		out.Blockers = append(out.Blockers, "旧账号数量超过每端口1000上限")
	}
	if e := wafpipeline.ValidateConfig(c, uint16(global.GWAF_LOCAL_SERVER_PORT)); e != nil {
		out.Blockers = append(out.Blockers, e.Error())
	} else if e := (&wafpipeline.Runtime{}).Prepare(&wafpipeline.Snapshot{Config: c, Secrets: out.secrets, Preview: true}); e != nil {
		out.Blockers = append(out.Blockers, e.Error())
	}
	if err := wafpipeline.ValidateBusinessHeaders(out.secrets.BusinessHeaders); err != nil {
		out.Blockers = append(out.Blockers, err.Error())
	}
	// Encrypt the commitment, including secret-bearing source state. Never
	// expose plaintext credentials or their testable raw digest in a preview.
	credentialSecrets := make([]string, 0, len(credentials))
	for _, user := range credentials {
		credentialSecrets = append(credentialSecrets, user.Password)
	}
	fingerprint, _ := json.Marshal(struct {
		Hosts             []model.Hosts
		Allow             []model.IPAllowList
		Deny              []model.IPBlockList
		Groups            map[string][]model.IPGroupItem
		CC                []model.AntiCC
		Accounts          []model.HttpAuthBase
		Selected          string
		CredentialSecrets []string
		Scripts           []model.Rules
		URLAllow          []model.URLAllowList
		URLDeny           []model.URLBlockList
		ScriptGroups      []model.IPGroup
		ScriptGroupItems  []model.IPGroupItem
		PathRules         []model.HostPathRule
		Balances          []model.LoadBalance
		CacheRules        []model.CacheRule
		DerivedConfig     wafpipeline.PortConfig
		Access            any
		SecretState       wafpipeline.PortSecrets
	}{hosts, allow, deny, groups, cc, credentials, selected.Code, credentialSecrets, scripts, urlAllow, urlDeny, scriptGroups, scriptGroupItems, pathRules, balances, cacheRules, c, accessEvidence, out.secrets})
	sum := sha256.Sum256(fingerprint)
	out.hash = hex.EncodeToString(sum[:])
	token, _ := json.Marshal(migrationToken{out.hash, port, selected.Code, time.Now().Add(15 * time.Minute).Unix()})
	out.PreviewToken, err = wafsec.EncryptBound(token, "pipeline:legacy-preview")
	if err != nil {
		return out, err
	}
	out.Config = c
	var count int64
	if err = db.Model(&model.PortConfig{}).Where("port = ?", port).Count(&count).Error; err != nil {
		return out, err
	}
	out.Imported = count > 0
	if db.Migrator().HasTable(&model.PipelinePortMigration{}) {
		var record model.PipelinePortMigration
		if e := db.First(&record, "port = ?", port).Error; e == nil {
			out.State = record.State
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return out, e
		}
	}
	return out, nil
}

func (s *WafPipelineService) ImportLegacyPort(req LegacyPortImportRequest, operator string) (wafpipeline.PortConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.previewLegacyPort(req.Port, req.PolicyHost)
	if err != nil {
		return p.Config, err
	}
	var token migrationToken
	if len(req.PreviewToken) > 8192 {
		return p.Config, errors.New("迁入预览凭证过长")
	}
	raw, e := wafsec.DecryptBound(req.PreviewToken, "pipeline:legacy-preview")
	if e != nil || json.Unmarshal(raw, &token) != nil || token.Port != req.Port || token.Host != p.PolicyHost || token.Hash != p.hash || token.Expires < time.Now().Unix() {
		return p.Config, errors.New("旧配置已变化或预览已过期，请重新预览")
	}
	if p.Imported {
		return p.Config, errors.New("端口配置已存在，不能重复迁入覆盖")
	}
	if len(p.Blockers) > 0 {
		return p.Config, fmt.Errorf("迁入前需处理：%s", strings.Join(p.Blockers, "；"))
	}
	if !req.AcceptChanges {
		return p.Config, errors.New("必须确认迁入差异与端口统一策略")
	}
	if req.Username != "" || req.Password != "" {
		if err = validateGateUsername(req.Username); err != nil {
			return p.Config, err
		}
		if err = ValidateGatePassword(req.Username, req.Password); err != nil {
			return p.Config, err
		}
		hash, e := utils.BcryptHash(req.Password)
		if e != nil {
			return p.Config, e
		}
		secret := ""
		if p.RequiresTOTP && req.TOTPSecret == "" {
			return p.Config, errors.New("原认证包含二次验证，替换账号必须配置动态口令密钥")
		}
		if req.TOTPSecret != "" {
			secret, err = wafpipeline.NormalizeTOTPSecret(req.TOTPSecret)
			if err != nil {
				return p.Config, err
			}
			p.Config.Policy.Gate.Mode = "form"
		}
		p.accounts = []model.GateUser{{PortID: wafpipeline.PortID(req.Port), Username: req.Username, PasswordHash: hash, TOTPSecret: secret, Enabled: true}}
	}
	if p.Config.Policy.Gate.Enabled && len(p.accounts) == 0 {
		return p.Config, errors.New("登录门启用前必须设置至少一个有效账号")
	}
	for i := range p.accounts {
		if p.accounts[i].PasswordHash == "" {
			plain, ok := p.passwords[p.accounts[i].Username]
			if !ok {
				return p.Config, errors.New("迁入账号缺少有效凭据")
			}
			p.accounts[i].PasswordHash, err = utils.BcryptHash(plain)
			if err != nil {
				return p.Config, err
			}
		}
	}
	db, err := pipelineDB()
	if err != nil {
		return p.Config, err
	}
	var count int64
	if err = db.Model(&model.PortConfig{}).Count(&count).Error; err != nil {
		return p.Config, err
	}
	if count >= int64(wafpipeline.Ports.Limit()) {
		return p.Config, errors.New("端口数量达到上限")
	}
	p.Config.Revision = 1
	p.Config.HostCode = "pipeline-" + strconv.Itoa(int(req.Port))
	p.Config.AccessChangedAt = time.Now().Unix()
	p.Config.AccessChangedBy = operator
	p.secrets.GateKey, err = pipelineSecret()
	if err != nil {
		return p.Config, err
	}
	configJSON, _ := json.Marshal(p.Config)
	secretJSON, _ := json.Marshal(p.secrets)
	sourcesJSON, _ := json.Marshal(p.Sources)
	err = db.Transaction(func(tx *gorm.DB) error {
		row := model.PortConfig{BaseOrm: pipelineORM(), Port: int(req.Port), ConfigJSON: string(configJSON), SecretsJSON: string(secretJSON), Revision: 1}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, user := range p.accounts {
			user.BaseOrm = pipelineORM()
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}
		migration := model.PipelinePortMigration{Port: int(req.Port)}
		e := tx.First(&migration, "port = ?", req.Port).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if migration.State != "active" {
			migration.State = "imported"
		}
		migration.SourcesJSON = string(sourcesJSON)
		migration.SourceHash = p.hash
		migration.ImportedAt = time.Now().Unix()
		migration.Operator = operator
		return tx.Save(&migration).Error
	})
	if err != nil {
		return p.Config, err
	}
	// No listening socket is touched here. Activation remains a separate,
	// previewed operation and is responsible for legacy listener handoff.
	s.audit(operator, req.Port, "legacy_port_imported")
	if err = WafPipelineGateServiceApp.Reload(req.Port); err != nil {
		return p.Config, err
	}
	return p.Config, nil
}

func (s *WafPipelineService) LoadLegacyPortOwnership() error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.PipelinePortMigration{}) {
		return nil
	}
	var rows []model.PipelinePortMigration
	if err = db.Where("state = ?", "active").Find(&rows).Error; err != nil {
		return err
	}
	ports := make([]uint16, 0, len(rows))
	for _, row := range rows {
		if row.Port > 0 && row.Port < 65536 {
			ports = append(ports, uint16(row.Port))
		}
	}
	wafpipeline.ReplaceLegacyPortOwners(ports)
	return nil
}
