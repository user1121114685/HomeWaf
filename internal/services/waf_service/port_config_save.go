package waf_service

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/instance/wafreg"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/protection/wafpipeline"
	"gorm.io/gorm"
)

type PortSaveRequest struct {
	defenseWrite           bool
	accessIntent           bool
	defenseOwner           string
	Config                 wafpipeline.PortConfig                 `json:"config"`
	Username               string                                 `json:"initial_username,omitempty"`
	Password               string                                 `json:"initial_password,omitempty"`
	InitialTOTP            string                                 `json:"initial_totp_secret,omitempty"`
	TLSCert                string                                 `json:"tls_certificate,omitempty"`
	TLSKey                 string                                 `json:"tls_private_key,omitempty"`
	TLSCertificateID       string                                 `json:"tls_certificate_id,omitempty"`
	ConfirmListenerRestart bool                                   `json:"confirm_listener_restart,omitempty"`
	UpstreamCA             string                                 `json:"upstream_ca,omitempty"`
	ClearUpstreamCA        bool                                   `json:"clear_upstream_ca,omitempty"`
	BusinessHeaders        map[string]wafpipeline.BusinessHeaders `json:"business_headers,omitempty"`
	PluginProfiles         map[string]wafpipeline.PluginLaunch    `json:"plugin_profiles,omitempty"`
	PrivacyConfig          string                                 `json:"privacy_engine_config,omitempty"`
	ClearPrivacyConfig     bool                                   `json:"clear_privacy_engine_config,omitempty"`
	Operator               string                                 `json:"-"`
	TLSRefresh             *pipelineTLSRefresh                    `json:"-"`
	TLSBind                *pipelineTLSRefresh                    `json:"-"`
	TamperUpdates          map[string]wafpipeline.TamperBaseline  `json:"-"`
}

func (s *WafPipelineService) Save(req PortSaveRequest) (wafpipeline.PortConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(req)
}

func (s *WafPipelineService) saveLocked(req PortSaveRequest) (wafpipeline.PortConfig, error) {
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
		if !c.Policy.Defense.Enabled {
			return c, errors.New("接入服务前必须选择并启用一种异常访问处置方式")
		}
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
		if c.Protocol != oldSnap.Config.Protocol && oldSnap.Config.ListenEnabled && c.ListenEnabled && !req.ConfirmListenerRestart {
			return c, errors.New("更改监听协议将平滑重启该服务，请确认后保存")
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
	if req.TLSCertificateID != "" {
		if req.TLSKey != "" || req.TLSCert != "" {
			return c, errors.New("已有证书和手动 PEM 只能选择一种")
		}
		selected, err := selectedPipelineCertificate(req.TLSCertificateID)
		if err != nil {
			return c, err
		}
		if err := validatePipelineTLS(wafpipeline.PortSecrets{TLSCert: selected.Certificate, TLSKey: selected.PrivateKey}, c.HostNames); err != nil {
			return c, err
		}
		if err := bindCertificateSource(&secrets, selected); err != nil {
			return c, err
		}
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
	if req.TLSBind != nil {
		if err := bindCertificateSource(&secrets, *req.TLSBind); err != nil {
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
	if c.Protocol == "https" && (isNew || req.TLSCertificateID != "" || req.TLSCert != "" || req.TLSKey != "" || oldSnap != nil && (oldSnap.Config.Protocol != "https" || !oldSnap.Config.ListenEnabled && c.ListenEnabled || !reflect.DeepEqual(oldSnap.Config.HostNames, c.HostNames))) {
		if err := validatePipelineTLS(secrets, c.HostNames); err != nil {
			return c, err
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
		if err := activeGateUsers(db, wafpipeline.PortID(c.Port)).Limit(1001).Find(&accounts).Error; err != nil {
			return c, err
		}
		for _, account := range accounts {
			if account.TOTPSecret != "" {
				return c, errors.New("该端口包含动态口令账号，不能改为Basic登录")
			}
		}
	}
	var users int64
	if err = activeGateUsers(db, wafpipeline.PortID(c.Port)).Count(&users).Error; err != nil {
		return c, err
	}
	if c.Policy.Gate.Enabled && users == 0 && req.Username == "" {
		return c, errors.New("登录门启用前必须建立至少一个账号")
	}
	if c.ListenEnabled && c.Policy.Gate.RequireToken && len(secrets.ServiceTokenHashes) == 0 {
		return c, errors.New("服务令牌认证开始监听前必须生成令牌；请先保存为未监听，再到登录门设置生成令牌")
	}
	if oldSnap != nil {
		if !req.defenseWrite {
			c.DefenseState = oldSnap.Config.DefenseState
			modeChanged := defenseModeChanged(oldSnap.Config.Policy.Defense, c.Policy.Defense)
			// Parameter edits never lose an automatic reopen. Changing modes while
			// automatically closed takes effect after that existing closure ends.
			if modeChanged && (c.DefenseState == nil || c.DefenseState.Mode != wafpipeline.DefenseClose) || c.Policy.Access.Enabled != oldSnap.Config.Policy.Access.Enabled || !c.ListenEnabled {
				c.DefenseState = nil
			}
		}
		c.AccessChangedAt = oldSnap.Config.AccessChangedAt
		c.AccessChangedBy = oldSnap.Config.AccessChangedBy
	}
	if isNew {
		c.DefenseState = nil
	}
	if !req.defenseWrite {
		if err := validateDefenseAvailability(c); err != nil {
			return c, err
		}
	}
	if isNew || req.accessIntent || oldSnap != nil && oldSnap.Config.Policy.Access.Enabled != c.Policy.Access.Enabled {
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
	wasLockingDSM := oldSnap != nil && oldSnap.Config.Policy.RuleSet.LocksDSMLogins()
	locksDSM := c.Policy.RuleSet.LocksDSMLogins()
	if isNew || !wasLockingDSM && locksDSM {
		if err := WafPipelineStateServiceApp.Punisher.ClearSynologyLock(c.Port); err != nil {
			return c, err
		}
	}
	change := portConfigChange{next: row}
	if !isNew {
		change.previous = &oldRow
	}
	if err := change.commit(db, req); err != nil {
		return c, err
	}
	if err = wafpipeline.Ports.Publish(c.Port, func() (*wafpipeline.Snapshot, error) { return snap, nil }); err != nil {
		if restoreErr := change.restore(db); restoreErr != nil {
			return c, fmt.Errorf("配置发布失败且回滚失败: %w", restoreErr)
		}
		return c, err
	}
	if s.OnChange != nil && req.TamperUpdates == nil {
		if err = s.OnChange(oldSnap, snap); err != nil {
			// Publish before opening the socket; failed binds restore the previous policy.
			if restoreErr := change.restore(db); restoreErr != nil {
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
			return c, err
		}
	}
	if wasLockingDSM && !locksDSM {
		_ = WafPipelineStateServiceApp.Punisher.ClearSynologyLock(c.Port)
	}
	s.auditDetail(req.Operator, c.Port, "port_config_saved", portChangeSummary(oldSnap, snap))
	if req.TamperUpdates == nil && !req.defenseWrite {
		if err := WafPipelineGateServiceApp.Reload(c.Port); err != nil {
			return c, err
		}
	}
	delete(s.issues, c.Port)
	if oldSnap == nil || defensePolicyChanged(oldSnap.Config.Policy.Defense, c.Policy.Defense) || oldSnap.Config.DefenseState != nil && c.DefenseState == nil {
		WafPipelineDefenseServiceApp.Engine.Reset(c.Port)
	}
	return wafpipeline.CloneConfig(c), nil
}
