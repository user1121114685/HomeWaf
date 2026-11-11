package waf_service

import (
	"HomeWaf/internal/licensing"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafreg"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/url"
	"sync"
	"time"
)

type PipelineFeedConfig struct {
	Enabled         bool   `json:"enabled"`
	URL             string `json:"url"`
	IntervalMinutes int    `json:"interval_minutes"`
	Token           string `json:"token,omitempty"`
	ClearToken      bool   `json:"clear_token,omitempty"`
	HasToken        bool   `json:"has_token,omitempty"`
}
type WafPipelineFeedService struct {
	mu   sync.Mutex
	Feed *wafpipeline.SignedFeed
}

var WafPipelineFeedServiceApp = &WafPipelineFeedService{}

// Availability reports public capability state only, never keys or license data.
func (s *WafPipelineFeedService) Availability() (authorized bool, trustedKeys int) {
	keys, err := licensing.ParseKeyring(licensing.TrustedKeysJSON)
	if err == nil {
		trustedKeys = len(keys)
	}
	return wafreg.HasFeature(wafpipeline.FeedFeature), trustedKeys
}

func (s *WafPipelineFeedService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Feed == nil {
		s.Feed = wafpipeline.NewSignedFeed(func() bool { return wafreg.HasFeature(wafpipeline.FeedFeature) }, func(cache wafpipeline.FeedCache) error {
			db, err := pipelineDB()
			if err != nil {
				return err
			}
			data, err := json.Marshal(cache)
			if err != nil {
				return err
			}
			return db.Model(&model.PipelineFeed{}).Where("id = ?", "signed-feed").Updates(map[string]any{"snapshot_json": string(data)}).Error
		})
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var row model.PipelineFeed
	if err = db.Where("id = ?", "signed-feed").First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return s.Feed.Configure(false, nil, nil, time.Hour)
	} else if err != nil {
		return err
	}
	if row.SnapshotJSON != "" {
		var cache wafpipeline.FeedCache
		if err = json.Unmarshal([]byte(row.SnapshotJSON), &cache); err != nil {
			return errors.New("authenticated feed cache is invalid")
		}
		if err = s.Feed.Restore(cache); err != nil {
			return err
		}
	}
	return s.configure(row)
}
func (s *WafPipelineFeedService) configure(row model.PipelineFeed) error {
	keys, err := licensing.ParseKeyring(licensing.TrustedKeysJSON)
	if err != nil {
		return err
	}
	minutes := row.IntervalMinutes
	if minutes < 1 {
		minutes = 60
	}
	provider := &wafpipeline.HTTPSFeedProvider{URL: row.URL, Token: row.AuthToken, Client: utils.SafeOutboundHTTPClient(30 * time.Second)}
	return s.Feed.Configure(row.Enabled, provider, keys, time.Duration(minutes)*time.Minute)
}
func (s *WafPipelineFeedService) Get() (PipelineFeedConfig, error) {
	db, err := pipelineDB()
	if err != nil {
		return PipelineFeedConfig{}, err
	}
	var row model.PipelineFeed
	if err = db.Where("id = ?", "signed-feed").First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return PipelineFeedConfig{IntervalMinutes: 60}, nil
	} else if err != nil {
		return PipelineFeedConfig{}, err
	}
	return PipelineFeedConfig{Enabled: row.Enabled, URL: row.URL, IntervalMinutes: row.IntervalMinutes, HasToken: row.AuthToken != ""}, nil
}
func (s *WafPipelineFeedService) Save(c PipelineFeedConfig, operator string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Feed == nil {
		return errors.New("feed service is not ready")
	}
	if c.IntervalMinutes < 1 || c.IntervalMinutes > 1440 {
		return errors.New("interval must be 1..1440 minutes")
	}
	if c.URL != "" {
		u, err := url.Parse(c.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("请配置不含凭据的HTTPS订阅地址")
		}
		if ok, _ := utils.PrecheckOutboundURL(c.URL); !ok {
			return errors.New("订阅地址不符合出站访问策略")
		}
	}
	if c.Enabled && c.URL == "" {
		return errors.New("启用前必须设置HTTPS地址")
	}
	if c.Enabled {
		keys, err := licensing.ParseKeyring(licensing.TrustedKeysJSON)
		if err != nil || len(keys) == 0 || !wafreg.HasFeature(wafpipeline.FeedFeature) {
			return errors.New("需要threat_feed授权和非空可信公钥库")
		}
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var row model.PipelineFeed
	err = db.Where("id = ?", "signed-feed").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row.BaseOrm = pipelineORM()
		row.Id = "signed-feed"
	} else if err != nil {
		return err
	}
	changed := row.URL != c.URL
	row.Enabled = c.Enabled
	row.URL = c.URL
	row.IntervalMinutes = c.IntervalMinutes
	if c.Token != "" {
		row.AuthToken = c.Token
	}
	if c.ClearToken {
		row.AuthToken = ""
	}
	if changed {
		row.SnapshotJSON = ""
		if c.Token == "" {
			row.AuthToken = ""
		}
	}
	keys, err := licensing.ParseKeyring(licensing.TrustedKeysJSON)
	if err != nil {
		return err
	}
	provider := &wafpipeline.HTTPSFeedProvider{URL: row.URL, Token: row.AuthToken, Client: utils.SafeOutboundHTTPClient(30 * time.Second)}
	if err = s.Feed.ConfigureTransaction(row.Enabled, provider, keys, time.Duration(row.IntervalMinutes)*time.Minute, changed, func() error { return db.Save(&row).Error }); err != nil {
		return err
	}
	WafPipelineServiceApp.audit(operator, 0, "signed_feed_config_saved")
	return nil
}
func (s *WafPipelineFeedService) Sync(ctx context.Context) error {
	if s.Feed == nil {
		return errors.New("feed service is not ready")
	}
	return s.Feed.Sync(ctx, true)
}
