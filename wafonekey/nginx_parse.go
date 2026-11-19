package wafonekey

import (
	"HomeWaf/global"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// 解析相关安全上限
const (
	maxNginxConfigLen = 1 << 20 // 粘贴文本上限 1MB
	maxScanFileCount  = 500     // 扫描目录最多读取的 .conf 文件数
	maxScanFileSize   = 2 << 20 // 单个 .conf 文件读取上限 2MB
	defaultNginxDir   = "/www/server/panel/vhost/nginx"
)

// NginxHostCandidate 从一个 nginx server{} 块解析出的待添加主机候选
type NginxHostCandidate struct {
	Domains    []string `json:"domains"`     // server_name 全部域名（已过滤 _ / 空 / 非法FQDN）
	Port       int      `json:"port"`        // listen 端口 → 拟作 remote_port
	Ssl        bool     `json:"ssl"`         // listen ssl / 存在 ssl_certificate
	Root       string   `json:"root"`        // nginx root，仅展示/备注用，不参与代理
	SourceFile string   `json:"source_file"` // 扫描模式下来源 .conf 文件名（粘贴模式为空）
}

var (
	reServerBlock = regexp.MustCompile(`(?:^|\s)server\s*\{`)
	reListen      = regexp.MustCompile(`(?mi)^\s*listen\s+([^;]+);`)
	reServerName  = regexp.MustCompile(`(?mi)^\s*server_name\s+([^;]+);`)
	reRoot        = regexp.MustCompile(`(?mi)^\s*root\s+([^;]+);`)
	reSslCert     = regexp.MustCompile(`(?mi)^\s*ssl_certificate\s+[^;]+;`)
	reFQDN        = regexp.MustCompile(`^(?:\*\.)?(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
)

// ParseNginxText 解析粘贴的 nginx 配置文本
func ParseNginxText(content string) ([]NginxHostCandidate, error) {
	if len(content) > maxNginxConfigLen {
		return nil, errors.New("配置内容过长")
	}
	return parseNginxContent(content, ""), nil
}

// ScanNginxDir 安全地扫描目录下 .conf 并解析
func ScanNginxDir(dirPath string) ([]NginxHostCandidate, error) {
	root, err := openNginxDir(dirPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	candidates := make([]NginxHostCandidate, 0)
	readCount := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".conf") {
			continue
		}
		if readCount >= maxScanFileCount {
			break
		}
		readCount++
		data, _, err := readNginxConfig(root, entry.Name())
		if err != nil {
			continue
		}
		candidates = append(candidates, parseNginxContent(string(data), entry.Name())...)
	}
	return candidates, nil
}

// Only the server's local configuration can designate a trusted Nginx root.
func openNginxDir(requested string) (*os.Root, error) {
	trusted := global.CurrentSettings().NginxConfigRoot
	if trusted == "" {
		trusted = defaultNginxDir
	} else if !filepath.IsAbs(trusted) {
		return nil, errors.New("security.nginx_config_root 必须是绝对目录")
	}
	base, err := filepath.Abs(trusted)
	if err != nil {
		return nil, err
	}
	if requested == "" {
		requested = base
	}
	requested, err = filepath.Abs(requested)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, requested)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, errors.New("目录不在允许范围内；自定义宝塔安装路径请在 config.yml 配置 security.nginx_config_root")
	}
	root, err := os.OpenRoot(base)
	if err != nil || rel == "." {
		return root, err
	}
	defer root.Close()
	return root.OpenRoot(rel)
}

func readNginxConfig(root *os.Root, name string) ([]byte, os.FileMode, error) {
	if !filepath.IsLocal(name) || name != filepath.Base(name) || !strings.EqualFold(filepath.Ext(name), ".conf") {
		return nil, 0, errors.New("无效的 Nginx 配置文件名")
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxScanFileSize {
		return nil, 0, errors.New("配置文件必须是范围内的普通文件，大小不超过 2 MiB")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, 0, errors.New("配置文件已变化，请重试")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxScanFileSize+1))
	if len(data) > maxScanFileSize {
		return nil, 0, fmt.Errorf("配置文件超过 %d 字节", maxScanFileSize)
	}
	return data, opened.Mode().Perm(), err
}

// parseNginxContent 从配置文本中提取每个 server{} 块的候选
func parseNginxContent(content, sourceFile string) []NginxHostCandidate {
	candidates := make([]NginxHostCandidate, 0)
	for _, block := range splitServerBlocks(content) {
		candidate, ok := parseServerBlock(block)
		if !ok {
			continue
		}
		candidate.SourceFile = sourceFile
		candidates = append(candidates, candidate)
	}
	return candidates
}

// splitServerBlocks 用花括号配对切分出所有 server{} 块的内部内容
func splitServerBlocks(content string) []string {
	blocks := make([]string, 0)
	locs := reServerBlock.FindAllStringIndex(content, -1)
	for _, loc := range locs {
		open := strings.IndexByte(content[loc[0]:loc[1]], '{')
		if open < 0 {
			continue
		}
		open += loc[0]
		depth := 0
		end := -1
		for i := open; i < len(content); i++ {
			switch content[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
				}
			}
			if end >= 0 {
				break
			}
		}
		if end > open {
			blocks = append(blocks, content[open+1:end])
		}
	}
	return blocks
}

// parseServerBlock 解析单个 server 块内容
func parseServerBlock(block string) (NginxHostCandidate, bool) {
	var candidate NginxHostCandidate

	// listen：取第一条含合法端口的；ssl 为任意一条含 ssl 关键字或存在 ssl_certificate
	for _, m := range reListen.FindAllStringSubmatch(block, -1) {
		port, ssl := parseListen(m[1])
		if candidate.Port == 0 && port > 0 && port <= 65535 {
			candidate.Port = port
		}
		if ssl {
			candidate.Ssl = true
		}
	}
	if reSslCert.MatchString(block) {
		candidate.Ssl = true
	}
	if candidate.Port == 0 {
		return candidate, false
	}

	// server_name：拆分并过滤非法域名
	if m := reServerName.FindStringSubmatch(block); m != nil {
		for _, name := range strings.Fields(m[1]) {
			name = strings.TrimSpace(strings.ToLower(name))
			if name == "" || name == "_" || name == "localhost" {
				continue
			}
			if reFQDN.MatchString(name) {
				candidate.Domains = append(candidate.Domains, name)
			}
		}
	}
	if len(candidate.Domains) == 0 {
		return candidate, false
	}

	// root：仅展示
	if m := reRoot.FindStringSubmatch(block); m != nil {
		candidate.Root = strings.TrimSpace(m[1])
	}

	return candidate, true
}

// parseListen 解析 listen 指令值，返回端口与是否 ssl
// 兼容: "81" / "127.0.0.1:81" / "[::]:81" / "443 ssl http2" / "0.0.0.0:443 ssl"
func parseListen(value string) (int, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false
	}
	ssl := false
	for _, f := range fields[1:] {
		if strings.EqualFold(f, "ssl") {
			ssl = true
		}
	}
	return extractPort(fields[0]), ssl
}

// extractPort 从 listen 的地址部分取端口
func extractPort(addr string) int {
	// IPv6: [::]:81
	if i := strings.LastIndex(addr, "]:"); i >= 0 {
		if n, err := strconv.Atoi(addr[i+2:]); err == nil {
			return n
		}
		return 0
	}
	// host:port
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		if n, err := strconv.Atoi(addr[i+1:]); err == nil {
			return n
		}
		return 0
	}
	// 纯端口
	if n, err := strconv.Atoi(addr); err == nil {
		return n
	}
	return 0
}
