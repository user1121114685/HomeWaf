package manager

import (
	plugininterface "HomeWaf/plugin/interface"
	process "HomeWaf/plugin/process"
	"HomeWaf/wafpipeline"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"
)

type portPluginStarter func(uint16, string, wafpipeline.PluginLaunch) (wafpipeline.PluginCheck, func(), error)
type PipelinePluginPool struct {
	slots chan struct{}
	start portPluginStarter
}

func NewPipelinePluginPool(limit int) *PipelinePluginPool {
	if limit < 1 || limit > 256 {
		limit = 32
	}
	return &PipelinePluginPool{slots: make(chan struct{}, limit), start: startSignedPortPlugin}
}
func (p *PipelinePluginPool) Stats() map[string]uint64 {
	return map[string]uint64{"processes": uint64(len(p.slots)), "capacity": uint64(cap(p.slots))}
}
func (p *PipelinePluginPool) Prepare(s *wafpipeline.Snapshot) error {
	policy := s.Config.Policy.RuleSet.Plugins
	if policy == nil || policy.Mode == wafpipeline.ModeOff || s.Preview || !s.Config.ListenEnabled {
		return nil
	}
	if err := wafpipeline.ValidatePluginProfiles(s.Secrets.PluginProfiles); err != nil {
		return err
	}
	profiles := map[string]wafpipeline.PluginLaunch{}
	for _, id := range policy.IDs {
		value, ok := s.Secrets.PluginProfiles[id]
		if !ok {
			return fmt.Errorf("端口插件 %s 缺少加密启动配置", id)
		}
		profiles[id] = value
	}
	raw, _ := json.Marshal(struct {
		Port       uint16
		Generation uint64
		IDs        []string
		Profiles   map[string]wafpipeline.PluginLaunch
	}{s.Config.Port, policy.Generation, policy.IDs, profiles})
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if old, ok := wafpipeline.Ports.Get(s.Config.Port); ok && old.Config.ListenEnabled && old.Compiled != nil && !old.Compiled.Plugins.Closed() && old.Compiled.Plugins.Digest == digest {
		s.Compiled.Plugins = old.Compiled.Plugins
		return nil
	}
	var checks []wafpipeline.PluginCheck
	var closeClients []func()
	stop := func() {
		for i := len(closeClients) - 1; i >= 0; i-- {
			closeClients[i]()
		}
	}
	for _, id := range policy.IDs {
		select {
		case p.slots <- struct{}{}:
		default:
			stop()
			return errors.New("端口插件进程总配额已满；可先停用该端口再更新，或调整启动配额")
		}
		data, _ := json.Marshal(profiles[id])
		var profile wafpipeline.PluginLaunch
		_ = json.Unmarshal(data, &profile)
		checker, closeClient, err := p.start(s.Config.Port, id, profile)
		if err != nil {
			if closeClient != nil {
				closeClient()
			}
			<-p.slots
			stop()
			return fmt.Errorf("端口插件 %s 启动/签名/初始化失败", id)
		}
		if checker.Check == nil {
			if closeClient != nil {
				closeClient()
			}
			<-p.slots
			stop()
			return errors.New("端口插件未提供可用检查接口")
		}
		closeClients = append(closeClients, func() {
			if closeClient != nil {
				closeClient()
			}
			<-p.slots
		})
		checks = append(checks, checker)
	}
	s.Compiled.Plugins = &wafpipeline.PreparedPlugins{Digest: digest, Checks: checks, Stop: stop}
	return nil
}
func startSignedPortPlugin(port uint16, id string, profile wafpipeline.PluginLaunch) (wafpipeline.PluginCheck, func(), error) {
	client := process.NewClient(&process.ClientConfig{Binary: profile.Binary, Timeout: 3 * time.Second, Output: io.Discard})
	rpc, err := client.Client()
	if err != nil {
		return wafpipeline.PluginCheck{}, client.Kill, err
	}
	instance, err := rpc.Dispense("ip_filter")
	if err != nil {
		return wafpipeline.PluginCheck{}, client.Kill, err
	}
	filter, ok := instance.(plugininterface.IPFilterPlugin)
	if !ok {
		return wafpipeline.PluginCheck{}, client.Kill, errors.New("not an IP filter plugin")
	}
	params := profile.Params
	if params == nil {
		params = map[string]any{}
	}
	params["port_id"] = wafpipeline.PortID(port)
	if err := filter.Init(params); err != nil {
		return wafpipeline.PluginCheck{}, client.Kill, err
	}
	checker := wafpipeline.PluginCheck{ID: id, Check: func(ctx context.Context, r wafpipeline.PluginRequest) (bool, error) {
		response, err := filter.Filter(ctx, &plugininterface.IPFilterRequest{IP: r.IP, RequestPath: r.Path, UserAgent: r.UserAgent, Extra: map[string]any{"port_id": r.PortID, "request_id": r.RequestID, "method": r.Method, "host": r.Host}})
		if err != nil || response == nil {
			return false, errors.New("local plugin unavailable")
		}
		return !response.Allowed, nil
	}}
	return checker, client.Kill, nil
}

// Copy active pre_check configuration without exposing live PluginInstance
// pointers. New ports launch their own signed processes from this snapshot.
func (pm *PluginManager) PipelineProfiles() ([]string, map[string]wafpipeline.PluginLaunch) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	profiles := map[string]wafpipeline.PluginLaunch{}
	if !pm.enabled {
		return nil, profiles
	}
	var instances []*PluginInstance
	for _, instance := range pm.plugins {
		if instance.Enabled && instance.PluginConfig != nil && instance.Plugin != nil && contains(instance.Groups, "pre_check") {
			instances = append(instances, instance)
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Priority == instances[j].Priority {
			return instances[i].ID < instances[j].ID
		}
		return instances[i].Priority > instances[j].Priority
	})
	var ids []string
	for _, instance := range instances {
		profile := wafpipeline.PluginLaunch{Binary: instance.PluginConfig.BinaryPath, Params: instance.Config}
		raw, _ := json.Marshal(profile)
		var copy wafpipeline.PluginLaunch
		_ = json.Unmarshal(raw, &copy)
		profiles[instance.ID] = copy
		ids = append(ids, instance.ID)
	}
	return ids, profiles
}
