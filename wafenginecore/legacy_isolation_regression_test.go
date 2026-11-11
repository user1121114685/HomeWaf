package wafenginecore

import (
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestLegacyListenerRejectsHostPortPolicySwitch(t *testing.T) {
	w, _, hits := legacyBodyFixture(t)
	w.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) { h.Host.START_STATUS = 1 })
	w.withWriteTable(func(rt *routingTable) {
		open := *rt.HostTarget["body.test:5000"]
		open.Host.Code = "open-host"
		open.Host.Port = 5001
		open.Host.START_STATUS = 0
		rt.HostTarget["body.test:5001"] = &open
		rt.HostCode["open-host"] = "body.test:5001"
		rt.HostTargetNoPort["body.test"] = "body.test:5001"
		rt.HostTargetNoPort["*"] = "body.test:5001"
	})
	closed := httptest.NewServer(w.boundPortHandler("5000"))
	defer closed.Close()
	open := httptest.NewServer(w.boundPortHandler("5001"))
	defer open.Close()
	for _, tc := range []struct {
		url, host string
		status    int
		hits      int64
	}{{closed.URL, "body.test:5001", 503, 0}, {closed.URL, "body.test", 503, 0}, {open.URL, "body.test:5000", 200, 1}} {
		req, _ := http.NewRequest("GET", tc.url+"/", nil)
		req.Host = tc.host
		res, err := closed.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != tc.status || hits.Load() != tc.hits {
			t.Fatalf("Host=%s status=%d upstream=%d", tc.host, res.StatusCode, hits.Load())
		}
	}
}

func TestLegacyResponseUsesRequestSnapshotDuringHotUpdate(t *testing.T) {
	w, front, _ := legacyBodyFixture(t)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	back := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release; rw.WriteHeader(200) }))
	t.Cleanup(back.Close)
	header := func(value string) string {
		raw, _ := json.Marshal(model.CustomResponseHeadersConfig{IsEnableCustomHeaders: 1, Headers: []model.CustomHeaderItem{{HeaderName: "X-Snapshot", HeaderValue: value}}})
		return string(raw)
	}
	w.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) {
		h.TargetHost = back.URL
		h.Host.CustomResponseHeadersJSON = header("original")
	})
	type outcome struct {
		res *http.Response
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		req, _ := http.NewRequest("GET", front.URL+"/", nil)
		req.Host = "body.test:5000"
		res, err := front.Client().Do(req)
		done <- outcome{res, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach upstream")
	}
	w.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) { h.Host.CustomResponseHeadersJSON = header("replacement") })
	unblock()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		defer result.res.Body.Close()
		if result.res.StatusCode != 200 || result.res.Header.Get("X-Snapshot") != "original" {
			t.Fatalf("in-flight response used another revision: %d %v", result.res.StatusCode, result.res.Header)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("response did not finish")
	}
}
