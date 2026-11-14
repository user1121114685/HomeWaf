package waf_service

import (
	"fmt"
	"net"
	"strings"

	"HomeWaf/global"
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"

	"golang.org/x/net/idna"
)

// LoadDNSCredentials fails closed on database/decryption errors.
func LoadDNSCredentials(provider, group string) (map[string]string, error) {
	if err := dnsproviders.ValidateProvider(provider); err != nil {
		return nil, err
	}
	if strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("请选择 DNS 密钥分组")
	}
	var entries []model.PrivateInfo
	if err := global.GWAF_LOCAL_DB.Where("private_group_name = ? AND private_group_belong_cloud = ?", group, provider).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("读取 DNS 密钥分组失败，请检查数据库和实例加密密钥")
	}
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		if _, exists := values[entry.PrivateKey]; exists {
			return nil, fmt.Errorf("DNS 密钥分组含重复配置项：%s", entry.PrivateKey)
		}
		values[entry.PrivateKey] = entry.PrivateValue
	}
	if err := dnsproviders.ValidateValues(provider, values); err != nil {
		return nil, err
	}
	return values, nil
}

// ValidateSSLOrderChallenge runs before both first issuance and renewal enqueue.
func ValidateSSLOrderChallenge(method, provider, group, domains string, skip int64) (string, error) {
	if method != "http01" && method != "dns01" {
		return "", fmt.Errorf("请选择 DNS 验证或 HTTP 文件验证")
	}
	if skip != 0 && skip != 1 {
		return "", fmt.Errorf("DNS 传播检查选项无效")
	}
	var normalized []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(domains, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '，' }) {
		domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(part), "."))
		wildcard := strings.HasPrefix(domain, "*.")
		base := strings.TrimPrefix(domain, "*.")
		if ip := net.ParseIP(base); ip != nil {
			if method != "http01" || wildcard {
				return "", fmt.Errorf("IP 证书需要公网 HTTP 80 端口；家庭网络请使用域名和 DNS 验证")
			}
			domain = ip.String()
		} else {
			ascii, err := idna.Lookup.ToASCII(base)
			if err != nil || !dnsproviders.ValidDomain(ascii) {
				return "", fmt.Errorf("申请域名格式无效，请填写域名（不含协议、路径或端口）")
			}
			if wildcard && method != "dns01" {
				return "", fmt.Errorf("泛域名证书必须使用 DNS 验证，无需开放公网 80/443 端口")
			}
			domain = ascii
			if wildcard {
				domain = "*." + ascii
			}
		}
		if !seen[domain] {
			normalized = append(normalized, domain)
			seen[domain] = true
		}
	}
	if len(normalized) == 0 {
		return "", fmt.Errorf("请填写申请域名")
	}
	if len(normalized) > 1 {
		for _, domain := range normalized {
			if net.ParseIP(domain) != nil {
				return "", fmt.Errorf("IP 证书请单独申请，不能与其他域名或 IP 混合")
			}
		}
	}
	if method == "dns01" {
		if _, err := LoadDNSCredentials(provider, group); err != nil {
			return "", err
		}
	}
	return strings.Join(normalized, ","), nil
}
