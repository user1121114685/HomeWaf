package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/wafnet"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProxyReadinessDoesNotProbeOldWorker(t *testing.T) {
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer old.Close()
	port := old.Listener.Addr().(*net.TCPAddr).Port
	waf := newTestWafEngine()
	waf.ServerOnline.Set(port, innerbean.ServerRunTime{Port: port, ServerType: "http"})
	ready := &wafnet.Readiness{}
	waf.listenerReadiness.Store(port, ready)
	if err := waf.WaitProxyReady(20 * time.Millisecond); err == nil {
		t.Fatal("old worker's listening port satisfied new worker readiness")
	}
	ready.Ready()
	if err := waf.WaitProxyReady(time.Second); err != nil {
		t.Fatal(err)
	}
	ready.Fail(context.Canceled)
	if err := waf.WaitProxyReady(time.Second); err == nil {
		t.Fatal("failed local listener was promoted")
	}
}

func TestProxyReadinessRequiresEveryListener(t *testing.T) {
	waf := newTestWafEngine()
	for _, port := range []int{8101, 8102} {
		waf.ServerOnline.Set(port, innerbean.ServerRunTime{Port: port, ServerType: "https"})
		ready := &wafnet.Readiness{}
		if port == 8101 {
			ready.Ready()
		}
		waf.listenerReadiness.Store(port, ready)
	}
	if err := waf.WaitProxyReady(20 * time.Millisecond); err == nil {
		t.Fatal("partially started worker was promoted")
	}
}
