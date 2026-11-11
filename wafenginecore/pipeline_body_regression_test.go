package wafenginecore

import (
	"HomeWaf/cache"
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafenginecore/accessgate"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func legacyBodyFixture(t *testing.T) (*WafEngine, *httptest.Server, *atomic.Int64) {
	t.Helper()
	oldCache, oldLogs, oldMessages := global.GCACHE_WAFCACHE, global.GQEQUE_LOG_DB, global.GQEQUE_MESSAGE_DB
	oldCode, oldName, oldType := global.GWAF_GLOBAL_HOST_CODE, global.GWAF_GLOBAL_HOST_NAME, global.CurrentSettings().GWAF_RUNTIME_RECORD_LOG_TYPE
	oldLimit, oldOWASP, oldMode, oldDLP := global.CurrentSettings().GCONFIG_RECORD_MAX_BODY_LENGTH, global.CurrentSettings().GCONFIG_RECORD_ENABLE_OWASP, global.CurrentSettings().GCONFIG_BODY_DETECT_MODE, global.CurrentSettings().GCONFIG_RECORD_LOG_DESENSITIZE
	oldAccess := accessgate.Get()
	t.Cleanup(func() {
		global.GCACHE_WAFCACHE = oldCache
		global.GQEQUE_LOG_DB = oldLogs
		global.GQEQUE_MESSAGE_DB = oldMessages
		global.GWAF_GLOBAL_HOST_CODE = oldCode
		global.GWAF_GLOBAL_HOST_NAME = oldName
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = oldType })
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_MAX_BODY_LENGTH = oldLimit })
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_ENABLE_OWASP = oldOWASP })
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = oldMode })
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_LOG_DESENSITIZE = oldDLP })
		accessgate.SetConfig(oldAccess)
	})
	memoryCache := cache.InitWafCache()
	t.Cleanup(memoryCache.Close)
	global.GCACHE_WAFCACHE = memoryCache
	global.GQEQUE_LOG_DB = queue.NewQueue()
	global.GQEQUE_MESSAGE_DB = queue.NewQueue()
	global.GWAF_GLOBAL_HOST_CODE = "body-global"
	global.GWAF_GLOBAL_HOST_NAME = "body-global:0"
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "abnormal" })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_MAX_BODY_LENGTH = 32 })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_ENABLE_OWASP = 0 })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = "block" })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_LOG_DESENSITIZE = 0 })
	accessgate.SetConfig(nil)
	hits := &atomic.Int64{}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.Copy(w, r.Body)
	}))
	t.Cleanup(origin.Close)
	w := &WafEngine{ServerOnline: wafenginmodel.NewSafeServerMap()}
	w.InitRouting()
	h := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "body-host", Host: "body.test", Port: 5000, GUARD_STATUS: 1, IPSourceMode: "nic", DEFENSE_JSON: `{"sqli":1,"xss":1,"scan":1,"rce":1,"traversal":1}`, ResponseTimeOut: 10}, TargetHost: origin.URL}
	w.withWriteTable(func(rt *routingTable) {
		rt.HostCode["body-host"] = "body.test:5000"
		rt.HostTarget["body.test:5000"] = h
		rt.HostCode[global.GWAF_GLOBAL_HOST_CODE] = global.GWAF_GLOBAL_HOST_NAME
		rt.HostTarget[global.GWAF_GLOBAL_HOST_NAME] = &wafenginmodel.HostSafe{}
	})
	front := httptest.NewServer(w)
	t.Cleanup(front.Close)
	t.Cleanup(func() {
		w.TransportMux.Lock()
		defer w.TransportMux.Unlock()
		for _, tr := range w.TransportPool {
			tr.CloseIdleConnections()
		}
	})
	return w, front, hits
}
func TestLegacyChunkedAndCompressedDetectionIndependentOfLogLength(t *testing.T) {
	_, front, hits := legacyBodyFixture(t)
	body := []byte(`{"q":"' OR 1=1 --","padding":"` + strings.Repeat("a", 5000) + `"}`)
	for _, encoding := range []string{"identity", "gzip"} {
		for _, chunked := range []bool{false, true} {
			wire := body
			if encoding == "gzip" {
				var buf bytes.Buffer
				w := gzip.NewWriter(&buf)
				w.Write(body)
				w.Close()
				wire = buf.Bytes()
			}
			req, _ := http.NewRequest("POST", front.URL+"/query", bytes.NewReader(wire))
			req.Host = "body.test:5000"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", encoding)
			if chunked {
				req.ContentLength = -1
				req.Body = io.NopCloser(bytes.NewReader(wire))
			}
			resp, err := front.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 403 || hits.Load() != 0 {
				t.Fatalf("body blind spot encoding=%s chunked=%v status=%d upstream=%d", encoding, chunked, resp.StatusCode, hits.Load())
			}
		}
	}
	raw := bytes.Repeat([]byte("b"), 2<<20)
	req, _ := http.NewRequest("POST", front.URL+"/backup", bytes.NewReader(raw))
	req.Host = "body.test:5000"
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || hits.Load() != 1 || !bytes.Equal(got, raw) {
		t.Fatalf("normal upload damaged status=%d bytes=%d/%d upstream=%d err=%v", resp.StatusCode, len(got), len(raw), hits.Load(), err)
	}
}
func TestLegacyRCEBodyObserveCannotBlock(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_BODY_DETECT_MODE
	defer func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = old })
	}()
	w := &WafEngine{}
	h := &wafenginmodel.HostSafe{}
	r := httptest.NewRequest("POST", "http://body.test/notes", nil)
	for _, mode := range []string{"off", "observe", "block"} {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = mode })
		log := &innerbean.WebLog{BODY: "; whoami", BodyDecoded: "; whoami"}
		got := w.CheckRce(r, log, url.Values{}, h, h)
		if got.IsBlock != (mode == "block") {
			t.Fatalf("body mode=%s block=%v", mode, got.IsBlock)
		}
	}
}

func TestLegacyHTTPAuthProtectsStaticBusinessExit(t *testing.T) {
	w, front, hits := legacyBodyFixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "private.txt"), []byte("private-family-file"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(model.StaticSiteConfig{IsEnableStaticSite: 1, StaticSitePath: dir, StaticSitePrefix: "/"})
	w.UpdateHost("body-host", func(h *wafenginmodel.HostSafe) {
		h.Host.StaticSiteJSON = string(raw)
		h.Host.IsEnableHttpAuthBase = 1
		h.Host.HttpAuthBaseType = model.HttpAuthTypeAuthorization
	})
	req, _ := http.NewRequest("GET", front.URL+"/private.txt", nil)
	req.Host = "body.test:5000"
	response, err := front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 401 || bytes.Contains(body, []byte("private-family-file")) || hits.Load() != 0 {
		t.Fatalf("static resource bypass: status=%d upstream=%d", response.StatusCode, hits.Load())
	}
}
