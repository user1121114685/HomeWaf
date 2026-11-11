// An isolated real-time soak runner. Only loopback listeners and a fresh
// temporary SQLite database are used; no production configuration is loaded.
package main

import (
	"HomeWaf/common/queue"
	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/service/waf_service"
	"HomeWaf/wafpipeline"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type target struct {
	port       uint16
	url, token string
	gate       bool
}

var buildRevision = "development"

func main() {
	duration := flag.Duration("duration", 75*time.Second, "wall-clock duration; maximum 3 minutes")
	portCount := flag.Int("ports", 100, "independent loopback ports")
	rate := flag.Int("rate", 64, "aggregate target requests per second")
	output := flag.String("output", "artifacts/pipeline-soak.jsonl", "append-only metrics evidence")
	flag.Parse()
	if *duration <= 0 || *duration > 3*time.Minute {
		fmt.Fprintln(os.Stderr, "duration must be greater than 0 and no more than 3 minutes")
		os.Exit(2)
	}
	if *portCount < 2 || *portCount > 200 || *rate < 16 {
		panic("invalid soak arguments")
	}
	releaseAwake, err := holdSystemAwake()
	must(err)
	defer releaseAwake()
	must(os.MkdirAll(filepath.Dir(*output), 0700))
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	defer file.Close()
	emit := func(v any) {
		must(json.NewEncoder(file).Encode(v))
		must(file.Sync())
		must(json.NewEncoder(os.Stdout).Encode(v))
	}
	dir, err := os.MkdirTemp(".cache", "pipeline-soak-")
	must(err)
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(dir, "soak.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(err)
	sqlDB, err := db.DB()
	must(err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	must(db.AutoMigrate(&model.PipelineState{}, &model.PipelineDailyMetrics{}, &model.PipelineLog{}))
	global.GWAF_LOCAL_DB = db
	global.GWAF_LOCAL_LOG_DB = db
	global.GQEQUE_LOG_DB = queue.NewQueue()
	punisher := wafpipeline.NewPunisher()
	state := &waf_service.WafPipelineStateService{Punisher: punisher}
	must(state.Load())
	gate := wafpipeline.NewGate(nil)
	rules := wafpipeline.NewRulesA(gate)
	engine := wafpipeline.DefaultRuleEngine()
	forward := &wafpipeline.Forwarder{}
	honey := wafpipeline.NewHoneyHandler()
	live := &wafpipeline.Runtime{Admission: wafpipeline.NewAdmission(256), State: punisher, Sink: punisher, Forward: forward.Serve, ForwardPrepare: forward.Prepare, MetricsForPort: state.MetricsForPort, OnResult: state.LogResult}
	live.RuleSteps = func(s *wafpipeline.Snapshot) []wafpipeline.Step { return engine.WithControls(s, rules.Steps(s)) }
	live.Respond = func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, w http.ResponseWriter, v wafpipeline.Verdict) bool {
		return honey.Respond(s, c, w, v) || gate.Respond(s, c, w, v)
	}
	var upstream, requests, successes, rejected, failures, logsWritten, updates atomic.Uint64
	back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		if r.Header.Get("X-HomeWaf-Token") != "" || strings.Contains(r.Header.Get("Cookie"), "hw_gate_") {
			failures.Add(1)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "soak-origin-ok")
	}))
	defer back.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logContext, stopLogs := context.WithCancel(context.Background())
	defer stopLogs()
	var logServices sync.WaitGroup
	startLogs := func(fn func(context.Context)) {
		logServices.Add(1)
		go func() { defer logServices.Done(); fn(logContext) }()
	}
	var services sync.WaitGroup
	startService := func(fn func(context.Context)) { services.Add(1); go func() { defer services.Done(); fn(ctx) }() }
	startService(rules.Run)
	startService(gate.Store.Run)
	startService(punisher.Run)
	startService(punisher.RunPersistence)
	startService(state.PruneLogs)
	startLogs(func(ctx context.Context) {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		flush := func() {
			rows := make([]*model.PipelineLog, 0, 256)
			for len(rows) < 256 {
				v, ok := global.GQEQUE_LOG_DB.Dequeue()
				if !ok {
					break
				}
				if row, ok := v.(*model.PipelineLog); ok {
					rows = append(rows, row)
				}
			}
			if len(rows) > 0 {
				if e := db.CreateInBatches(rows, 100).Error; e != nil {
					failures.Add(1)
				} else {
					logsWritten.Add(uint64(len(rows)))
				}
			}
		}
		for {
			select {
			case <-ctx.Done():
				for global.GQEQUE_LOG_DB.Stats()["queued"] > 0 {
					flush()
				}
				return
			case <-ticker.C:
				flush()
			}
		}
	})
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	startup := time.Now()
	fronts := make([]*httptest.Server, 0, *portCount)
	targets := make([]target, 0, *portCount)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("isolated-soak-account-fixture"), bcrypt.DefaultCost)
	must(err)
	randomSecret := func() string {
		var data [32]byte
		_, err := rand.Read(data[:])
		must(err)
		return base64.RawURLEncoding.EncodeToString(data[:])
	}
	for i := 0; i < *portCount; i++ {
		front := httptest.NewUnstartedServer(nil)
		port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
		front.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := wafpipeline.Ports.Get(port)
			if !ok {
				http.Error(w, "unavailable", 503)
				return
			}
			live.Serve(s, w, r)
		})
		policy := wafpipeline.DefaultPolicy()
		policy.Network.TrustedProxies = []string{"127.0.0.1/32"}
		policy.RuleSet.QPS.PerIP = 10000
		policy.RuleSet.QPS.PerPort = 20000
		policy.RuleSet.QPS.Burst = 10000
		policy.Gate.Enabled = i%2 == 0
		policy.Gate.TokenHeader = "X-HomeWaf-Token"
		policy.Honeypot.Enabled = true
		token := randomSecret()
		sum := sha256.Sum256([]byte(token))
		s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: port, Name: "soak", Protocol: "http", Upstream: back.URL, ListenEnabled: true, Policy: policy}, Secrets: wafpipeline.PortSecrets{GateKey: randomSecret(), ServiceTokenHashes: []string{hex.EncodeToString(sum[:])}}}
		gate.Store.ReplaceAccounts(wafpipeline.PortID(port), []wafpipeline.GateAccount{{PortID: wafpipeline.PortID(port), Username: "owner", PasswordHash: string(passwordHash), Version: "soak-fixture", Enabled: true}})
		must(live.Prepare(s))
		must(wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return s, nil }))
		front.Start()
		fronts = append(fronts, front)
		targets = append(targets, target{port, front.URL, token, policy.Gate.Enabled})
	}
	runtime.GC()
	var ready runtime.MemStats
	runtime.ReadMemStats(&ready)
	started := time.Now()
	emit(map[string]any{"kind": "start", "revision": buildRevision, "started_at": started.UTC(), "requested_seconds": duration.Seconds(), "ports": *portCount, "rate": *rate, "startup_ms": time.Since(startup).Milliseconds(), "startup_heap_delta": int64(ready.HeapAlloc) - int64(before.HeapAlloc), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "database": dir})
	transport := &http.Transport{MaxIdleConns: 400, MaxIdleConnsPerHost: 4, IdleConnTimeout: 90 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var workers sync.WaitGroup
	traffic, stopTraffic := context.WithCancel(context.Background())
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			tick := time.NewTicker(time.Second * 16 / time.Duration(*rate))
			defer tick.Stop()
			for {
				select {
				case <-traffic.Done():
					return
				case <-tick.C:
				}
				n := requests.Add(1)
				target := targets[int(n)%len(targets)]
				path, method, body, ip := "/album?name=family", "GET", "", "198.51.100.10"
				want := 200
				if n%11 == 0 {
					path, method, body = "/upload", "POST", string([]byte{0xac, 0xed, 0x00, 0x05})
					ip = fmt.Sprintf("203.%d.%d.%d", (n>>16)%200+1, (n>>8)%256, n%250+1)
					want = 403
				}
				if n%17 == 0 && target.gate {
					want = 401
				}
				req, e := http.NewRequest(method, target.url+path, strings.NewReader(body))
				if e != nil {
					failures.Add(1)
					continue
				}
				req.Header.Set("X-Forwarded-For", ip)
				req.Header.Set("User-Agent", "Mozilla/5.0 soak")
				if want != 401 {
					req.Header.Set("X-HomeWaf-Token", target.token)
				}
				if body != "" {
					req.Header.Set("Content-Type", "application/x-java-serialized-object")
				}
				res, e := client.Do(req)
				if e != nil {
					failures.Add(1)
					continue
				}
				payload, e := io.ReadAll(io.LimitReader(res.Body, 65536))
				res.Body.Close()
				if e != nil || (res.StatusCode != want && !(want == 403 && res.StatusCode == 429)) || (want == 200 && string(payload) != "soak-origin-ok") {
					if failures.Add(1) <= 5 {
						fmt.Fprintf(os.Stderr, "request=%d port=%d status=%d want=%d error=%v\n", n, target.port, res.StatusCode, want, e)
					}
				} else if want == 200 {
					successes.Add(1)
				} else {
					rejected.Add(1)
				}
			}
		}()
	}
	lastSample := started
	maxGap := time.Duration(0)
	sample := func(kind string) {
		gap := time.Since(lastSample)
		lastSample = time.Now()
		if gap > maxGap {
			maxGap = gap
		}
		if gap > 2*time.Minute {
			failures.Add(1)
		}
		diskBytes := int64(0)
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if info, err := os.Stat(filepath.Join(dir, "soak.db") + suffix); err == nil {
				diskBytes += info.Size()
			}
		}
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		emit(map[string]any{"kind": kind, "database_bytes": diskBytes, "max_sample_gap_seconds": maxGap.Seconds(), "gc_cycles": m.NumGC, "at": time.Now().UTC(), "elapsed_seconds": time.Since(started).Seconds(), "heap_alloc": m.HeapAlloc, "heap_objects": m.HeapObjects, "sys": m.Sys, "goroutines": runtime.NumGoroutine(), "requests": requests.Load(), "passed": successes.Load(), "rejected": rejected.Load(), "failures": failures.Load(), "upstream": upstream.Load(), "logs_written": logsWritten.Load(), "hot_updates": updates.Load(), "punishment": punisher.Stats(), "log_queue": global.GQEQUE_LOG_DB.Stats(), "admission": live.Admission.Stats(), "metrics_persist_errors": state.MetricsPersistErrors.Load()})
	}
	timer := time.NewTimer(*duration)
	ticker := time.NewTicker(time.Minute)
	running := true
	for running {
		select {
		case <-timer.C:
			running = false
		case <-ticker.C:
			index := int(updates.Load()) % len(targets)
			old, _ := wafpipeline.Ports.Get(targets[index].port)
			next := &wafpipeline.Snapshot{Config: wafpipeline.CloneConfig(old.Config), Secrets: old.Secrets}
			next.Config.Name = "soak-revision-" + strconv.FormatUint(updates.Add(1), 10)
			next.Config.Revision++
			must(live.Prepare(next))
			must(wafpipeline.Ports.Publish(next.Config.Port, func() (*wafpipeline.Snapshot, error) { return next, nil }))
			wafpipeline.CloseRetiredProxy(old, next)
			// Keep the SQLite trace fixture bounded while exercising real inserts.
			must(db.Exec("DELETE FROM pipeline_log WHERE time_millis < ?", time.Now().Add(-2*time.Minute).UnixMilli()).Error)
			sample("sample")
			if failures.Load() > 0 {
				running = false
			}
		}
	}
	ticker.Stop()
	stopTraffic()
	workers.Wait()
	transport.CloseIdleConnections()
	for _, front := range fronts {
		front.Close()
	}
	cancel()
	services.Wait()
	must(state.PersistMetrics())
	must(punisher.Flush())
	stopLogs()
	logServices.Wait()
	for _, target := range targets {
		s, _ := wafpipeline.Ports.Get(target.port)
		wafpipeline.CloseRetiredProxy(s, nil)
		wafpipeline.Ports.Remove(target.port)
	}
	sample("finish")
	if failures.Load() > 0 || punisher.PersistErrors.Load() > 0 || state.MetricsPersistErrors.Load() > 0 || time.Since(started) < *duration || upstream.Load() != successes.Load() || global.GQEQUE_LOG_DB.Stats()["queued"] != 0 || punisher.Stats()["queued"] != 0 {
		panic("soak acceptance failed; inspect evidence")
	}
}
