//go:build !linux && !windows

package wafhostguard

// newSources 在不支持主机日志的平台返回不可用原因，供引擎进入待命状态。
func newSources() ([]Source, string) {
	_, reason, _ := checkLogCapability()
	return nil, reason
}
