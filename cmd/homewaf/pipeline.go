package main

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/protection/wafenginecore"
	"HomeWaf/internal/protection/wafenginecore/ipset"
	"HomeWaf/internal/protection/wafhostguard"
	"HomeWaf/internal/protection/wafpipeline"
	"HomeWaf/internal/services/waf_service"
	"HomeWaf/plugin/manager"
	"context"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var stopPortPipeline context.CancelFunc
var finishPortPipeline func()

func shutdownPortPipeline() {
	wafpipeline.Listeners.StopAll()
	if stopPortPipeline != nil {
		stopPortPipeline()
	}
	if finishPortPipeline != nil {
		finishPortPipeline()
	}
}

func initializePortPipeline() {
	waf_service.WafPipelineServiceApp.OnChange = func(old, next *wafpipeline.Snapshot) error {
		if err := wafpipeline.Listeners.Apply(old, next); err != nil {
			return err
		}
		wafpipeline.CloseRetiredProxy(old, next)
		return nil
	}
	if raw := os.Getenv("HOMEWAF_PIPELINE_MAX_PORTS"); raw != "" {
		if limit, err := strconv.Atoi(raw); err == nil && limit >= 1 && limit <= 2000 && len(wafpipeline.Ports.Ports()) == 0 {
			wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](limit)
		} else {
			zlog.Warn("HOMEWAF_PIPELINE_MAX_PORTS应为1..2000，仅可在未加载端口时设置；保留默认上限")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopPortPipeline = cancel
	wafpipeline.Live.Admission = wafpipeline.NewAdmission(256)
	wafpipeline.Live.CertificateChallenge = wafenginecore.PipelineCertificateChallenge
	pluginLimit := 32
	if raw := os.Getenv("HOMEWAF_PIPELINE_PLUGIN_PROCESSES"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value <= 256 {
			pluginLimit = value
		} else {
			zlog.Warn("HOMEWAF_PIPELINE_PLUGIN_PROCESSES应为1..256，保留默认32")
		}
	}
	pluginPool := manager.NewPipelinePluginPool(pluginLimit)
	wafpipeline.Live.PluginPrepare = pluginPool.Prepare
	wafpipeline.Live.PluginStats = pluginPool.Stats
	stateService := waf_service.WafPipelineStateServiceApp
	if err := stateService.LoadMetrics(); err != nil {
		zlog.Error("端口每日计数加载失败", err.Error())
	}
	wafpipeline.Live.MetricsForPort = stateService.MetricsForPort
	if err := stateService.Load(); err != nil {
		zlog.Error("端口处罚状态加载失败", err.Error())
	}
	punisher := stateService.Punisher
	wafpipeline.Listeners.SetEvents(punisher)
	wafpipeline.Live.Sink = punisher
	wafpipeline.Live.State = punisher
	defenseService := waf_service.WafPipelineDefenseServiceApp
	defenseService.Engine.RequireRegistered = true
	wafpipeline.Live.Defense = defenseService.Engine
	wafpipeline.Live.SilentDrop = waf_service.DropPipelineConnection
	if err := waf_service.WafPipelineHoneypotServiceApp.Load(); err != nil {
		zlog.Error("统一蜜罐设置加载失败", err.Error())
	}
	wafpipeline.Live.OnResult = func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
		stateService.LogResult(s, c, result)
		wafenginecore.RecordPipelineWebLog(s, c, result)
	}
	honey := wafpipeline.NewHoneyHandler()
	gateStore := waf_service.WafPipelineGateServiceApp.Store
	if err := waf_service.WafPipelineGateServiceApp.Load(); err != nil {
		zlog.Error("端口登录门加载失败", err.Error())
	}
	gate := wafpipeline.NewGate(gateStore)
	gate.OnAccessEvent = func(c *wafpipeline.RequestContext, event, account string) {
		outcome := model.AccessAuditOK
		if event == model.AccessEventBadReturnTo {
			outcome = model.AccessAuditFail
		}
		waf_service.WafSecurityAuditServiceApp.Write(waf_service.AuditEntry{
			Event: event, AccountName: account, Host: c.Request.Host, HostCode: c.PortID,
			URL: c.Request.URL.EscapedPath(), ClientIP: c.ClientIP.String(), Result: outcome,
		})
	}
	defenseService.OwnsRuntime = waf_service.WafPipelineGateServiceApp.OwnsRuntime
	defenseService.Owner = waf_service.WafPipelineGateServiceApp.RuntimeOwner()
	if err := defenseService.LoadHistory(); err != nil {
		zlog.Error("成功来源记录加载失败", err.Error())
	}
	rules := wafpipeline.NewRulesA(gate)
	bots := wafpipeline.NewBOTCache()
	rules.BotIdentity = bots.Lookup
	detectors := wafpipeline.DefaultRuleEngine()
	crs := &wafpipeline.CRSProvider{Root: filepath.Join(utils.GetCurrentDir(), "data", "owasp")}
	wafpipeline.Live.RulePrepare = func(s *wafpipeline.Snapshot) error {
		if s.Config.Policy.Gate.Enabled && !waf_service.WafPipelineGateServiceApp.Ready() {
			return errors.New("登录门持久化服务未就绪")
		}
		return crs.Prepare(s)
	}
	forwarder := &wafpipeline.Forwarder{StaticHandler: wafenginecore.PipelineStaticHandler, RequestHeaders: wafenginecore.PipelineRequestHeaders, ResponseHeaders: wafenginecore.PipelineResponseHeaders}
	forwarder.DecodeResponse = wafenginecore.PipelineDecodeResponse
	forwarder.EncodeResponse = wafenginecore.PipelineEncodeResponse
	forwarder.SecureCookie = wafenginecore.PipelineSecureCookie
	tamperService := waf_service.WafPipelineTamperServiceApp
	forwarder.CaptureTamper = tamperService.Capture
	wafpipeline.Live.ForwardPrepare = forwarder.Prepare
	wafpipeline.Live.Forward = forwarder.Serve
	wafpipeline.Live.RuleSteps = func(s *wafpipeline.Snapshot) []wafpipeline.Step {
		controls := rules.Steps(s)
		withTrap := make([]wafpipeline.Step, 0, len(controls)+1)
		for _, step := range controls {
			if step.Name == "auth_gate" {
				withTrap = append(withTrap, wafpipeline.TrapStep(s))
			}
			withTrap = append(withTrap, step)
		}
		steps := detectors.WithControls(s, withTrap)
		if s.Config.Policy.RuleSet.OWASP != wafpipeline.ModeOff {
			steps = append(steps, crs.Step(s))
		}
		return steps
	}
	wafpipeline.Live.Respond = func(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext, w http.ResponseWriter, v wafpipeline.Verdict) bool {
		return honey.Respond(s, c, w, v) || gate.Respond(s, c, w, v)
	}
	go rules.Run(ctx)
	tamperDone := make(chan struct{})
	go func() { defer close(tamperDone); tamperService.Run(ctx) }()
	go bots.Run(ctx)
	done := make(chan struct{})
	go func() { defer close(done); gateStore.Run(ctx) }()
	stateDone := make(chan struct{})
	go func() { defer close(stateDone); punisher.Run(ctx) }()
	go punisher.RunPersistence(ctx)
	go stateService.PruneLogs(ctx)
	defenseDone := make(chan struct{})
	go func() { defer close(defenseDone); defenseService.Run(ctx) }()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		_ = waf_service.PipelineFlowDropper.Reconcile()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = waf_service.PipelineFlowDropper.Reconcile()
			}
		}
	}()
	finishPortPipeline = func() {
		<-defenseDone
		<-tamperDone
		<-done
		<-stateDone
		if err := stateService.PersistMetrics(); err != nil {
			zlog.Error("端口每日计数保存失败", err.Error())
		}
		if err := punisher.Flush(); err != nil {
			zlog.Error("端口处罚持久化失败", err.Error())
		}
		if err := waf_service.WafPipelineGateServiceApp.Close(); err != nil {
			zlog.Error("端口会话持久化失败", err.Error())
		}
	}
	wafpipeline.Live.Geo = wafpipeline.NewGeoCache(func(ip netip.Addr) wafpipeline.GeoResult {
		values, ok := utils.GetCountryEx(ip.String())
		if len(values) < 5 {
			return wafpipeline.GeoResult{}
		}
		return wafpipeline.GeoResult{Country: values[0], Province: values[2], City: values[3], ISP: values[4], OK: ok}
	}, func() uint64 {
		if global.GIPLOCATION_MANAGER == nil {
			return 0
		}
		return global.GIPLOCATION_MANAGER.Generation()
	})
	var protected atomic.Pointer[ipset.MatchSet]
	var protectedMu sync.Mutex
	refresh := func(extra ...string) {
		protectedMu.Lock()
		defer protectedMu.Unlock()
		sources := wafhostguard.AutoExcludeSources()
		entries := make([]string, 0, len(sources))
		for _, source := range sources {
			entries = append(entries, source.Entry)
		}
		entries = append(entries, extra...)
		protected.Store(ipset.BuildMatchSet(entries))
	}
	refresh()
	wafhostguard.SetAdminIPObserver(func(ip string) { refresh(ip) })
	wafpipeline.Live.Protected = func(ip netip.Addr) bool { return protected.Load().Contains(ip.AsSlice()) }
	punisher.Protected = wafpipeline.Live.Protected
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh()
				if err := waf_service.WafPipelineServiceApp.RefreshSourceCertificates(); err != nil {
					zlog.Warn("端口证书同步失败，保留有效旧证书", err.Error())
				}
			}
		}
	}()
	if err := waf_service.WafPipelineFeedServiceApp.Load(); err != nil {
		zlog.Warn("签名订阅未启用", err.Error())
	}
	if feed := waf_service.WafPipelineFeedServiceApp.Feed; feed != nil {
		wafpipeline.Live.Feed = feed
		go feed.Run(ctx)
	}
	waf_service.WafPipelineServiceApp.Prepare = wafpipeline.Live.Prepare
	wafpipeline.Listeners.SetHandler(func(s *wafpipeline.Snapshot, w http.ResponseWriter, r *http.Request) {
		if global.GWAF_DEBUG_WORKER_HEADER {
			w.Header().Set("X-SamWaf-Worker", global.GWAF_WORKER_TAG)
		}
		wafpipeline.Live.Serve(s, w, r)
	})
}
