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
	wafacme.Present("source-native", token, "fixture-key-authorization")
	defer wafacme.CleanUp("source-native", token)
	gate := wafpipeline.NewGate(nil)
	rules := wafpipeline.NewRulesA(gate)
	calls := 0
	runtime := &wafpipeline.Runtime{CertificateChallenge: PipelineCertificateChallenge, RuleSteps: rules.Steps, Respond: gate.Respond, Forward: func(*wafpipeline.Snapshot, *wafpipeline.RequestContext, http.ResponseWriter) { calls++ }}
	s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: 5123, Protocol: "http", Upstream: "http://127.0.0.1:8080", ACMEHosts: map[string]string{"family.test": "source-native"}, Policy: wafpipeline.DefaultPolicy()}, Secrets: wafpipeline.PortSecrets{GateKey: "acme-fixture-key-not-production-32-bytes"}}
	s.Config.Policy.Gate.Enabled = true
	if err := runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host, path string
		want       int
	}{{"family.test", "/.well-known/acme-challenge/" + token, 200}, {"other.test", "/.well-known/acme-challenge/" + token, 401}, {"family.test", "/.well-known/acme-challenge/missing", 401}, {"family.test", "/.well-known/acme-challenge/%2e%2e/admin", 401}, {"family.test", "/.well-known/acme-challenge/a/" + token, 401}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+tc.path, nil)
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 5123}))
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
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: 5123}))
	w := httptest.NewRecorder()
	runtime.Serve(s, w, req)
	if w.Code != 503 {
		t.Fatal("ACME bypassed master access switch")
	}
}
