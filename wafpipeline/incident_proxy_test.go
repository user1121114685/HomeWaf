package wafpipeline

import (
	"context"
	"net/http"
	"testing"
)

func TestIncidentAlternateIdentityHeadersNeverReachBackend(t *testing.T) {
	seen := make(chan http.Header, 1)
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Clone(context.Background()).Header
		w.WriteHeader(200)
	}, false)
	spoofed := map[string]string{
		"CF-Connecting-IP": "127.0.0.1", "True-Client-IP": "127.0.0.1", "Client-IP": "127.0.0.1", "X-Client-IP": "127.0.0.1",
		"X-Originating-IP": "127.0.0.1", "X-Remote-IP": "127.0.0.1", "X-Remote-Addr": "127.0.0.1", "Proxy-Client-IP": "127.0.0.1",
		"WL-Proxy-Client-IP": "127.0.0.1", "X-ProxyUser-IP": "127.0.0.1", "X-Original-URL": "/admin", "X-Rewrite-URL": "/admin",
	}
	status, _, _ := proxyCall(t, f, "GET", "/api/health", "", spoofed)
	if status != 200 {
		t.Fatalf("unexpected response: %d", status)
	}
	h := <-seen
	for name := range spoofed {
		if h.Get(name) != "" {
			t.Errorf("untrusted %s reached backend: %q", name, h.Get(name))
		}
	}
	if h.Get("X-Real-IP") != "127.0.0.1" || h.Get("X-Forwarded-For") != "127.0.0.1" {
		t.Fatalf("canonical identity missing: %v", h)
	}
}

func TestIncidentH2CRejectedBeforeBackend(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }, false)
	for _, value := range []string{"h2c", "H2C", "websocket, h2c"} {
		status, _, _ := proxyCall(t, f, "GET", "/api/health", "", map[string]string{"Upgrade": value, "Connection": "Upgrade", "HTTP2-Settings": "AAMAAABkAAQAAP__"})
		if status != 403 || f.requests.Load() != 0 {
			t.Fatalf("upgrade %q: status=%d backend=%d", value, status, f.requests.Load())
		}
	}
	status, _, _ := proxyCall(t, f, "GET", "/api/health", "", nil)
	if status != 200 || f.requests.Load() != 1 {
		t.Fatal("ordinary health request broken")
	}
}
