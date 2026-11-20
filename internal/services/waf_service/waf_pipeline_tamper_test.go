package waf_service

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/protection/wafpipeline"
	"context"
	"testing"
	"time"
)

func TestTamperCaptureQueueBoundsStaleRevisionAndShutdownDrain(t *testing.T) {
	s, db := nativePortFixture(t)
	changes := 0
	s.OnChange = func(*wafpipeline.Snapshot, *wafpipeline.Snapshot) error { changes++; return nil }
	rule := wafpipeline.TamperRule{ID: "home", Path: "/", Mode: wafpipeline.ModeBlock, Action: "replace", MaxBytes: 1024}
	c := wafpipeline.PortConfig{Schema: 1, Port: 5405, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: newServicePolicyFixture()}
	c.Policy.Network.Response.Tamper = []wafpipeline.TamperRule{rule}
	var err error
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := wafpipeline.Ports.Get(c.Port)
	service := NewPipelineTamperService(s)
	service.running.Store(true)
	body := []byte("first-baseline")
	if !service.Capture(snapshot, rule, body, "text/html", "utf-8") || service.Capture(snapshot, rule, body, "text/html", "utf-8") {
		t.Fatal("per-port capture admission failed")
	}
	body[0] = 'X'
	current, err := s.Get(c.Port)
	if err != nil || current.Revision != 1 {
		t.Fatal("request capture wrote synchronously", err)
	}
	entry := <-service.queue
	if err := service.apply(entry); err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatal("background baseline changed listener lifecycle")
	}
	if service.apply(entry) == nil {
		t.Fatal("stale capture overwrote current version")
	}
	var row model.PortConfig
	db.First(&row, "port = ?", c.Port)
	stored, err := decodePort(row)
	if err != nil || string(stored.Secrets.TamperBaselines[rule.ID].Content) != "first-baseline" {
		t.Fatal("capture did not copy/persist its input", err)
	}
	for i := 0; i < 32; i++ {
		candidate := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: uint16(6000 + i), Revision: 1}}
		if !service.Capture(candidate, rule, []byte("x"), "text/plain", "utf-8") {
			t.Fatal("bounded queue filled too early")
		}
	}
	if service.Capture(&wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Port: 7000}}, rule, []byte("x"), "text/plain", "utf-8") {
		t.Fatal("full capture queue accepted more data")
	}
	current, err = s.Get(c.Port)
	if err != nil {
		t.Fatal(err)
	}
	current.Policy.Network.Response.Tamper[0].Generation++
	current.Policy.Network.Response.Tamper[0].LearnedAt = 0
	current, err = s.Save(PortSaveRequest{Config: current})
	if err != nil {
		t.Fatal(err)
	}
	before := changes
	snapshot, _ = wafpipeline.Ports.Get(c.Port)
	worker := NewPipelineTamperService(s)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { worker.Run(ctx); close(done) }()
	<-worker.Started
	if !worker.Capture(snapshot, current.Policy.Network.Response.Tamper[0], []byte("relearned"), "text/plain", "utf-8") {
		t.Fatal("running worker rejected capture")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("capture worker did not drain")
	}
	if changes != before {
		t.Fatal("shutdown drain restarted listener")
	}
	db.First(&row, "port = ?", c.Port)
	stored, err = decodePort(row)
	if err != nil || string(stored.Secrets.TamperBaselines[rule.ID].Content) != "relearned" {
		t.Fatal("queued baseline lost at shutdown", err)
	}
}
