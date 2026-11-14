package acmelease

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledJobNeverEntersAndTimedOutRestartReopensAdmission(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if guard, err := AcquireJob(ctx, dir); !errors.Is(err, context.Canceled) {
		if guard != nil {
			guard.Release()
		}
		t.Fatalf("canceled queue claimed work: %v", err)
	}
	job, err := AcquireJob(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Release()
	ctx, cancel = context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if guard, err := AcquireRestart(ctx, dir); !errors.Is(err, context.DeadlineExceeded) {
		if guard != nil {
			guard.Release()
		}
		t.Fatalf("restart did not wait: %v", err)
	}
	nextCtx, nextCancel := context.WithTimeout(context.Background(), time.Second)
	defer nextCancel()
	next, err := AcquireJob(nextCtx, dir)
	if err != nil {
		t.Fatalf("failed restart kept admission locked: %v", err)
	}
	next.Release()
}

func TestRestartWaitsForCertificateAndBlocksNewWork(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job, err := AcquireJob(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Release()
	gateHeld := make(chan struct{})
	restarted := make(chan *Guard, 1)
	go func() {
		guard, err := acquireRestart(ctx, dir, func() { close(gateHeld) })
		if err != nil {
			t.Error(err)
		}
		restarted <- guard
	}()
	<-gateHeld
	short, cancelShort := context.WithTimeout(ctx, 80*time.Millisecond)
	if another, err := AcquireJob(short, dir); !errors.Is(err, context.DeadlineExceeded) {
		if another != nil {
			another.Release()
		}
		t.Fatalf("new job entered during restart: %v", err)
	}
	cancelShort()
	select {
	case <-restarted:
		t.Fatal("restart did not wait for active certificate")
	default:
	}
	job.Release()
	guard := <-restarted
	if guard == nil {
		t.Fatal("restart failed to acquire idle activity")
	}
	short, cancelShort = context.WithTimeout(ctx, 80*time.Millisecond)
	if another, err := AcquireJob(short, dir); !errors.Is(err, context.DeadlineExceeded) {
		if another != nil {
			another.Release()
		}
		t.Fatalf("job entered during worker handoff: %v", err)
	}
	cancelShort()
	guard.Release()
	if next, err := AcquireJob(ctx, dir); err != nil {
		t.Fatal(err)
	} else {
		next.Release()
	}
}
