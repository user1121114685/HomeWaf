package main

import (
	"HomeWaf/wafpipeline"
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func TestPortShutdownDrainsResponseEventsBeforeStoppingConsumer(t *testing.T) {
	oldPorts, oldListeners := wafpipeline.Ports, wafpipeline.Listeners
	oldStop, oldFinish := stopPortPipeline, finishPortPipeline
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](2)
	wafpipeline.Listeners = &wafpipeline.ListenerManager{}
	t.Cleanup(func() {
		wafpipeline.Ports = oldPorts
		wafpipeline.Listeners = oldListeners
		stopPortPipeline = oldStop
		finishPortPipeline = oldFinish
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := wafpipeline.NewPunisher()
	done := make(chan struct{})
	go func() { defer close(done); p.Run(ctx) }()
	stopPortPipeline = cancel
	finishPortPipeline = func() { <-done }
	entered, release := make(chan struct{}, 1), make(chan struct{})
	live := &wafpipeline.Runtime{Sink: p, State: p, Forward: func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, w http.ResponseWriter) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(204)
		c.Hit("rce", "shutdown-fixture", wafpipeline.Critical, false, "response event")
	}}
	policy := wafpipeline.DefaultPolicy()
	policy.Access.DrainSeconds = 2
	policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
	s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: "http://upstream.test", ListenEnabled: true, Policy: policy}}
	if err := live.Prepare(s); err != nil {
		t.Fatal(err)
	}
	if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	wafpipeline.Listeners.SetHandler(live.Serve)
	if err := wafpipeline.Listeners.Start(s); err != nil {
		t.Fatal(err)
	}
	defer wafpipeline.Listeners.StopAll()
	clientDone := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(int(port))+"/", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.10")
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err == nil {
			_, err = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		clientDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("request not started")
	}
	go func() { time.Sleep(50 * time.Millisecond); close(release) }()
	shutdownPortPipeline()
	if err := <-clientDone; err != nil {
		t.Fatal(err)
	}
	if p.BanReason(wafpipeline.PortID(port), netip.MustParseAddr("198.51.100.10")) == "" {
		t.Fatal("shutdown lost the response-stage event")
	}
	if p.Stats()["queued"] != 0 {
		t.Fatal("shutdown left unprocessed events")
	}
}
