package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"errors"
	"sync"

	"gorm.io/gorm"
)

type WafPipelineHoneypotService struct{ mu sync.Mutex }

var WafPipelineHoneypotServiceApp = &WafPipelineHoneypotService{}

func validateSharedHoneypot(cfg wafpipeline.HoneypotConfig) error {
	policy := wafpipeline.DefaultPolicy()
	policy.Honeypot = cfg
	return wafpipeline.ValidateConfig(wafpipeline.PortConfig{Schema: wafpipeline.SchemaVersion, Port: 1, Protocol: "http", Upstream: "http://127.0.0.1", Policy: policy}, 0)
}

func (s *WafPipelineHoneypotService) Get() (wafpipeline.SharedHoneypotConfig, error) {
	out := wafpipeline.SharedHoneypotConfig{Config: wafpipeline.SharedHoneypotPolicy(wafpipeline.DefaultPolicy().Honeypot)}
	db, err := pipelineDB()
	if err != nil {
		return out, err
	}
	var row model.PipelineHoneypot
	err = db.First(&row, "id = ?", "shared").Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := wafpipeline.StrictDecode([]byte(row.ConfigJSON), &out.Config); err != nil {
		return out, err
	}
	out.Config = wafpipeline.SharedHoneypotPolicy(out.Config)
	if err := validateSharedHoneypot(out.Config); err != nil {
		return out, err
	}
	out.Configured, out.Revision = true, row.Revision
	return out, nil
}

func (s *WafPipelineHoneypotService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.Get()
	if err != nil {
		wafpipeline.PublishSharedHoneypot(nil)
		return err
	}
	wafpipeline.PublishSharedHoneypot(&value)
	return nil
}

func (s *WafPipelineHoneypotService) Save(value wafpipeline.SharedHoneypotConfig, operator string) (wafpipeline.SharedHoneypotConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := wafpipeline.SharedHoneypotPolicy(value.Config)
	policy := wafpipeline.DefaultPolicy()
	policy.Honeypot = cfg
	if err := wafpipeline.ValidateConfig(wafpipeline.PortConfig{Schema: wafpipeline.SchemaVersion, Port: 1, Protocol: "http", Upstream: "http://127.0.0.1", Policy: policy}, 0); err != nil {
		return value, err
	}
	db, err := pipelineDB()
	if err != nil {
		return value, err
	}
	raw, _ := json.Marshal(cfg)
	err = db.Transaction(func(tx *gorm.DB) error {
		if value.Revision == 0 {
			return tx.Create(&model.PipelineHoneypot{ID: "shared", Revision: 1, ConfigJSON: string(raw)}).Error
		}
		result := tx.Model(&model.PipelineHoneypot{}).Where("id = ? AND revision = ?", "shared", value.Revision).Updates(map[string]any{"revision": value.Revision + 1, "config_json": string(raw)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("蜜罐设置已被修改，请重新加载并核对草稿")
		}
		return nil
	})
	if err != nil {
		return value, err
	}
	value = wafpipeline.SharedHoneypotConfig{Revision: value.Revision + 1, Configured: true, Config: cfg}
	wafpipeline.PublishSharedHoneypot(&value)
	WafPipelineServiceApp.audit(operator, 0, "shared_honeypot_saved")
	return value, nil
}
