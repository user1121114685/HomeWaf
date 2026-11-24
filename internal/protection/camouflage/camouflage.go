// Package camouflage provides a small, protocol-aware decoy listener.
//
// The listener is deliberately independent from the HTTP pipeline. It can
// answer common service probes before a request reaches the access-log path,
// while keeping the configured service identity and the resulting evidence
// visible to the management console.
package camouflage

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultTCPPort    uint16 = 8080
	DefaultTLSPort    uint16 = 8443
	DefaultBanDays           = 90
	DefaultBanSeconds        = DefaultBanDays * 24 * 60 * 60
	maxReadBytes             = 64 * 1024
	maxEvents                = 500
)

type Config struct {
	Enabled         bool   `json:"enabled"`
	Port            uint16 `json:"port"`
	Profile         string `json:"profile"`
	Version         string `json:"version"`
	ServerName      string `json:"server_name"`
	BlockPublic     bool   `json:"block_public"`
	BanSeconds      int    `json:"ban_seconds"`
	ListenIPVersion string `json:"listen_ip_version"`
}

type Profile struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Group          string `json:"group"`
	Transport      string `json:"transport"`
	TLS            bool   `json:"tls"`
	DefaultPort    uint16 `json:"default_port"`
	DefaultVersion string `json:"default_version"`
	Description    string `json:"description"`
}

type Status struct {
	Enabled    bool     `json:"enabled"`
	Running    bool     `json:"running"`
	Port       uint16   `json:"port"`
	Profile    string   `json:"profile"`
	Transport  string   `json:"transport"`
	Active     int64    `json:"active"`
	TotalHits  uint64   `json:"total_hits"`
	LastHitAt  int64    `json:"last_hit_at"`
	LastError  string   `json:"last_error,omitempty"`
	LocalCIDRs []string `json:"local_cidrs"`
}

type Hit struct {
	ID         string `json:"id"`
	Time       int64  `json:"time"`
	ClientIP   string `json:"client_ip"`
	Port       uint16 `json:"port"`
	Profile    string `json:"profile"`
	Transport  string `json:"transport"`
	Probe      string `json:"probe"`
	Protocol   string `json:"protocol"`
	Product    string `json:"product,omitempty"`
	Version    string `json:"version,omitempty"`
	Score      int    `json:"score"`
	Confidence string `json:"confidence"`
	Evidence   string `json:"evidence"`
	LocalLAN   bool   `json:"local_lan"`
	Public     bool   `json:"public"`
	Blocked    bool   `json:"blocked"`
	BlockError string `json:"block_error,omitempty"`
}

type HitHandler func(Hit) Hit

var profiles = []Profile{
	{Code: "http", Name: "通用 Web HTTP", Group: "常用协议", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "nginx/1.24.0", Description: "返回稳定的 HTTP/1.1 头部与常见首页响应"},
	{Code: "https", Name: "通用 Web HTTPS", Group: "常用协议", Transport: "tcp", TLS: true, DefaultPort: DefaultTLSPort, DefaultVersion: "nginx/1.24.0", Description: "先完成 TLS，再返回普通 HTTPS 服务响应"},
	{Code: "ssh", Name: "OpenSSH", Group: "常用协议", Transport: "tcp", DefaultPort: 22, DefaultVersion: "OpenSSH_9.6p1", Description: "发送标准 SSH-2.0 服务端版本行"},
	{Code: "ftp", Name: "FTP", Group: "常用协议", Transport: "tcp", DefaultPort: 21, DefaultVersion: "vsftpd 3.0.5", Description: "返回 FTP 220 欢迎语和基础命令响应"},
	{Code: "ftps", Name: "FTP over TLS", Group: "常用协议", Transport: "tcp", TLS: true, DefaultPort: 990, DefaultVersion: "FileZilla Server 1.8.2", Description: "在 TLS 上提供兼容 FTP 的欢迎语"},
	{Code: "smtp", Name: "SMTP", Group: "常用协议", Transport: "tcp", DefaultPort: 25, DefaultVersion: "Postfix", Description: "返回 ESMTP 欢迎语和 EHLO 能力列表"},
	{Code: "imap", Name: "IMAP4", Group: "常用协议", Transport: "tcp", DefaultPort: 143, DefaultVersion: "Dovecot", Description: "返回 IMAP4rev1 能力与退出响应"},
	{Code: "pop3", Name: "POP3", Group: "常用协议", Transport: "tcp", DefaultPort: 110, DefaultVersion: "Dovecot", Description: "返回 POP3 ready 欢迎语"},
	{Code: "dns", Name: "DNS / BIND", Group: "家庭常用", Transport: "udp", DefaultPort: 53, DefaultVersion: "BIND 9.18.24", Description: "回答 version.bind TXT 查询和基础 DNS 探针"},
	{Code: "smb", Name: "SMB / CIFS", Group: "家庭常用", Transport: "tcp", DefaultPort: 445, DefaultVersion: "Samba 4.19.5", Description: "返回 SMB 协商响应，适合 NAS 探测画像"},
	{Code: "rtsp", Name: "RTSP 摄像头", Group: "家庭常用", Transport: "tcp", DefaultPort: 554, DefaultVersion: "go2rtc", Description: "返回 RTSP OPTIONS 响应和 Server 字段"},
	{Code: "sip", Name: "SIP / VoIP", Group: "家庭常用", Transport: "udp", DefaultPort: 5060, DefaultVersion: "Asterisk 20.6.0", Description: "回答 SIP OPTIONS 请求"},
	{Code: "mqtt", Name: "MQTT", Group: "家庭常用", Transport: "tcp", DefaultPort: 1883, DefaultVersion: "Eclipse Mosquitto 2.0.18", Description: "返回 MQTT 连接确认帧"},
	{Code: "nas-generic", Name: "通用家庭 NAS", Group: "NAS 厂商画像", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "NAS Web Console", Description: "不携带厂商标识的通用 NAS 管理入口，默认画像"},
	{Code: "synology-http", Name: "Synology DSM", Group: "NAS 厂商画像", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "DSM 7.2.1-69057", Description: "返回 DSM 风格首页和 nginx 头部"},
	{Code: "fnos-http", Name: "飞牛 fnOS", Group: "NAS 厂商画像", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "fnOS 1.1.16", Description: "返回飞牛管理入口风格首页，不暴露真实后端"},
	{Code: "qnap-http", Name: "威联通 QNAP", Group: "NAS 厂商画像", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "QTS 5.1.7", Description: "返回 QNAP 管理入口风格首页"},
	{Code: "xiaomi-http", Name: "小米家庭设备", Group: "NAS 厂商画像", Transport: "tcp", DefaultPort: DefaultTCPPort, DefaultVersion: "Mi Router 4A", Description: "返回小米家庭设备管理入口风格首页"},
	{Code: "mysql", Name: "MySQL / MariaDB", Group: "数据库", Transport: "tcp", DefaultPort: 3306, DefaultVersion: "8.0.36", Description: "返回兼容 MySQL 协议的初始握手包"},
	{Code: "postgresql", Name: "PostgreSQL", Group: "数据库", Transport: "tcp", DefaultPort: 5432, DefaultVersion: "16.2", Description: "返回 PostgreSQL 错误与启动包响应"},
	{Code: "redis", Name: "Redis", Group: "数据库", Transport: "tcp", DefaultPort: 6379, DefaultVersion: "7.2.4", Description: "回答 INFO 和基础 RESP 探针"},
	{Code: "memcached", Name: "Memcached", Group: "数据库", Transport: "tcp", DefaultPort: 11211, DefaultVersion: "1.6.21", Description: "回答 stats version 探针"},
	{Code: "ldap", Name: "LDAP", Group: "基础服务", Transport: "tcp", DefaultPort: 389, DefaultVersion: "OpenLDAP 2.6.7", Description: "返回基础 LDAP bind/search 响应"},
	{Code: "snmp", Name: "SNMP", Group: "基础服务", Transport: "udp", DefaultPort: 161, DefaultVersion: "Net-SNMP 5.9.4", Description: "回答 SNMPv1/v3 的最小可识别响应"},
	{Code: "ntp", Name: "NTP", Group: "基础服务", Transport: "udp", DefaultPort: 123, DefaultVersion: "chrony 4.5", Description: "返回标准 NTP server response"},
	{Code: "rdp", Name: "RDP", Group: "远程管理", Transport: "tcp", DefaultPort: 3389, DefaultVersion: "Microsoft Terminal Services", Description: "返回 X.224/RDP 协商响应"},
}

func Profiles() []Profile {
	out := append([]Profile(nil), profiles...)
	return out
}

func ProfileByCode(code string) (Profile, bool) {
	for _, p := range profiles {
		if p.Code == code {
			return p, true
		}
	}
	return Profile{}, false
}

func DefaultConfig() Config {
	return Config{Enabled: false, Port: DefaultTCPPort, Profile: "nas-generic", Version: "NAS Web Console", ServerName: "nas", BlockPublic: true, BanSeconds: DefaultBanSeconds, ListenIPVersion: "dual"}
}

func NormalizeConfig(cfg Config) (Config, error) {
	if cfg.Port == 0 {
		cfg.Port = DefaultTCPPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, errors.New("camouflage port must be between 1 and 65535")
	}
	if cfg.Profile == "" {
		cfg.Profile = "nas-generic"
	}
	p, ok := ProfileByCode(cfg.Profile)
	if !ok {
		return cfg, fmt.Errorf("unknown camouflage profile %q", cfg.Profile)
	}
	if cfg.Version == "" {
		cfg.Version = p.DefaultVersion
	}
	if cfg.ServerName == "" {
		cfg.ServerName = "nas"
	}
	if len(cfg.ServerName) > 128 || strings.ContainsAny(cfg.ServerName, "\r\n") {
		return cfg, errors.New("invalid camouflage server name")
	}
	if len(cfg.Version) > 128 || strings.ContainsAny(cfg.Version, "\r\n") {
		return cfg, errors.New("invalid camouflage version")
	}
	// Automatic non-local hits always use the three-month defense policy. Keep
	// the persisted field for compatibility, but do not let API input weaken it.
	cfg.BanSeconds = DefaultBanSeconds
	cfg.BlockPublic = true
	if cfg.ListenIPVersion == "" {
		cfg.ListenIPVersion = "dual"
	}
	if cfg.ListenIPVersion != "dual" && cfg.ListenIPVersion != "ipv4" && cfg.ListenIPVersion != "ipv6" {
		return cfg, errors.New("listen_ip_version must be dual, ipv4 or ipv6")
	}
	return cfg, nil
}

type Manager struct {
	mu             sync.Mutex
	lifecycleMu    sync.Mutex
	cfg            Config
	profile        Profile
	cert           tls.Certificate
	listener       net.Listener
	packet         net.PacketConn
	stop           chan struct{}
	wg             sync.WaitGroup
	active         atomic.Int64
	total          atomic.Uint64
	lastHit        atomic.Int64
	lastErr        string
	sink           func(Hit) Hit
	events         []Hit
	managementLANs map[string]struct{}
}

func NewManager(sink func(Hit) Hit) *Manager {
	cfg := DefaultConfig()
	profile, _ := ProfileByCode(cfg.Profile)
	return &Manager{cfg: cfg, profile: profile, sink: sink, managementLANs: map[string]struct{}{}}
}

func (m *Manager) ObserveManagementClientIP(raw string) {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return
	}
	addr = addr.Unmap()
	if !addr.IsPrivate() {
		return
	}
	bits := 24
	if addr.Is6() {
		bits = 64
	}
	prefix := netip.PrefixFrom(addr, bits).Masked().String()
	m.mu.Lock()
	m.managementLANs[prefix] = struct{}{}
	m.mu.Unlock()
}

func (m *Manager) isLocalLAN(addr netip.Addr) bool {
	return isLocalLANWithCIDRs(addr, m.localCIDRs())
}

func (m *Manager) localCIDRs() []string {
	out := LocalCIDRs()
	seen := make(map[string]struct{}, len(out))
	for _, prefix := range out {
		seen[prefix] = struct{}{}
	}
	m.mu.Lock()
	for prefix := range m.managementLANs {
		if _, ok := seen[prefix]; !ok {
			out = append(out, prefix)
		}
	}
	m.mu.Unlock()
	sort.Strings(out)
	return out
}

func (m *Manager) Apply(cfg Config) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	normalized, err := NormalizeConfig(cfg)
	if err != nil {
		return err
	}
	m.stopLocked()
	p, _ := ProfileByCode(normalized.Profile)
	var cert tls.Certificate
	if p.TLS {
		cert, err = selfSignedCertificate(normalized.ServerName)
		if err != nil {
			m.setError(err)
			return err
		}
	}
	m.mu.Lock()
	m.cfg, m.profile, m.cert, m.lastErr = normalized, p, cert, ""
	m.mu.Unlock()
	if !normalized.Enabled {
		return nil
	}
	network := "tcp"
	if normalized.ListenIPVersion == "ipv4" {
		network = "tcp4"
	} else if normalized.ListenIPVersion == "ipv6" {
		network = "tcp6"
	}
	stop := make(chan struct{})
	if p.Transport == "udp" {
		udpNetwork := strings.Replace(network, "tcp", "udp", 1)
		pc, listenErr := net.ListenPacket(udpNetwork, ":"+strconv.Itoa(int(normalized.Port)))
		if listenErr != nil {
			m.setError(listenErr)
			return fmt.Errorf("camouflage port %d cannot listen: %w", normalized.Port, listenErr)
		}
		m.mu.Lock()
		m.packet, m.stop = pc, stop
		m.mu.Unlock()
		m.wg.Add(1)
		go m.serveUDP(pc, stop, normalized, p)
		return nil
	}
	ln, listenErr := net.Listen(network, ":"+strconv.Itoa(int(normalized.Port)))
	if listenErr != nil {
		m.setError(listenErr)
		return fmt.Errorf("camouflage port %d cannot listen: %w", normalized.Port, listenErr)
	}
	m.mu.Lock()
	m.listener, m.stop = ln, stop
	m.mu.Unlock()
	m.wg.Add(1)
	go m.serveTCP(ln, stop, normalized, p, cert)
	return nil
}

func (m *Manager) Stop() {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	m.mu.Lock()
	ln, pc, stop := m.listener, m.packet, m.stop
	m.listener, m.packet, m.stop = nil, nil, nil
	m.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if ln != nil {
		_ = ln.Close()
	}
	if pc != nil {
		_ = pc.Close()
	}
	m.wg.Wait()
}

func (m *Manager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.lastErr = err.Error()
	}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	cfg, p, lastErr := m.cfg, m.profile, m.lastErr
	running := m.listener != nil || m.packet != nil
	m.mu.Unlock()
	return Status{Enabled: cfg.Enabled, Running: running, Port: cfg.Port, Profile: cfg.Profile, Transport: p.Transport, Active: m.active.Load(), TotalHits: m.total.Load(), LastHitAt: m.lastHit.Load(), LastError: lastErr, LocalCIDRs: m.localCIDRs()}
}

func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) Events(limit int) []Hit {
	if limit <= 0 || limit > maxEvents {
		limit = 100
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}
	return append([]Hit(nil), m.events[start:]...)
}

func (m *Manager) ClearEvents() {
	m.mu.Lock()
	m.events = nil
	m.mu.Unlock()
}

func (m *Manager) record(hit Hit) {
	hit.Time = time.Now().Unix()
	hit.ID = randomID()
	hit.LocalLAN = m.isLocalLAN(mustAddr(hit.ClientIP))
	hit.Public = !hit.LocalLAN && isPublic(mustAddr(hit.ClientIP))
	m.total.Add(1)
	m.lastHit.Store(hit.Time)
	m.mu.Lock()
	if len(m.events) >= maxEvents {
		copy(m.events, m.events[len(m.events)-maxEvents+1:])
		m.events = m.events[:maxEvents-1]
	}
	m.events = append(m.events, hit)
	sink := m.sink
	m.mu.Unlock()
	if sink != nil {
		updated := sink(hit)
		if updated.Blocked || updated.BlockError != "" {
			m.mu.Lock()
			for i := len(m.events) - 1; i >= 0; i-- {
				if m.events[i].ID == hit.ID {
					m.events[i] = updated
					break
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) serveTCP(ln net.Listener, stop <-chan struct{}, cfg Config, p Profile, cert tls.Certificate) {
	defer m.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			m.setError(err)
			return
		}
		m.active.Add(1)
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			defer m.active.Add(-1)
			m.handleTCP(conn, cfg, p, cert)
		}()
	}
}

func (m *Manager) handleTCP(raw net.Conn, cfg Config, p Profile, cert tls.Certificate) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(3 * time.Second))
	conn := raw
	if p.TLS {
		tlsConn := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}})
		if err := tlsConn.Handshake(); err != nil {
			m.record(Hit{ClientIP: remoteIP(raw), Port: cfg.Port, Profile: p.Code, Transport: p.Transport, Probe: "TLSHandshake", Protocol: "TLS", Version: cfg.Version, Score: 80, Confidence: confidence(80), Evidence: "TLS handshake probe failed"})
			return
		}
		conn = tlsConn
	}
	var payload []byte
	if greeting := tcpGreeting(p, cfg); len(greeting) > 0 {
		_, _ = conn.Write(greeting)
	}
	payload = readSome(conn)
	probe, protocol, version, score, evidence := classify(p.Code, payload, cfg.Version)
	if response := tcpResponse(p, cfg, payload); len(response) > 0 {
		_, _ = conn.Write(response)
	}
	m.record(Hit{ClientIP: remoteIP(raw), Port: cfg.Port, Profile: p.Code, Transport: p.Transport, Probe: probe, Protocol: protocol, Product: p.Name, Version: version, Score: score, Confidence: confidence(score), Evidence: evidence})
}

func (m *Manager) serveUDP(pc net.PacketConn, stop <-chan struct{}, cfg Config, p Profile) {
	defer m.wg.Done()
	buffer := make([]byte, maxReadBytes)
	for {
		_ = pc.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := pc.ReadFrom(buffer)
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			m.setError(err)
			return
		}
		payload := append([]byte(nil), buffer[:n]...)
		probe, protocol, version, score, evidence := classify(p.Code, payload, cfg.Version)
		if response := udpResponse(p, cfg, payload); len(response) > 0 {
			_, _ = pc.WriteTo(response, addr)
		}
		client := ""
		if host, _, splitErr := net.SplitHostPort(addr.String()); splitErr == nil {
			client = host
		}
		m.record(Hit{ClientIP: client, Port: cfg.Port, Profile: p.Code, Transport: p.Transport, Probe: probe, Protocol: protocol, Product: p.Name, Version: version, Score: score, Confidence: confidence(score), Evidence: evidence})
	}
}

func tcpGreeting(p Profile, cfg Config) []byte {
	version := cfg.Version
	switch p.Code {
	case "ssh":
		return []byte("SSH-2.0-" + version + "\r\n")
	case "ftp", "ftps":
		return []byte("220 " + cfg.ServerName + " FTP server (" + version + ") ready.\r\n")
	case "smtp":
		return []byte("220 " + cfg.ServerName + " ESMTP " + version + "\r\n")
	case "pop3":
		return []byte("+OK " + cfg.ServerName + " POP3 server ready\r\n")
	case "imap":
		return []byte("* OK " + cfg.ServerName + " IMAP4rev1 " + version + " ready\r\n")
	case "mysql":
		return mysqlHandshake(version)
	}
	return nil
}

func tcpResponse(p Profile, cfg Config, payload []byte) []byte {
	switch p.Code {
	case "http", "https":
		return httpResponse(cfg, "Home NAS Web Console", "")
	case "synology-http":
		return httpResponse(cfg, "Synology DiskStation", "Synology")
	case "fnos-http":
		return httpResponse(cfg, "飞牛 NAS", "fnOS")
	case "qnap-http":
		return httpResponse(cfg, "QNAP TurboStation", "QTS")
	case "xiaomi-http":
		return httpResponse(cfg, "Mi Home", "Mi Router")
	case "nas-generic":
		return httpResponse(cfg, "NAS Web Console", "NAS")
	case "ssh":
		return nil
	case "ftp", "ftps":
		if bytes.HasPrefix(bytes.ToUpper(payload), []byte("SYST")) {
			return []byte("215 UNIX Type: L8\r\n")
		}
		return []byte("530 Please login with USER and PASS.\r\n")
	case "smtp":
		if bytes.HasPrefix(bytes.ToUpper(payload), []byte("EHLO")) || bytes.HasPrefix(bytes.ToUpper(payload), []byte("HELO")) {
			return []byte("250-" + cfg.ServerName + "\r\n250-SIZE 52428800\r\n250-STARTTLS\r\n250 HELP\r\n")
		}
		return []byte("221 2.0.0 Bye\r\n")
	case "imap":
		return []byte("* CAPABILITY IMAP4rev1 STARTTLS AUTH=PLAIN\r\nA001 OK Completed\r\n")
	case "pop3":
		return []byte("+OK Capability list follows\r\n+OK STLS USER\r\n.\r\n")
	case "rtsp":
		return []byte("RTSP/1.0 200 OK\r\nCSeq: 1\r\nServer: " + cfg.Version + "\r\nPublic: OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN\r\nContent-Length: 0\r\n\r\n")
	case "smb":
		return smbResponseV2()
	case "rdp":
		return []byte{0x03, 0x00, 0x00, 0x0b, 0x06, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00}
	case "mysql":
		return nil
	case "postgresql":
		return postgresErrorResponse()
	case "redis":
		return redisInfoResponse(cfg.Version)
	case "memcached":
		return memcachedStatsResponse(cfg.Version)
	case "ldap":
		return []byte{0x30, 0x0c, 0x02, 0x01, 0x01, 0x61, 0x07, 0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00}
	case "mqtt":
		return []byte{0x20, 0x02, 0x00, 0x00}
	}
	return nil
}

func udpResponse(p Profile, cfg Config, payload []byte) []byte {
	switch p.Code {
	case "dns":
		return dnsResponse(payload, cfg.Version)
	case "sip":
		return []byte("SIP/2.0 200 OK\r\nVia: SIP/2.0/UDP nm\r\nFrom: <sip:nm@nm>;tag=root\r\nTo: <sip:nm2@nm2>;tag=home\r\nCall-ID: 50000\r\nCSeq: 42 OPTIONS\r\nServer: " + cfg.Version + "\r\nContent-Length: 0\r\n\r\n")
	case "ntp":
		return ntpResponse()
	case "snmp":
		return snmpResponse(payload, cfg.Version)
	case "memcached":
		return memcachedUDPResponse(payload, cfg.Version)
	}
	return nil
}

func httpResponse(cfg Config, title, server string) []byte {
	if server == "" {
		server = cfg.Version
	}
	body := "<!doctype html><html><head><meta charset=\"utf-8\"><title>" + title + "</title></head><body><h1>" + title + "</h1></body></html>"
	return []byte("HTTP/1.1 200 OK\r\nServer: " + server + "\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\nConnection: close\r\nX-Content-Type-Options: nosniff\r\n\r\n" + body)
}

func classify(profile string, payload []byte, configuredVersion string) (probe, protocol, version string, score int, evidence string) {
	version = configuredVersion
	upper := bytes.ToUpper(payload)
	switch {
	case bytes.HasPrefix(payload, []byte("SSH-")) || profile == "ssh" && len(payload) == 0:
		return "SSHVersion", "SSH", version, 88, "SSH version exchange"
	case profile == "mysql" && len(payload) == 0:
		return "MySQLHandshake", "MySQL", version, 96, "MySQL initial handshake"
	case bytes.HasPrefix(upper, []byte("OPTIONS / RTSP")) || bytes.HasPrefix(upper, []byte("RTSP/")):
		return "RTSPRequest", "RTSP", version, 94, "RTSP request probe"
	case bytes.HasPrefix(upper, []byte("SIP/")) || bytes.HasPrefix(upper, []byte("OPTIONS SIP")):
		return "SIPOptions", "SIP", version, 93, "SIP request probe"
	case bytes.HasPrefix(upper, []byte("GET ")) || bytes.HasPrefix(upper, []byte("OPTIONS ")) || bytes.HasPrefix(upper, []byte("HEAD ")) || bytes.HasPrefix(upper, []byte("POST ")) || bytes.HasPrefix(upper, []byte("PUT ")) || bytes.HasPrefix(upper, []byte("DELETE ")):
		return "GetRequest", "HTTP", version, 94, "HTTP request probe"
	case bytes.HasPrefix(payload, []byte{0x16, 0x03}) || bytes.HasPrefix(payload, []byte{0x80, 0x9e, 0x01, 0x03}):
		return "TLSSessionReq", "TLS", version, 96, "TLS ClientHello probe"
	case bytes.Contains(payload, []byte("\xffSMB")) || bytes.Contains(payload, []byte("SMB2")):
		return "SMBProgNeg", "SMB", version, 97, "SMB negotiate probe"
	case bytes.Contains(upper, []byte("VERSION")) && bytes.Contains(upper, []byte("BIND")):
		return "DNSVersionBindReq", "DNS", version, 95, "version.bind query"
	case isSNMPProbe(payload):
		if bytes.Contains(payload, []byte{0x02, 0x01, 0x03}) {
			return "SNMPv3GetRequest", "SNMP", version, 95, "SNMPv3 GetRequest probe"
		}
		return "SNMPv1public", "SNMP", version, 95, "SNMPv1 public community probe"
	case bytes.HasPrefix(payload, []byte{0x05, 0x00}) || bytes.HasPrefix(payload, []byte{0x03, 0x00, 0x00}):
		return "TerminalServer", "RDP", version, 92, "RDP/X.224 probe"
	case bytes.HasPrefix(payload, []byte("*1\r\n$4\r\nINFO")):
		return "redis-server", "Redis", version, 95, "RESP INFO probe"
	case bytes.HasPrefix(upper, []byte("STATS")):
		return "Memcache", "Memcached", version, 94, "memcached stats probe"
	case profile == "memcached" && len(payload) >= 8 && (bytes.Contains(upper[8:], []byte("STATS")) || bytes.Contains(upper[8:], []byte("VERSION"))):
		return "memcached-udp", "Memcached", version, 94, "memcached UDP stats probe"
	case bytes.HasPrefix(payload, []byte{0x12, 0x01}) || bytes.Contains(payload, []byte("MSSQLServer")):
		return "ms-sql-s", "MS-SQL", version, 95, "TDS probe"
	case profile == "postgresql" && isPostgreSQLProbe(payload):
		return "PostgreSQLStartup", "PostgreSQL", version, 94, "PostgreSQL startup or SSL request probe"
	case bytes.HasPrefix(payload, []byte{0x30}) && bytes.Contains(payload, []byte("objectClass")):
		return "LDAPSearchReq", "LDAP", version, 94, "LDAP search probe"
	case bytes.HasPrefix(payload, []byte{0x10, 0x10}) && bytes.Contains(payload, []byte("MQTT")):
		return "MQTT", "MQTT", version, 95, "MQTT CONNECT probe"
	case profile == "ftp" && (bytes.HasPrefix(upper, []byte("USER ")) || bytes.HasPrefix(upper, []byte("SYST")) || bytes.HasPrefix(upper, []byte("FEAT"))):
		return "FTPProbe", "FTP", version, 91, "FTP command probe"
	case profile == "smtp" && (bytes.HasPrefix(upper, []byte("EHLO ")) || bytes.HasPrefix(upper, []byte("HELO "))):
		return "SMTPProbe", "SMTP", version, 91, "SMTP greeting probe"
	case profile == "imap" && (bytes.Contains(upper, []byte(" CAPABILITY")) || bytes.Contains(upper, []byte(" LOGOUT"))):
		return "IMAPProbe", "IMAP", version, 90, "IMAP command probe"
	case profile == "pop3" && (bytes.HasPrefix(upper, []byte("CAPA")) || bytes.HasPrefix(upper, []byte("USER "))):
		return "POP3Probe", "POP3", version, 90, "POP3 command probe"
	case isNTPProbe(payload):
		return "NTPRequest", "NTP", version, 88, "NTP request probe"
	case len(payload) > 0:
		return "GenericProbe", profileProtocol(profile), version, 54, "connection payload matched configured decoy"
	default:
		return "Banner", profileProtocol(profile), version, 45, "connection reached configured decoy"
	}
}

func isSNMPProbe(payload []byte) bool {
	return bytes.HasPrefix(payload, []byte{0x30, 0x82, 0x00, 0x2f, 0x02, 0x01, 0x00}) ||
		bytes.HasPrefix(payload, []byte{0x30, 0x3a, 0x02, 0x01, 0x03})
}

func isPostgreSQLProbe(payload []byte) bool {
	if len(payload) < 8 || binary.BigEndian.Uint32(payload[:4]) != uint32(len(payload)) {
		return false
	}
	code := binary.BigEndian.Uint32(payload[4:8])
	return code == 196608 || code == 80877103
}

func isNTPProbe(payload []byte) bool {
	if len(payload) < 48 {
		return false
	}
	version := (payload[0] >> 3) & 0x07
	mode := payload[0] & 0x07
	return version >= 3 && version <= 4 && mode >= 1 && mode <= 4
}

func profileProtocol(profile string) string {
	if p, ok := ProfileByCode(profile); ok {
		return p.Name
	}
	return profile
}

func confidence(score int) string {
	switch {
	case score >= 90:
		return "high"
	case score >= 70:
		return "medium"
	default:
		return "low"
	}
}

func readSome(conn net.Conn) []byte {
	buffer := make([]byte, maxReadBytes)
	n, err := conn.Read(buffer)
	if err != nil && n == 0 {
		return nil
	}
	return append([]byte(nil), buffer[:n]...)
}

func remoteIP(conn net.Conn) string {
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return conn.RemoteAddr().String()
	}
	return host
}

func mustAddr(raw string) netip.Addr {
	addr, _ := netip.ParseAddr(raw)
	return addr
}

func isPublic(addr netip.Addr) bool {
	return addr.IsValid() && addr.IsGlobalUnicast() && !addr.IsPrivate() && !addr.IsLoopback() && !addr.IsLinkLocalUnicast()
}

func LocalCIDRs() []string {
	seen := map[string]struct{}{}
	var out []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, raw := range addresses {
			var ip netip.Addr
			switch value := raw.(type) {
			case *net.IPNet:
				ip, _ = netip.ParseAddr(value.IP.String())
			case *net.IPAddr:
				ip, _ = netip.ParseAddr(value.IP.String())
			}
			if !ip.IsValid() {
				continue
			}
			ip = ip.Unmap()
			if !ip.IsPrivate() {
				continue
			}
			bits := 24
			if ip.Is6() {
				bits = 64
			}
			prefix := netip.PrefixFrom(ip, bits).Masked().String()
			if _, ok := seen[prefix]; ok {
				continue
			}
			seen[prefix] = struct{}{}
			out = append(out, prefix)
		}
	}
	sort.Strings(out)
	return out
}

func IsLocalLAN(addr netip.Addr) bool {
	return isLocalLANWithCIDRs(addr, LocalCIDRs())
}

func isLocalLANWithCIDRs(addr netip.Addr, cidrs []string) bool {
	if !addr.IsValid() {
		return false
	}
	for _, raw := range cidrs {
		prefix, err := netip.ParsePrefix(raw)
		if err == nil && prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return addr.IsLoopback()
}

func randomID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(raw[:])
}

func selfSignedCertificate(name string) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(365 * 24 * time.Hour), DNSNames: []string{name, "localhost"}, KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return tls.X509KeyPair(certPEM, keyPEM)
}

func smbResponse() []byte {
	// Minimal NetBIOS session response containing an SMB negotiate signature.
	return []byte{0x00, 0x00, 0x00, 0x48, 0xff, 0x53, 0x4d, 0x42, 0x72, 0x00, 0x00, 0x00, 0x00, 0x98, 0x53, 0xc8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func smbResponseV2() []byte {
	// NetBIOS session response with stable SMB1 fields used by Nmap's
	// netbios-ssn and microsoft-ds match lines.
	response := []byte{0, 0, 0, 0, 0xff, 'S', 'M', 'B', 'r', 0, 0, 0, 0, 0x88, 0x01, 0x40, 0, 0, 0, '@', 0x06, 0, 0, 1, 0, 0x11, 0x06, 0}
	response = append(response, make([]byte, 42)...)
	response = append(response, []byte("WORKGROUP")...)
	response = append(response, 0, 0)
	response = append(response, []byte("NAS")...)
	response = append(response, 0, 0)
	length := len(response) - 4
	response[2] = byte(length >> 8)
	response[3] = byte(length)
	return response
}

func mysqlHandshake(version string) []byte {
	payload := []byte{0x0a}
	payload = append(payload, version...)
	payload = append(payload, 0x00)
	payload = append(payload, 0x01, 0x00, 0x00, 0x00)
	payload = append(payload, []byte("homewaf1")...)
	payload = append(payload, 0x00)
	payload = append(payload, 0xff, 0xff, 0x21, 0x02, 0x00, 0x80, 0x15, 0x15)
	payload = append(payload, make([]byte, 10)...)
	payload = append(payload, []byte("camouflage12")...)
	payload = append(payload, 0x00)
	payload = append(payload, []byte("mysql_native_password")...)
	payload = append(payload, 0x00)
	header := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), 0x00}
	return append(header, payload...)
}

func redisInfoResponse(version string) []byte {
	body := "# Server\r\nredis_version:" + version + "\r\nredis_mode:standalone\r\n\r\n"
	return []byte("$" + strconv.Itoa(len(body)) + "\r\n" + body + "\r\n")
}

func memcachedStatsResponse(version string) []byte {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	return []byte("STAT pid 1\r\nSTAT uptime 1\r\nSTAT time " + now + "\r\nSTAT version " + version + "\r\nEND\r\n")
}

func memcachedUDPResponse(query []byte, version string) []byte {
	if len(query) < 8 {
		return nil
	}
	response := append([]byte(nil), query[:8]...)
	return append(response, memcachedStatsResponse(version)...)
}

func postgresErrorResponse() []byte {
	body := []byte("SFATAL\x00C08P01\x00Minvalid length of startup packet\x00Fpostmaster.c\x00L1\x00RProcessStartupPacket\x00\x00")
	length := len(body) + 4
	response := []byte{'E', byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
	return append(response, body...)
}

func dnsResponse(query []byte, version string) []byte {
	if len(query) < 12 {
		return nil
	}
	end := 12
	for end < len(query) && query[end] != 0 {
		length := int(query[end])
		end += length + 1
	}
	if end+5 > len(query) {
		return nil
	}
	questionEnd := end + 5
	response := make([]byte, 0, questionEnd+32+len(version))
	response = append(response, query[0], query[1], 0x81, 0x80, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00)
	response = append(response, query[12:questionEnd]...)
	rdLength := len(version) + 1
	response = append(response, 0xc0, 0x0c, 0x00, 0x10, 0x00, 0x03, 0x00, 0x00, 0x00, 0x3c, byte(rdLength>>8), byte(rdLength))
	response = append(response, byte(len(version)))
	response = append(response, []byte(version)...)
	return response
}

func ntpResponse() []byte {
	response := make([]byte, 48)
	response[0] = 0x24
	response[1] = 1
	response[2] = 6
	response[3] = 0xec
	return response
}

func snmpResponse(query []byte, version string) []byte {
	if len(query) < 10 {
		return nil
	}
	if bytes.Contains(query, []byte{0x02, 0x01, 0x00, 0x04}) {
		return snmpV1Response(version)
	}
	return snmpV3ReportResponse()
}

func berLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	if length <= 0xff {
		return []byte{0x81, byte(length)}
	}
	return []byte{0x82, byte(length >> 8), byte(length)}
}

func ber(tag byte, value []byte) []byte {
	result := []byte{tag}
	result = append(result, berLength(len(value))...)
	return append(result, value...)
}

func snmpV1Response(version string) []byte {
	requestID := []byte{0x02, 0x04, 0x4c, 0x33, 0xa7, 0x56}
	errorStatus := []byte{0x02, 0x01, 0x00}
	errorIndex := []byte{0x02, 0x01, 0x00}
	oid := ber(0x06, []byte{0x2b, 0x06, 0x01, 0x02, 0x01, 0x01, 0x05, 0x00})
	value := ber(0x04, []byte(version))
	varBind := ber(0x30, append(oid, value...))
	varBinds := ber(0x30, varBind)
	pdu := ber(0xa2, append(append(requestID, errorStatus...), append(errorIndex, varBinds...)...))
	body := append([]byte{0x02, 0x01, 0x00}, ber(0x04, []byte("public"))...)
	body = append(body, pdu...)
	return ber(0x30, body)
}

func snmpV3ReportResponse() []byte {
	engineID := []byte{0x80, 0x00, 0x00, 0x01, 0x01}
	usmFields := append(ber(0x04, engineID), ber(0x02, []byte{0x01})...)
	usmFields = append(usmFields, ber(0x02, []byte{0x01})...)
	usmFields = append(usmFields, ber(0x04, nil)...)
	usmFields = append(usmFields, ber(0x04, nil)...)
	usmFields = append(usmFields, ber(0x04, nil)...)
	usm := ber(0x30, usmFields)
	headerFields := append(ber(0x02, []byte{0x4a, 0x69}), ber(0x02, []byte{0x00, 0xff, 0xe3})...)
	headerFields = append(headerFields, ber(0x04, []byte{0x00})...)
	headerFields = append(headerFields, ber(0x02, []byte{0x03})...)
	header := ber(0x30, headerFields)
	varBindValue := ber(0x41, []byte{0x01})
	varBindOID := ber(0x06, []byte{0x2b, 0x06, 0x01, 0x06, 0x03, 0x0f, 0x01, 0x01, 0x04, 0x00})
	varBind := ber(0x30, append(varBindOID, varBindValue...))
	varBinds := ber(0x30, varBind)
	reportFields := append(ber(0x02, []byte{0x37, 0xf0}), ber(0x02, []byte{0x00})...)
	reportFields = append(reportFields, ber(0x02, []byte{0x00})...)
	reportPDU := ber(0xa8, append(reportFields, varBinds...))
	scopedFields := append(ber(0x04, engineID), ber(0x04, nil)...)
	scopedPDU := ber(0x30, append(scopedFields, reportPDU...))
	messageFields := []byte{0x02, 0x01, 0x03}
	messageFields = append(messageFields, header...)
	messageFields = append(messageFields, ber(0x04, usm)...)
	messageFields = append(messageFields, scopedPDU...)
	return ber(0x30, messageFields)
}
