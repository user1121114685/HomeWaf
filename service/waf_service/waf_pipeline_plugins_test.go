package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"strings"
	"testing"
)

func TestPortPluginSecretsPrunedAndFailedLoadClosesCandidate(t *testing.T) {
	s, db := legacyMigrationFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 54101, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: wafpipeline.DefaultPolicy()}
	c.Policy.Gate.Enabled = false
	c.Policy.RuleSet.Plugins = &wafpipeline.PluginPolicy{Mode: wafpipeline.ModeBlock, IDs: []string{"a", "b", "c", "d"}}
	profiles := map[string]wafpipeline.PluginLaunch{}
	for _, id := range c.Policy.RuleSet.Plugins.IDs {
		profiles[id] = wafpipeline.PluginLaunch{Binary: "private-path", Params: map[string]any{"token": "secret-plugin-credential"}}
	}
	var err error
	c, err = s.Save(PortSaveRequest{Config: c, PluginProfiles: profiles})
	if err != nil {
		t.Fatal(err)
	}
	var row model.PortConfig
	if err = db.First(&row, "port = ?", c.Port).Error; err != nil {
		t.Fatal(err)
	}
	var storedSecret string
	if err = db.Raw("SELECT secrets_json FROM port_config WHERE port = ?", c.Port).Scan(&storedSecret).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(row.ConfigJSON, "secret-plugin-credential") || strings.Contains(storedSecret, "secret-plugin-credential") {
		t.Fatal("plugin secrets stored in plaintext")
	}
	old, _ := wafpipeline.Ports.Get(c.Port)
	c.Policy.RuleSet.Plugins.IDs = []string{"e"}
	c, err = s.Save(PortSaveRequest{Config: c, PluginProfiles: map[string]wafpipeline.PluginLaunch{"e": profiles["a"]}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := wafpipeline.Ports.Get(c.Port)
	if len(next.Secrets.PluginProfiles) != 1 || len(old.Secrets.PluginProfiles) != 4 {
		t.Fatal("stale profiles retained or previous snapshot mutated")
	}
	if err = db.First(&row, "port = ?", c.Port).Error; err != nil {
		t.Fatal(err)
	}
	c2 := wafpipeline.CloneConfig(c)
	c2.Port++
	raw, _ := json.Marshal(c2)
	row.Port = int(c2.Port)
	row.Id = "second-port-fixture"
	row.ConfigJSON = string(raw)
	if err = db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](1)
	prepared := map[uint16]*wafpipeline.PreparedPlugins{}
	s.Prepare = func(snapshot *wafpipeline.Snapshot) error {
		p := &wafpipeline.PreparedPlugins{}
		snapshot.Compiled = &wafpipeline.CompiledPolicy{Plugins: p}
		prepared[snapshot.Config.Port] = p
		return nil
	}
	if err = s.Load(); err == nil {
		t.Fatal("registry capacity failure not reported")
	}
	if len(prepared) != 2 || prepared[c.Port].Closed() || !prepared[c2.Port].Closed() {
		t.Fatal("failed publish leaked candidate or closed healthy port")
	}
	prepared[c.Port].Close()
}
