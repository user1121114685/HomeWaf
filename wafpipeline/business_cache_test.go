package wafpipeline

import (
	"HomeWaf/model"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBusinessCacheCannotBypassGateOrCacheCredentialsAndStreams(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cookie":
			w.Header().Set("Set-Cookie", "app_session=private")
		case "/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.(http.Flusher).Flush()
		case "/large":
			io.WriteString(w, strings.Repeat("x", 128<<10))
			return
		}
		io.WriteString(w, "family-content")
	}, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"*"}, Upstream: f.back.URL, Cache: &BusinessCacheConfig{Rules: []model.CacheRule{{RuleType: 2, RuleContent: "/", ParamType: 2, CacheTime: 60, RequestMethod: "GET;HEAD"}}}}}
	})
	for _, path := range []string{"/public", "/public", "/cookie", "/cookie", "/stream", "/stream", "/large", "/large"} {
		code, body, _ := proxyCall(t, f, "GET", path, "", nil)
		if code != 200 {
			t.Fatal("cached business unavailable")
		}
		if path == "/large" && len(body) != 128<<10 {
			t.Fatal("cache capture truncated large stream")
		}
	}
	if f.requests.Load() != 7 {
		t.Fatalf("incorrect cache eligibility: upstream=%d", f.requests.Load())
	}
	for i := 0; i < 2; i++ {
		code, _, _ := proxyCall(t, f, "GET", "/public", "", map[string]string{"Cookie": "app_identity=family"})
		if code != 200 {
			t.Fatal("normal cookie request blocked")
		}
	}
	if f.requests.Load() != 9 {
		t.Fatal("credential-bearing request served shared cache")
	}
	m := f.current.Load().Compiled.Metrics
	stats := m.Stats()
	if stats["business_cache_hits"] != 1 || stats["business_cache_bytes"] > uint64((64<<20)/Ports.Limit()) {
		t.Fatal("cache accounting or quota failed", stats)
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.Gate.Enabled = true })
	code, body, _ := proxyCall(t, f, "GET", "/public", "", nil)
	if code != 401 || strings.Contains(string(body), "family-content") || f.requests.Load() != 9 {
		t.Fatal("cached response bypassed newly enabled login gate")
	}
}

func TestBusinessCacheInvalidatesWhenUpstreamChanges(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "old-origin") }, false)
	var otherHits atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherHits.Add(1); io.WriteString(w, "new-origin") }))
	defer other.Close()
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"*"}, Upstream: f.back.URL, Cache: &BusinessCacheConfig{Rules: []model.CacheRule{{RuleType: 3, RuleContent: "/asset", ParamType: 2, CacheTime: 60}}}}}
	})
	for i := 0; i < 2; i++ {
		_, body, _ := proxyCall(t, f, "GET", "/asset", "", nil)
		if string(body) != "old-origin" {
			t.Fatal("old origin unavailable")
		}
	}
	if f.requests.Load() != 1 {
		t.Fatal("public response was not cached")
	}
	f.update(t, func(s *Snapshot) { s.Config.Routes[0].Upstream = other.URL })
	for i := 0; i < 2; i++ {
		_, body, _ := proxyCall(t, f, "GET", "/asset", "", nil)
		if string(body) != "new-origin" {
			t.Fatal("stale business content survived upstream change")
		}
	}
	if otherHits.Load() != 1 {
		t.Fatal("new cache namespace did not stabilize")
	}
}
