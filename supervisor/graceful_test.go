package supervisor

import (
	"HomeWaf/wafipc"
	"HomeWaf/wafnet"
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Use the real supervisor/IPC and real child processes without a database or WAF configuration.
func TestMain(m *testing.M) {
	if os.Getenv("HOMEWAF_SUPERVISOR_TEST_WORKER") == "1" {
		os.Exit(runTestWorker())
	}
	os.Exit(m.Run())
}

func runTestWorker() int {
	var addr, token string
	takeover := false
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--ctrl-addr=") {
			addr = strings.TrimPrefix(arg, "--ctrl-addr=")
		}
		if strings.HasPrefix(arg, "--token=") {
			token = strings.TrimPrefix(arg, "--token=")
		}
		if arg == "--takeover" {
			takeover = true
		}
	}
	ctrl, err := wafipc.Dial(addr)
	if err != nil {
		return 2
	}
	defer ctrl.Close()
	_ = ctrl.Send(wafipc.Message{Type: wafipc.MsgHello, Token: token, PID: os.Getpid(), Version: "test", ProtoVer: wafipc.ProtoVersion})
	if takeover && os.Getenv("HOMEWAF_TEST_FAIL_READY") == "1" {
		_ = ctrl.Send(wafipc.Message{Type: wafipc.MsgReadyFailed, PID: os.Getpid(), Err: "test listener startup failure"})
		_, _ = ctrl.Recv()
		return 0
	}
	ln, err := wafnet.ReusePortTCPListen(os.Getenv("HOMEWAF_TEST_HTTP_ADDR"))
	if err != nil {
		_ = ctrl.Send(wafipc.Message{Type: wafipc.MsgReadyFailed, Err: err.Error()})
		return 3
	}
	var ready wafnet.Readiness
	var conns wafnet.ConnTracker
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Worker-PID", strconv.Itoa(os.Getpid()))
		if r.URL.Path == "/slow" {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			time.Sleep(1500 * time.Millisecond)
		}
		if r.URL.Path == "/upgrade" {
			c, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer c.Close()
			_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: upgrade\r\nUpgrade: test-echo\r\n\r\n")
			_ = rw.Flush()
			for {
				line, err := rw.ReadString('\n')
				if err != nil {
					return
				}
				_, _ = rw.WriteString(line)
				if rw.Flush() != nil {
					return
				}
			}
		}
		_, _ = fmt.Fprintf(w, "worker=%d", os.Getpid())
	})}
	go srv.Serve(wafnet.WithReadiness(conns.Listener(ln), &ready))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ready.Wait(ctx); err != nil {
		return 4
	}
	_ = ctrl.Send(wafipc.Message{Type: wafipc.MsgReady, PID: os.Getpid()})
	for {
		m, err := ctrl.Recv()
		if err != nil {
			return 5
		}
		if m.Type == wafipc.MsgDrain || m.Type == wafipc.MsgShutdown {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = srv.Shutdown(ctx)
			_ = conns.Drain(ctx)
			cancel()
			_ = ctrl.Send(wafipc.Message{Type: wafipc.MsgGone, PID: os.Getpid()})
			return 0
		}
	}
}

func testSupervisor(t *testing.T) (*Supervisor, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	t.Setenv("HOMEWAF_SUPERVISOR_TEST_WORKER", "1")
	t.Setenv("HOMEWAF_TEST_HTTP_ADDR", addr)
	exe, _ := os.Executable()
	s := New(Options{ExePath: exe, DataDir: t.TempDir(), DrainTimeout: 3, ReadyTimeout: 3})
	done := make(chan error, 1)
	go func() { done <- s.Run() }()
	t.Cleanup(func() {
		s.Shutdown()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("supervisor did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		w := s.workers[s.activePID]
		ready := w != nil && w.state == "active-candidate"
		s.mu.Unlock()
		if ready {
			return s, "http://" + addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("initial worker did not become ready")
	return nil, ""
}

func TestRollingRestartPreservesTraffic(t *testing.T) {
	s, base := testSupervisor(t)
	// Exercise normal HTTP connection reuse. Forcing thousands of fresh TCP
	// sockets into TIME_WAIT can exhaust a Windows runner's ephemeral ports,
	// turning a process-handoff regression into an unrelated load test.
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{MaxIdleConns: 8, MaxIdleConnsPerHost: 8}}
	defer client.CloseIdleConnections()
	var count, failures atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopTraffic := func() { stopOnce.Do(func() { close(stop) }); wg.Wait() }
	defer stopTraffic()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				resp, err := client.Get(base)
				if err != nil {
					failures.Add(1)
					t.Logf("traffic request failed: %v", err)
				} else {
					body, err := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					if err != nil || resp.StatusCode != 200 || !strings.HasPrefix(string(body), "worker=") {
						failures.Add(1)
						t.Logf("traffic response failed: status=%d body=%q err=%v", resp.StatusCode, body, err)
					}
				}
				count.Add(1)
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	for round := 0; round < 2; round++ {
		slow, err := client.Get(base + "/slow")
		if err != nil {
			t.Fatal(err)
		}
		oldPID := slow.Header.Get("X-Worker-PID")
		upgraded, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
		if err != nil {
			t.Fatal(err)
		}
		_ = upgraded.SetDeadline(time.Now().Add(8 * time.Second))
		rw := bufio.NewReadWriter(bufio.NewReader(upgraded), bufio.NewWriter(upgraded))
		_, _ = rw.WriteString("GET /upgrade HTTP/1.1\r\nHost: test\r\nConnection: upgrade\r\nUpgrade: test-echo\r\n\r\n")
		_ = rw.Flush()
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\r\n" {
				break
			}
		}
		done := make(chan struct{})
		go func() { s.doUpgrade(false); close(done) }()
		body, err := io.ReadAll(slow.Body)
		_ = slow.Body.Close()
		if err != nil || string(body) != "worker="+oldPID {
			t.Errorf("slow request interrupted: %q %v", body, err)
		}
		_, _ = rw.WriteString("still-alive\n")
		_ = rw.Flush()
		line, err := rw.ReadString('\n')
		if err != nil || line != "still-alive\n" {
			t.Errorf("upgraded connection interrupted: %q %v", line, err)
		}
		_ = upgraded.Close()
		select {
		case <-done:
		case <-time.After(8 * time.Second):
			t.Fatal("rolling restart did not finish")
		}
		resp, err := client.Get(base)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Header.Get("X-Worker-PID") == oldPID {
			t.Error("new worker did not take over")
		}
		_ = resp.Body.Close()
	}
	stopTraffic()
	if count.Load() < 100 {
		t.Fatalf("insufficient continuous traffic: %d requests", count.Load())
	}
	if failures.Load() != 0 {
		t.Fatalf("%d failures in %d requests", failures.Load(), count.Load())
	}
	t.Logf("%d requests, zero failures across two real process handoffs; slow and upgraded connections survived", count.Load())
}

func TestRollingRestartKeepsOldWorkerOnReadinessFailure(t *testing.T) {
	t.Setenv("HOMEWAF_TEST_FAIL_READY", "1")
	s, base := testSupervisor(t)
	s.mu.Lock()
	oldPID := s.activePID
	s.mu.Unlock()
	start := time.Now()
	s.doUpgrade(false)
	if time.Since(start) > 2*time.Second {
		t.Fatal("explicit failure waited for readiness timeout")
	}
	s.mu.Lock()
	active := s.activePID
	s.mu.Unlock()
	if active != oldPID {
		t.Fatal("failed candidate replaced old worker")
	}
	resp, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
}
