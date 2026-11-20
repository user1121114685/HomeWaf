package wafpipeline

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGateLogoIsEmbeddedAndNeverExposesUpstream(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	logo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	if err := validateGateLogo(logo); err != nil {
		t.Fatal(err)
	}
	var snap *Snapshot
	gate := NewGate(nil)
	rules := NewRulesA(gate)
	hits := 0
	runtime := &Runtime{RuleSteps: rules.Steps, Respond: gate.Respond, Forward: func(s *Snapshot, c *RequestContext, w http.ResponseWriter) { hits++; w.WriteHeader(502) }}
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { runtime.Serve(snap, w, r) }))
	defer front.Close()
	snap = &Snapshot{Config: PortConfig{Schema: 1, Port: uint16(front.Listener.Addr().(*net.TCPAddr).Port), Protocol: "http", Upstream: "http://do-not-expose.internal:7890", Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: randomToken()}}
	snap.Config.Policy.Gate.Enabled = true
	snap.Config.Policy.Gate.LogoData = logo
	if err := runtime.Prepare(snap); err != nil {
		t.Fatal(err)
	}
	front.Start()
	res, err := front.Client().Get(front.URL + snap.Config.Policy.Gate.Prefix + "/login")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(body, []byte(logo)) || !bytes.Contains(body, []byte("<img")) || bytes.Contains(body, []byte("do-not-expose")) || hits != 0 || !strings.Contains(res.Header.Get("Content-Security-Policy"), "img-src data:") {
		t.Fatal("logo page absent, external or exposed upstream")
	}
	for _, invalid := range []string{"https://external.test/logo.png", "data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64,invalid", strings.Repeat("x", 90001)} {
		if validateGateLogo(invalid) == nil {
			t.Fatal("unsafe logo accepted")
		}
	}
}
