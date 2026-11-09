package main

import (
	"HomeWaf/globalobj"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafenginecore"
	"HomeWaf/wafmangeweb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerReadinessWaitsForInitializationAndManager(t *testing.T) {
	oldEngine, oldManager := globalobj.GWAF_RUNTIME_OBJ_WAF_ENGINE, webmanager
	t.Cleanup(func() { globalobj.GWAF_RUNTIME_OBJ_WAF_ENGINE, webmanager = oldEngine, oldManager })
	globalobj.GWAF_RUNTIME_OBJ_WAF_ENGINE = &wafenginecore.WafEngine{ServerOnline: wafenginmodel.NewSafeServerMap()}
	webmanager = &wafmangeweb.WafWebManager{}
	prg := &wafSystenService{}
	for _, initialized := range []bool{false, true} {
		if initialized {
			prg.startup.Ready()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		err := prg.waitReady(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("initialized=%v: falsely ready, err=%v", initialized, err)
		}
	}
}
