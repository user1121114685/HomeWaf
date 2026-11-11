package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model/wafenginmodel"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestLegacyRequestKeepsDetectionAndResponseSettings(t *testing.T) {
	old := global.CurrentSettings()
	defer global.UpdateSettings(func(cfg *global.RuntimeSettings) { *cfg = old })
	w, front, _ := legacyBodyFixture(t)
	global.UpdateSettings(func(cfg *global.RuntimeSettings) {
		cfg.GCONFIG_RECORD_HIDE_SERVER_HEADER = 1
		cfg.GCONFIG_BODY_DETECT_MODE = "observe"
	})
	r := httptest.NewRequest("POST", "http://body.test/notes", nil)
	r = r.WithContext(global.WithSettings(r.Context()))
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = "block" })
	check := func(req *http.Request) bool {
		log := &innerbean.WebLog{BODY: "; whoami", BodyDecoded: "; whoami"}
		return w.CheckRce(req, log, url.Values{}, &wafenginmodel.HostSafe{}, &wafenginmodel.HostSafe{}).IsBlock
	}
	if check(r) {
		t.Fatal("reload changed detection mode of existing request")
	}
	if !check(httptest.NewRequest("POST", "http://body.test/notes", nil)) {
		t.Fatal("new request ignored current mode")
	}
	entered, release := make(chan struct{}, 1), make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/waiting" {
			entered <- struct{}{}
			<-release
		}
		out.Header().Set("Server", "family-origin")
		out.WriteHeader(204)
	}))
	defer func() { close(release); origin.Close() }()
	front.Client().Timeout = 3 * time.Second
	w.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) { h.TargetHost = origin.URL })
	done := make(chan *http.Response, 1)
	failed := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("GET", front.URL+"/waiting", nil)
		req.Host = "body.test:5000"
		resp, err := front.Client().Do(req)
		if err != nil {
			failed <- err
			return
		}
		done <- resp
	}()
	select {
	case <-entered:
	case err := <-failed:
		t.Fatal(err)
	}
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_HIDE_SERVER_HEADER = 0 })
	release <- struct{}{}
	select {
	case resp := <-done:
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 204 || resp.Header.Get("Server") != "" {
			t.Fatal("in-flight response used a newer setting")
		}
	case err := <-failed:
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", front.URL+"/next", nil)
	req.Host = "body.test:5000"
	resp, err := front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Server") != "family-origin" {
		t.Fatal("new response ignored updated setting")
	}
}
