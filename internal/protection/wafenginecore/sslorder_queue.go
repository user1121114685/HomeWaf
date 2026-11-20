package wafenginecore

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/enums"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/services/waf_service"
	"context"
	"time"
)

// RunSSLOrderQueue keeps slow DNS/CA requests off the service configuration
// event loop. Database scans recover submitted work after missed notifications.
func RunSSLOrderQueue(ctx context.Context) {
	go runCertificateMaintenance(ctx)

	runSSLOrderQueue(ctx, func(kind int, order model.SslOrder) { applySSLOrder(ctx, order) })
}

func runSSLOrderQueue(ctx context.Context, apply func(int, model.SslOrder)) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := wafSslOrderService.FailInterrupted(time.Now()); err != nil {
			zlog.Error("证书任务状态检查失败", err.Error())
		}
		pending, err := wafSslOrderService.Pending()
		if err != nil {
			zlog.Error("读取待处理证书任务失败", err.Error())
		}
		for _, order := range pending {
			if ctx.Err() != nil {
				return
			}
			kind := enums.ChanSslOrderSubmitted
			if order.RenewFrom != "" || len(order.ResultCertificate) > 0 {
				kind = enums.ChanSslOrderrenew
			}
			apply(kind, order)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-global.GWAF_CHAN_SSLOrder:
		}
	}
}

func runCertificateMaintenance(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	lastRiskCheck := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		now := time.Now()
		if err := waf_service.RunCertificateRenewals(now, true); err != nil {
			zlog.Error("证书恢复检查失败", err.Error())
		}
		if now.Sub(lastRiskCheck) >= time.Hour {
			if err := waf_service.CheckCertificateExpiryRisks(now); err != nil {
				zlog.Error("证书风险检查失败", err.Error())
			} else {
				lastRiskCheck = now
			}
		}
		if err := waf_service.DispatchCertificateNotifications(now); err != nil {
			zlog.Error("证书通知投递失败", err.Error())
		}
	}
}
