package supervisor

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"HomeWaf/internal/acmelease"
)

func TestCertificateGuardWaitsThroughRealWorkerHandoff(t *testing.T) {
	s, base := testSupervisor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	job, err := acmelease.AcquireJob(ctx, s.opts.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Release()
	slow, err := http.Get(base + "/slow")
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Body.Close()
	before := loadPrevStateFile(s.opts.DataDir)
	if before == nil {
		t.Fatal("missing state")
	}
	done := make(chan error, 1)
	go func() {
		guard, err := acmelease.AcquireRestart(ctx, s.opts.DataDir)
		if err != nil {
			done <- err
			return
		}
		defer guard.Release()
		done <- TriggerUpgradeAndWait(ctx, s.opts.DataDir)
	}()
	select {
	case err := <-done:
		t.Fatalf("restart interrupted active certificate: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	state := loadPrevStateFile(s.opts.DataDir)
	if state == nil || state.ActivePID != before.ActivePID || state.Upgrading {
		t.Fatal("worker changed while certificate active")
	}
	job.Release()
	if _, err := io.ReadAll(slow.Body); err != nil {
		t.Fatalf("in-flight request interrupted: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	state = loadPrevStateFile(s.opts.DataDir)
	if state == nil || state.ActivePID == before.ActivePID || state.Upgrading {
		t.Fatal("returned before handoff")
	}
	for _, pid := range state.PIDs {
		if pid == before.ActivePID {
			t.Fatal("returned while old worker still present")
		}
	}
	next, err := acmelease.AcquireJob(ctx, s.opts.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	next.Release()
}

func TestWaitForUpgradeReportsCandidateFailure(t *testing.T) {
	t.Setenv("HOMEWAF_TEST_FAIL_READY", "1")
	s, _ := testSupervisor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := TriggerUpgradeAndWait(ctx, s.opts.DataDir); err == nil || ctx.Err() != nil {
		t.Fatalf("expected immediate readiness failure, got %v", err)
	}
}
