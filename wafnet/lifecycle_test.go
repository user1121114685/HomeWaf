package wafnet

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestReadinessRequiresOwnAcceptLoop(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var ready Readiness
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := ready.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unstarted listener reported ready: %v", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })}
	defer srv.Close()
	go srv.Serve(WithReadiness(ln, &ready))
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := ready.Wait(ctx2); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ready.Fail(errors.New("listener failed"))
	ready.Ready()
	if err := ready.Wait(ctx2); err == nil {
		t.Fatal("failed listener was revived by READY")
	}
}

func TestConnTrackerDrainsHijackedConnections(t *testing.T) {
	for _, expire := range []bool{false, true} {
		t.Run(map[bool]string{false: "natural-close", true: "deadline"}[expire], func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			var tracker ConnTracker
			hijacked := make(chan net.Conn, 1)
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					hijacked <- c
				}
			})}
			go srv.Serve(tracker.Listener(ln))
			defer srv.Close()
			client, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			_, _ = io.WriteString(client, "GET / HTTP/1.1\r\nHost: test\r\n\r\n")
			serverConn := <-hijacked
			defer serverConn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			if tracker.Count() != 1 {
				t.Fatal("hijacked connection was lost")
			}
			drainCtx, stop := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer stop()
			done := make(chan error, 1)
			go func() { done <- tracker.Drain(drainCtx) }()
			select {
			case <-done:
				t.Fatal("drain returned while connection was live")
			case <-time.After(30 * time.Millisecond):
			}
			if !expire {
				_ = serverConn.Close()
			}
			err = <-done
			if errors.Is(err, context.DeadlineExceeded) != expire {
				t.Fatalf("unexpected drain result %v", err)
			}
			if tracker.Count() != 0 {
				t.Fatal("connection leaked after drain")
			}
		})
	}
}
