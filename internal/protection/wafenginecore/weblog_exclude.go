// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/protection/wafenginecore/ipset"
	"sync/atomic"
)

// ipLogExcludeCompiled 是全局清单的编译结果，配置热更新时整体替换。
type ipLogExcludeCompiled struct {
	index      *ipset.MatchSet
	groupCodes []string
}

func (c *ipLogExcludeCompiled) match(clientIP string) bool {
	if c == nil {
		return false
	}
	if c.index.ContainsStr(clientIP) {
		return true
	}
	for _, code := range c.groupCodes {
		if ipset.GetGroupMatcher(code).ContainsStr(clientIP) {
			return true
		}
	}
	return false
}

// globalIPLogExclude 全局排除清单的编译快照；nil 表示未配置。
// MatchSet.ContainsStr / GetGroupMatcher 均为 nil 安全（返回 false）。
var globalIPLogExclude atomic.Pointer[ipLogExcludeCompiled]

// SetGlobalIPLogExclude 由系统配置热更新入口调用（internal/jobs/waftask/task_config.go）。
func SetGlobalIPLogExclude(raw string) {
	patterns, codes := ipset.ParseListText(raw)
	var index *ipset.MatchSet
	if len(patterns) > 0 {
		index = ipset.BuildMatchSet(patterns)
	}
	globalIPLogExclude.Store(&ipLogExcludeCompiled{index: index, groupCodes: codes})
}
