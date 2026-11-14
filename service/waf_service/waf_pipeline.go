package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafpipeline"
	"HomeWaf/wafreg"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type WafPipelineService struct {
	mu     sync.Mutex
	issues map[uint16]string
	// Set during startup, before management starts accepting requests.
	OnChange          func(old, next *wafpipeline.Snapshot) error
	Prepare           func(*wafpipeline.Snapshot) error
	NormalizeBaseline func([]byte, string, string) ([]byte, string, error)
	LegacyPlugins     func() ([]string, map[string]wafpipeline.PluginLaunch)
}

var WafPipelineServiceApp = &WafPipelineService{}

func pipelineORM() baseorm.BaseOrm {
	now := customtype.JsonTime(time.Now())
	return baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: now, UPDATE_TIME: now}
}
func pipelineDB() (*gorm.DB, error) {
	if global.GWAF_LOCAL_DB == nil {
		return nil, errors.New("configuration database unavailable")
	}
	return global.GWAF_LOCAL_DB, nil
}
func pipelineSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func ValidateGatePassword(username, password string) error {
	return accessgate.ValidatePassword(username, password)
}
func (s *WafPipelineService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues = map[uint16]string{}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var ports []int
	if err := db.Model(&model.PortConfig{}).Order("port").Limit(wafpipeline.Ports.Limit()+1).Pluck("port", &ports).Error; err != nil {
		return err
	}
	var failures []error
	for _, port := range ports {
		if port < 1 || port > 65535 {
			failures = append(failures, fmt.Errorf("invalid stored port %d", port))
			continue
		}
		var row model.PortConfig
		if err := db.Where("port = ?", port).First(&row).Error; err != nil {
			s.issues[uint16(port)] = "端口记录或密钥无法读取"
			failures = append(failures, fmt.Errorf("port %d could not be loaded", port))
			continue
		}
		snap, err := decodePort(row)
		if err != nil {
			s.issues[uint16(port)] = err.Error()
			failures = append(failures, fmt.Errorf("port %d configuration: %w", row.Port, err))
			continue
		}
		previous, _ := wafpipeline.Ports.Get(snap.Config.Port)
		if s.Prepare != nil {
			if err = s.Prepare(snap); err != nil {
				wafpipeline.CloseRetiredProxy(snap, previous)
				s.issues[uint16(port)] = err.Error()
				failures = append(failures, fmt.Errorf("port %d prepare: %w", port, err))
				continue
			}
		}
		if err = wafpipeline.Ports.Publish(snap.Config.Port, func() (*wafpipeline.Snapshot, error) { return snap, nil }); err != nil {
			current, _ := wafpipeline.Ports.Get(snap.Config.Port)
			wafpipeline.CloseRetiredProxy(snap, current)
			s.issues[uint16(port)] = err.Error()
			failures = append(failures, err)
		} else {
			wafpipeline.CloseRetiredProxy(previous, snap)
		}
	}
	return errors.Join(failures...)
}

func (s *WafPipelineService) Issues() map[uint16]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := map[uint16]string{}
	for port, message := range s.issues {
		copy[port] = message
	}
	return copy
}
func decodePort(row model.PortConfig) (*wafpipeline.Snapshot, error) {
	var c wafpipeline.PortConfig
	var secrets wafpipeline.PortSecrets
	if err := wafpipeline.StrictDecode([]byte(row.ConfigJSON), &c); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(row.SecretsJSON), &secrets); err != nil {
		return nil, errors.New("port secret unavailable")
	}
	if err := wafpipeline.ValidateConfig(c, uint16(global.GWAF_LOCAL_SERVER_PORT)); err != nil {
		return nil, err
	}
	if c.Port != uint16(row.Port) {
		return nil, errors.New("port identity mismatch")
	}
	c.Revision = row.Revision
	return &wafpipeline.Snapshot{Config: c, Secrets: secrets, PublishedAt: time.Now()}, nil
}
func (s *WafPipelineService) List() ([]wafpipeline.PortConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.issues == nil {
		s.issues = map[uint16]string{}
	}
	db, err := pipelineDB()
	if err != nil {
		return nil, err
	}
	var rows []model.PortConfig
	if err = db.Omit("secrets_json").Order("port").Limit(wafpipeline.Ports.Limit()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]wafpipeline.PortConfig, 0, len(rows))
	for _, row := range rows {
		var config wafpipeline.PortConfig
		err := wafpipeline.StrictDecode([]byte(row.ConfigJSON), &config)
		if err == nil {
			err = wafpipeline.ValidateConfig(config, uint16(global.GWAF_LOCAL_SERVER_PORT))
		}
		if err == nil && config.Port != uint16(row.Port) {
			err = errors.New("port identity mismatch")
		}
		if err != nil {
			s.issues[uint16(row.Port)] = err.Error()
			continue
		}
		config.Revision = row.Revision
		out = append(out, config)
	}
	return out, nil
}
func (s *WafPipelineService) Get(port uint16) (wafpipeline.PortConfig, error) {
	if snap, ok := wafpipeline.Ports.Get(port); ok {
		return wafpipeline.CloneConfig(snap.Config), nil
	}
	db, err := pipelineDB()
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	var row model.PortConfig
	if err = db.Where("port = ?", port).First(&row).Error; err != nil {
		return wafpipeline.PortConfig{}, err
	}
	snap, err := decodePort(row)
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	return snap.Config, nil
}

type PortSaveRequest struct {
	Config             wafpipeline.PortConfig                 `json:"config"`
	Username           string                                 `json:"initial_username,omitempty"`
	Password           string                                 `json:"initial_password,omitempty"`
	InitialTOTP        string                                 `json:"initial_totp_secret,omitempty"`
	TLSCert            string                                 `json:"tls_certificate,omitempty"`
	TLSKey             string                                 `json:"tls_private_key,omitempty"`
	UpstreamCA         string                                 `json:"upstream_ca,omitempty"`
	ClearUpstreamCA    bool                                   `json:"clear_upstream_ca,omitempty"`
	BusinessHeaders    map[string]wafpipeline.BusinessHeaders `json:"business_headers,omitempty"`
	PluginProfiles     map[string]wafpipeline.PluginLaunch    `json:"plugin_profiles,omitempty"`
	PrivacyConfig      string                                 `json:"privacy_engine_config,omitempty"`
	ClearPrivacyConfig bool                                   `json:"clear_privacy_engine_config,omitempty"`
	Operator           string                                 `json:"-"`
	TLSRefresh         *pipelineTLSRefresh                    `json:"-"`
	TamperUpdates      map[string]wafpipeline.TamperBaseline  `json:"-"`
}

func (s *WafPipelineService) Save(req PortSaveRequest) (wafpipeline.PortConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pipelineGroupReferenceMu.RLock()
	defer pipelineGroupReferenceMu.RUnlock()
	c := wafpipeline.CloneConfig(req.Config)
	if err := wafpipeline.ValidateConfig(c, uint16(global.GWAF_LOCAL_SERVER_PORT)); err != nil {
		return c, err
	}
	if c.Policy.IPGroup.FeedEnabled && !wafreg.HasFeature(wafpipeline.FeedFeature) {
		return c, errors.New("需要threat_feed授权")
	}
	db, err := pipelineDB()
	if err != nil {
		return c, err
	}
	var oldRow model.PortConfig
	if err := validatePipelineGroups(db, c.Policy.IPGroup); err != nil {
		return c, err
	}
	find := db.Where("port = ?", c.Port).First(&oldRow).Error
	isNew := errors.Is(find, gorm.ErrRecordNotFound)
	if find != nil && !isNew {
		return c, find
	}
	var oldSnap *wafpipeline.Snapshot
	secrets := wafpipeline.PortSecrets{}
	if isNew {
		var count int64
		if err = db.Model(&model.PortConfig{}).Count(&count).Error; err != nil {
			return c, err
		}
		if count >= int64(wafpipeline.Ports.Limit()) {
			return c, fmt.Errorf("端口数量已达%d上限", wafpipeline.Ports.Limit())
		}
		if c.ListenEnabled {
			return c, errors.New("新建端口必须先保存并预览，再启用")
		}
		key, e := pipelineSecret()
		if e != nil {
			return c, e
		}
		secrets.GateKey = key
		c.Revision = 1
		c.HostCode = "pipeline-" + strconv.Itoa(int(c.Port))
	} else {
		oldSnap, err = decodePort(oldRow)
		if err != nil {
			return c, err
		}
		if published, ok := wafpipeline.Ports.Get(c.Port); ok && published.Config.Revision == oldRow.Revision {
			oldSnap = published
		} else if s.Prepare != nil {
			if err = s.Prepare(oldSnap); err != nil {
				return c, err
			}
		}
		if c.Revision != oldRow.Revision {
			return c, errors.New("配置已被修改，请刷新后重试")
		}
		if c.Protocol != oldSnap.Config.Protocol && oldSnap.Config.ListenEnabled {
			return c, errors.New("更改协议前请先停止该端口")
		}
		if c.Policy.Network.ListenIPVersion != oldSnap.Config.Policy.Network.ListenIPVersion && oldSnap.Config.ListenEnabled {
			return c, errors.New("更改IP监听范围前请先停止该端口")
		}
		if oldSnap.Config.ListenEnabled && (c.Policy.Network.MaxHeaderBytes != oldSnap.Config.Policy.Network.MaxHeaderBytes || c.Policy.Network.ReadHeaderSeconds != oldSnap.Config.Policy.Network.ReadHeaderSeconds || c.Policy.Network.MaxInFlight != oldSnap.Config.Policy.Network.MaxInFlight || c.Policy.Network.DisableHTTP2 != oldSnap.Config.Policy.Network.DisableHTTP2) {
			return c, errors.New("更改监听限制或HTTP/2设置前请先停止该端口")
		}
		secrets = wafpipeline.CloneSecrets(oldSnap.Secrets)
		c.HostCode = oldSnap.Config.HostCode
		merge := func(current, previous []string, foldCase bool) []string {
			for _, value := range previous {
				found := false
				for _, existing := range current {
					if existing == value || (foldCase && strings.EqualFold(existing, value)) {
						found = true
						break
					}
				}
				if !found && value != "" {
					current = append(current, value)
				}
			}
			return current
		}
		c.Policy.Gate.CredentialCookiePrefixes = merge(c.Policy.Gate.CredentialCookiePrefixes, oldSnap.Config.Policy.Gate.CredentialCookiePrefixes, false)
		c.Policy.Gate.CredentialHeaders = merge(c.Policy.Gate.CredentialHeaders, oldSnap.Config.Policy.Gate.CredentialHeaders, true)
		if oldSnap.Config.Policy.Gate.TokenHeader != c.Policy.Gate.TokenHeader {
			c.Policy.Gate.CredentialHeaders = merge(c.Policy.Gate.CredentialHeaders, []string{oldSnap.Config.Policy.Gate.TokenHeader}, true)
		}
		if len(c.Policy.Gate.CredentialCookiePrefixes) > 32 || len(c.Policy.Gate.CredentialHeaders) > 32 {
			return c, errors.New("历史凭据命名空间超过上限")
		}
		c.Revision++
	}
	if len(req.BusinessHeaders) > 0 {
		if secrets.BusinessHeaders == nil {
			secrets.BusinessHeaders = map[string]wafpipeline.BusinessHeaders{}
		}
		for key, profile := range req.BusinessHeaders {
			secrets.BusinessHeaders[key] = profile
		}
		secrets = wafpipeline.CloneSecrets(secrets)
	}
	if len(req.PluginProfiles) > 0 {
		if secrets.PluginProfiles == nil {
			secrets.PluginProfiles = map[string]wafpipeline.PluginLaunch{}
		}
		for id, profile := range req.PluginProfiles {
			secrets.PluginProfiles[id] = profile
		}
	}
	// Removing an ID also discards its encrypted launch configuration, so
	// replacing plugins cannot accumulate stale secrets or exhaust the quota.
	activePlugins := map[string]bool{}
	if p := c.Policy.RuleSet.Plugins; p != nil {
		for _, id := range p.IDs {
			activePlugins[id] = true
		}
	}
	for id := range secrets.PluginProfiles {
		if !activePlugins[id] {
			delete(secrets.PluginProfiles, id)
		}
	}
	if err := wafpipeline.ValidatePluginProfiles(secrets.PluginProfiles); err != nil {
		return c, err
	}
	if len(req.PrivacyConfig) > 256<<10 {
		return c, errors.New("隐私引擎配置超过256KiB")
	}
	if req.ClearPrivacyConfig {
		secrets.PrivacyConfig = ""
	}
	if req.PrivacyConfig != "" {
		secrets.PrivacyConfig = req.PrivacyConfig
	}
	if err := wafpipeline.ValidateBusinessHeaders(secrets.BusinessHeaders); err != nil {
		return c, err
	}
	if req.TLSKey != "" || req.TLSCert != "" {
		if req.TLSKey == "" || req.TLSCert == "" {
			return c, errors.New("证书与私钥须同时提交")
		}
		secrets.TLSKey = req.TLSKey
		secrets.TLSCert = req.TLSCert
		secrets.TLSBindings = nil
	}
	if req.TLSRefresh != nil {
		if err := refreshManagedTLS(&secrets, *req.TLSRefresh); err != nil {
			return c, err
		}
	}
	if len(req.TamperUpdates) > 0 {
		if secrets.TamperBaselines == nil {
			secrets.TamperBaselines = map[string]wafpipeline.TamperBaseline{}
		}
		for i, rule := range c.Policy.Network.Response.Tamper {
			if baseline, ok := req.TamperUpdates[rule.ID]; ok {
				if baseline.Binding != wafpipeline.TamperBinding(rule) {
					return c, errors.New("防篡改策略已变化，拒绝旧采样")
				}
				secrets.TamperBaselines[rule.ID] = baseline
				c.Policy.Network.Response.Tamper[i].LearnedAt = baseline.LearnedAt
			}
		}
	}
	activeBaselines := map[string]bool{}
	for _, rule := range c.Policy.Network.Response.Tamper {
		activeBaselines[rule.ID] = true
	}
	for id := range secrets.TamperBaselines {
		if !activeBaselines[id] {
			delete(secrets.TamperBaselines, id)
		}
	}
	if err := wafpipeline.ValidateTamperBaselines(secrets.TamperBaselines); err != nil {
		return c, err
	}
	if req.ClearUpstreamCA {
		secrets.UpstreamCAPEM = ""
	}
	if req.UpstreamCA != "" {
		if len(req.UpstreamCA) > 256<<10 {
			return c, errors.New("上游CA证书超过256KiB")
		}
		secrets.UpstreamCAPEM = req.UpstreamCA
	}
	if c.Protocol == "https" && c.ListenEnabled && (secrets.TLSKey == "" || secrets.TLSCert == "") {
		return c, errors.New("HTTPS端口必须设置证书和私钥")
	}
	if secrets.TLSKey != "" || secrets.TLSCert != "" {
		if _, err := tls.X509KeyPair([]byte(secrets.TLSCert), []byte(secrets.TLSKey)); err != nil {
			return c, errors.New("证书或私钥不合法")
		}
	}
	if c.ListenEnabled && s.OnChange == nil {
		return c, errors.New("监听管理器尚未就绪")
	}
	if req.Username != "" || req.Password != "" {
		if !isNew {
			return c, errors.New("请使用账号管理接口添加账号")
		}
		if err = ValidateGatePassword(req.Username, req.Password); err != nil {
			return c, err
		}
		if err = validateGateUsername(req.Username); err != nil {
			return c, err
		}
	}
	if req.InitialTOTP != "" {
		if !isNew || req.Username == "" {
			return c, errors.New("初始动态口令只用于新建账号")
		}
		req.InitialTOTP, err = wafpipeline.NormalizeTOTPSecret(req.InitialTOTP)
		if err != nil {
			return c, err
		}
	}
	if c.Policy.Gate.Mode == "basic" {
		if req.InitialTOTP != "" {
			return c, errors.New("动态口令须使用表单登录")
		}
		var accounts []model.GateUser
		if err := db.Where("port_id = ? AND enabled = ? AND (expires_at = 0 OR expires_at > ?)", wafpipeline.PortID(c.Port), true, time.Now().Unix()).Limit(1001).Find(&accounts).Error; err != nil {
			return c, err
		}
		for _, account := range accounts {
			if account.TOTPSecret != "" {
				return c, errors.New("该端口包含动态口令账号，不能改为Basic登录")
			}
		}
	}
	var users int64
	if err = db.Model(&model.GateUser{}).Where("port_id = ? AND enabled = ? AND (expires_at = 0 OR expires_at > ?)", wafpipeline.PortID(c.Port), true, time.Now().Unix()).Count(&users).Error; err != nil {
		return c, err
	}
	if c.Policy.Gate.Enabled && users == 0 && req.Username == "" {
		return c, errors.New("登录门启用前必须建立至少一个账号")
	}
	if c.ListenEnabled && c.Policy.Gate.RequireToken && len(secrets.ServiceTokenHashes) == 0 {
		return c, errors.New("服务令牌认证开始监听前必须生成令牌；请先保存为未监听，再到登录门设置生成令牌")
	}
	if isNew {
		var hosts []model.Hosts
		if err = db.Where("global_host = ?", 0).Find(&hosts).Error; err != nil {
			return c, err
		}
		for _, h := range hosts {
			for _, l := range utils.ResolveHostListens(h) {
				if l.Port == int(c.Port) && !wafpipeline.OwnsLegacyPort(c.Port) {
					return c, errors.New("端口已属于现有站点，请选择未占用端口")
				}
			}
		}
	}
	if oldSnap != nil {
		c.AccessChangedAt = oldSnap.Config.AccessChangedAt
		c.AccessChangedBy = oldSnap.Config.AccessChangedBy
	}
	if isNew || oldSnap != nil && oldSnap.Config.Policy.Access.Enabled != c.Policy.Access.Enabled {
		c.AccessChangedAt = time.Now().Unix()
		c.AccessChangedBy = req.Operator
	}
	snap := &wafpipeline.Snapshot{Config: c, Secrets: secrets, PublishedAt: time.Now()}
	if s.Prepare != nil {
		if err = s.Prepare(snap); err != nil {
			return c, err
		}
	}
	raw, _ := json.Marshal(c)
	defer func() {
		current, _ := wafpipeline.Ports.Get(c.Port)
		if current != snap {
			wafpipeline.CloseRetiredProxy(snap, current)
		}
	}()
	secretRaw, _ := json.Marshal(secrets)
	row := oldRow
	if isNew {
		row.BaseOrm = pipelineORM()
		row.Port = int(c.Port)
	}
	row.ConfigJSON = string(raw)
	row.SecretsJSON = string(secretRaw)
	row.Revision = c.Revision
	row.UPDATE_TIME = customtype.JsonTime(time.Now())
	var migration model.PipelinePortMigration
	hasMigration := false
	if db.Migrator().HasTable(&model.PipelinePortMigration{}) {
		e := db.First(&migration, "port = ?", c.Port).Error
		if e == nil {
			hasMigration = true
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return c, e
		}
	}
	previousMigration := migration
	previousOwner := wafpipeline.OwnsLegacyPort(c.Port)
	createdUserID := ""
	err = db.Transaction(func(tx *gorm.DB) error {
		if isNew {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			result := tx.Model(&model.PortConfig{}).Where("port = ? AND revision = ?", c.Port, oldRow.Revision).Select("ConfigJSON", "SecretsJSON", "Revision", "UPDATE_TIME").Updates(&row)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("配置已被其他进程修改，请刷新后重试")
			}
		}
		if req.Username != "" {
			hash, err := utils.BcryptHash(req.Password)
			if err != nil {
				return err
			}
			u := model.GateUser{BaseOrm: pipelineORM(), PortID: wafpipeline.PortID(c.Port), Username: req.Username, PasswordHash: hash, TOTPSecret: req.InitialTOTP, Enabled: true}
			if err := tx.Create(&u).Error; err != nil {
				return err
			}
			createdUserID = u.Id
		}
		if hasMigration && c.ListenEnabled && migration.State != "active" {
			migration.State = "active"
			if err := tx.Save(&migration).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c, err
	}
	restoreData := func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			var result *gorm.DB
			if isNew {
				result = tx.Where("port = ? AND revision = ?", c.Port, c.Revision).Delete(&model.PortConfig{})
			} else {
				result = tx.Model(&model.PortConfig{}).Where("port = ? AND revision = ?", c.Port, c.Revision).Select("ConfigJSON", "SecretsJSON", "Revision", "UPDATE_TIME").Updates(&oldRow)
			}
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("配置已被较新请求修改，回滚不会覆盖新版本")
			}
			if createdUserID != "" {
				if e := tx.Where("id = ?", createdUserID).Delete(&model.GateUser{}).Error; e != nil {
					return e
				}
			}
			if hasMigration {
				return tx.Save(&previousMigration).Error
			}
			return nil
		})
	}
	if err = wafpipeline.Ports.Publish(c.Port, func() (*wafpipeline.Snapshot, error) { return snap, nil }); err != nil {
		if restoreErr := restoreData(); restoreErr != nil {
			return c, fmt.Errorf("配置发布失败且回滚失败: %w", restoreErr)
		}
		return c, err
	}
	if hasMigration && migration.State == "active" {
		wafpipeline.SetLegacyPortOwner(c.Port, true)
	}
	if s.OnChange != nil && req.TamperUpdates == nil {
		if err = s.OnChange(oldSnap, snap); err != nil {
			// Publish before opening the socket; failed binds restore the previous policy.
			if restoreErr := restoreData(); restoreErr != nil {
				if current, ok := wafpipeline.Ports.Get(c.Port); ok && current.Config.Revision == c.Revision {
					wafpipeline.Ports.Remove(c.Port)
				}
				return c, fmt.Errorf("监听失败，配置未覆盖较新版本或数据库回滚失败: %w", restoreErr)
			}
			if oldSnap != nil {
				_ = wafpipeline.Ports.Publish(c.Port, func() (*wafpipeline.Snapshot, error) { return oldSnap, nil })
			} else {
				wafpipeline.Ports.Remove(c.Port)
			}
			wafpipeline.CloseRetiredProxy(snap, oldSnap)
			wafpipeline.SetLegacyPortOwner(c.Port, previousOwner)
			return c, err
		}
	}
	s.auditDetail(req.Operator, c.Port, "port_config_saved", portChangeSummary(oldSnap, snap))
	if req.TamperUpdates == nil {
		if err := WafPipelineGateServiceApp.Reload(c.Port); err != nil {
			return c, err
		}
	}
	delete(s.issues, c.Port)
	return c, nil
}
func (s *WafPipelineService) Delete(port uint16, operator string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var row model.PortConfig
	if err = db.Where("port = ?", port).First(&row).Error; err != nil {
		return err
	}
	snap, err := decodePort(row)
	if err != nil {
		return err
	}
	if snap.Config.ListenEnabled {
		return errors.New("请先停止端口再删除")
	}
	if s.OnChange != nil {
		if err = s.OnChange(snap, nil); err != nil {
			return err
		}
	}
	if err = db.Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&model.GateUser{}, &model.GateSession{}} {
			if err := tx.Where("port_id = ?", wafpipeline.PortID(port)).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&row).Error
	}); err != nil {
		return err
	}
	wafpipeline.Ports.Remove(port)
	if err := WafPipelineGateServiceApp.Reload(port); err != nil {
		return err
	}
	s.audit(operator, port, "port_deleted")
	return nil
}
func (s *WafPipelineService) audit(operator string, port uint16, event string) {
	WafSecurityAuditServiceApp.Write(AuditEntry{Event: event, AccountName: operator, HostCode: wafpipeline.PortID(port), Message: "端口配置变更", Result: model.AccessAuditOK})
}
func (s *WafPipelineService) Templates() ([]wafpipeline.Template, error) {
	db, err := pipelineDB()
	if err != nil {
		return nil, err
	}
	out := wafpipeline.BuiltinTemplates()
	var rows []model.ConfigTemplate
	if err = db.Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		var t wafpipeline.Template
		if err = wafpipeline.StrictDecode([]byte(r.ConfigJSON), &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
func (s *WafPipelineService) Template(id string) (wafpipeline.Template, error) {
	all, err := s.Templates()
	if err != nil {
		return wafpipeline.Template{}, err
	}
	for _, t := range all {
		if t.ID == id {
			return t, nil
		}
	}
	return wafpipeline.Template{}, gorm.ErrRecordNotFound
}
func (s *WafPipelineService) SaveTemplate(t wafpipeline.Template, operator string) (wafpipeline.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, builtin := range wafpipeline.BuiltinTemplates() {
		if t.ID == builtin.ID {
			return t, errors.New("内置模板只读")
		}
	}
	t.Builtin = false
	if err := wafpipeline.ValidateTemplate(t); err != nil {
		return t, err
	}
	db, err := pipelineDB()
	if err != nil {
		return t, err
	}
	var row model.ConfigTemplate
	err = db.Where("code = ?", t.ID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row.BaseOrm = pipelineORM()
		row.Code = t.ID
		t.Version = 1
	} else if err != nil {
		return t, err
	} else {
		if row.Version != t.Version {
			return t, errors.New("模板已更新，请刷新")
		}
		t.Version++
	}
	data, _ := json.Marshal(t)
	row.Name = t.Name
	row.Version = t.Version
	row.ConfigJSON = string(data)
	row.UPDATE_TIME = customtype.JsonTime(time.Now())
	if err = db.Save(&row).Error; err != nil {
		return t, err
	}
	s.auditDetail(operator, 0, "template_saved", fmt.Sprintf("template=%s; version=%d", t.ID, t.Version))
	return t, nil
}
func (s *WafPipelineService) DeleteTemplate(id, operator string) error {
	for _, t := range wafpipeline.BuiltinTemplates() {
		if t.ID == id {
			return errors.New("内置模板只读")
		}
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	if err = db.Where("code = ?", id).Delete(&model.ConfigTemplate{}).Error; err != nil {
		return err
	}
	s.auditDetail(operator, 0, "template_deleted", "template="+id)
	return nil
}
func (s *WafPipelineService) Copy(source, target uint16, includeIPs bool) (wafpipeline.PortConfig, error) {
	c, err := s.Get(source)
	if err != nil {
		return c, err
	}
	c.Port = target
	c.HostCode = ""
	c.ListenEnabled = false
	c.Revision = 0
	c.Template = wafpipeline.TemplateSource{}
	c.AccessChangedAt = 0
	c.AccessChangedBy = ""
	if !includeIPs {
		c.Policy.IPGroup.Entries = []wafpipeline.IPEntry{}
		c.Policy.IPGroup.AllowGroups = nil
		c.Policy.IPGroup.DenyGroups = nil
		c.Policy.Gate.ExemptIPs = nil
		c.ScriptGroups = nil
	}
	c.Policy.Gate.Enabled = false
	c.ACMEHosts = nil
	if c.Policy.RuleSet.Plugins != nil {
		c.Policy.RuleSet.Plugins.Mode = wafpipeline.ModeOff
	}
	if !includeIPs && c.Policy.RuleSet.Callback != nil {
		c.Policy.RuleSet.Callback.Networks = nil
	}
	for i := range c.Policy.Network.Response.Tamper {
		c.Policy.Network.Response.Tamper[i].LearnedAt = 0
	}
	c.Policy.Gate.CredentialCookiePrefixes = nil
	c.Policy.Gate.CredentialHeaders = nil
	c.HeaderProfile = ""
	for i := range c.Routes {
		c.Routes[i].HeaderProfile = ""
	}
	return c, nil
}
func (s *WafPipelineService) CheckUpstream(raw string) (string, error) {
	return s.CheckUpstreamDial(raw, "")
}
func (s *WafPipelineService) CheckUpstreamDial(raw, dialIP string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", errors.New("invalid upstream")
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	host := u.Hostname()
	if dialIP != "" {
		if net.ParseIP(dialIP) == nil {
			return "", errors.New("连接IP不合法")
		}
		host = dialIP
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return "", errors.New("无法连接配置的上游地址")
	}
	defer conn.Close()
	return "连接成功", nil
}

// CheckNewPort is an explicit wizard preflight, not a reservation. Save and
// listener startup repeat validation because other processes can bind later.
func (s *WafPipelineService) CheckNewPort(c wafpipeline.PortConfig) error {
	if err := wafpipeline.ValidateConfig(c, uint16(global.GWAF_LOCAL_SERVER_PORT)); err != nil {
		return err
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var count int64
	if err = db.Model(&model.PortConfig{}).Where("port = ?", c.Port).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return errors.New("端口已有独立配置")
	}
	var hosts []model.Hosts
	if err = db.Where("global_host = ?", 0).Find(&hosts).Error; err != nil {
		return err
	}
	for _, h := range hosts {
		for _, listen := range utils.ResolveHostListens(h) {
			if listen.Port == int(c.Port) {
				return errors.New("端口已属于现有站点")
			}
		}
	}
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(c.Port)))
	if err != nil {
		return errors.New("端口已被进程占用或不可监听")
	}
	return listener.Close()
}
func (s *WafPipelineService) SortedPorts() []uint16 {
	ports := wafpipeline.Ports.Ports()
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports
}
