package wafpipeline

import (
	"net/url"
	"strconv"
	"strings"
)

// GeneratedAssessmentSamples covers request locations and encodings, adapting
// paths to the selected service. Samples never contain an imported service host.
func GeneratedAssessmentSamples(config PortConfig) []AssessmentSample {
	paths := []string{"/"}
	seen := map[string]bool{"/": true}
	addPath := func(path string) {
		if path != "" && strings.HasPrefix(path, "/") && !strings.ContainsAny(path, "*?\\") && !seen[path] && len(paths) < 5 {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, route := range config.Routes {
		addPath(route.PathPrefix)
	}
	for _, rule := range config.Policy.RuleSet.Rules {
		for _, path := range rule.Scope.Paths {
			addPath(path)
		}
	}
	out := []AssessmentSample{}
	for i, path := range paths {
		for _, method := range []string{"GET", "HEAD"} {
			out = append(out, AssessmentSample{ID: "normal-" + strconv.Itoa(i) + "-" + method, Category: "normal", Kind: "benign", Method: method, Path: path})
		}
	}
	out = append(out, AssessmentSample{ID: "normal-query", Category: "normal", Kind: "benign", Method: "GET", Path: paths[0] + "?q=" + url.QueryEscape("家庭相册 2026")})
	payloads := []struct{ category, query, body string }{
		{"sqli", "1' OR '1'='1'--", `{"id":"1' OR '1'='1'--"}`},
		{"xss", "<script>alert(1)</script>", `{"name":"<svg onload=alert(1)>"}`},
		{"lfi", "../../../../etc/passwd", `{"file":"../../../../etc/passwd"}`},
		{"rce", ";cat /etc/passwd;id", `{"cmd":";cat /etc/passwd;id"}`},
		{"ssti", "{{7*7}}", `{"template":"{{7*7}}"}`},
		{"ssrf", "http://169.254.169.254/latest/meta-data/", `{"url":"http://169.254.169.254/latest/meta-data/"}`},
	}
	for _, payload := range payloads {
		path := paths[0]
		for _, rule := range config.Policy.RuleSet.Rules {
			if rule.Category == payload.category && len(rule.Scope.Paths) > 0 {
				candidate := rule.Scope.Paths[0]
				if strings.HasPrefix(candidate, "/") && !strings.ContainsAny(candidate, "*?\\") {
					path = candidate
				}
			}
		}
		out = append(out,
			AssessmentSample{ID: payload.category + "-query", Category: payload.category, Kind: "attack", Method: "GET", Path: path + "?value=" + url.QueryEscape(payload.query)},
			AssessmentSample{ID: payload.category + "-json", Category: payload.category, Kind: "attack", Method: "POST", Path: path, Headers: map[string]string{"Content-Type": "application/json"}, Body: payload.body},
			AssessmentSample{ID: payload.category + "-form", Category: payload.category, Kind: "attack", Method: "POST", Path: path, Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, Body: "value=" + url.QueryEscape(payload.query)},
		)
	}
	out = append(out,
		AssessmentSample{ID: "scanner-tool", Category: "scanner", Kind: "attack", Method: "GET", Path: "/.env", Headers: map[string]string{"User-Agent": "sqlmap/1.8"}},
		AssessmentSample{ID: "scanner-path", Category: "scanner", Kind: "attack", Method: "GET", Path: "/.git/config", Headers: map[string]string{"User-Agent": "Nikto"}},
		AssessmentSample{ID: "bot-headless", Category: "bot", Kind: "attack", Method: "GET", Path: paths[0], Headers: map[string]string{"User-Agent": "Mozilla/5.0 HeadlessChrome/131.0.0.0"}},
		AssessmentSample{ID: "cookie-injection", Category: "sqli", Kind: "attack", Method: "GET", Path: paths[0], Headers: map[string]string{"Cookie": "filter=1' OR '1'='1'--"}},
	)
	return out
}

func AssessmentCategoryName(category string) string {
	if value := map[string]string{"normal": "正常访问", "source_boundary": "来源访问限制", "source_spoof": "伪造来源", "sqli": "SQL 注入", "xss": "跨站脚本", "lfi": "文件读取与路径穿越", "rce": "命令注入", "ssti": "模板注入", "ssrf": "内网资源探测", "scanner": "扫描器与敏感路径", "bot": "自动化探测", "imported": "导入实录", "custom": "自定义样本"}[category]; value != "" {
		return value
	}
	return category
}
