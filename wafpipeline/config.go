package wafpipeline

import (
	"HomeWaf/model"
	"HomeWaf/wafenginecore/ipset"
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/idna"
	"io"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const SchemaVersion = 1

type Mode string

const (
	ModeOff     Mode = "off"
	ModeObserve Mode = "observe"
	ModeBlock   Mode = "block"
)

type AccessConfig struct {
	Enabled        bool   `json:"enabled"`
	DisabledAction Action `json:"disabled_action"`
	DrainSeconds   int    `json:"drain_seconds"`
}
type IPEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Trust     string `json:"trust,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Remark    string `json:"remark,omitempty"`
}
type Region struct {
	Country  string `json:"country"`
	Province string `json:"province,omitempty"`
}
type IPGroupConfig struct {
	Entries              []IPEntry `json:"entries"`
	Regions              []Region  `json:"regions"`
	AllowGroups          []string  `json:"allow_groups,omitempty"`
	DenyGroups           []string  `json:"deny_groups,omitempty"`
	DeniedRegions        []Region  `json:"denied_regions,omitempty"`
	BlockOutsideMainland bool      `json:"block_outside_mainland,omitempty"`
	UnlistedAction       Action    `json:"unlisted_action"`
	FeedEnabled          bool      `json:"feed_enabled"`
}
type Scope struct {
	Hosts         []string `json:"hosts,omitempty"`
	Paths         []string `json:"paths,omitempty"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
	Methods       []string `json:"methods,omitempty"`
	ExcludeFields []string `json:"exclude_fields,omitempty"`
}
type Rule struct {
	ID       string                     `json:"id"`
	Category string                     `json:"category"`
	Severity Severity                   `json:"severity"`
	Mode     Mode                       `json:"mode"`
	Scope    Scope                      `json:"scope"`
	Params   map[string]json.RawMessage `json:"params,omitempty"`
}
type RateConfig struct {
	Mode             Mode               `json:"mode"`
	PerIP            float64            `json:"per_ip"`
	PerPort          float64            `json:"per_port"`
	Burst            int                `json:"burst"`
	StaticMultiplier float64            `json:"static_multiplier"`
	Paths            map[string]float64 `json:"paths,omitempty"`
}
type CCConfig struct {
	Mode             Mode   `json:"mode"`
	WindowSeconds    int    `json:"window_seconds"`
	Threshold        int    `json:"threshold"`
	Action           Action `json:"action"`
	ByPath           bool   `json:"by_path"`
	ExcludeStatic    bool   `json:"exclude_static"`
	ChallengeSeconds int    `json:"challenge_seconds"`
}
type RuleSetConfig struct {
	Plugins           *PluginPolicy               `json:"plugins,omitempty"`
	Callback          *CallbackPolicy             `json:"callback,omitempty"`
	Sensitive         *SensitivePolicy            `json:"sensitive,omitempty"`
	UploadOptions     *model.UploadSecurityConfig `json:"upload_options,omitempty"`
	CSRFOptions       *model.CsrfConfig           `json:"csrf_options,omitempty"`
	AntiLeech         *AntiLeechPolicy            `json:"anti_leech,omitempty"`
	Challenge         ChallengePolicy             `json:"challenge"`
	AI                Mode                        `json:"ai,omitempty"`
	AIScope           Scope                       `json:"ai_scope"`
	Scripts           []ScriptRule                `json:"scripts,omitempty"`
	URLs              []URLRule                   `json:"urls,omitempty"`
	Rules             []Rule                      `json:"rules"`
	QPS               RateConfig                  `json:"qps"`
	CC                CCConfig                    `json:"cc"`
	BOT               Mode                        `json:"bot"`
	BOTVerifyDNS      bool                        `json:"bot_verify_dns"`
	BudgetMillis      int                         `json:"budget_ms"`
	View              ViewLimits                  `json:"view"`
	AllowedMethods    []string                    `json:"allowed_methods"`
	CSRFOrigins       []string                    `json:"csrf_origins,omitempty"`
	CSRFMissingOrigin string                      `json:"csrf_missing_origin"`
	OWASP             Mode                        `json:"owasp"`
	OWASPScope        Scope                       `json:"owasp_scope"`
}
type GateConfig struct {
	Enabled                  bool     `json:"enabled"`
	RequireToken             bool     `json:"require_token"`
	Mode                     string   `json:"mode"`
	Prefix                   string   `json:"prefix"`
	Title                    string   `json:"title"`
	Hint                     string   `json:"hint,omitempty"`
	LogoData                 string   `json:"logo_data,omitempty"`
	IdleSeconds              int      `json:"idle_seconds"`
	AbsoluteSeconds          int      `json:"absolute_seconds"`
	RememberDays             int      `json:"remember_days"`
	BindIP                   bool     `json:"bind_ip"`
	MaxSessions              int      `json:"max_sessions"`
	Exempt                   Scope    `json:"exempt"`
	Protect                  Scope    `json:"protect,omitempty"`
	ExemptIPs                []string `json:"exempt_ips,omitempty"`
	BypassTrusted            bool     `json:"bypass_trusted"`
	AllowOPTIONS             bool     `json:"allow_options"`
	LoginPerMinute           int      `json:"login_per_minute"`
	TokenHeader              string   `json:"token_header,omitempty"`
	CredentialCookiePrefixes []string `json:"credential_cookie_prefixes,omitempty"`
	CredentialHeaders        []string `json:"credential_headers,omitempty"`
}
type Threshold struct {
	WindowSeconds int `json:"window_seconds"`
	Count         int `json:"count"`
	BanSeconds    int `json:"ban_seconds"`
}
type LockdownConfig struct {
	Enabled         bool   `json:"enabled"`
	WindowSeconds   int    `json:"window_seconds"`
	DistinctIPs     int    `json:"distinct_ips"`
	Seconds         int    `json:"seconds"`
	CooldownSeconds int    `json:"cooldown_seconds"`
	Action          Action `json:"action"`
}
type HitPolicyConfig struct {
	Enabled         bool               `json:"enabled"`
	BanScope        string             `json:"ban_scope"`
	IPv6Prefix      int                `json:"ipv6_prefix"`
	Critical        Threshold          `json:"critical"`
	High            Threshold          `json:"high"`
	Medium          Threshold          `json:"medium"`
	Lockdown        LockdownConfig     `json:"lockdown"`
	ProtectPrivate  bool               `json:"protect_private"`
	MaxKeys         int                `json:"max_keys"`
	ResponseSignals bool               `json:"response_signals"`
	IgnorePaths     []string           `json:"ignore_paths,omitempty"`
	BusinessLogin   []LoginObservation `json:"business_login,omitempty"`
	LoginBan        Threshold          `json:"login_ban"`
	Response404     Threshold          `json:"response_404"`
	ResponseAuth    Threshold          `json:"response_auth"`
	Response5xx     Threshold          `json:"response_5xx"`
}
type LoginObservation struct {
	Path                  string `json:"path"`
	Method                string `json:"method"`
	UsernameField         string `json:"username_field,omitempty"`
	PasswordField         string `json:"password_field,omitempty"`
	SuccessStatuses       []int  `json:"success_statuses"`
	SuccessLocationPrefix string `json:"success_location_prefix,omitempty"`
	SuccessBodyContains   string `json:"success_body_contains,omitempty"`
	BodyPrefixBytes       int    `json:"body_prefix_bytes"`
}
type HoneypotConfig struct {
	Enabled               bool                    `json:"enabled"`
	CaptureDenied         bool                    `json:"capture_denied"`
	TTLSeconds            int                     `json:"ttl_seconds"`
	MaxConcurrent         int                     `json:"max_concurrent"`
	MaxDelayMillis        int                     `json:"max_delay_ms"`
	TrapPaths             []string                `json:"trap_paths,omitempty"`
	AuthFailThreshold     int                     `json:"auth_fail_threshold"`
	WeakPasswordThreshold int                     `json:"weak_password_threshold"`
	Lures                 []HoneypotLure          `json:"lures,omitempty"`
	DropAfterHits         int                     `json:"drop_after_hits"`
	DropWindowSeconds     int                     `json:"drop_window_seconds"`
	DropSeconds           int                     `json:"drop_seconds"`
	DistributedDrop       HoneypotDistributedDrop `json:"distributed_drop"`
	ArchiveBody           bool                    `json:"archive_body"`
	ArchiveMaxBytes       int                     `json:"archive_max_bytes"`
}

type HoneypotLure struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Weight  int    `json:"weight"`
}

type HoneypotDistributedDrop struct {
	Enabled       bool `json:"enabled"`
	WindowSeconds int  `json:"window_seconds"`
	DistinctIPs   int  `json:"distinct_ips"`
	Seconds       int  `json:"seconds"`
}
type NetworkConfig struct {
	MaxProxyHops          int            `json:"max_proxy_hops"`
	DisableHTTP2          bool           `json:"disable_http2"`
	Response              ResponsePolicy `json:"response"`
	ListenIPVersion       string         `json:"listen_ip_version,omitempty"`
	TrustedProxies        []string       `json:"trusted_proxies"`
	RealIPHeader          string         `json:"real_ip_header"`
	MaxHeaderBytes        int            `json:"max_header_bytes"`
	ReadHeaderSeconds     int            `json:"read_header_seconds"`
	ConnectSeconds        int            `json:"connect_seconds"`
	ResponseHeaderSeconds int            `json:"response_header_seconds"`
	MaxRequestBytes       int64          `json:"max_request_bytes"`
	MaxInFlight           int            `json:"max_in_flight"`
	PreserveHost          bool           `json:"preserve_host"`
}
type Policy struct {
	Access    AccessConfig    `json:"access"`
	IPGroup   IPGroupConfig   `json:"ip_group"`
	RuleSet   RuleSetConfig   `json:"ruleset"`
	HitPolicy HitPolicyConfig `json:"hit_policy"`
	Honeypot  HoneypotConfig  `json:"honeypot"`
	Gate      GateConfig      `json:"gate"`
	Network   NetworkConfig   `json:"network"`
}
type TemplateSource struct {
	ID        string `json:"id,omitempty"`
	Version   int    `json:"version,omitempty"`
	AppliedAt int64  `json:"applied_at,omitempty"`
}
type PortConfig struct {
	ACMEHosts       map[string]string   `json:"acme_hosts,omitempty"`
	Schema          int                 `json:"schema_version"`
	Port            uint16              `json:"port"`
	Name            string              `json:"name"`
	Protocol        string              `json:"protocol"`
	ListenEnabled   bool                `json:"listen_enabled"`
	Upstream        string              `json:"upstream"`
	UpstreamDialIP  string              `json:"upstream_dial_ip,omitempty"`
	HeaderProfile   string              `json:"header_profile,omitempty"`
	Routes          []UpstreamRoute     `json:"routes,omitempty"`
	ScriptGroups    map[string][]string `json:"script_groups,omitempty"`
	HostNames       []string            `json:"host_names"`
	HostCode        string              `json:"host_code,omitempty"`
	Policy          Policy              `json:"policy"`
	Template        TemplateSource      `json:"template"`
	Revision        int64               `json:"revision"`
	AccessChangedAt int64               `json:"access_changed_at,omitempty"`
	AccessChangedBy string              `json:"access_changed_by,omitempty"`
}

// Routes select business destinations within one listener. Security policy,
// accounts, counters and access switch remain properties of the port.
type UpstreamRoute struct {
	Hosts         []string             `json:"hosts"`
	PathPrefix    string               `json:"path_prefix,omitempty"`
	Upstream      string               `json:"upstream"`
	DialIP        string               `json:"dial_ip,omitempty"`
	MatchType     string               `json:"match_type,omitempty"`
	Priority      int                  `json:"priority,omitempty"`
	StripPrefix   bool                 `json:"strip_prefix,omitempty"`
	TargetType    string               `json:"target_type,omitempty"`
	RedirectURL   string               `json:"redirect_url,omitempty"`
	RedirectCode  int                  `json:"redirect_code,omitempty"`
	RedirectPort  uint16               `json:"redirect_port,omitempty"`
	Static        *StaticBusiness      `json:"static,omitempty"`
	HeaderProfile string               `json:"header_profile,omitempty"`
	Cache         *BusinessCacheConfig `json:"cache,omitempty"`
	Balance       string               `json:"balance,omitempty"`
	Backends      []Backend            `json:"backends,omitempty"`
}
type Backend struct {
	Upstream string `json:"upstream"`
	DialIP   string `json:"dial_ip,omitempty"`
	Weight   int    `json:"weight"`
}
type StaticBusiness struct {
	Settings    model.StaticSiteConfig `json:"settings"`
	SPAFallback bool                   `json:"spa_fallback"`
}
type PortSecrets struct {
	PluginProfiles     map[string]PluginLaunch    `json:"plugin_profiles,omitempty"`
	PrivacyConfig      string                     `json:"privacy_config,omitempty"`
	TamperBaselines    map[string]TamperBaseline  `json:"tamper_baselines,omitempty"`
	TLSBindings        map[string]int             `json:"tls_bindings,omitempty"`
	BusinessHeaders    map[string]BusinessHeaders `json:"business_headers,omitempty"`
	GateKey            string                     `json:"gate_key"`
	TLSKey             string                     `json:"tls_key,omitempty"`
	TLSCert            string                     `json:"tls_cert,omitempty"`
	ServiceTokenHashes []string                   `json:"service_token_hashes,omitempty"`
	UpstreamCAPEM      string                     `json:"upstream_ca_pem,omitempty"`
	AdditionalTLS      []TLSCertificate           `json:"additional_tls,omitempty"`
}

type TLSCertificate struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}
type Snapshot struct {
	Config      PortConfig
	Secrets     PortSecrets
	PublishedAt time.Time
	Compiled    *CompiledPolicy
	Preview     bool
}

var Categories = []string{"malformed", "scanner", "sqli", "xss", "upload", "lfi", "rce", "java_inject", "java_deser", "php_deser", "php_inject", "asp_inject", "ssti", "csrf", "ssrf"}

func DefaultPolicy() Policy {
	rules := make([]Rule, 0, len(Categories))
	for _, category := range Categories {
		mode := ModeObserve
		severity := High
		if category == "malformed" || category == "scanner" || category == "sqli" || category == "lfi" || category == "java_inject" || category == "java_deser" {
			mode = ModeBlock
		}
		if category == "rce" || strings.HasPrefix(category, "java_") {
			severity = Critical
		}
		if category == "csrf" {
			severity = Medium
		}
		rules = append(rules, Rule{ID: category, Category: category, Severity: severity, Mode: mode})
	}
	return Policy{
		Access: AccessConfig{Enabled: true, DisabledAction: Deny, DrainSeconds: 30}, IPGroup: IPGroupConfig{Entries: []IPEntry{}, Regions: []Region{}, UnlistedAction: Continue},
		RuleSet:   RuleSetConfig{Rules: rules, QPS: RateConfig{Mode: ModeBlock, PerIP: 30, PerPort: 300, Burst: 100, StaticMultiplier: 4}, CC: CCConfig{Mode: ModeObserve, WindowSeconds: 10, Threshold: 150, Action: Challenge, ByPath: true, ExcludeStatic: true, ChallengeSeconds: 300}, BOT: ModeObserve, BudgetMillis: 20, View: DefaultViewLimits(), AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "PROPFIND", "MKCOL", "MOVE", "COPY", "LOCK", "UNLOCK"}, CSRFMissingOrigin: "observe", OWASP: ModeOff},
		Gate:      GateConfig{Mode: "form", Prefix: "/__homewaf/gate", Title: "HomeWaf 访问验证", IdleSeconds: 7200, AbsoluteSeconds: 86400, BypassTrusted: true, LoginPerMinute: 10},
		HitPolicy: HitPolicyConfig{Enabled: true, BanScope: "global", IPv6Prefix: 64, Critical: Threshold{600, 1, 3600}, High: Threshold{600, 3, 1800}, Medium: Threshold{600, 10, 600}, Lockdown: LockdownConfig{WindowSeconds: 60, DistinctIPs: 5, Seconds: 600, CooldownSeconds: 60, Action: Deny}, ProtectPrivate: true, MaxKeys: 1000, IgnorePaths: []string{"/favicon.ico", "/robots.txt"}},
		Honeypot:  HoneypotConfig{TTLSeconds: 3600, MaxConcurrent: 32, MaxDelayMillis: 250, AuthFailThreshold: 5, WeakPasswordThreshold: 3, DropWindowSeconds: 600, DropSeconds: 10800, DistributedDrop: HoneypotDistributedDrop{WindowSeconds: 600, DistinctIPs: 10, Seconds: 10800}, ArchiveMaxBytes: 16 << 10}, Network: NetworkConfig{MaxProxyHops: 10, TrustedProxies: []string{}, RealIPHeader: "X-Forwarded-For", MaxHeaderBytes: 32 << 10, ReadHeaderSeconds: 10},
	}
}
func StrictDecode(data []byte, out any) error {
	if len(data) > 1<<20 {
		return fmt.Errorf("configuration exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("exactly one JSON value is required")
	}
	return nil
}
func CloneConfig(c PortConfig) PortConfig {
	data, _ := json.Marshal(c)
	var out PortConfig
	_ = json.Unmarshal(data, &out)
	return out
}
func ValidMode(m Mode) bool { return m == ModeOff || m == ModeObserve || m == ModeBlock }
func ValidateConfig(c PortConfig, managePort uint16) error {
	if len(c.ACMEHosts) > 100 {
		return fmt.Errorf("too many certificate challenge hosts")
	}
	for host, source := range c.ACMEHosts {
		if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\\r\n@*") || source == "" || len(source) > 128 || strings.Contains(source, "..") || strings.ContainsAny(source, "/\\\r\n\x00") {
			return fmt.Errorf("invalid certificate challenge binding")
		}
	}
	if c.Schema != SchemaVersion {
		return fmt.Errorf("unsupported schema version")
	}
	if c.Port == 0 || c.Port == managePort {
		return fmt.Errorf("invalid port or management port conflict")
	}
	if c.Protocol != "http" && c.Protocol != "https" {
		return fmt.Errorf("protocol must be http or https")
	}
	if len(c.Name) > 128 || len(c.HostNames) > 100 {
		return fmt.Errorf("name or hostname limit exceeded")
	}
	u, err := url.Parse(c.Upstream)
	if c.Upstream != "" && (err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "") || c.Upstream == "" && len(c.Routes) == 0 {
		return fmt.Errorf("upstream must be an HTTP(S) URL without credentials or fragment")
	}
	if c.UpstreamDialIP != "" {
		if _, err := netip.ParseAddr(c.UpstreamDialIP); err != nil {
			return fmt.Errorf("invalid upstream dial IP")
		}
	}
	if len(c.HeaderProfile) > 128 {
		return fmt.Errorf("invalid business header profile reference")
	}
	if len(c.Routes) > 200 {
		return fmt.Errorf("too many business routes")
	}
	routeKeys := map[string]bool{}
	backendCount := 0
	for _, route := range c.Routes {
		if err := validateBusinessCache(route.Cache); err != nil {
			return err
		}
		backendCount += len(route.Backends)
		if backendCount > 128 || len(route.Backends) > 32 {
			return fmt.Errorf("business backend quota exceeded")
		}
		if len(route.Backends) > 0 && route.TargetType != "" && route.TargetType != "proxy" {
			return fmt.Errorf("only proxy routes support load balancing")
		}
		if route.Balance != "" && route.Balance != "wrr" && route.Balance != "ip_hash" {
			return fmt.Errorf("invalid load balance policy")
		}
		totalWeight := 0
		for _, backend := range route.Backends {
			u, e := url.Parse(backend.Upstream)
			if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" || backend.Weight < 0 || backend.Weight > 1000 {
				return fmt.Errorf("invalid business backend")
			}
			if backend.DialIP != "" {
				if _, err := netip.ParseAddr(backend.DialIP); err != nil {
					return fmt.Errorf("invalid backend connection IP")
				}
			}
			totalWeight += backend.Weight
		}
		if len(route.Backends) > 0 && (totalWeight == 0 || totalWeight > 4096) {
			return fmt.Errorf("backend total weight outside 1..4096")
		}
		if len(route.HeaderProfile) > 128 {
			return fmt.Errorf("invalid route header profile reference")
		}
		if route.DialIP != "" {
			if _, err := netip.ParseAddr(route.DialIP); err != nil {
				return fmt.Errorf("invalid business route dial IP")
			}
		}
		u, err := url.Parse(route.Upstream)
		if (route.TargetType == "" || route.TargetType == "proxy") && (err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "") {
			return fmt.Errorf("invalid business route upstream")
		}
		switch route.TargetType {
		case "", "proxy", "not_found", "unavailable":
		case "https_redirect":
			if route.RedirectPort == 0 || route.RedirectPort == c.Port || c.Protocol != "http" {
				return fmt.Errorf("invalid HTTPS redirect listener")
			}
			if route.RedirectCode != 0 && route.RedirectCode != 301 && route.RedirectCode != 302 && route.RedirectCode != 307 && route.RedirectCode != 308 {
				return fmt.Errorf("invalid HTTPS redirect status")
			}
		case "redirect":
			u, err := url.Parse(route.RedirectURL)
			if err != nil || route.RedirectURL == "" || strings.ContainsAny(route.RedirectURL, "\r\n") || u.User != nil || (u.IsAbs() && (u.Scheme != "https" && u.Scheme != "http")) || (!u.IsAbs() && (!strings.HasPrefix(route.RedirectURL, "/") || strings.HasPrefix(route.RedirectURL, "//"))) {
				return fmt.Errorf("invalid business redirect URL")
			}
			if route.RedirectCode != 0 && route.RedirectCode != 301 && route.RedirectCode != 302 && route.RedirectCode != 307 && route.RedirectCode != 308 {
				return fmt.Errorf("invalid redirect status")
			}
		case "static":
			if route.Static == nil || !filepath.IsAbs(route.Static.Settings.StaticSitePath) || len(route.Static.Settings.StaticSitePath) > 4096 {
				return fmt.Errorf("static business requires an absolute root path")
			}
		default:
			return fmt.Errorf("invalid business route target")
		}
		if route.Priority < -100000 || route.Priority > 100000 {
			return fmt.Errorf("invalid business route priority")
		}
		switch route.MatchType {
		case "", "prefix", "exact":
		case "regex":
			if route.PathPrefix == "" || len(route.PathPrefix) > 1024 {
				return fmt.Errorf("invalid route expression")
			}
			if _, err := regexp.Compile(route.PathPrefix); err != nil {
				return fmt.Errorf("invalid route expression")
			}
		default:
			return fmt.Errorf("invalid route match type")
		}
		if route.StripPrefix && route.MatchType == "regex" {
			return fmt.Errorf("regex routes cannot strip a literal prefix")
		}
		if len(route.Hosts) == 0 || len(route.Hosts) > 100 {
			return fmt.Errorf("business route requires 1..100 hostnames")
		}
		canonicalPrefix := strings.TrimSuffix(route.PathPrefix, "/")
		if canonicalPrefix == "" && route.PathPrefix == "/" {
			canonicalPrefix = "/"
		}
		if route.MatchType != "regex" && route.PathPrefix != "" && (!strings.HasPrefix(route.PathPrefix, "/") || len(route.PathPrefix) > 1024 || NormalizePath(route.PathPrefix) != canonicalPrefix) {
			return fmt.Errorf("invalid business route path prefix")
		}
		for _, host := range route.Hosts {
			if host == "" || len(host) > 253 || strings.ContainsAny(host, "/:@\\\x00\r\n \t") || (strings.Contains(host, "*") && host != "*" && (!strings.HasPrefix(host, "*.") || strings.Count(host, "*") != 1)) {
				return fmt.Errorf("invalid business route hostname")
			}
			matchType := route.MatchType
			if matchType == "" {
				matchType = "prefix"
			}
			key := strings.TrimSuffix(strings.ToLower(host), ".") + "\x00" + matchType + "\x00" + route.PathPrefix
			if routeKeys[key] {
				return fmt.Errorf("duplicate business route match")
			}
			routeKeys[key] = true
		}
	}
	for _, host := range c.HostNames {
		if host == "" || len(host) > 253 || strings.ContainsAny(host, "/:@\\\x00\r\n ") {
			return fmt.Errorf("invalid route hostname")
		}
	}
	if _, err := upstreamDialOverrides(c); err != nil {
		return err
	}
	groupEntries := 0
	if len(c.ScriptGroups) > 1000 {
		return fmt.Errorf("too many script IP groups")
	}
	for name, entries := range c.ScriptGroups {
		if name == "" || len(name) > 255 {
			return fmt.Errorf("invalid script IP group name")
		}
		groupEntries += len(entries)
		if groupEntries > 100000 {
			return fmt.Errorf("script IP group quota exceeded")
		}
		for _, entry := range entries {
			if _, err := ipset.ParsePattern(entry); err != nil {
				return fmt.Errorf("invalid script IP group entry")
			}
		}
	}
	return ValidatePolicy(c.Policy)
}

// Transport canonicalizes internationalized names before dialing. Validate and
// prepare against the same authority keys so a route never bypasses its fixed IP.
func upstreamDialOverrides(c PortConfig) (map[string]string, error) {
	out := map[string]string{}
	add := func(raw, ip string) error {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return fmt.Errorf("invalid upstream")
		}
		host := strings.ToLower(u.Hostname())
		if _, err := netip.ParseAddr(host); err != nil {
			host, err = idna.Lookup.ToASCII(host)
			if err != nil {
				return fmt.Errorf("invalid upstream hostname")
			}
		}
		port := u.Port()
		if port == "" {
			port = "80"
			if u.Scheme == "https" {
				port = "443"
			}
		}
		key := net.JoinHostPort(host, port)
		if previous, ok := out[key]; ok && previous != ip {
			return fmt.Errorf("同一上游地址配置了不同连接IP，请使用明确独立的上游地址")
		}
		out[key] = ip
		return nil
	}
	if c.Upstream != "" {
		if err := add(c.Upstream, c.UpstreamDialIP); err != nil {
			return nil, err
		}
	}
	for _, route := range c.Routes {
		if route.TargetType != "" && route.TargetType != "proxy" {
			continue
		}
		if err := add(route.Upstream, route.DialIP); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func ValidatePolicy(p Policy) error {
	if p.Network.MaxProxyHops < 0 || p.Network.MaxProxyHops > 64 {
		return fmt.Errorf("invalid proxy hop limit")
	}
	if err := validateResponsePolicy(p.Network.Response); err != nil {
		return err
	}
	if err := validateRequestOptions(p.RuleSet); err != nil {
		return err
	}
	if len(p.Gate.CredentialCookiePrefixes) > 32 || len(p.Gate.CredentialHeaders) > 32 {
		return fmt.Errorf("credential namespace quota exceeded")
	}
	for _, name := range append(append([]string{}, p.Gate.CredentialCookiePrefixes...), p.Gate.CredentialHeaders...) {
		if len(name) > 128 || !httpguts.ValidHeaderFieldName(name) {
			return fmt.Errorf("invalid credential namespace")
		}
	}
	if err := ValidateScope(p.Gate.Protect); err != nil {
		return err
	}
	if len(p.Gate.ExemptIPs) > 10000 {
		return fmt.Errorf("too many gate IP exceptions")
	}
	for _, entry := range p.Gate.ExemptIPs {
		if _, err := ipset.ParsePattern(entry); err != nil {
			return fmt.Errorf("invalid gate IP exception")
		}
	}
	if err := validateGateLogo(p.Gate.LogoData); err != nil {
		return err
	}
	if v := p.Network.ListenIPVersion; v != "" && v != "both" && v != "ipv4" && v != "ipv6" {
		return fmt.Errorf("invalid listener IP version")
	}
	if p.Access.DisabledAction != Deny && p.Access.DisabledAction != Drop {
		return fmt.Errorf("invalid access disabled action")
	}
	if p.Access.DrainSeconds < 0 || p.Access.DrainSeconds > 300 {
		return fmt.Errorf("drain_seconds outside 0..300")
	}
	if p.IPGroup.UnlistedAction != Continue && p.IPGroup.UnlistedAction != Deny && p.IPGroup.UnlistedAction != Honeypot {
		return fmt.Errorf("invalid unlisted action")
	}
	if len(p.IPGroup.Entries) > 100000 || len(p.IPGroup.Regions) > 1000 || len(p.IPGroup.DeniedRegions) > 1000 || len(p.IPGroup.AllowGroups)+len(p.IPGroup.DenyGroups) > 256 {
		return fmt.Errorf("IP group quota exceeded")
	}
	for _, codes := range [][]string{p.IPGroup.AllowGroups, p.IPGroup.DenyGroups} {
		seen := map[string]bool{}
		for _, code := range codes {
			if code == "" || len(code) > 128 || strings.TrimSpace(code) != code || seen[code] {
				return fmt.Errorf("invalid or duplicate IP group reference")
			}
			seen[code] = true
		}
	}
	for _, regions := range [][]Region{p.IPGroup.Regions, p.IPGroup.DeniedRegions} {
		for _, region := range regions {
			if strings.TrimSpace(region.Country) == "" || len(region.Country) > 128 || len(region.Province) > 128 {
				return fmt.Errorf("invalid IP region")
			}
		}
	}
	for _, e := range p.IPGroup.Entries {
		if _, err := ipset.ParsePattern(e.Value); err != nil {
			return fmt.Errorf("invalid IP entry: %w", err)
		}
		if e.Type != "allow" && e.Type != "deny" {
			return fmt.Errorf("invalid IP entry type")
		}
		if e.Type == "allow" && e.Trust != "trusted" && e.Trust != "allow_only" {
			return fmt.Errorf("invalid IP trust")
		}
		if e.Value == "" || len(e.Value) > 128 || len(e.Remark) > 500 {
			return fmt.Errorf("invalid IP entry")
		}
	}
	r := p.RuleSet
	if r.AI != "" && !ValidMode(r.AI) {
		return fmt.Errorf("invalid AI rule mode")
	}
	if err := ValidateScope(r.AIScope); err != nil {
		return err
	}
	if len(r.AIScope.ExcludeFields) > 0 {
		return fmt.Errorf("AI field exclusions are unsupported; use path/method scope")
	}
	if err := validateExtensions(r); err != nil {
		return err
	}
	if err := ValidateScope(r.OWASPScope); err != nil {
		return err
	}
	if len(r.OWASPScope.ExcludeFields) > 0 {
		return fmt.Errorf("CRS field exclusions are unsupported; use a path/method scope or native detector field exclusions")
	}
	if r.CSRFMissingOrigin != "allow" && r.CSRFMissingOrigin != "observe" && r.CSRFMissingOrigin != "block" {
		return fmt.Errorf("invalid CSRF missing-origin action")
	}
	if len(r.CSRFOrigins) > 100 {
		return fmt.Errorf("too many CSRF origin exceptions")
	}
	for _, origin := range r.CSRFOrigins {
		if _, ok := canonicalOrigin(origin, false); !ok {
			return fmt.Errorf("invalid CSRF origin exception")
		}
	}
	if len(r.Rules) > 256 || r.BudgetMillis < 1 || r.BudgetMillis > 1000 || r.View.BodyBytes < 1024 || r.View.BodyBytes > 16<<20 || r.View.ExpansionRatio < 1 || r.View.ExpansionRatio > 100 || r.View.MaxFields < 1 || r.View.MaxFields > 10000 {
		return fmt.Errorf("invalid detector budget or quota")
	}
	categories := map[string]bool{}
	for _, s := range Categories {
		categories[s] = true
	}
	ids := map[string]bool{}
	for _, rule := range r.Rules {
		if rule.ID == "" || len(rule.ID) > 128 || ids[rule.ID] || !categories[rule.Category] || !ValidMode(rule.Mode) || rule.Severity.Rank() == 0 {
			return fmt.Errorf("invalid or duplicate rule %q", rule.ID)
		}
		ids[rule.ID] = true
		if err := ValidateScope(rule.Scope); err != nil {
			return err
		}
		if len(rule.Params) > 32 {
			return fmt.Errorf("too many rule parameters")
		}
		for key, value := range rule.Params {
			if rule.Category != "scanner" || key != "probe_paths" {
				return fmt.Errorf("unsupported parameter %q for %s", key, rule.Category)
			}
			var paths []string
			if err := json.Unmarshal(value, &paths); err != nil || len(paths) > 128 {
				return fmt.Errorf("invalid scanner probe_paths")
			}
			for _, p := range paths {
				if !strings.HasPrefix(p, "/") || len(p) > 1024 || NormalizePath(p) != p {
					return fmt.Errorf("scanner probe paths must be canonical absolute paths")
				}
			}
		}
	}
	if !ValidMode(r.QPS.Mode) || !ValidMode(r.CC.Mode) || !ValidMode(r.BOT) || !ValidMode(r.OWASP) {
		return fmt.Errorf("invalid detector mode")
	}
	if r.QPS.PerIP < 1 || r.QPS.PerIP > 100000 || r.QPS.PerPort < 1 || r.QPS.PerPort > 1000000 || r.QPS.Burst < 1 || r.QPS.Burst > 100000 || r.QPS.StaticMultiplier < 1 || r.QPS.StaticMultiplier > 100 || len(r.QPS.Paths) > 200 {
		return fmt.Errorf("invalid QPS quota")
	}
	for p, n := range r.QPS.Paths {
		if !strings.HasPrefix(p, "/") || len(p) > 1024 || n < 1 || n > 100000 {
			return fmt.Errorf("invalid path QPS quota")
		}
	}
	if r.CC.WindowSeconds < 1 || r.CC.WindowSeconds > 3600 || r.CC.Threshold < 1 || r.CC.Threshold > 1000000 || (r.CC.Action != Deny && r.CC.Action != Challenge) {
		return fmt.Errorf("invalid CC configuration")
	}
	for _, method := range r.AllowedMethods {
		if method == "" || len(method) > 32 || strings.ContainsAny(method, " \r\n\t") {
			return fmt.Errorf("invalid HTTP method")
		}
	}
	g := p.Gate
	if g.Mode != "form" && g.Mode != "basic" {
		return fmt.Errorf("invalid gate mode")
	}
	if !strings.HasPrefix(g.Prefix, "/") || strings.ContainsAny(g.Prefix, "?#%\\\r\n") || strings.Contains(g.Prefix, "..") || strings.Trim(g.Prefix, "/") == "" || len(g.Prefix) > 128 {
		return fmt.Errorf("invalid gate path prefix")
	}
	if NormalizePath(g.Prefix) != strings.TrimRight(g.Prefix, "/") {
		return fmt.Errorf("gate prefix must be a canonical path")
	}
	if g.IdleSeconds < 0 || g.AbsoluteSeconds < 60 || g.AbsoluteSeconds > 30*86400 || g.RememberDays < 0 || g.RememberDays > 30 || g.LoginPerMinute < 1 || g.LoginPerMinute > 100 || g.MaxSessions < 0 || g.MaxSessions > 10000 {
		return fmt.Errorf("invalid gate session or login quota")
	}
	if g.IdleSeconds > 30*86400 || len(g.Title) > 200 || len(g.Hint) > 1000 || r.CC.ChallengeSeconds < 1 || r.CC.ChallengeSeconds > 3600 {
		return fmt.Errorf("invalid gate display or timeout")
	}
	if g.TokenHeader != "" && !validGateTokenHeader(g.TokenHeader) {
		return fmt.Errorf("token_header must be a dedicated X- header, excluding forwarding and internal control headers")
	}
	if g.RequireToken && (g.Enabled || g.TokenHeader == "") {
		return fmt.Errorf("require_token needs a token header and the account login gate disabled")
	}
	if err := ValidateScope(g.Exempt); err != nil {
		return err
	}
	h := p.HitPolicy
	for _, threshold := range []Threshold{h.LoginBan, h.Response404, h.ResponseAuth, h.Response5xx} {
		if threshold == (Threshold{}) {
			continue
		}
		if threshold.WindowSeconds < 1 || threshold.WindowSeconds > 86400 || threshold.Count < 1 || threshold.Count > 100000 || threshold.BanSeconds < 0 || threshold.BanSeconds > 7*86400 {
			return fmt.Errorf("invalid response or login threshold")
		}
	}
	if h.LoginBan != (Threshold{}) && h.LoginBan.BanSeconds == 0 {
		return fmt.Errorf("login ban duration is required")
	}
	if h.Response404.Count > 128 {
		return fmt.Errorf("distinct 404 path limit is 128")
	}
	if len(h.BusinessLogin) > 10 {
		return fmt.Errorf("too many business login observers")
	}
	for _, login := range h.BusinessLogin {
		if login.Path == "" || NormalizePath(login.Path) != login.Path || !strings.HasPrefix(login.Path, "/") || len(login.Path) > 1024 || len(login.Method) > 32 || login.Method == "" || strings.ContainsAny(login.Method, " \r\n\t") || len(login.SuccessStatuses) == 0 || len(login.SuccessStatuses) > 20 || login.BodyPrefixBytes < 1 || login.BodyPrefixBytes > 8192 || len(login.SuccessBodyContains) > 1024 || len(login.SuccessLocationPrefix) > 1024 || len(login.UsernameField) > 128 || len(login.PasswordField) > 128 {
			return fmt.Errorf("invalid business login observer")
		}
		for _, status := range login.SuccessStatuses {
			if status < 200 || status > 399 {
				return fmt.Errorf("business login success status must be 2xx/3xx")
			}
		}
	}
	if h.BanScope != "global" && h.BanScope != "port" {
		return fmt.Errorf("invalid ban scope")
	}
	if h.IPv6Prefix < 32 || h.IPv6Prefix > 128 || h.MaxKeys < 100 || h.MaxKeys > 100000 {
		return fmt.Errorf("invalid state quota")
	}
	for _, q := range []Threshold{h.Critical, h.High, h.Medium} {
		if q.WindowSeconds < 1 || q.WindowSeconds > 86400 || q.Count < 1 || q.Count > 100000 || q.BanSeconds < 1 || q.BanSeconds > 7*86400 {
			return fmt.Errorf("invalid hit threshold")
		}
	}
	l := h.Lockdown
	if l.WindowSeconds < 1 || l.WindowSeconds > 3600 || l.DistinctIPs < 2 || l.DistinctIPs > 1000 || l.Seconds < 1 || l.Seconds > 86400 || l.CooldownSeconds < 0 || l.CooldownSeconds > 86400 || (l.Action != Deny && l.Action != Drop) {
		return fmt.Errorf("invalid lockdown configuration")
	}
	hp := p.Honeypot
	if len(hp.TrapPaths) > 128 || len(h.IgnorePaths) > 128 {
		return fmt.Errorf("too many signal paths")
	}
	for _, path := range append(append([]string{}, hp.TrapPaths...), h.IgnorePaths...) {
		if !strings.HasPrefix(path, "/") || len(path) > 1024 || NormalizePath(path) != path {
			return fmt.Errorf("signal paths must be canonical absolute paths")
		}
	}
	if hp.TTLSeconds < 1 || hp.TTLSeconds > 86400 || hp.MaxConcurrent < 1 || hp.MaxConcurrent > 256 || hp.MaxDelayMillis < 0 || hp.MaxDelayMillis > 2000 || hp.AuthFailThreshold < 1 || hp.WeakPasswordThreshold < 1 {
		return fmt.Errorf("invalid honeypot configuration")
	}
	if err := validateHoneypotControls(hp); err != nil {
		return err
	}
	if p.Network.MaxHeaderBytes < 4096 || p.Network.MaxHeaderBytes > 1<<20 || p.Network.ReadHeaderSeconds < 1 || p.Network.ReadHeaderSeconds > 60 || len(p.Network.TrustedProxies) > 256 {
		return fmt.Errorf("invalid network limits")
	}
	if p.Network.ConnectSeconds < 0 || p.Network.ConnectSeconds > 120 || p.Network.ResponseHeaderSeconds < 0 || p.Network.ResponseHeaderSeconds > 300 || p.Network.MaxRequestBytes < 0 || p.Network.MaxRequestBytes > 1<<50 || p.Network.MaxInFlight < 0 || p.Network.MaxInFlight > 256 {
		return fmt.Errorf("invalid proxy limits")
	}
	for _, prefix := range p.Network.TrustedProxies {
		if q, err := netip.ParsePrefix(prefix); err != nil || q.Bits() == 0 {
			return fmt.Errorf("trusted proxies require explicit non-catchall CIDRs")
		}
	}
	return nil
}
func ValidateScope(s Scope) error {
	if len(s.Hosts)+len(s.Paths)+len(s.ExcludePaths)+len(s.ExcludeFields)+len(s.Methods) > 512 {
		return fmt.Errorf("scope quota exceeded")
	}
	for _, h := range s.Hosts {
		if h == "" || len(h) > 255 || strings.ContainsAny(h, "/\\?#@\r\n\t ") {
			return fmt.Errorf("invalid scope host")
		}
	}
	for _, p := range append(append([]string{}, s.Paths...), s.ExcludePaths...) {
		if !strings.HasPrefix(p, "/") || len(p) > 1024 {
			return fmt.Errorf("scope paths must be absolute path prefixes")
		}
	}
	return nil
}
