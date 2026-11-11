package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/wafnet"
	"HomeWaf/wafpipeline"
	"context"
	"errors"
	"time"
)

// The caller first publishes native ownership. Stop the old listener before
// opening the new one; SO_REUSEPORT must never split traffic across policies.
func (waf *WafEngine) DetachPipelinePort(port uint16, drain time.Duration) func() error {
	if waf.ServerOnline == nil {
		return nil
	}
	old, ok := waf.ServerOnline.Get(int(port))
	if !ok {
		return nil
	}
	waf.gracefulStopServer(old, drain)
	waf.ServerOnline.Delete(int(port))
	return func() error {
		// Used only when the new listener failed; the service then restores its
		// old database row and ownership marker before returning the error.
		wafpipeline.SetLegacyPortOwner(port, false)
		restored := innerbean.ServerRunTime{Port: old.Port, ServerType: old.ServerType, IPVersion: old.IPVersion, Status: 1}
		waf.ServerOnline.Set(int(port), restored)
		waf.StartProxyServer(restored)
		ready, ok := waf.listenerReadiness.Load(int(port))
		if !ok {
			return errors.New("旧监听恢复未启动")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return ready.(*wafnet.Readiness).Wait(ctx)
	}
}
