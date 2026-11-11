package zlog

import (
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"sync"
	"testing"
)

func TestConcurrentReconfigurationPreservesExtraCoreAndWriter(t *testing.T) {
	oldLogger, oldBase, oldExtra := logger.Load(), baseCores, extraCore
	defer func() {
		initMu.Lock()
		defer initMu.Unlock()
		baseCores = oldBase
		extraCore = oldExtra
		logger.Store(oldLogger)
	}()
	InitZLog(false, "json")
	originalWriter := fileWriter
	core, observed := observer.New(zapcore.DebugLevel)
	AddCore(core)
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if i%4 == 0 {
					InitZLog(false, "json")
					AddCore(core)
				}
				Debug("concurrent fixture")
			}
		}()
	}
	wg.Wait()
	if observed.Len() != 640 {
		t.Fatalf("reconfiguration lost logs: %d", observed.Len())
	}
	if fileWriter != originalWriter {
		t.Fatal("reconfiguration replaced the shared file writer")
	}
}
