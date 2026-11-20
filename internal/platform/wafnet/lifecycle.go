package wafnet

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Readiness is a process-local startup barrier. A shared TCP port being reachable
// is insufficient: the accepting socket may belong to the previous worker.
// The zero value is ready for use; a failure is terminal for this instance.
type Readiness struct {
	once      sync.Once
	mu        sync.Mutex
	done      chan struct{}
	completed bool
	err       error
}

func (r *Readiness) init() { r.once.Do(func() { r.done = make(chan struct{}) }) }

func (r *Readiness) Ready() {
	r.init()
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.completed {
		r.completed = true
		close(r.done)
	}
}

func (r *Readiness) Fail(err error) {
	if err == nil {
		err = errors.New("listener stopped")
	}
	r.init()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
	if !r.completed {
		r.completed = true
		close(r.done)
	}
}

func (r *Readiness) Wait(ctx context.Context) error {
	r.init()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.done:
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.err
	}
}

type readyListener struct {
	net.Listener
	ready *Readiness
}

// WithReadiness signals only when this server has reached its Accept loop,
// after bind and HTTP/TLS setup have succeeded.
func WithReadiness(ln net.Listener, ready *Readiness) net.Listener {
	return &readyListener{Listener: ln, ready: ready}
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.ready.Ready()
	return l.Listener.Accept()
}

// ConnTracker retains upgraded/hijacked connections that http.Server.Shutdown
// deliberately does not wait for, including proxied WebSockets.
type ConnTracker struct {
	mu    sync.Mutex
	conns map[*trackedConn]struct{}
}

type trackedListener struct {
	net.Listener
	tracker *ConnTracker
}
type trackedConn struct {
	net.Conn
	tracker *ConnTracker
	once    sync.Once
}

func (t *ConnTracker) Listener(ln net.Listener) net.Listener {
	return &trackedListener{Listener: ln, tracker: t}
}

func (l *trackedListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	t := l.tracker
	cTracked := &trackedConn{Conn: c, tracker: t}
	t.mu.Lock()
	if t.conns == nil {
		t.conns = make(map[*trackedConn]struct{})
	}
	t.conns[cTracked] = struct{}{}
	t.mu.Unlock()
	return cTracked, nil
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		c.tracker.mu.Lock()
		delete(c.tracker.conns, c)
		c.tracker.mu.Unlock()
	})
	return err
}

func (t *ConnTracker) Count() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return int64(len(t.conns))
}

// Drain is called after Shutdown has stopped accepting connections. Existing
// connections finish naturally; only those exceeding the deadline are closed.
func (t *ConnTracker) Drain(ctx context.Context) error {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for t.Count() > 0 {
		select {
		case <-ctx.Done():
			t.mu.Lock()
			connections := make([]*trackedConn, 0, len(t.conns))
			for c := range t.conns {
				connections = append(connections, c)
			}
			t.mu.Unlock()
			for _, c := range connections {
				_ = c.Close()
			}
			return ctx.Err()
		case <-tick.C:
		}
	}
	return nil
}
