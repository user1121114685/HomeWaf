package main

import (
	"HomeWaf/internal/management"
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerReadinessWaitsForInitializationAndManager(t *testing.T) {
	oldManager := webmanager
	t.Cleanup(func() { webmanager = oldManager })
	webmanager = &management.WafWebManager{}
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
