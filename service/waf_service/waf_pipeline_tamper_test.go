package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafsec"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTamperMigrationEncryptsBaselineAndRejectsStaleContentPreview(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	if err := db.AutoMigrate(&model.TamperRule{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Hosts{}).Where("code = ?", "nas").Update("tamper_json", `{"is_enable":1,"action":"replace","max_size_kb":1024}`).Error; err != nil {
		t.Fatal(err)
	}
	body := []byte("trusted-baseline-fixture")
	sum := sha256.Sum256(body)
	rule := model.TamperRule{BaseOrm: pipelineORM(), HostCode: "nas", Url: "/index.html", IsEnable: 1, BaselineStatus: 1, BaselineContent: body, BaselineHash: hex.EncodeToString(sum[:]), ContentType: "text/html; charset=utf-8"}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewLegacyPort(5300, "nas")
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	public, _ := json.Marshal(p)
	if strings.Contains(string(public), string(body)) {
		t.Fatal("preview leaked protected content")
	}
	rule.BaselineContent = []byte("renewed-baseline-fixture")
	sum = sha256.Sum256(rule.BaselineContent)
	rule.BaselineHash = hex.EncodeToString(sum[:])
	if err := db.Save(&rule).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err == nil {
		t.Fatal("stale baseline preview accepted")
	}
	p, err = s.PreviewLegacyPort(5300, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportLegacyPort(LegacyPortImportRequest{Port: 5300, PolicyHost: "nas", PreviewToken: p.PreviewToken, AcceptChanges: true}, "tester"); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", 5300).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, string(rule.BaselineContent)) {
		t.Fatal("baseline persisted without encryption")
	}
	plain, err := wafsec.DecryptBound(raw, "port_config/secrets_json")
	if err != nil || string(plain) == raw || !json.Valid(plain) {
		t.Fatal("baseline did not use authenticated encrypted storage", err)
	}
	var row model.PortConfig
	if err := db.First(&row, "port = ?", 5300).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodePort(row)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot.Secrets.TamperBaselines[rule.Id].Content) != string(rule.BaselineContent) {
		t.Fatal("baseline not recovered from persistence")
	}
	exported, _ := json.Marshal(wafpipeline.TemplateFromPort("test", "test", snapshot.Config))
	if strings.Contains(string(exported), "renewed-baseline-fixture") {
		t.Fatal("template leaked baseline")
	}
}

func TestTamperCaptureQueueBoundsStaleRevisionAndShutdownDrain(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	changes := 0
	s.OnChange = func(*wafpipeline.Snapshot, *wafpipeline.Snapshot) error { changes++; return nil }
	rule := wafpipeline.TamperRule{ID: "home", Path: "/", Mode: wafpipeline.ModeBlock, Action: "replace", MaxBytes: 1024}
	c := wafpipeline.PortConfig{Schema: 1, Port: 5405, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
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
