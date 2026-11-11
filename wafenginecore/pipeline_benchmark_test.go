package wafenginecore

import (
	"HomeWaf/cache"
	"HomeWaf/common/queue"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafpipeline"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func phase9LegacyEngine(t testing.TB) (*WafEngine, *httptest.Server, *atomic.Int64) {
	t.Helper()
	memoryCache := cache.InitWafCache()
	t.Cleanup(memoryCache.Close)
	global.GCACHE_WAFCACHE = memoryCache
	global.GQEQUE_LOG_DB = queue.NewQueue()
	global.GQEQUE_MESSAGE_DB = queue.NewQueue()
	global.GWAF_GLOBAL_HOST_CODE = "p0-global"
	global.GWAF_GLOBAL_HOST_NAME = "p0-global:0"
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GWAF_RUNTIME_RECORD_LOG_TYPE = "abnormal" })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_MAX_BODY_LENGTH = 1024 * 1024 })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_ENABLE_OWASP = 0 })
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_BODY_DETECT_MODE = "block" })
	// This fixture has no production DLP initialization; only synthetic data is used.
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_LOG_DESENSITIZE = 0 })
	accessgate.SetConfig(nil)
	hits := &atomic.Int64{}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		data, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		if len(data) > 0 {
			_, _ = w.Write(data)
		} else {
			_, _ = io.WriteString(w, "audit-origin-ok")
		}
	}))
	t.Cleanup(origin.Close)
	w := &WafEngine{ServerOnline: wafenginmodel.NewSafeServerMap()}
	w.InitRouting()
	hs := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "p0-host", Host: "audit.test", Port: 5000, GUARD_STATUS: 1, IPSourceMode: "nic", DEFENSE_JSON: `{"sqli":1,"xss":1,"scan":1,"rce":1,"traversal":1}`, IsEnableResponseBuffering: 0, ResponseTimeOut: 10}, TargetHost: origin.URL}
	w.withWriteTable(func(rt *routingTable) {
		rt.HostCode["p0-host"] = "audit.test:5000"
		rt.HostTarget["audit.test:5000"] = hs
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
func runPhase9HTTPBenchmark(t *testing.T, front *httptest.Server, hits *atomic.Int64, label string) {
	const n = 12000
	for _, workers := range []int{1, 16} {
		tr := &http.Transport{MaxIdleConns: 64, MaxIdleConnsPerHost: 64}
		client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
		request := func() (time.Duration, error) {
			req, _ := http.NewRequest("GET", front.URL+"/album?name=family", nil)
			req.Host = "audit.test:5000"
			req.Header.Set("User-Agent", "Mozilla/5.0 audit")
			begin := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				return 0, err
			}
			data, e := io.ReadAll(resp.Body)
			resp.Body.Close()
			if e != nil {
				return 0, e
			}
			if resp.StatusCode != 200 || !bytes.Equal(data, []byte("audit-origin-ok")) {
				return 0, fmt.Errorf("status=%d body=%q", resp.StatusCode, data)
			}
			return time.Since(begin), nil
		}
		for i := 0; i < 200; i++ {
			if _, err := request(); err != nil {
				t.Fatal(err)
			}
		}
		before := hits.Load()
		lat := make([]time.Duration, n)
		var next atomic.Int64
		var failed atomic.Int64
		var wg sync.WaitGroup
		var memoryBefore, memoryAfter runtime.MemStats
		runtime.ReadMemStats(&memoryBefore)
		start := time.Now()
		for k := 0; k < workers; k++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					i := int(next.Add(1)) - 1
					if i >= n {
						return
					}
					d, e := request()
					if e != nil {
						failed.Add(1)
					}
					lat[i] = d
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)
		runtime.ReadMemStats(&memoryAfter)
		sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
		row := map[string]interface{}{"requests": n, "concurrency": workers, "seconds": elapsed.Seconds(), "qps": float64(n) / elapsed.Seconds(), "p50_ms": float64(lat[n/2]) / float64(time.Millisecond), "p99_ms": float64(lat[(n*99+99)/100-1]) / float64(time.Millisecond), "failures": failed.Load(), "upstream_requests": hits.Load() - before, "allocated_bytes_per_request": float64(memoryAfter.TotalAlloc-memoryBefore.TotalAlloc) / n, "allocations_per_request": float64(memoryAfter.Mallocs-memoryBefore.Mallocs) / n, "gc_cycles": memoryAfter.NumGC - memoryBefore.NumGC}
		b, _ := json.Marshal(row)
		t.Log("PHASE9_BASELINE " + label + " " + string(b))
		tr.CloseIdleConnections()
		if failed.Load() != 0 || hits.Load()-before != n {
			t.Fatal("baseline requests failed")
		}
	}
}

// Explicit performance run, separate from ordinary correctness tests.
func TestPhase9FullChainBenchmark(t *testing.T) {
	if os.Getenv("HOMEWAF_PIPELINE_PERF") != "1" {
		t.Skip("set HOMEWAF_PIPELINE_PERF=1 for the phase-0 matched performance run")
	}
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
	t.Run("legacy", func(t *testing.T) {
		_, front, hits := phase9LegacyEngine(t)
		runPhase9HTTPBenchmark(t, front, hits, "legacy")
	})
	t.Run("pipeline", func(t *testing.T) {
		hits := &atomic.Int64{}
		back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, "audit-origin-ok")
		}))
		defer back.Close()
		gate := wafpipeline.NewGate(nil)
		rules := wafpipeline.NewRulesA(gate)
		engine := wafpipeline.DefaultRuleEngine()
		forward := &wafpipeline.Forwarder{}
		punisher := wafpipeline.NewPunisher()
		live := &wafpipeline.Runtime{Admission: wafpipeline.NewAdmission(256), State: punisher, Sink: punisher, Forward: forward.Serve, ForwardPrepare: forward.Prepare, Respond: gate.Respond}
		live.RuleSteps = func(s *wafpipeline.Snapshot) []wafpipeline.Step { return engine.WithControls(s, rules.Steps(s)) }
		front := httptest.NewUnstartedServer(nil)
		defer front.Close()
		port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
		policy := wafpipeline.DefaultPolicy()
		policy.RuleSet.QPS.Mode = wafpipeline.ModeOff
		policy.RuleSet.CC.Mode = wafpipeline.ModeOff
		snap := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: port, Name: "perf", Protocol: "http", Upstream: back.URL, ListenEnabled: true, Policy: policy}, Secrets: wafpipeline.PortSecrets{GateKey: strings.Repeat("x", 48)}}
		if err := live.Prepare(snap); err != nil {
			t.Fatal(err)
		}
		if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return snap, nil }); err != nil {
			t.Fatal(err)
		}
		defer wafpipeline.Ports.Remove(port)
		defer wafpipeline.CloseRetiredProxy(snap, nil)
		front.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { live.Serve(snap, w, r) })
		front.Start()
		runPhase9HTTPBenchmark(t, front, hits, "pipeline")
	})
}
