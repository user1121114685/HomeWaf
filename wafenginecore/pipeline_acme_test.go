package wafenginecore

import (
	"HomeWaf/wafacme"
	"HomeWaf/wafpipeline"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeACMEOnlyServesRegisteredExactHostAndToken(t *testing.T) {
	const token = "native_challenge_fixture_20261111"
	wafacme.SetDataDir(t.TempDir())
	wafacme.Present("pipeline-80", token, "fixture-key-authorization")
	defer wafacme.CleanUp("pipeline-80", token)
	gate := wafpipeline.NewGate(nil)
	rules := wafpipeline.NewRulesA(gate)
	calls := 0
	runtime := &wafpipeline.Runtime{CertificateChallenge: PipelineCertificateChallenge, RuleSteps: rules.Steps, Respond: gate.Respond, Forward: func(*wafpipeline.Snapshot, *wafpipeline.RequestContext, http.ResponseWriter) { calls++ }}
	s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: 80, Protocol: "http", Upstream: "http://127.0.0.1:8080", HostCode: "pipeline-80", HostNames: []string{"family.test"}, Policy: wafpipeline.DefaultPolicy()}, Secrets: wafpipeline.PortSecrets{GateKey: "acme-fixture-key-not-production-32-bytes"}}
	s.Config.Policy.Gate.Enabled = true
	if err := runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host, path string
		want       int
	}{{"family.test", "/.well-known/acme-challenge/" + token, 200}, {"other.test", "/.well-known/acme-challenge/" + token, 403}, {"family.test", "/.well-known/acme-challenge/missing", 401}, {"family.test", "/.well-known/acme-challenge/%2e%2e/admin", 401}, {"family.test", "/.well-known/acme-challenge/a/" + token, 401}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+tc.path, nil)
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 80}))
		w := httptest.NewRecorder()
		runtime.Serve(s, w, req)
		if w.Code != tc.want || calls != 0 {
			t.Fatalf("%s %s: %d calls=%d", tc.host, tc.path, w.Code, calls)
		}
		if tc.want == 200 && w.Body.String() != "fixture-key-authorization" {
			t.Fatal("wrong challenge value")
		}
	}
	s.Config.Policy.Access.Enabled = false
	if err := runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://family.test/.well-known/acme-challenge/"+token, nil)
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 80}))
	w := httptest.NewRecorder()
	runtime.Serve(s, w, req)
	if w.Code != 503 {
		t.Fatal("ACME bypassed master access switch")
	}
}

func TestNativeACMEMatchesExplicitRouteDomainInPreview(t *testing.T) {
	const token = "native_service_challenge_fixture"
	wafacme.SetDataDir(t.TempDir())
	wafacme.Present("pipeline-80", token, "fixture-key-authorization")
	defer wafacme.CleanUp("pipeline-80", token)
	snapshot := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{HostCode: "pipeline-80", Routes: []wafpipeline.UpstreamRoute{{Hosts: []string{"nas.example.com", "*.family.test"}}}}}
	for _, host := range []string{"nas.example.com", "other.example.com", "nas.family.test", "*.family.test"} {
		request := httptest.NewRequest("GET", "http://"+host+"/.well-known/acme-challenge/"+token, nil)
		value, found := PipelineCertificateChallenge(snapshot, &wafpipeline.RequestContext{Request: request, DryRun: true})
		if found != (host == "nas.example.com") || found && value != "fixture-key-authorization" {
			t.Fatalf("incorrect native challenge binding for %s", host)
		}
	}
}
