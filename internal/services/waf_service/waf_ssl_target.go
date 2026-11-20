package waf_service

import (
	"fmt"
	"strings"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/protection/wafpipeline"
)

// SSLCertificateTarget is shared by selection and server-side order validation.
// It deliberately contains neither service policies nor credential/certificate values.
type SSLCertificateTarget struct {
	Code        string   `json:"value"`
	Label       string   `json:"label"`
	Source      string   `json:"source"`
	Port        int      `json:"port"`
	Protocol    string   `json:"protocol"`
	Domains     []string `json:"domains"`
	HTTP01Ready bool     `json:"http01_ready"`
}

func certificateDomains(names ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, entry := range names {
		for _, name := range strings.FieldsFunc(entry, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
			if name != "" && name != "*" && !seen[name] {
				out = append(out, name)
				seen[name] = true
			}
		}
	}
	return out
}

func pipelineCertificateCode(config wafpipeline.PortConfig) string {
	if config.HostCode != "" {
		return config.HostCode
	}
	return fmt.Sprintf("pipeline-%d", config.Port)
}

func GetSSLCertificateTargets() ([]SSLCertificateTarget, error) {
	if global.GWAF_LOCAL_DB == nil {
		return nil, fmt.Errorf("配置数据库不可用")
	}
	targets := []SSLCertificateTarget{}

	configs, err := WafPipelineServiceApp.List()
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		code := pipelineCertificateCode(config)
		names := append([]string{}, config.HostNames...)
		for _, route := range config.Routes {
			names = append(names, route.Hosts...)
		}
		name := strings.TrimSpace(config.Name)
		if name == "" {
			name = "未命名服务"
		}
		targets = append(targets, SSLCertificateTarget{Code: code, Label: fmt.Sprintf("%s · %s :%d", name, strings.ToUpper(config.Protocol), config.Port), Source: "pipeline", Port: int(config.Port), Protocol: config.Protocol, Domains: certificateDomains(names...), HTTP01Ready: config.Port == 80 && config.Protocol == "http" && config.ListenEnabled && config.Policy.Access.Enabled})
	}
	return targets, nil
}

func GetSSLCertificateTarget(code string) (SSLCertificateTarget, error) {
	targets, err := GetSSLCertificateTargets()
	if err != nil {
		return SSLCertificateTarget{}, err
	}
	for _, target := range targets {
		if code != "" && target.Code == code {
			return target, nil
		}
	}
	return SSLCertificateTarget{}, fmt.Errorf("关联服务不存在或已删除，请刷新服务列表")
}

func ValidateSSLCertificateTarget(code, method, domains string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		if method == "dns01" {
			return nil
		}
		return fmt.Errorf("独立申请请使用 DNS 验证；HTTP 文件验证需要提供挑战响应的 HTTP 80 端口服务")
	}
	target, err := GetSSLCertificateTarget(code)
	if err != nil {
		return err
	}
	if method != "http01" {
		return nil
	}
	if !target.HTTP01Ready {
		return fmt.Errorf("HTTP 文件验证需要可访问的 HTTP 80 端口；家庭网络请使用 DNS 验证，无需入站 80/443")
	}
	if target.Source == "pipeline" {
		for _, domain := range strings.Split(domains, ",") {
			found := false
			for _, configured := range target.Domains {
				if configured == domain {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("HTTP 验证域名 %s 未配置在该服务上，请先设置服务域名或改用 DNS 验证", domain)
			}
		}
	}
	return nil
}
