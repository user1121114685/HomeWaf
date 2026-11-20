// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/model"
	"net/http"
	"regexp"
	"strings"
)

// The native port proxy shares the existing variable and response-rule
// semantics using copied settings, never a live host/global database lookup.
func PipelineRequestHeaders(headers []model.CustomHeaderItem, r *http.Request, ip string) map[string]string {
	out := make(map[string]string, len(headers))
	for _, header := range headers {
		out[header.HeaderName] = parseHeaderValue(header.HeaderValue, r, ip)
	}
	return out
}
func PipelineResponseHeaders(rules []model.CustomResponseHeaderRule, r *http.Request, ip string) map[string]string {
	return applyCustomResponseHeaders(rules, r, ip)
}

// matchResponseHeaderRule 判断请求路径是否匹配某条响应头规则
// MatchType: global(全部匹配) / prefix(路径前缀) / suffix(文件后缀，分号分隔) / exact(精确) / regex(正则)
func matchResponseHeaderRule(rule model.CustomResponseHeaderRule, urlPath string) bool {
	switch rule.MatchType {
	case "global", "":
		return true
	case "prefix":
		return strings.HasPrefix(urlPath, rule.MatchValue)
	case "exact":
		return urlPath == rule.MatchValue
	case "suffix":
		// 支持分号分隔多个后缀，如 ".mp4;.mp3;.webm"
		ext := ""
		if idx := strings.LastIndex(urlPath, "."); idx != -1 {
			ext = strings.ToLower(urlPath[idx:])
		}
		if ext == "" {
			return false
		}
		for _, s := range strings.Split(rule.MatchValue, ";") {
			s = strings.TrimSpace(strings.ToLower(s))
			if s != "" && ext == s {
				return true
			}
		}
		return false
	case "regex":
		matched, err := regexp.MatchString(rule.MatchValue, urlPath)
		if err != nil {
			return false
		}
		return matched
	default:
		return false
	}
}

// applyCustomResponseHeaders 根据规则列表和请求路径，计算最终需要设置的响应头 map
// 合并逻辑：先汇总所有 global 规则的 headers，再找第一条匹配的 location 规则按 MergeMode 合并。
func applyCustomResponseHeaders(
	rules []model.CustomResponseHeaderRule,
	r *http.Request,
	clientIP string,
) map[string]string {
	result := map[string]string{}

	// 1. 收集全局规则 headers（可能有多条 global 规则）
	for _, rule := range rules {
		if rule.MatchType == "global" || rule.MatchType == "" {
			for _, h := range rule.Headers {
				if h.HeaderName == "" {
					continue
				}
				result[h.HeaderName] = parseHeaderValue(h.HeaderValue, r, clientIP)
			}
		}
	}

	// 2. 按顺序找第一条匹配的非 global 规则
	for _, rule := range rules {
		if rule.MatchType == "global" || rule.MatchType == "" {
			continue
		}
		if !matchResponseHeaderRule(rule, r.URL.Path) {
			continue
		}
		// 找到匹配规则
		locationHeaders := map[string]string{}
		for _, h := range rule.Headers {
			if h.HeaderName == "" {
				continue
			}
			locationHeaders[h.HeaderName] = parseHeaderValue(h.HeaderValue, r, clientIP)
		}
		if rule.MergeMode == "override" {
			// override：丢弃全局，仅用本规则
			return locationHeaders
		}
		// merge（默认）：全局 + 本规则，同名以本规则为准
		for k, v := range locationHeaders {
			result[k] = v
		}
		return result
	}

	return result
}

// parseHeaderValue 解析头信息值中的变量
// 支持的变量：
// ${header:HeaderName} - 获取原始请求头
// ${client_ip} - 获取客户端IP
// ${remote_addr} - 获取远程地址
// ${host} - 获取主机名
// ${scheme} - 获取协议（http/https）
// ${request_uri} - 获取请求URI
// ${request_method} - 获取请求方法
func parseHeaderValue(value string, r *http.Request, clientIP string) string {
	// 正则表达式匹配 ${变量名} 或 ${header:头名称}
	varPattern := regexp.MustCompile(`\$\{([^}]+)\}`)

	result := varPattern.ReplaceAllStringFunc(value, func(match string) string {
		// 提取变量名（去掉 ${ 和 }）
		varName := match[2 : len(match)-1]

		// 检查是否是header变量
		if strings.HasPrefix(varName, "header:") {
			headerName := strings.TrimPrefix(varName, "header:")
			headerValue := r.Header.Get(headerName)
			if headerValue == "" {
				return "" // 如果header不存在，返回空字符串
			}
			return headerValue
		}

		// 处理其他内置变量
		switch varName {
		case "client_ip":
			return clientIP
		case "remote_addr":
			return r.RemoteAddr
		case "host":
			return r.Host
		case "scheme":
			if r.TLS != nil {
				return "https"
			}
			return "http"
		case "request_uri":
			return r.RequestURI
		case "request_method":
			return r.Method
		default:
			// 未知变量，保持原样
			return match
		}
	})

	return result
}
