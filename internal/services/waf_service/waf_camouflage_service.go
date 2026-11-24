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
	"strings"
	"sync"
	"time"
)

const camouflageConfigItem = "protocol_camouflage_defense"

type WafCamouflageService struct {
	mu      sync.Mutex
	blockMu sync.Mutex
	saveMu  sync.Mutex
	manager *camouflage.Manager
	blocked map[string]time.Time
	blockIP func(request.WafFirewallIPBlockAddReq) error
}

var WafCamouflageServiceApp = newWafCamouflageService()

func newWafCamouflageService() *WafCamouflageService {
	s := &WafCamouflageService{
		blocked: map[string]time.Time{},
		blockIP: WafFirewallIPBlockServiceApp.AddApi,
	}
	s.manager = camouflage.NewManager(s.handleHit)
	return s
}

func (s *WafCamouflageService) GetConfig() camouflage.Config {
	config := camouflage.DefaultConfig()
	bean := WafSystemConfigServiceApp.GetDetailByItem(camouflageConfigItem)
	if bean.Value == "" {
		return config
	}
	if err := json.Unmarshal([]byte(bean.Value), &config); err != nil {
		return camouflage.DefaultConfig()
	}
	normalized, err := camouflage.NormalizeConfig(config)
	if err != nil {
		return camouflage.DefaultConfig()
	}
	return normalized
}

func (s *WafCamouflageService) LoadAndStart() error {
	return s.apply(s.GetConfig())
}

func (s *WafCamouflageService) Stop() { s.manager.Stop() }

func (s *WafCamouflageService) Save(config camouflage.Config, operator string) (camouflage.Config, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	normalized, err := camouflage.NormalizeConfig(config)
	if err != nil {
		return config, err
	}
	if err := s.validatePortConflict(normalized); err != nil {
		return normalized, err
	}
	previous := s.GetConfig()
	if err := s.apply(normalized); err != nil {
		_ = s.apply(previous)
		return normalized, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		_ = s.apply(previous)
		return normalized, err
	}
	bean := WafSystemConfigServiceApp.GetDetailByItem(camouflageConfigItem)
	if bean.Id == "" {
		err = WafSystemConfigServiceApp.AddManagedApi(request.WafSystemConfigAddReq{
			ItemClass: "protection",
			Item:      camouflageConfigItem,
			Value:     string(raw),
			ItemType:  "json",
			Remarks:   "Protocol camouflage profiles and scoring settings",
		})
	} else {
		err = WafSystemConfigServiceApp.ModifyByItemApi(request.WafSystemConfigEditByItemReq{Item: camouflageConfigItem, Value: string(raw)})
	}
	if err != nil {
		_ = s.apply(previous)
		return normalized, err
	}
	if operator != "" {
		WafPipelineServiceApp.audit(operator, normalized.Port, "camouflage_config_saved")
	}
	return normalized, nil
}

func (s *WafCamouflageService) apply(config camouflage.Config) error {
	if err := s.validatePortConflict(config); err != nil {
		return err
	}
	return s.manager.Apply(config)
}

func (s *WafCamouflageService) validatePortConflict(config camouflage.Config) error {
	normalized, err := camouflage.NormalizeConfig(config)
	if err != nil {
		return err
	}
	if !normalized.Enabled {
		return nil
	}
	if normalized.Port == uint16(global.GWAF_LOCAL_SERVER_PORT) {
		return fmt.Errorf("camouflage port %d conflicts with the management listener", normalized.Port)
	}
	if snapshot, ok := wafpipeline.Ports.Get(normalized.Port); ok && snapshot.Config.ListenEnabled {
		return fmt.Errorf("camouflage port %d is already used by service %s", normalized.Port, snapshot.Config.Name)
	}
	return nil
}

func (s *WafCamouflageService) Status() camouflage.Status { return s.manager.Status() }

func (s *WafCamouflageService) ObserveManagementClientIP(ip string) {
	s.manager.ObserveManagementClientIP(ip)
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
	seen := map[string]bool{}
	for _, row := range rows {
		var hit camouflage.Hit
		if err := json.Unmarshal([]byte(row.DetailJSON), &hit); err != nil {
			continue
		}
		seen[hit.ID] = true
		result = append(result, hit)
	}
	for _, hit := range s.manager.Events(limit) {
		if !seen[hit.ID] {
			result = append(result, hit)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *WafCamouflageService) handleHit(hit camouflage.Hit) camouflage.Hit {
	config := s.manager.Config()
	if !hit.LocalLAN && config.BlockPublic {
		hit = s.blockPublic(hit, config)
	}
	s.persistHit(hit)
	return hit
}

func (s *WafCamouflageService) blockPublic(hit camouflage.Hit, config camouflage.Config) camouflage.Hit {
	s.blockMu.Lock()
	defer s.blockMu.Unlock()
	ip, err := netip.ParseAddr(hit.ClientIP)
	if err != nil || !ip.IsValid() {
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
	reason := fmt.Sprintf("protocol camouflage probe on %s score=%d probe=%s", config.Profile, hit.Score, hit.Probe)
	err = s.blockIP(request.WafFirewallIPBlockAddReq{
		IP:         hit.ClientIP,
		Reason:     reason,
		BlockType:  "auto",
		ExpireTime: time.Now().Add(time.Duration(config.BanSeconds) * time.Second).Unix(),
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
	s.blocked[hit.ClientIP] = time.Now().Add(time.Duration(config.BanSeconds) * time.Second)
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
	s.manager.ClearEvents()
	return nil
}
