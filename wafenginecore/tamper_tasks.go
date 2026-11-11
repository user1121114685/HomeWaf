package wafenginecore

import (
	"context"
	"sync"
)

type legacyTamperExecutor struct {
	mu              sync.Mutex
	queue           chan func()
	active          bool
	dropped, failed uint64
	started         chan struct{}
}

func newLegacyTamperExecutor(size int) *legacyTamperExecutor {
	return &legacyTamperExecutor{queue: make(chan func(), size), started: make(chan struct{})}
}

var legacyTamperTasks = newLegacyTamperExecutor(32)

// The factory runs only after capacity is reserved; large baselines are not
// copied when the worker is stopped or its bounded queue is full.
func (e *legacyTamperExecutor) submit(factory func() func()) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.active || len(e.queue) == cap(e.queue) {
		e.dropped++
		return false
	}
	e.queue <- factory()
	return true
}
func (e *legacyTamperExecutor) execute(task func()) {
	defer func() {
		if recover() != nil {
			e.mu.Lock()
			e.failed++
			e.mu.Unlock()
		}
	}()
	task()
}
func (e *legacyTamperExecutor) run(ctx context.Context) {
	e.mu.Lock()
	e.active = true
	close(e.started)
	e.mu.Unlock()
	for {
		select {
		case task := <-e.queue:
			e.execute(task)
		case <-ctx.Done():
			e.mu.Lock()
			e.active = false
			e.mu.Unlock()
			for {
				select {
				case task := <-e.queue:
					e.execute(task)
				default:
					return
				}
			}
		}
	}
}
func RunLegacyTamperTasks(ctx context.Context) { legacyTamperTasks.run(ctx) }
func LegacyTamperTaskStats() map[string]uint64 {
	e := legacyTamperTasks
	e.mu.Lock()
	defer e.mu.Unlock()
	return map[string]uint64{"queued": uint64(len(e.queue)), "capacity": uint64(cap(e.queue)), "dropped": e.dropped, "failed": e.failed}
}
