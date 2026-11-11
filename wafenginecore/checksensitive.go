// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	goahocorasick "HomeWaf/internal/multimatch"
	"HomeWaf/model"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"net/http"
	"net/url"
	"sort"
)

/*
*
检测敏感词
*/
func (waf *WafEngine) CheckSensitive(r *http.Request, log *innerbean.WebLog, formValue url.Values, hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{}
	state := waf.sensitive.Load()
	if state == nil || !state.in {
		return result
	}
	urlMatches := prioritizeSensitiveMatches(state.machine.MultiPatternSearch([]rune(log.URL), false), "in")
	bodyMatches := prioritizeSensitiveMatches(state.machine.MultiPatternSearch([]rune(log.BODY), false), "in")
	matches := prioritizeSensitiveMatches(append(append([]*goahocorasick.Term{}, urlMatches...), bodyMatches...), "in")
	if len(matches) == 0 {
		return result
	}
	log.RISK_LEVEL = 1
	log.GUEST_IDENTIFICATION = "触发敏感词"
	log.RULE = "敏感词检测"
	if matches[0].CustomData.(model.Sensitive).Action == "deny" {
		result.IsBlock = true
		result.Title, result.Content = "敏感词检测", "敏感词内容"
		return result
	}
	if hostTarget.Host.LogOnlyMode != 1 {
		if words := processSensitiveWords(urlMatches, "out"); len(words) > 0 {
			_ = waf.ReplaceURLContent(r, words, global.GWAF_HTTP_SENSITIVE_REPLACE_STRING)
		}
		if words := processSensitiveWords(bodyMatches, "out"); len(words) > 0 {
			_ = waf.ReplaceBodyContent(r, words, global.GWAF_HTTP_SENSITIVE_REPLACE_STRING)
		}
	}
	return result
}

// 排除某个
func prioritizeSensitiveMatches(input []*goahocorasick.Term, direction string) []*goahocorasick.Term {
	out := make([]*goahocorasick.Term, 0, len(input))
	for _, term := range input {
		rule := term.CustomData.(model.Sensitive)
		if rule.CheckDirection == direction || rule.CheckDirection == "all" {
			out = append(out, term)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CustomData.(model.Sensitive).Action == "deny" && out[j].CustomData.(model.Sensitive).Action != "deny"
	})
	return out
}

func processSensitiveWords(input []*goahocorasick.Term, except string) []string {
	replaceStringsMap := make(map[string]bool) // 使用map去重
	var replaceStrings []string

	for _, term := range input {
		sensitive := term.CustomData.(model.Sensitive)
		if sensitive.CheckDirection == except {
			continue
		}
		if sensitive.Action == "replace" {
			// 检查是否已经存在，避免重复添加
			if !replaceStringsMap[sensitive.Content] {
				replaceStringsMap[sensitive.Content] = true
				replaceStrings = append(replaceStrings, sensitive.Content)
			}
		}
	}
	return replaceStrings
}
