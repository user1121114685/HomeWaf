// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafnotify

import (
	"fmt"

	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/zlog"
)

// LogNotifier delivers access logs to a configured output.
type LogNotifier interface {
	NotifyBatch(logs []*innerbean.WebLog) error
	NotifySingle(log *innerbean.WebLog) error
}

type WafNotifyService struct {
	notifier LogNotifier
	enable   int64
}

// 初始化 NotifyService
func NewWafNotifyService(notifier LogNotifier, enable int64) *WafNotifyService {
	return &WafNotifyService{notifier: notifier, enable: enable}
}

// 改激活状态
func (ls *WafNotifyService) ChangeEnable(enable int64) {
	if ls == nil {
		return
	}
	ls.enable = enable
}

// GetNotifier 获取底层通知器
func (ls *WafNotifyService) GetNotifier() LogNotifier {
	if ls == nil {
		return nil
	}
	return ls.notifier
}

// 处理并发送多条日志
func (ls *WafNotifyService) ProcessBatchLogs(logs []*innerbean.WebLog) error {
	if ls == nil || ls.enable == 0 || ls.notifier == nil {
		zlog.Debug("kafka没有开启")
		return nil
	}
	// 日志处理逻辑
	zlog.Debug("Processing batch logs...")

	// 通知处理
	safe := make([]*innerbean.WebLog, 0, len(logs))
	for _, log := range logs {
		if log != nil {
			safe = append(safe, log.RedactedCopy())
		}
	}
	if err := ls.notifier.NotifyBatch(safe); err != nil {
		return fmt.Errorf("failed to notify batch logs: %v", err)
	}
	return nil
}
