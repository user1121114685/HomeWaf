package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafpipeline"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WafPipelineGateService struct {
	mu    sync.Mutex
	Store *wafpipeline.GateStore
	owner string // immutable after Load, before the persistence worker starts
}

var errGateOwnerChanged = errors.New("gate persistence belongs to a newer worker")

var WafPipelineGateServiceApp = &WafPipelineGateService{Store: wafpipeline.NewGateStore()}

func (s *WafPipelineGateService) Ready() bool { return s.owner != "" }

func gateAccount(u model.GateUser) wafpipeline.GateAccount {
	version := u.AuthVersion
	if version == "" {
		seed := u.PasswordHash
		if u.TOTPSecret != "" {
			seed += "\x00totp=" + u.TOTPSecret
		}
		h := sha256.Sum256([]byte(seed))
		version = hex.EncodeToString(h[:])
	}
	return wafpipeline.GateAccount{PortID: u.PortID, Username: u.Username, PasswordHash: u.PasswordHash, TOTPSecret: u.TOTPSecret, Version: version, Enabled: u.Enabled, ExpiresAt: u.ExpiresAt}
}
func (s *WafPipelineGateService) Reload(port uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reload(port)
}
func (s *WafPipelineGateService) reload(port uint16) error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var users []model.GateUser
	if err = db.Where("port_id = ?", wafpipeline.PortID(port)).Find(&users).Error; err != nil {
		return err
	}
	accounts := make([]wafpipeline.GateAccount, 0, len(users))
	for _, u := range users {
		accounts = append(accounts, gateAccount(u))
	}
	s.Store.ReplaceAccounts(wafpipeline.PortID(port), accounts)
	return nil
}
func (s *WafPipelineGateService) Load() error {
	owner, err := pipelineSecret()
	if err != nil {
		return err
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var ports []int
	if err = db.Model(&model.PortConfig{}).Order("port").Limit(wafpipeline.Ports.Limit()).Pluck("port", &ports).Error; err != nil {
		return err
	}
	var issues []error
	for _, p := range ports {
		if p < 1 || p > 65535 {
			continue
		}
		if err = s.Reload(uint16(p)); err != nil {
			issues = append(issues, fmt.Errorf("gate accounts for port %d could not be loaded", p))
		}
	}
	var rows []model.GateSession
	if err = db.Transaction(func(tx *gorm.DB) error {
		var state model.GateRuntime
		found := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, 1).Error
		if found != nil && !errors.Is(found, gorm.ErrRecordNotFound) {
			return found
		}
		if state.CleanShutdown {
			if e := tx.Where("revoked = ? AND expires_at > ?", false, time.Now().Unix()).Limit(20000).Find(&rows).Error; e != nil {
				return e
			}
		} else {
			if e := tx.Where("1 = 1").Delete(&model.GateSession{}).Error; e != nil {
				return e
			}
		}
		return tx.Save(&model.GateRuntime{ID: 1, CleanShutdown: false, Owner: owner}).Error
	}); err != nil {
		return err
	}
	sessions := make([]wafpipeline.GateSessionRecord, 0, len(rows))
	for _, r := range rows {
		sessions = append(sessions, wafpipeline.GateSessionRecord{ID: r.Id, PortID: r.PortID, Username: r.Username, TokenHash: r.TokenHash, Version: r.AuthVersion, ClientIP: r.ClientIP, CreatedAt: r.CreatedAt, LastSeen: r.LastSeen, ExpiresAt: r.ExpiresAt})
	}
	s.owner = owner
	s.Store.Restore(sessions)
	s.Store.Persist = s.persist
	return errors.Join(issues...)
}

// Call only after listeners have drained and the persistence worker has stopped.
func (s *WafPipelineGateService) Close() error {
	if s.owner == "" {
		return nil
	}
	if err := s.Store.Flush(); err != nil {
		if errors.Is(err, errGateOwnerChanged) {
			return nil
		}
		return err
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	return db.Model(&model.GateRuntime{}).Where("id = ? AND owner = ?", 1, s.owner).Update("clean_shutdown", true).Error
}
func (s *WafPipelineGateService) persist(rows []wafpipeline.GateSessionRecord) error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var runtime model.GateRuntime
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&runtime, 1).Error; err != nil {
			return err
		}
		if s.owner == "" || runtime.Owner != s.owner {
			return errGateOwnerChanged
		}
		if err := tx.Where("1 = 1").Delete(&model.GateSession{}).Error; err != nil {
			return err
		}
		models := make([]model.GateSession, 0, len(rows))
		now := time.Now().Unix()
		for _, r := range rows {
			if r.ExpiresAt <= now {
				continue
			}
			v := model.GateSession{BaseOrm: pipelineORM(), PortID: r.PortID, Username: r.Username, TokenHash: r.TokenHash, AuthVersion: r.Version, ClientIP: r.ClientIP, CreatedAt: r.CreatedAt, LastSeen: r.LastSeen, ExpiresAt: r.ExpiresAt}
			v.Id = r.ID
			models = append(models, v)
		}
		if len(models) > 0 {
			return tx.CreateInBatches(models, 100).Error
		}
		return nil
	})
}

type GateUserSaveRequest struct {
	Port       uint16 `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	TOTPSecret string `json:"totp_secret,omitempty"`
	ClearTOTP  bool   `json:"clear_totp,omitempty"`
	Enabled    bool   `json:"enabled"`
	ExpiresAt  int64  `json:"expires_at"`
	Remark     string `json:"remark"`
}

func validateGateUsername(name string) error {
	if name == "" || strings.TrimSpace(name) != name || len(name) > 128 || strings.ContainsAny(name, "\r\n\t\x00") {
		return errors.New("账号不能为空、含控制字符或超过128字节")
	}
	return nil
}

func (s *WafPipelineGateService) Users(port uint16) ([]model.GateUser, error) {
	db, err := pipelineDB()
	if err != nil {
		return nil, err
	}
	rows := []model.GateUser{}
	err = db.Where("port_id = ?", wafpipeline.PortID(port)).Order("username").Find(&rows).Error
	for i := range rows {
		rows[i].TOTPConfigured = rows[i].TOTPSecret != ""
	}
	return rows, err
}
func (s *WafPipelineGateService) Save(req GateUserSaveRequest, operator string) error {
	WafPipelineServiceApp.mu.Lock()
	defer WafPipelineServiceApp.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateGateUsername(req.Username); err != nil {
		return err
	}
	if req.Username == "" || strings.TrimSpace(req.Username) != req.Username || len(req.Username) > 128 || strings.ContainsAny(req.Username, "\r\n\t\x00") || len(req.Remark) > 500 || req.ExpiresAt < 0 {
		return errors.New("账号字段不合法")
	}
	cfg, err := WafPipelineServiceApp.Get(req.Port)
	if err != nil {
		return err
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var u model.GateUser
	err = db.Where("port_id = ? AND username = ?", wafpipeline.PortID(req.Port), req.Username).First(&u).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}
	if isNew {
		var n int64
		if err = db.Model(&model.GateUser{}).Where("port_id = ?", wafpipeline.PortID(req.Port)).Count(&n).Error; err != nil {
			return err
		}
		if n >= 1000 {
			return errors.New("该端口账号数量已达上限")
		}
		u = model.GateUser{BaseOrm: pipelineORM(), PortID: wafpipeline.PortID(req.Port), Username: req.Username}
	}
	if req.Password != "" || isNew {
		if err = ValidateGatePassword(req.Username, req.Password); err != nil {
			return err
		}
		u.PasswordHash, err = utils.BcryptHash(req.Password)
		if err != nil {
			return err
		}
	}
	if req.ClearTOTP {
		u.TOTPSecret = ""
	}
	if req.TOTPSecret != "" {
		u.TOTPSecret, err = wafpipeline.NormalizeTOTPSecret(req.TOTPSecret)
		if err != nil {
			return err
		}
	}
	if u.TOTPSecret != "" && cfg.Policy.Gate.Mode == "basic" && req.Enabled && (req.ExpiresAt == 0 || req.ExpiresAt > time.Now().Unix()) {
		return errors.New("动态口令账号须使用表单登录模式")
	}
	if cfg.Policy.Gate.Enabled && (!req.Enabled || req.ExpiresAt > 0 && req.ExpiresAt <= time.Now().Unix()) {
		var n int64
		if err = db.Model(&model.GateUser{}).Where("port_id = ? AND username <> ? AND enabled = ? AND (expires_at = 0 OR expires_at > ?)", u.PortID, u.Username, true, time.Now().Unix()).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return errors.New("请先关闭登录门，不能停用最后一个有效账号")
		}
	}
	u.Enabled = req.Enabled
	u.ExpiresAt = req.ExpiresAt
	u.Remark = req.Remark
	u.AuthVersion, err = pipelineSecret()
	if err != nil {
		return err
	}
	if err = db.Save(&u).Error; err != nil {
		return err
	}
	if err = s.reload(req.Port); err != nil {
		return err
	}
	if err = s.Store.Flush(); err != nil {
		return err
	}
	WafPipelineServiceApp.audit(operator, req.Port, "gate_account_saved")
	return nil
}
func (s *WafPipelineGateService) Delete(port uint16, username, operator string) error {
	WafPipelineServiceApp.mu.Lock()
	defer WafPipelineServiceApp.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := WafPipelineServiceApp.Get(port)
	if err != nil {
		return err
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	if cfg.Policy.Gate.Enabled {
		var n int64
		if err = db.Model(&model.GateUser{}).Where("port_id = ? AND username <> ? AND enabled = ? AND (expires_at = 0 OR expires_at > ?)", wafpipeline.PortID(port), username, true, time.Now().Unix()).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return errors.New("请先关闭登录门，不能删除最后一个有效账号")
		}
	}
	if err = db.Where("port_id = ? AND username = ?", wafpipeline.PortID(port), username).Delete(&model.GateUser{}).Error; err != nil {
		return err
	}
	if err = s.reload(port); err != nil {
		return err
	}
	if err = s.Store.Flush(); err != nil {
		return err
	}
	WafPipelineServiceApp.audit(operator, port, "gate_account_deleted")
	return nil
}
func (s *WafPipelineGateService) Kick(port uint16, id, operator string) error {
	if !s.Store.Revoke(wafpipeline.PortID(port), id) {
		return errors.New("会话不存在或已失效")
	}
	if err := s.Store.Flush(); err != nil {
		return err
	}
	WafPipelineServiceApp.audit(operator, port, "gate_session_revoked")
	return nil
}
