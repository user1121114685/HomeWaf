package wafenginecore

import (
	"HomeWaf/common/zlog"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/model"
	"context"
	"time"
)

// RunSSLOrderQueue keeps slow DNS/CA requests off the service configuration
// event loop. Database scans recover submitted work after missed notifications.
func (waf *WafEngine) RunSSLOrderQueue(ctx context.Context) {
	runSSLOrderQueue(ctx, func(kind int, order model.SslOrder) { waf.applySSLOrder(ctx, kind, order) })
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
