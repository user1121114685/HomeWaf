package wafpipeline

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPortBusinessRoutesSharePolicyAndKeepDestinationsIndependent(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "default") }, false)
	var otherHits atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherHits.Add(1); io.WriteString(w, "other:"+r.URL.Path) }))
	defer other.Close()
	s := f.current.Load()
	next := &Snapshot{Config: CloneConfig(s.Config), Secrets: s.Secrets}
	next.Config.Routes = []UpstreamRoute{{Hosts: []string{"nas.test"}, Upstream: f.back.URL}, {Hosts: []string{"camera.test"}, Upstream: other.URL}, {Hosts: []string{"nas.test"}, PathPrefix: "/camera/", Upstream: other.URL}}
	if err := f.runtime.Prepare(next); err != nil {
		t.Fatal(err)
	}
	f.current.Store(next)
	for _, route := range next.Compiled.Proxy.routes {
		if route.target.transport != next.Compiled.Proxy.transport {
			t.Fatal("routes multiplied per-port idle connection pools")
		}
	}
	t.Cleanup(func() { CloseRetiredProxy(next, nil) })
	request := func(host, path string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", f.front.URL+path, nil)
		req.Host = host
		res, err := f.front.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(raw)
	}
	for _, tc := range []struct{ host, path, want string }{{"nas.test", "/photo", "default"}, {"camera.test:65530", "/live", "other:/live"}, {"nas.test", "/camera/live", "other:/camera/live"}} {
		status, body := request(tc.host, tc.path)
		if status != 200 || body != tc.want {
			t.Fatalf("%s%s: %d %s", tc.host, tc.path, status, body)
		}
	}
	if f.requests.Load() != 1 || otherHits.Load() != 2 {
		t.Fatal("hostname or path selected wrong upstream")
	}
	closed := &Snapshot{Config: CloneConfig(next.Config), Secrets: next.Secrets}
	closed.Config.Policy.Access.Enabled = false
	if err := f.runtime.Prepare(closed); err != nil {
		t.Fatal(err)
	}
	f.current.Store(closed)
	t.Cleanup(func() { CloseRetiredProxy(closed, nil) })
	for _, host := range []string{"nas.test", "camera.test"} {
		status, _ := request(host, "/")
		if status != 503 {
			t.Fatal("port switch did not apply to every hostname")
		}
	}
	if f.requests.Load() != 1 || otherHits.Load() != 2 {
		t.Fatal("closed port reached an upstream")
	}
	copy := CloneConfig(next.Config)
	copy.Routes[0].Hosts[0] = "changed.test"
	if next.Config.Routes[0].Hosts[0] != "nas.test" {
		t.Fatal("business routes share mutable config")
	}
	copy = CloneConfig(next.Config)
	copy.Routes = append(copy.Routes, copy.Routes[0])
	if ValidateConfig(copy, 0) == nil {
		t.Fatal("ambiguous duplicate route accepted")
	}
	copy = CloneConfig(next.Config)
	copy.Routes[0].Upstream = "http://secret:password@upstream.test"
	if ValidateConfig(copy, 0) == nil {
		t.Fatal("credential-bearing route accepted")
	}
	if strings.Contains(next.Config.Routes[0].Upstream, "secret") {
		t.Fatal("copy changed original route")
	}
}

func TestUpstreamDialOverrideCanonicalizationAndConflicts(t *testing.T) {
	c := PortConfig{Schema: 1, Port: 5300, Protocol: "http", Upstream: "https://BÜCHER.test", UpstreamDialIP: "127.0.0.1", Policy: DefaultPolicy()}
	c.Routes = []UpstreamRoute{{Hosts: []string{"nas.test"}, Upstream: "https://xn--bcher-kva.test:443", DialIP: "127.0.0.2"}}
	if ValidateConfig(c, 0) == nil {
		t.Fatal("conflicting IDN connection overrides passed preview")
	}
	c.Routes[0].DialIP = "127.0.0.1"
	if err := ValidateConfig(c, 0); err != nil {
		t.Fatal(err)
	}
	overrides, err := upstreamDialOverrides(c)
	if err != nil || len(overrides) != 1 || overrides["xn--bcher-kva.test:443"] != "127.0.0.1" {
		t.Fatal("dial key differs from transport normalization", overrides, err)
	}
}

func TestBusinessPathPriorityStripPrefixAndRedirect(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "origin:"+r.URL.RequestURI()) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = []UpstreamRoute{
			{Hosts: []string{"*"}, PathPrefix: "/api/", Priority: 20, StripPrefix: true, Upstream: f.back.URL + "/service"},
			{Hosts: []string{"*"}, PathPrefix: "/api/private", MatchType: "exact", Priority: 10, TargetType: "redirect", RedirectURL: "/login", RedirectCode: 307},
			{Hosts: []string{"*"}, PathPrefix: `^/api/[0-9]+$`, MatchType: "regex", Priority: 5, Upstream: f.back.URL + "/numeric"},
		}
	})
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	for _, tc := range []struct {
		path string
		code int
		body string
	}{{"/api/file?q=a%2Fb", 200, "origin:/service/file?q=a%2Fb"}, {"/api/12", 200, "origin:/numeric/api/12"}, {"/api/private", 307, ""}, {"/ordinary", 200, "origin:/ordinary"}} {
		res, err := client.Get(f.front.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != tc.code || tc.body != "" && string(body) != tc.body {
			t.Fatalf("%s status=%d body=%q", tc.path, res.StatusCode, body)
		}
		if tc.code == 307 && res.Header.Get("Location") != "/login" {
			t.Fatal("redirect target changed")
		}
	}
	if f.requests.Load() != 3 {
		t.Fatal("redirect reached origin or proxy request was lost")
	}
}

func TestRouteOnlyTLSOriginsRetainConfiguredTrust(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "private-ca-origin") }, true)
	f.update(t, func(s *Snapshot) {
		s.Config.Upstream = ""
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"*"}, Upstream: f.back.URL}}
	})
	res, err := f.front.Client().Get(f.front.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "private-ca-origin" || f.requests.Load() != 1 {
		t.Fatal("route-only config lost CA trust")
	}
}

func TestNativeLoadBalancePreservesTargetsWeightsAndHealth(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "primary") }, false)
	var secondHits atomic.Int64
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { secondHits.Add(1); io.WriteString(w, "second") }))
	defer second.Close()
	logical := func(raw string) string { u, _ := url.Parse(raw); return "http://backend.test:" + u.Port() }
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"*"}, Upstream: f.back.URL, Balance: "wrr", Backends: []Backend{{Upstream: logical(f.back.URL), DialIP: "127.0.0.1", Weight: 3}, {Upstream: logical(second.URL), DialIP: "127.0.0.1", Weight: 1}}}}
	})
	request := func() {
		t.Helper()
		res, err := f.front.Client().Get(f.front.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatal("backend unavailable")
		}
	}
	for i := 0; i < 8; i++ {
		request()
	}
	if f.requests.Load() != 6 || secondHits.Load() != 2 {
		t.Fatalf("weights or destinations changed: %d/%d", f.requests.Load(), secondHits.Load())
	}
	root := f.current.Load().Compiled.Proxy
	balance := root.routes[0].target.balance
	if balance.targets[0].transport == balance.targets[1].transport {
		t.Fatal("different fixed-IP backends share authority pool")
	}
	for i := 0; i < 3; i++ {
		balance.targets[0].Health.Record(503)
	}
	for i := 0; i < 4; i++ {
		request()
	}
	if f.requests.Load() != 6 || secondHits.Load() != 6 {
		t.Fatal("unhealthy backend not avoided")
	}
	f.update(t, func(s *Snapshot) { s.Config.Routes[0].Balance = "ip_hash" })
	balance = f.current.Load().Compiled.Proxy.routes[0].target.balance
	first := balance.choose("198.51.100.8")
	for i := 0; i < 4; i++ {
		if balance.choose("198.51.100.8") != first {
			t.Fatal("IP affinity changed without config or health change")
		}
	}
}

func TestLoadBalanceIdleBudgetRemainsPerPort(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = nil
		for i := 0; i < 4; i++ {
			route := UpstreamRoute{Hosts: []string{fmt.Sprintf("h%d.test", i)}, Upstream: f.back.URL}
			for j := 0; j < 32; j++ {
				route.Backends = append(route.Backends, Backend{Upstream: f.back.URL, Weight: 1})
			}
			s.Config.Routes = append(s.Config.Routes, route)
		}
	})
	root := f.current.Load().Compiled.Proxy
	budget := root.transport.MaxIdleConns
	count := 0
	for _, route := range root.routes {
		for _, backend := range route.target.balance.targets {
			count++
			if !backend.transport.DisableKeepAlives {
				budget += backend.transport.MaxIdleConns
			}
		}
	}
	if count != 128 || budget != 32 {
		t.Fatalf("per-port idle connection quota multiplied: backends=%d budget=%d", count, budget)
	}
}

func TestLocalOnlyHostCannotFallThroughToOtherBusiness(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "other-private-business") }, false)
	f.update(t, func(s *Snapshot) {
		s.Config.Routes = []UpstreamRoute{{Hosts: []string{"local.test"}, TargetType: "not_found"}, {Hosts: []string{"local.test"}, PathPrefix: "/files/", TargetType: "redirect", RedirectURL: "/local"}, {Hosts: []string{"other.test"}, Upstream: f.back.URL}}
	})
	req, _ := http.NewRequest("GET", f.front.URL+"/not-a-local-file", nil)
	req.Host = "local.test"
	res, err := f.front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 404 || strings.Contains(string(body), "other-private-business") || f.requests.Load() != 0 {
		t.Fatal("local-only hostname fell through to another origin")
	}
}
