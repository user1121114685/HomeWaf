package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIncidentReplay123Requests(t *testing.T) {
	raw, err := os.ReadFile("../test/fixtures/waf-boundary-20261113.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Requests []struct {
			Sequence                 int
			Method, Host, Path, Body string
			Headers                  http.Header
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Requests) != 123 {
		t.Fatalf("fixture has %d events", len(fixture.Requests))
	}
	waf, _, hits := legacyBodyFixture(t)
	oldHeader := global.CurrentSettings().GCONFIG_RECORD_PROXY_HEADER
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = "X-Forwarded-For,X-Real-IP" })
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = oldHeader })
	})
	waf.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) { h.Host.IPMode = "proxy"; h.Host.IPSourceMode = "" })
	setBlockList(waf, "body-host", []model.IPBlockList{{Ip: "198.51.100.179"}})
	messageQueue := global.GQEQUE_MESSAGE_DB
	notifications := 0
	for _, event := range fixture.Requests {
		t.Run(fmt.Sprintf("event_%03d", event.Sequence), func(t *testing.T) {
			r := httptest.NewRequest(event.Method, "https://body.test:5000"+event.Path, strings.NewReader(event.Body))
			r.Host = strings.ReplaceAll(event.Host, "cqh.example.test:15000", "body.test:5000")
			r.RemoteAddr = "198.51.100.179:54321"
			r.Header = event.Headers.Clone()
			w := httptest.NewRecorder()
			waf.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("bypassed policy: host=%s path=%s status=%d", r.Host, event.Path, w.Code)
			}
			entry, ok := global.GQEQUE_LOG_DB.Dequeue()
			if !ok {
				t.Fatal("denied request has no audit event")
			}
			log, ok := entry.(*innerbean.WebLog)
			if !ok {
				t.Fatalf("unexpected log type %T", entry)
			}
			if log.SRC_IP != "198.51.100.179" || log.NetSrcIp != "198.51.100.179" || log.STATUS_CODE != 403 {
				t.Fatalf("incorrect logged identity/status: %+v", log)
			}
			if log.HOST_CODE != "" && log.RULE != "不支持的协议升级" {
				notifications++
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("blocked replay reached backend %d times", hits.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for messageQueue.Size() < notifications && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if messageQueue.Size() < notifications {
		t.Fatal("block notifications did not finish before fixture cleanup")
	}
}

func TestIncidentUnmatchedHostLogCannotBeSpoofedOrSuppressed(t *testing.T) {
	waf, _, _ := legacyBodyFixture(t)
	old := global.CurrentSettings().GCONFIG_RECORD_PROXY_HEADER
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = "X-Forwarded-For" })
	t.Cleanup(func() { global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_RECORD_PROXY_HEADER = old }) })
	for _, value := range []string{"127.0.0.1", "not-an-ip", "1.2.3.4, 5.6.7.8"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:15000/", nil)
		r.RemoteAddr = "198.51.100.179:54321"
		r.Header.Set("X-Forwarded-For", value)
		w := httptest.NewRecorder()
		waf.ServeHTTP(w, r)
		entry, ok := global.GQEQUE_LOG_DB.Dequeue()
		if !ok {
			t.Fatalf("spoofed header suppressed audit: %q", value)
		}
		log := entry.(*innerbean.WebLog)
		if w.Code != 403 || log.SRC_IP != "198.51.100.179" || log.NetSrcIp != log.SRC_IP {
			t.Fatalf("wrong rejection identity: %+v", log)
		}
	}
}

func TestIncidentLegacyH2CBlockedWithDetectionDisabled(t *testing.T) {
	waf, _, hits := legacyBodyFixture(t)
	waf.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) { h.Host.GUARD_STATUS = 0 })
	r := httptest.NewRequest("GET", "http://body.test:5000/api/health", nil)
	r.Header.Set("Upgrade", "h2c")
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("HTTP2-Settings", "AAMAAABkAAQAAP__")
	w := httptest.NewRecorder()
	waf.ServeHTTP(w, r)
	if w.Code != 403 || hits.Load() != 0 {
		t.Fatalf("unchecked tunnel reached backend: %d, %d", w.Code, hits.Load())
	}
	entry, ok := global.GQEQUE_LOG_DB.Dequeue()
	if !ok || entry.(*innerbean.WebLog).STATUS_CODE != 403 {
		t.Fatal("upgrade rejection missing audit")
	}
}
