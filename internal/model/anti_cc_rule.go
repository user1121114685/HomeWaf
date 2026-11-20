// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package model

import (
	"encoding/json"
	"strings"
)

// AntiCCRule 描述 ccrule 匹配请求时使用的范围、计数阶段和分组维度。
type AntiCCRule struct {
	MatchMode    string `json:"match_mode"`
	MatchJson    string `json:"match_json"`
	CountPhase   string `json:"count_phase"`
	CountScope   string `json:"count_scope"`
	ExcludeExts  string `json:"exclude_exts"`
	StatDim      string `json:"stat_dim"`
	StatDimField string `json:"stat_dim_field"`
}

const (
	CCMatchModeAll    = "all"
	CCMatchModeSimple = "simple"
	CCMatchModeExpr   = "expr"
)

const (
	CCCountPhaseRequest  = "request"
	CCCountPhaseResponse = "response"
)

const (
	CCCountScopeAll        = "all"         // 全部请求
	CCCountScopeDynamic    = "dynamic"     // 排除静态资源（新建规则默认）
	CCCountScopeDocument   = "document"    // 仅页面文档请求
	CCCountScopeOriginOnly = "origin_only" // 仅回源未命中缓存的请求
)

const (
	CCStatDimIP        = "ip"
	CCStatDimIPURI     = "ip_uri"
	CCStatDimSession   = "session"
	CCStatDimHeader    = "header"
	CCStatDimCookie    = "cookie"
	CCStatDimQuery     = "query"
	CCStatDimBody      = "body"
	CCStatDimHostTotal = "host_total"
)

// CCDefaultExcludeExts 统计口径 dynamic 的内置静态后缀。
// 取值参照 OWASP CRS DoS 插件的 static_extensions，并补上常见字体与媒体后缀。
var CCDefaultExcludeExts = []string{
	".js", ".css", ".jpg", ".jpeg", ".png", ".gif", ".ico", ".svg", ".webp",
	".woff", ".woff2", ".ttf", ".otf", ".eot", ".map", ".mp4", ".webm", ".mp3", ".bmp", ".avif",
}

// ExcludeExtList 返回本规则实际生效的静态后缀清单（未配置时用内置默认）。
// 逐项归一化成小写并补上前导点，用户填 "js" 或 "JS" 都能命中。
func (r *AntiCCRule) ExcludeExtList() []string {
	raw := strings.TrimSpace(r.ExcludeExts)
	if raw == "" {
		return CCDefaultExcludeExts
	}
	seps := strings.FieldsFunc(raw, func(c rune) bool {
		return c == ',' || c == '\n' || c == '\r' || c == ' ' || c == '\t' || c == ';'
	})
	out := make([]string, 0, len(seps))
	for _, item := range seps {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, ".") {
			item = "." + item
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return CCDefaultExcludeExts
	}
	return out
}

// Conditions 解析结构化匹配条件。MatchMode 不是 simple 时返回空。
func (r *AntiCCRule) Conditions() ([]MatchCondition, error) {
	if r.MatchMode != CCMatchModeSimple || strings.TrimSpace(r.MatchJson) == "" {
		return nil, nil
	}
	var conds []MatchCondition
	if err := json.Unmarshal([]byte(r.MatchJson), &conds); err != nil {
		return nil, err
	}
	return conds, nil
}
