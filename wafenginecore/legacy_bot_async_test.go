package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafbot"
	"HomeWaf/wafpipeline"
	"context"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyBOTDNSOptInDoesNotBlockOrTrustPendingIdentity(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_RECORD_SPIDER_DENY
	global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_SPIDER_DENY = 1 })
	t.Cleanup(func() {
		global.UpdateSettings(func(cfg *global.RuntimeSettings) { cfg.GCONFIG_RECORD_SPIDER_DENY = old })
	})
	host := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "async-bot", Port: 5000}}
	req := httptest.NewRequest("GET", "http://self.test:5001/", nil)
	var calls atomic.Int64
	b := wafpipeline.NewBOTCache()
	release := make(chan struct{})
	var releaseOnce sync.Once
	b.Resolve = func(ua, ip string) wafbot.BotResult {
		calls.Add(1)
		<-release
		return wafbot.BotResult{IsBot: true, IsNormalBot: true, StrongVerified: true}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); cancel(); <-done })
	log := &innerbean.WebLog{SRC_IP: "192.0.2.1", USER_AGENT: "Googlebot/2.1"}
	defaultEngine := &WafEngine{}
	if defaultEngine.CheckBot(req, log, nil, host, nil).IsBlock || log.IsSafeBot() || calls.Load() != 0 {
		t.Fatal("default DNS must be off and unverified identity is not trusted")
	}
	engine := &WafEngine{BotIdentity: b.Lookup}
	returned := make(chan bool, 1)
	go func() { returned <- engine.CheckBot(req, log, nil, host, nil).IsBlock }()
	select {
	case blocked := <-returned:
		if blocked || log.IsSafeBot() {
			t.Fatal("pending lookup changed an ordinary verdict")
		}
	case <-time.After(time.Second):
		t.Fatal("DNS blocked the request handler")
	}
	for n := 0; n < 1000; n++ {
		engine.CheckBot(req, log, nil, host, nil)
	}
	releaseOnce.Do(func() { close(release) })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		verified, _ := b.Lookup("legacy:5000", "googlebot", log.SRC_IP, true)
		if verified {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if engine.CheckBot(req, log, nil, host, nil).IsBlock || !log.IsSafeBot() || calls.Load() != 1 {
		t.Fatal("resolved strong identity not published or lookup was duplicated")
	}
	otherHost := &wafenginmodel.HostSafe{Host: model.Hosts{Code: "other", Port: 5002}}
	if engine.CheckBot(req, log, nil, otherHost, nil).IsBlock || log.IsSafeBot() {
		t.Fatal("another listener reused first listener identity")
	}
}
