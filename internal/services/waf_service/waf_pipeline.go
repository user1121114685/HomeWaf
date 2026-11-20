package waf_service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/protection/wafenginecore/accessgate"
	"HomeWaf/internal/protection/wafpipeline"
	"gorm.io/gorm"
)

type WafPipelineService struct {
	mu     sync.Mutex
	issues map[uint16]string
	// Set during startup, before management starts accepting requests.
	OnChange func(old, next *wafpipeline.Snapshot) error
	Prepare  func(*wafpipeline.Snapshot) error
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
	return s.load(false)
}

// Reload reconciles listeners with persisted port policies while retaining a working
// snapshot whenever a replacement policy cannot be prepared or bound.
func (s *WafPipelineService) Reload() error {
	if s.OnChange == nil {
		return errors.New("服务监听管理器未就绪")
	}
	return s.load(true)
}

func (s *WafPipelineService) load(reconcile bool) error {
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
	known := make(map[uint16]bool, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			failures = append(failures, fmt.Errorf("invalid stored port %d", port))
			continue
		}
		known[uint16(port)] = true
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
			continue
		}
		if reconcile {
			if err = s.OnChange(previous, snap); err != nil {
				if previous == nil {
					wafpipeline.Ports.Remove(snap.Config.Port)
				} else {
					_ = wafpipeline.Ports.Publish(snap.Config.Port, func() (*wafpipeline.Snapshot, error) { return previous, nil })
				}
				wafpipeline.CloseRetiredProxy(snap, previous)
				s.issues[snap.Config.Port] = err.Error()
				failures = append(failures, fmt.Errorf("port %d reload: %w", port, err))
				continue
			}
		}
		wafpipeline.CloseRetiredProxy(previous, snap)
	}
	if reconcile && len(ports) <= wafpipeline.Ports.Limit() {
		for _, port := range wafpipeline.Ports.Ports() {
			if known[port] {
				continue
			}
			previous, _ := wafpipeline.Ports.Get(port)
			if err := s.OnChange(previous, nil); err != nil {
				failures = append(failures, err)
				continue
			}
			wafpipeline.Ports.Remove(port)
			wafpipeline.CloseRetiredProxy(previous, nil)
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
	if err := wafpipeline.ValidateDefenseState(c.DefenseState); err != nil {
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

func (s *WafPipelineService) Delete(port uint16, operator string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defenseHistoryMu.Lock()
	defer defenseHistoryMu.Unlock()
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
		if tx.Migrator().HasTable(&model.PipelineSuccessSource{}) {
			if err := tx.Where("port = ?", port).Delete(&model.PipelineSuccessSource{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&row).Error
	}); err != nil {
		return err
	}
	wafpipeline.Ports.Remove(port)
	WafPipelineDefenseServiceApp.Engine.Forget(port)
	_ = WafPipelineStateServiceApp.Punisher.ClearSynologyLock(port)
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
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(c.Port)))
	if err != nil {
		return errors.New("端口已被进程占用或不可监听")
	}
	return listener.Close()
}
