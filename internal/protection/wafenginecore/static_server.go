// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/model"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

// 敏感文件和路径黑名单
var (
	// 路径穿越攻击模式
	pathTraversalPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.\.[\\/]`),
		regexp.MustCompile(`[\\/]\.\.[\\/]`),
		regexp.MustCompile(`%2e%2e[\\/]`),
		regexp.MustCompile(`%2e%2e%2f`),
		regexp.MustCompile(`%2e%2e%5c`),
		regexp.MustCompile(`\.\.[\\/%]`),
		regexp.MustCompile(`%252e%252e`),
		regexp.MustCompile(`%c0%ae%c0%ae`),
		regexp.MustCompile(`%uff0e%uff0e`),
		regexp.MustCompile(`[\\/]{2,}`),
		regexp.MustCompile(`\\{2,}`),
	}
)

// splitConfigString 分割配置字符串为数组，去除空白项
func splitConfigString(configStr string) []string {
	if configStr == "" {
		return []string{}
	}

	parts := strings.Split(configStr, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// getDefaultSensitivePathsString 获取默认敏感路径配置字符串
func getDefaultSensitivePathsString() string {
	return "/etc/passwd,/etc/shadow,/etc/group,/etc/gshadow,/etc/hosts,/etc/hostname,/etc/resolv.conf,/etc/ssh/,/var/log/,/.ssh/,/.bash_history,/.profile,/.bashrc,/etc/crontab,/var/spool/cron/,/etc/apache2/,/etc/nginx/,/etc/httpd/,/var/www/,/usr/share/,/var/tmp/,/var/run/,c:\\windows\\,c:\\program files\\,c:\\program files (x86)\\,c:\\users\\,c:\\documents and settings\\,c:\\windows\\system32\\,c:\\windows\\syswow64\\,c:\\boot.ini,c:\\autoexec.bat,c:\\config.sys,\\windows\\,\\program files\\,\\program files (x86)\\,\\users\\,\\documents and settings\\,\\windows\\system32\\,\\windows\\syswow64\\,boot.ini,autoexec.bat,config.sys,ntuser.dat,pagefile.sys,hiberfil.sys,swapfile.sys"
}

// getDefaultSensitiveExtensionsString 获取默认敏感文件扩展名字符串
func getDefaultSensitiveExtensionsString() string {
	return ".key,.pem,.crt,.p12,.pfx,.jks,.bak,.backup,.old,.orig,.save,.sql,.db,.sqlite,.mdb,.env,.htaccess,.htpasswd,.git,.svn,.hg,.bzr,.DS_Store,Thumbs.db,desktop.ini,.tmp,.temp,.lock,.pid"
}

// getDefaultAllowedExtensionsString 获取默认允许的文件扩展名字符串
func getDefaultAllowedExtensionsString() string {
	return ".html,.htm,.css,.js,.mjs,.cjs,.json,.map,.png,.jpg,.jpeg,.gif,.svg,.ico,.webp,.avif,.pdf,.txt,.md,.xml,.woff,.woff2,.ttf,.eot,.otf,.mp4,.webm,.ogg,.mp3,.wav,.zip,.tar,.gz,.rar"
}

// getDefaultSensitivePatternsString 获取默认敏感文件名模式字符串
func getDefaultSensitivePatternsString() string {
	return `(?i)\.git(/|\\),(?i)\.svn(/|\\),(?i)\.env,(?i)database\.(php|xml|json|yaml|yml),(?i)(backup|dump|export)\.(sql|db|tar|zip|gz),(?i)(id_rsa|id_dsa|id_ecdsa|id_ed25519),(?i)\.ssh(/|\\).*,(?i)(access|error|debug)\.log,(?i)web\.config,(?i)phpinfo\.php`
}

// getSensitivePathsFromConfig 从配置获取敏感路径数组
func getSensitivePathsFromConfig(config model.StaticSiteConfig) []string {
	if config.SensitivePaths != "" {
		return splitConfigString(config.SensitivePaths)
	}
	return splitConfigString(getDefaultSensitivePathsString())
}

// getSensitiveExtensionsFromConfig 从配置获取敏感扩展名数组
func getSensitiveExtensionsFromConfig(config model.StaticSiteConfig) []string {
	if config.SensitiveExtensions != "" {
		return splitConfigString(config.SensitiveExtensions)
	}
	return splitConfigString(getDefaultSensitiveExtensionsString())
}

// getAllowedExtensionsFromConfig 从配置获取允许扩展名数组
func getAllowedExtensionsFromConfig(config model.StaticSiteConfig) []string {
	if config.AllowedExtensions != "" {
		return splitConfigString(config.AllowedExtensions)
	}
	return splitConfigString(getDefaultAllowedExtensionsString())
}

// getSensitivePatternsFromConfig 从配置获取敏感模式数组
func getSensitivePatternsFromConfig(config model.StaticSiteConfig) []string {
	if config.SensitivePatterns != "" {
		return splitConfigString(config.SensitivePatterns)
	}
	return splitConfigString(getDefaultSensitivePatternsString())
}

// containsPathTraversal 检查路径是否包含路径穿越攻击模式（增强版）
func containsPathTraversal(filePath string) bool {
	// 转换为小写进行检查
	lowerPath := strings.ToLower(filePath)

	// 使用正则表达式检查
	for _, pattern := range pathTraversalPatterns {
		if pattern.MatchString(lowerPath) {
			return true
		}
	}

	// 额外的字符串检查
	suspiciousPatterns := []string{
		"../", "..\\", ".../", "...\\",
		"%2e%2e%2f", "%2e%2e/", "..%2f",
		"%2e%2e%5c", "..%5c", "\\\\", "//",
		"%252e", "%c0%ae", "%uff0e",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// containsEncodedThreats 检查编码攻击
func containsEncodedThreats(filePath string) bool {
	lowerPath := strings.ToLower(filePath)

	// 检查各种编码绕过
	encodedThreats := []string{
		"%252e", "%c0%ae", "%uff0e", // 双重编码
		"%2e%2e%2f", "%2e%2e%5c", // URL编码
		"..%c0%af", "..%ef%bc%8f", // UTF-8编码
		"..%c1%9c", "..%c1%pc", // 畸形UTF-8
	}

	for _, threat := range encodedThreats {
		if strings.Contains(lowerPath, threat) {
			return true
		}
	}

	return false
}

// isSensitivePath 检查是否为敏感路径
func isSensitivePath(filePath string, config model.StaticSiteConfig) bool {
	lowerPath := strings.ToLower(filePath)

	// 从配置获取敏感路径数组
	sensitivePaths := getSensitivePathsFromConfig(config)

	// 检查敏感路径
	for _, sensitivePath := range sensitivePaths {
		if strings.Contains(lowerPath, strings.ToLower(sensitivePath)) {
			return true
		}
	}

	return false
}

// isSensitiveFile 检查是否为敏感文件
func isSensitiveFile(filePath string, config model.StaticSiteConfig) bool {
	lowerPath := strings.ToLower(filePath)
	fileName := filepath.Base(lowerPath)

	// 从配置获取敏感扩展名数组
	sensitiveExtensions := getSensitiveExtensionsFromConfig(config)

	// 检查敏感文件扩展名
	for _, ext := range sensitiveExtensions {
		if strings.HasSuffix(fileName, ext) {
			return true
		}
	}

	// 从配置获取敏感模式数组
	sensitivePatterns := getSensitivePatternsFromConfig(config)

	// 检查敏感文件名模式
	for _, patternStr := range sensitivePatterns {
		if pattern, err := regexp.Compile(patternStr); err == nil {
			if pattern.MatchString(lowerPath) {
				return true
			}
		}
	}

	// 检查隐藏文件（以.开头，但不是../或./）
	if strings.HasPrefix(fileName, ".") && fileName != "." && fileName != ".." {
		return true
	}

	return false
}

// isAllowedFileType 检查是否为允许的文件类型
func isAllowedFileType(filePath string, config model.StaticSiteConfig) bool {
	// 从配置获取允许扩展名数组
	allowedExtensions := getAllowedExtensionsFromConfig(config)

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false // 没有扩展名的文件不允许
	}

	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			return true
		}
	}

	return false
}

// defaultSecurityHeaders 内置默认安全响应头（挪到前端配置）
var defaultSecurityHeaders = []struct{ name, value string }{}

// setSecurityHeaders 设置安全响应头。
// 若 config.SecurityHeaders 为空，则全部使用默认值；
// 否则以用户配置为准，某项 HeaderValue 为空时该项回退到默认值。
func setSecurityHeaders(w http.ResponseWriter, config model.StaticSiteConfig) {
	if len(config.SecurityHeaders) == 0 {
		// 未配置，直接写入全部默认头
		for _, h := range defaultSecurityHeaders {
			w.Header().Set(h.name, h.value)
		}
		return
	}

	// 把用户配置转为 map，便于 O(1) 查找
	userHeaders := make(map[string]string, len(config.SecurityHeaders))
	for _, h := range config.SecurityHeaders {
		if h.HeaderName != "" {
			userHeaders[strings.ToLower(h.HeaderName)] = h.HeaderValue
		}
	}

	// 遍历默认头：用户配置了就用用户的，值为空则回退默认
	writtenKeys := make(map[string]bool)
	for _, def := range defaultSecurityHeaders {
		key := strings.ToLower(def.name)
		if val, ok := userHeaders[key]; ok {
			if val == "" {
				w.Header().Set(def.name, def.value) // 值为空，回退默认
			} else {
				w.Header().Set(def.name, val) // 使用用户自定义值
			}
			writtenKeys[key] = true
		} else {
			w.Header().Set(def.name, def.value) // 用户未配置该项，使用默认
		}
	}

	// 写入用户额外配置的非默认头（不在 defaultSecurityHeaders 列表中的）
	for _, h := range config.SecurityHeaders {
		if h.HeaderName == "" || h.HeaderValue == "" {
			continue
		}
		if !writtenKeys[strings.ToLower(h.HeaderName)] {
			w.Header().Set(h.HeaderName, h.HeaderValue)
		}
	}
}
