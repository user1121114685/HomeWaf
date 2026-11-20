// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/internal/acmelease"
	"HomeWaf/internal/acmelog"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/services/waf_service"
	"context"
	"errors"
	"path/filepath"
	"time"
)

var wafSslOrderService = waf_service.WafSSLOrderServiceApp

func applySSLOrder(ctx context.Context, bean model.SslOrder) {
	guard, err := acmelease.AcquireJob(ctx, filepath.Join(utils.GetCurrentDir(), "data"))
	if err != nil {
		if ctx.Err() == nil {
			zlog.Error("证书任务运行锁获取失败", err.Error())
		}
		return
	}
	defer guard.Release()
	claimed, ok, err := wafSslOrderService.Claim(bean.Id)
	if err != nil {
		zlog.Error("领取证书任务失败", err.Error())
		return
	}
	if !ok {
		return
	}
	bean = claimed
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ticker.C:
				_ = wafSslOrderService.Progress(bean.Id, "")
			}
		}
	}()
	defer func() {
		if recover() != nil {
			var current model.SslOrder
			if global.GWAF_LOCAL_DB.First(&current, "id = ?", bean.Id).Error == nil {
				current.FailureKind = "interrupted"
				_ = waf_service.RecordCertificateFailure(current, errors.New("证书任务异常中断，请核对结果后重试"), time.Now())
			}
		}
	}()
	_ = wafSslOrderService.AppendLog(bean.Id, "info", "后台已领取任务，正在检查 CA 与 DNS 配置")
	acmelog.Capture(func(level, message string) {
		if err := wafSslOrderService.AppendLog(bean.Id, level, message); err != nil {
			zlog.Warn("保存证书申请日志失败", err.Error())
		}
	}, func() { applyLifecycleCertificate(bean) })
}
