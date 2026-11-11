package wafenginecore

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNativeBusinessHeadersUseSealedValuesAndObserveOriginalResponse(t *testing.T) {
	var hits atomic.Int64
	var credential atomic.Value
	credential.Store("Bearer fixture-one")
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != credential.Load().(string) || r.Header.Get("X-Client") != "127.0.0.1" || strings.Contains(r.Header.Get("Cookie"), "hw_gate_") {
			t.Error("upstream headers or credential stripping incorrect")
		}
		w.Header().Set("Location", "/actual-success")
		w.WriteHeader(302)
	}))
	defer origin.Close()
	oldPorts := wafpipeline.Ports
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
	defer func() { wafpipeline.Ports = oldPorts }()
	var current atomic.Pointer[wafpipeline.Snapshot]
	results := make(chan wafpipeline.Result, 4)
	forward := &wafpipeline.Forwarder{RequestHeaders: PipelineRequestHeaders, ResponseHeaders: PipelineResponseHeaders}
	runtime := &wafpipeline.Runtime{ForwardPrepare: forward.Prepare, Forward: forward.Serve, OnResult: func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
		results <- result
	}}
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { runtime.Serve(current.Load(), w, r) }))
	defer front.Close()
	port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
	config := wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: origin.URL, HeaderProfile: "origin-headers", Policy: wafpipeline.DefaultPolicy()}
	config.Policy.Gate.Enabled = false
	config.Policy.HitPolicy.BusinessLogin = []wafpipeline.LoginObservation{{Path: "/login", Method: "POST", SuccessStatuses: []int{302}, SuccessLocationPrefix: "/actual-success", BodyPrefixBytes: 1024}}
	set := func(value string) {
		t.Helper()
		s := &wafpipeline.Snapshot{Config: wafpipeline.CloneConfig(config), Secrets: wafpipeline.PortSecrets{BusinessHeaders: map[string]wafpipeline.BusinessHeaders{"origin-headers": {Request: []model.CustomHeaderItem{{HeaderName: "Authorization", HeaderValue: value}, {HeaderName: "X-Client", HeaderValue: "${client_ip}"}}, Response: []model.CustomResponseHeaderRule{{MatchType: "global", Headers: []model.CustomHeaderItem{{HeaderName: "Location", HeaderValue: "/public-destination"}, {HeaderName: "X-Family", HeaderValue: "${host}"}}}}}}}}
		if err := runtime.Prepare(s); err != nil {
			t.Fatal(err)
		}
		old := current.Swap(s)
		if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return s, nil }); err != nil {
			t.Fatal(err)
		}
		wafpipeline.CloseRetiredProxy(old, s)
	}
	set("Bearer fixture-one")
	defer func() { wafpipeline.CloseRetiredProxy(current.Load(), nil) }()
	front.Start()
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	request := func() {
		t.Helper()
		req, _ := http.NewRequest("POST", front.URL+"/login", nil)
		req.Host = "family.test"
		req.Header.Set("Cookie", "hw_gate_1=private; app_cookie=normal")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != 302 || res.Header.Get("Location") != "/public-destination" || res.Header.Get("X-Family") != "family.test" {
			t.Fatal("response header policy lost")
		}
		result := <-results
		found := false
		for _, event := range result.Hits {
			if event.Category == "business_auth_success" {
				found = true
			}
		}
		if !found {
			t.Fatal("response observation used transformed Location")
		}
	}
	request()
	credential.Store("Bearer fixture-two")
	set("Bearer fixture-two")
	request()
	if hits.Load() != 2 {
		t.Fatal("business request lost")
	}
}
