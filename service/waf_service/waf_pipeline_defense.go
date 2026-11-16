package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WafPipelineDefenseService struct {
	Owner       string
	Engine      *wafpipeline.DefenseEngine
	Ports       *WafPipelineService
	tickMu      sync.Mutex
	OwnsRuntime func() bool
}

var WafPipelineDefenseServiceApp = &WafPipelineDefenseService{Engine: wafpipeline.NewDefenseEngine()}

var defenseHistoryMu sync.Mutex

func defenseTransactionOwner(tx *gorm.DB, owner string) error {
	if owner == "" {
		return nil
	}
	var lease model.GateRuntime
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lease, "id = ?", 1).Error; err != nil {
		return err
	}
	if lease.Owner != owner {
		return errors.New("端口处置运行权已移交，旧进程不能再修改状态")
	}
	return nil
}

func (s *WafPipelineDefenseService) ports() *WafPipelineService {
	if s.Ports != nil {
		return s.Ports
	}
	return WafPipelineServiceApp
}

func (s *WafPipelineDefenseService) Ready() bool { return s.OwnsRuntime == nil || s.OwnsRuntime() }

func (s *WafPipelineDefenseService) Status(c wafpipeline.PortConfig, ready bool) wafpipeline.DefenseStatus {
	out := s.Engine.Status(&wafpipeline.Snapshot{Config: c})
	if out.Mode == wafpipeline.DefenseHoney && out.Enabled && !out.HoneypotReady {
		out.Error = "统一蜜罐未就绪，安全拒绝请求保持拒绝且不回源"
	}
	if d := c.Policy.Defense; !ready && d != nil && d.Enabled && d.Mode != wafpipeline.DefenseHoney {
		out.Error = "自动处置运行权未就绪，请检查实例运行与数据库状态"
	}
	return out
}

func defensePolicyChanged(a, b *wafpipeline.DefenseConfig) bool {
	if a == nil || b == nil {
		return a != b
	}
	return *a != *b
}

func defenseModeChanged(a, b *wafpipeline.DefenseConfig) bool {
	if a == nil || b == nil {
		return a != b
	}
	return a.Mode != b.Mode || a.Enabled != b.Enabled
}

func validateDefenseAvailability(c wafpipeline.PortConfig) error {
	d := c.Policy.Defense
	if d == nil || !d.Enabled {
		return nil
	}
	if d.Mode != wafpipeline.DefenseHoney && !WafPipelineDefenseServiceApp.Ready() {
		return errors.New("自动处置运行权未就绪，暂不能启用")
	}
	if d.Mode == wafpipeline.DefenseHoney && !wafpipeline.SharedHoneypotReady() {
		return errors.New("请先在统一蜜罐页面保存设置，再启用蜜罐模式")
	}
	if d.Mode == wafpipeline.DefenseQuiet {
		if len(c.Policy.Network.TrustedProxies) != 0 {
			return errors.New("前置代理可能复用连接，无法按真实访客执行TCP拒绝；请选择其他处置模式")
		}
		if d.TCPAction == "drop" {
			return defenseDropCapability(c.Policy.Network.ListenIPVersion)
		}
	}
	return nil
}

func (s *WafPipelineDefenseService) LoadHistory() error {
	db, err := pipelineDB()
	if err != nil {
		return s.Engine.LoadHistory(nil, err)
	}
	var rows []model.PipelineSuccessSource
	err = db.Where("last_seen > ?", s.Engine.Now().Unix()-wafpipeline.SuccessHistorySeconds).Limit(wafpipeline.MaxTotalSuccessPrefixes + wafpipeline.Ports.Limit() + 1).Find(&rows).Error
	values := make([]wafpipeline.SuccessPrefix, 0, len(rows))
	for _, row := range rows {
		values = append(values, wafpipeline.SuccessPrefix{Port: row.Port, Prefix: row.Prefix, LastSeen: row.LastSeen})
	}
	return s.Engine.LoadHistory(values, err)
}

func (s *WafPipelineDefenseService) PersistHistory() error {
	defenseHistoryMu.Lock()
	defer defenseHistoryMu.Unlock()
	if s.Engine.NeedsHistoryLoad() {
		if err := s.LoadHistory(); err != nil {
			return err
		}
	}
	rows := s.Engine.DirtyHistory()
	db, err := pipelineDB()
	if err != nil {
		s.Engine.HistorySaved(rows, err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := defenseTransactionOwner(tx, s.Owner); err != nil {
			return err
		}
		values := make([]model.PipelineSuccessSource, 0, len(rows))
		for _, row := range rows {
			values = append(values, model.PipelineSuccessSource{Port: row.Port, Prefix: row.Prefix, LastSeen: row.LastSeen})
		}
		if len(values) > 0 {
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "port"}, {Name: "prefix"}}, DoUpdates: clause.AssignmentColumns([]string{"last_seen"})}).CreateInBatches(values, 100).Error; err != nil {
				return err
			}
		}
		return tx.Where("last_seen <= ?", s.Engine.Now().Unix()-wafpipeline.SuccessHistorySeconds).Delete(&model.PipelineSuccessSource{}).Error
	})
	s.Engine.HistorySaved(rows, err)
	return err
}

func (s *WafPipelineDefenseService) Activate(t wafpipeline.DefenseTransition) error {
	ports := s.ports()
	ports.mu.Lock()
	defer ports.mu.Unlock()
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var row model.PortConfig
	if err := db.First(&row, "port = ?", t.Port).Error; err != nil {
		return err
	}
	snap, err := decodePort(row)
	if err != nil {
		return err
	}
	c := snap.Config
	if c.Revision != t.Revision || c.Policy.Defense == nil || *c.Policy.Defense != t.Config || !c.Policy.Defense.Enabled || c.DefenseState != nil || !c.Policy.Access.Enabled || !c.ListenEnabled {
		s.Engine.Reset(t.Port)
		return nil // Stale observations never close a newly edited/replaced service.
	}
	if t.State.ExpiresAt <= s.Engine.Now().Unix() {
		s.Engine.Reset(t.Port)
		return nil
	}
	c.DefenseState = &t.State
	if t.State.Mode == wafpipeline.DefenseClose {
		c.Policy.Access.Enabled = false
	}
	_, err = ports.saveLocked(PortSaveRequest{Config: c, Operator: "system:defense", defenseWrite: true, defenseOwner: s.Owner})
	if err == nil {
		s.Engine.Reset(t.Port)
		s.record(c, "defense_started")
	}
	return err
}

type DefenseAccessRequest struct {
	Port     uint16 `json:"port"`
	Revision int64  `json:"revision"`
	StateID  string `json:"state_id"`
	Action   string `json:"action"`
}

// Explicit close also claims ownership when the boolean is already false.
// This is how "keep closed" cancels a previously scheduled automatic reopen.
func (s *WafPipelineDefenseService) Access(req DefenseAccessRequest, operator string) (wafpipeline.PortConfig, error) {
	return s.access(req, operator, "")
}

func (s *WafPipelineDefenseService) access(req DefenseAccessRequest, operator, owner string) (wafpipeline.PortConfig, error) {
	ports := s.ports()
	ports.mu.Lock()
	defer ports.mu.Unlock()
	if req.Port == 0 || req.Revision < 1 || req.Action != "open" && req.Action != "close" && req.Action != "release" {
		return wafpipeline.PortConfig{}, errors.New("无效的访问操作")
	}
	db, err := pipelineDB()
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	var row model.PortConfig
	if err := db.First(&row, "port = ?", req.Port).Error; err != nil {
		return wafpipeline.PortConfig{}, err
	}
	snap, err := decodePort(row)
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	c := snap.Config
	if c.Revision != req.Revision {
		return c, errors.New("服务状态已变化，请刷新后重试")
	}
	if req.Action == "release" && (c.DefenseState == nil || c.DefenseState.ID != req.StateID) {
		return c, errors.New("该次处置已结束或已被替换，请刷新状态")
	}
	if req.Action == "open" {
		c.Policy.Access.Enabled = true
	}
	if req.Action == "close" {
		c.Policy.Access.Enabled = false
	}
	if req.Action == "release" && c.DefenseState.Mode == wafpipeline.DefenseClose && c.DefenseState.AccessWasEnabled {
		c.Policy.Access.Enabled = true
	}
	previousState := c.DefenseState
	accessIntent := req.Action == "open" || req.Action == "close" || c.DefenseState != nil && c.DefenseState.Mode == wafpipeline.DefenseClose
	c.DefenseState = nil
	result, err := ports.saveLocked(PortSaveRequest{Config: c, Operator: operator, defenseWrite: true, accessIntent: accessIntent, defenseOwner: owner})
	if err == nil {
		s.Engine.Reset(req.Port)
		eventConfig := result
		eventConfig.DefenseState = previousState
		s.record(eventConfig, "defense_"+req.Action)
	}
	return result, err
}

func (s *WafPipelineDefenseService) Tick() {
	s.tickMu.Lock()
	defer s.tickMu.Unlock()
	if s.OwnsRuntime != nil && !s.OwnsRuntime() {
		return
	}
	for _, t := range s.Engine.Pending() {
		if err := s.Activate(t); err != nil {
			s.Engine.TransitionFailed(t.Port, err)
		}
	}
	for _, port := range wafpipeline.Ports.Ports() {
		snap, ok := wafpipeline.Ports.Get(port)
		if !ok || snap.Config.DefenseState == nil || snap.Config.DefenseState.ExpiresAt > s.Engine.Now().Unix() {
			continue
		}
		_, err := s.access(DefenseAccessRequest{Port: port, Revision: snap.Config.Revision, StateID: snap.Config.DefenseState.ID, Action: "release"}, "system:defense_expiry", s.Owner)
		if err != nil {
			s.Engine.TransitionFailed(port, err)
		}
	}
}

func (s *WafPipelineDefenseService) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	persist := time.NewTicker(2 * time.Second)
	defer persist.Stop()
	for {
		select {
		case <-ctx.Done():
			if s.OwnsRuntime == nil || s.OwnsRuntime() {
				_ = s.PersistHistory()
			}
			return
		case <-tick.C:
			s.Tick()
		case <-persist.C:
			if s.OwnsRuntime == nil || s.OwnsRuntime() {
				_ = s.PersistHistory()
			}
		}
	}
}

func (s *WafPipelineDefenseService) record(c wafpipeline.PortConfig, event string) {
	raw, _ := json.Marshal(c.DefenseState)
	if global.GQEQUE_LOG_DB != nil {
		global.GQEQUE_LOG_DB.Enqueue(&model.PipelineLog{ID: uuid.GenUUID(), PortID: wafpipeline.PortID(c.Port), TimeMillis: time.Now().UnixMilli(), Kind: "state", Verdict: "defense", Reason: event, DetailJSON: string(raw)})
	}
	if global.GQEQUE_MESSAGE_DB != nil {
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.SystemErrorMessageInfo{BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "HomeWaf port protection", Server: wafpipeline.PortID(c.Port)}, ErrorType: "pipeline_defense", ErrorMsg: fmt.Sprintf("%s: %s", c.Name, event)})
	}
}
