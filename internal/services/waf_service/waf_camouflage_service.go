package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/protection/camouflage"
	"HomeWaf/internal/protection/wafpipeline"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const camouflageConfigItem = "protocol_camouflage_defense"
const maxCamouflageServiceEvents = 500

type WafCamouflageService struct {
	mu         sync.Mutex
	blockMu    sync.Mutex
	saveMu     sync.Mutex
	config     camouflage.DefenseConfig
	managers   map[string]*camouflage.Manager
	events     []camouflage.Hit
	blocked    map[string]time.Time
	blockIP    func(request.WafFirewallIPBlockAddReq) error
	management map[string]struct{}
}

var WafCamouflageServiceApp = newWafCamouflageService()

func newWafCamouflageService() *WafCamouflageService {
	return &WafCamouflageService{
		config:     camouflage.DefaultDefenseConfig(),
		managers:   map[string]*camouflage.Manager{},
		blocked:    map[string]time.Time{},
		blockIP:    WafFirewallIPBlockServiceApp.AddApi,
		management: map[string]struct{}{},
	}
}

func (s *WafCamouflageService) loadConfig() (camouflage.DefenseConfig, error) {
	bean := WafSystemConfigServiceApp.GetDetailByItem(camouflageConfigItem)
	if bean.Value == "" {
		return camouflage.DefaultDefenseConfig(), nil
	}
	return decodeCamouflageConfig(bean.Value)
}

func decodeCamouflageConfig(value string) (camouflage.DefenseConfig, error) {
	config := camouflage.DefaultDefenseConfig()
	raw := []byte(value)
	var shape struct {
		Profiles json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return config, fmt.Errorf("invalid protocol camouflage config: %w", err)
	}
	if len(shape.Profiles) == 0 || string(shape.Profiles) == "null" {
		var legacy camouflage.Config
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return config, fmt.Errorf("invalid legacy protocol camouflage config: %w", err)
		}
		if legacy.Profile == "nas-generic" {
			legacy.Profile = "http"
		}
		normalizedLegacy, legacyErr := camouflage.NormalizeConfig(legacy)
		if legacyErr != nil {
			return config, legacyErr
		}
		profile := camouflage.ProfileConfig{
			ID:              uuid.GenUUID(),
			Name:            "旧版伪装画像",
			Enabled:         normalizedLegacy.Enabled,
			Port:            normalizedLegacy.Port,
			ProtocolProfile: normalizedLegacy.Profile,
			Version:         normalizedLegacy.Version,
			ServerName:      normalizedLegacy.ServerName,
			ListenIPVersion: normalizedLegacy.ListenIPVersion,
		}
		config.Profiles = []camouflage.ProfileConfig{profile}
	} else if err := json.Unmarshal(raw, &config); err != nil {
		return config, fmt.Errorf("invalid protocol camouflage config: %w", err)
	}
	return camouflage.NormalizeDefenseConfig(config)
}

func (s *WafCamouflageService) Config() camouflage.DefenseConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneCamouflageConfig(s.config)
}

func cloneCamouflageConfig(config camouflage.DefenseConfig) camouflage.DefenseConfig {
	config.Profiles = append([]camouflage.ProfileConfig{}, config.Profiles...)
	config.LocalCIDRs = append([]string{}, config.LocalCIDRs...)
	return config
}

func (s *WafCamouflageService) LoadAndStart() error {
	config, err := s.loadConfig()
	if err != nil {
		return err
	}
	return s.apply(config)
}

func (s *WafCamouflageService) Stop() {
	s.saveMu.Lock()
	s.mu.Lock()
	managers := s.managers
	s.managers = map[string]*camouflage.Manager{}
	s.mu.Unlock()
	for _, manager := range managers {
		manager.Stop()
	}
	s.saveMu.Unlock()
}

func (s *WafCamouflageService) Save(config camouflage.DefenseConfig, operator string) (camouflage.DefenseConfig, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	normalized, err := camouflage.NormalizeDefenseConfig(config)
	if err != nil {
		return config, err
	}
	if err := s.validatePortConflicts(normalized); err != nil {
		return config, err
	}
	previous := s.Config()
	if err := s.apply(normalized); err != nil {
		_ = s.apply(previous)
		return config, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		_ = s.apply(previous)
		return config, err
	}
	bean := WafSystemConfigServiceApp.GetDetailByItem(camouflageConfigItem)
	if bean.Id == "" {
		err = WafSystemConfigServiceApp.AddManagedApi(request.WafSystemConfigAddReq{
			ItemClass: "protection",
			Item:      camouflageConfigItem,
			Value:     string(raw),
			ItemType:  "json",
			Remarks:   "Protocol camouflage profiles and LAN scoring policy",
		})
	} else {
		err = WafSystemConfigServiceApp.ModifyByItemApi(request.WafSystemConfigEditByItemReq{Item: camouflageConfigItem, Value: string(raw)})
	}
	if err != nil {
		_ = s.apply(previous)
		return config, err
	}
	if operator != "" {
		for _, profile := range normalized.Profiles {
			WafPipelineServiceApp.audit(operator, profile.Port, "camouflage_profile_saved")
		}
	}
	return cloneCamouflageConfig(normalized), nil
}

func (s *WafCamouflageService) apply(config camouflage.DefenseConfig) error {
	if err := s.validatePortConflicts(config); err != nil {
		return err
	}
	s.mu.Lock()
	previous := s.managers
	s.managers = map[string]*camouflage.Manager{}
	s.config = cloneCamouflageConfig(config)
	management := make([]string, 0, len(s.management))
	for prefix := range s.management {
		management = append(management, prefix)
	}
	s.mu.Unlock()
	for _, manager := range previous {
		manager.Stop()
	}

	started := make([]*camouflage.Manager, 0, len(config.Profiles))
	for _, image := range config.Profiles {
		if !image.Enabled {
			continue
		}
		manager := camouflage.NewManager(s.handleHit)
		manager.SetLocalCIDRs(config.LocalCIDRMode, config.LocalCIDRs)
		for _, ip := range management {
			manager.ObserveManagementClientIP(ip)
		}
		listenerConfig := camouflage.Config{
			Enabled:         image.Enabled,
			Port:            image.Port,
			Profile:         image.ProtocolProfile,
			Version:         image.Version,
			ServerName:      image.ServerName,
			BlockPublic:     true,
			BanSeconds:      camouflage.DefaultBanSeconds,
			ListenIPVersion: image.ListenIPVersion,
		}
		if err := manager.Apply(listenerConfig); err != nil {
			for _, active := range started {
				active.Stop()
			}
			s.mu.Lock()
			s.managers = map[string]*camouflage.Manager{}
			s.mu.Unlock()
			return fmt.Errorf("profile %q: %w", image.Name, err)
		}
		started = append(started, manager)
		s.mu.Lock()
		s.managers[image.ID] = manager
		s.mu.Unlock()
	}
	return nil
}

func (s *WafCamouflageService) validatePortConflicts(config camouflage.DefenseConfig) error {
	used := map[string]string{}
	for _, image := range config.Profiles {
		if !image.Enabled {
			continue
		}
		profile, _ := camouflage.ProfileByCode(image.ProtocolProfile)
		if profile.Transport == "tcp" && image.Port == uint16(global.GWAF_LOCAL_SERVER_PORT) {
			return fmt.Errorf("profile %q port %d conflicts with the management listener", image.Name, image.Port)
		}
		key := fmt.Sprintf("%s:%d", profile.Transport, image.Port)
		if previous, ok := used[key]; ok {
			return fmt.Errorf("profiles %q and %q use the same port %d", previous, image.Name, image.Port)
		}
		used[key] = image.Name
		if profile.Transport == "tcp" {
			if snapshot, ok := wafpipeline.Ports.Get(image.Port); ok && snapshot.Config.ListenEnabled {
				return fmt.Errorf("profile %q port %d is already used by service %s", image.Name, image.Port, snapshot.Config.Name)
			}
		}
	}
	return nil
}

func (s *WafCamouflageService) ObserveManagementClientIP(ip string) {
	parsed, err := netip.ParseAddr(ip)
	if err != nil || !parsed.Unmap().IsPrivate() {
		return
	}
	s.mu.Lock()
	for _, manager := range s.managers {
		manager.ObserveManagementClientIP(ip)
	}
	addr := parsed.Unmap()
	bits := 24
	if addr.Is6() {
		bits = 64
	}
	prefix := netip.PrefixFrom(addr, bits).Masked().String()
	s.management[prefix] = struct{}{}
	s.mu.Unlock()
}

func (s *WafCamouflageService) Status() []camouflage.ProfileStatus {
	s.mu.Lock()
	config := cloneCamouflageConfig(s.config)
	managers := make(map[string]*camouflage.Manager, len(s.managers))
	for id, manager := range s.managers {
		managers[id] = manager
	}
	s.mu.Unlock()
	rows := make([]camouflage.ProfileStatus, 0, len(config.Profiles))
	for _, image := range config.Profiles {
		status := camouflage.ProfileStatus{ID: image.ID, Name: image.Name, Enabled: image.Enabled, Port: image.Port}
		if manager := managers[image.ID]; manager != nil {
			runtime := manager.Status()
			status.Running = runtime.Running
			status.LastError = runtime.LastError
		}
		rows = append(rows, status)
	}
	return rows
}

func (s *WafCamouflageService) LocalCIDRs() []string {
	s.mu.Lock()
	mode := s.config.LocalCIDRMode
	custom := append([]string(nil), s.config.LocalCIDRs...)
	managers := make([]*camouflage.Manager, 0, len(s.managers))
	for _, manager := range s.managers {
		managers = append(managers, manager)
	}
	s.mu.Unlock()
	if mode == "custom" {
		return custom
	}
	result := camouflage.LocalCIDRs()
	seen := map[string]struct{}{}
	for _, value := range result {
		seen[value] = struct{}{}
	}
	for _, manager := range managers {
		for _, value := range manager.Status().LocalCIDRs {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	s.mu.Lock()
	for prefix := range s.management {
		if _, ok := seen[prefix]; !ok {
			seen[prefix] = struct{}{}
			result = append(result, prefix)
		}
	}
	s.mu.Unlock()
	sort.Strings(result)
	return result
}

func (s *WafCamouflageService) Profiles() []camouflage.Profile { return camouflage.Profiles() }

func (s *WafCamouflageService) Events(limit int) ([]camouflage.Hit, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []model.PipelineLog
	if global.GWAF_LOCAL_LOG_DB != nil {
		if err := global.GWAF_LOCAL_LOG_DB.Where("kind = ?", "camouflage").Order("time_millis DESC").Limit(limit).Find(&rows).Error; err != nil {
			return nil, err
		}
	}
	result := make([]camouflage.Hit, 0, limit)
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		var hit camouflage.Hit
		if json.Unmarshal([]byte(row.DetailJSON), &hit) == nil {
			seen[hit.ID] = struct{}{}
			result = append(result, hit)
		}
	}
	s.mu.Lock()
	for _, hit := range s.events {
		if _, exists := seen[hit.ID]; exists {
			continue
		}
		seen[hit.ID] = struct{}{}
		result = append(result, hit)
	}
	managers := make([]*camouflage.Manager, 0, len(s.managers))
	for _, manager := range s.managers {
		managers = append(managers, manager)
	}
	s.mu.Unlock()
	for _, manager := range managers {
		for _, hit := range manager.Events(limit) {
			if _, exists := seen[hit.ID]; exists {
				continue
			}
			seen[hit.ID] = struct{}{}
			result = append(result, hit)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Time > result[j].Time })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *WafCamouflageService) handleHit(hit camouflage.Hit) camouflage.Hit {
	if !hit.LocalLAN {
		hit = s.blockNonLocal(hit)
	}
	s.persistHit(hit)
	s.mu.Lock()
	if len(s.events) >= maxCamouflageServiceEvents {
		copy(s.events, s.events[len(s.events)-maxCamouflageServiceEvents+1:])
		s.events = s.events[:maxCamouflageServiceEvents-1]
	}
	s.events = append(s.events, hit)
	s.mu.Unlock()
	return hit
}

func (s *WafCamouflageService) blockNonLocal(hit camouflage.Hit) camouflage.Hit {
	s.blockMu.Lock()
	defer s.blockMu.Unlock()
	if _, err := netip.ParseAddr(hit.ClientIP); err != nil {
		hit.BlockError = "invalid client IP"
		return hit
	}
	s.mu.Lock()
	if until, ok := s.blocked[hit.ClientIP]; ok && until.After(time.Now()) {
		s.mu.Unlock()
		hit.Blocked = true
		return hit
	}
	s.mu.Unlock()
	err := s.blockIP(request.WafFirewallIPBlockAddReq{
		IP:         hit.ClientIP,
		Reason:     fmt.Sprintf("protocol camouflage probe on %s score=%d probe=%s", hit.Profile, hit.Score, hit.Probe),
		BlockType:  "auto",
		ExpireTime: time.Now().Add(camouflage.DefaultBanDays * 24 * time.Hour).Unix(),
		Remarks:    "Automatic 90-day block from protocol camouflage",
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already") || strings.Contains(err.Error(), "已经") {
			hit.Blocked = true
			return hit
		}
		hit.BlockError = err.Error()
		return hit
	}
	s.mu.Lock()
	s.blocked[hit.ClientIP] = time.Now().Add(camouflage.DefaultBanDays * 24 * time.Hour)
	s.mu.Unlock()
	hit.Blocked = true
	return hit
}

func (s *WafCamouflageService) persistHit(hit camouflage.Hit) {
	if global.GQEQUE_LOG_DB == nil {
		return
	}
	raw, _ := json.Marshal(hit)
	severity := "medium"
	if hit.Score >= 90 {
		severity = "high"
	}
	verdict := "observe"
	if hit.Blocked {
		verdict = "deny"
	}
	global.GQEQUE_LOG_DB.Enqueue(&model.PipelineLog{
		ID:         uuid.GenUUID(),
		ReqID:      hit.ID,
		PortID:     fmt.Sprintf("camouflage:%d", hit.Port),
		TimeMillis: time.Now().UnixMilli(),
		Kind:       "camouflage",
		ClientIP:   hit.ClientIP,
		Stage:      1,
		Verdict:    verdict,
		RuleID:     "protocol_camouflage",
		Category:   "protocol_fingerprint",
		Severity:   severity,
		Observe:    !hit.Blocked,
		Reason:     hit.Evidence,
		DetailJSON: string(raw),
	})
}

func (s *WafCamouflageService) ClearEvents() error {
	if global.GWAF_LOCAL_LOG_DB == nil {
		return errors.New("log database unavailable")
	}
	if err := global.GWAF_LOCAL_LOG_DB.Where("kind = ?", "camouflage").Delete(&model.PipelineLog{}).Error; err != nil {
		return err
	}
	s.mu.Lock()
	managers := make([]*camouflage.Manager, 0, len(s.managers))
	for _, manager := range s.managers {
		managers = append(managers, manager)
	}
	s.mu.Unlock()
	for _, manager := range managers {
		manager.ClearEvents()
	}
	s.mu.Lock()
	s.events = nil
	s.mu.Unlock()
	return nil
}
