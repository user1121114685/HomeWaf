package wafpipeline

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var Ports = NewRegistry[Snapshot](200)

func (r *SnapshotRegistry[T]) Limit() int { return r.limit }

// SnapshotRegistry publishes a complete immutable per-port value. A failed
// prepare function leaves both the port and every other port unchanged.
type SnapshotRegistry[T any] struct {
	mu      sync.Mutex
	current atomic.Pointer[map[uint16]*T]
	limit   int
}

func NewRegistry[T any](limit int) *SnapshotRegistry[T] {
	if limit <= 0 {
		limit = 200
	}
	r := &SnapshotRegistry[T]{limit: limit}
	m := map[uint16]*T{}
	r.current.Store(&m)
	return r
}
func (r *SnapshotRegistry[T]) Get(port uint16) (*T, bool) {
	m := r.current.Load()
	if m == nil {
		return nil, false
	}
	v, ok := (*m)[port]
	return v, ok
}
func (r *SnapshotRegistry[T]) Publish(port uint16, prepare func() (*T, error)) error {
	if port == 0 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	next, err := prepare()
	if err != nil {
		return err
	}
	if next == nil {
		return fmt.Errorf("empty port snapshot")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.current.Load()
	copy := map[uint16]*T{}
	if cur != nil {
		for k, v := range *cur {
			copy[k] = v
		}
	}
	if _, exists := copy[port]; !exists && len(copy) >= r.limit {
		return fmt.Errorf("port quota reached (%d)", r.limit)
	}
	copy[port] = next
	r.current.Store(&copy)
	return nil
}
func (r *SnapshotRegistry[T]) Remove(port uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.current.Load()
	if cur == nil {
		return
	}
	next := map[uint16]*T{}
	for k, v := range *cur {
		if k != port {
			next[k] = v
		}
	}
	r.current.Store(&next)
}
func (r *SnapshotRegistry[T]) Ports() []uint16 {
	m := r.current.Load()
	if m == nil {
		return nil
	}
	out := make([]uint16, 0, len(*m))
	for p := range *m {
		out = append(out, p)
	}
	return out
}
