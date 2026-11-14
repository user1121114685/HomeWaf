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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type WafPipelineStateService struct {
	Punisher             *wafpipeline.Punisher
	persistMu            sync.Mutex
	persisted            map[string]model.PipelineState
	metricsMu            sync.Mutex
	metrics              map[uint16]*wafpipeline.PortMetrics
	metricsPersistMu     sync.Mutex
	metricsSaved         map[uint16]wafpipeline.DailyMetrics
	MetricsPersistErrors atomic.Uint64
}

var WafPipelineStateServiceApp = &WafPipelineStateService{Punisher: wafpipeline.NewPunisher()}

func (s *WafPipelineStateService) Load() error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var models []model.PipelineState
	if err = db.Limit(20001).Find(&models).Error; err != nil {
		return err
	}
	rows := make([]wafpipeline.PunishmentRecord, 0, len(models))
	for _, row := range models {
		var record wafpipeline.PunishmentRecord
		if err := wafpipeline.StrictDecode([]byte(row.DataJSON), &record); err != nil {
			return errors.New("punishment state is invalid")
		}
		if row.ID != record.ID || row.PortID != record.PortID || row.Kind != record.Kind {
			return errors.New("punishment state identity mismatch")
		}
		rows = append(rows, record)
	}
	if err = s.Punisher.Restore(rows); err != nil {
		return err
	}
	s.Punisher.Persist = s.persist
	s.Punisher.OnChange = s.recordChange
	s.Punisher.OnEvent = func(e wafpipeline.HitEvent) {
		if e.RuleID != "http_parser" || global.GQEQUE_LOG_DB == nil {
			return
		}
		raw, _ := json.Marshal(e)
		global.GQEQUE_LOG_DB.Enqueue(&model.PipelineLog{ID: uuid.GenUUID(), ReqID: e.ReqID, PortID: e.PortID, TimeMillis: time.Now().UnixMilli(), Kind: "protocol", ClientIP: e.ClientIP.String(), Stage: 3, Verdict: "deny", RuleID: e.RuleID, Category: e.Category, Severity: string(e.Severity), Observe: true, Reason: e.Evidence, DetailJSON: string(raw)})
	}
	return nil
}
func (s *WafPipelineStateService) persist(rows []wafpipeline.PunishmentRecord) error {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.persisted == nil {
		var stored []model.PipelineState
		if err := db.WithContext(ctx).Limit(20001).Find(&stored).Error; err != nil {
			return err
		}
		s.persisted = make(map[string]model.PipelineState, len(stored))
		for _, row := range stored {
			s.persisted[row.ID] = row
		}
	}
	next := make(map[string]model.PipelineState, len(rows))
	changed := make([]model.PipelineState, 0)
	for _, r := range rows {
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		row := model.PipelineState{ID: r.ID, PortID: r.PortID, Kind: r.Kind, DataJSON: string(raw)}
		next[row.ID] = row
		if old, ok := s.persisted[row.ID]; !ok || old != row {
			changed = append(changed, row)
		}
	}
	removed := make([]string, 0)
	for id := range s.persisted {
		if _, ok := next[id]; !ok {
			removed = append(removed, id)
		}
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for offset := 0; offset < len(removed); offset += 400 {
			if err := tx.Where("id IN ?", removed[offset:min(offset+400, len(removed))]).Delete(&model.PipelineState{}).Error; err != nil {
				return err
			}
		}
		if len(changed) > 0 {
			return tx.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(changed, 100).Error
		}
		return nil
	})
	if err == nil {
		s.persisted = next
	}
	return err
}
func (s *WafPipelineStateService) recordChange(change wafpipeline.PunishmentChange) {
	r := change.Record
	raw, _ := json.Marshal(change)
	if global.GQEQUE_LOG_DB != nil {
		global.GQEQUE_LOG_DB.Enqueue(&model.PipelineLog{ID: uuid.GenUUID(), PortID: r.PortID, TimeMillis: time.Now().UnixMilli(), Kind: "state", ClientIP: r.Prefix, Stage: 4, Verdict: r.Kind, Reason: change.Event, RuleID: r.RuleID, Category: r.Reason, DetailJSON: string(raw)})
	}
	// Existing notification channels remain opt-in and process this asynchronously.
	if global.GQEQUE_MESSAGE_DB != nil && (change.Event == "lockdown_started" || change.Event == "lockdown_released" || change.Event == "honeypot_lockdown_started" || change.Event == "honeypot_lockdown_released" || change.Event == "released" && (r.Kind == "lockdown" || r.Kind == "honeypot_lockdown")) {
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.SystemErrorMessageInfo{BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "HomeWaf port protection", Server: r.PortID}, ErrorType: "pipeline_lockdown", ErrorMsg: fmt.Sprintf("%s: %s", r.PortID, change.Event)})
	}
}
func (s *WafPipelineStateService) LogResult(snapshot *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	s.diagnosticAlert(snapshot, c, result)
	if global.GQEQUE_LOG_DB == nil {
		return
	}
	raw, _ := json.Marshal(result)
	entry := &model.PipelineLog{ID: uuid.GenUUID(), ReqID: c.ReqID, PortID: c.PortID, TimeMillis: time.Now().UnixMilli(), Kind: "request", ClientIP: c.ClientIP.String(), Stage: result.Verdict.Stage, Verdict: string(result.Verdict.Action), RuleID: result.Verdict.RuleID, Observe: result.Verdict.Observe, Reason: result.Verdict.Reason, ElapsedMicros: time.Since(c.Started).Microseconds(), DetailJSON: string(raw)}
	if len(result.Hits) > 0 {
		e := result.Hits[0]
		entry.Category = e.Category
		entry.Severity = string(e.Severity)
		entry.Observe = e.Observe
	}
	global.GQEQUE_LOG_DB.Enqueue(entry)
}

// A fixed atomic value on the existing per-port metrics object bounds both
// memory and notifications. No network or database operation occurs here.
func (s *WafPipelineStateService) diagnosticAlert(snapshot *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	if c.DryRun || global.GQEQUE_MESSAGE_DB == nil || snapshot == nil || snapshot.Compiled == nil || snapshot.Compiled.Metrics == nil {
		return
	}
	category := ""
	protected, _ := c.Values["protected_source"].(bool)
	for _, event := range result.Hits {
		if !event.Observe && (event.Trusted || protected) {
			category = "protected_match"
		}
		switch event.Category {
		case "protected_match", "detector_error", "detector_timeout", "plugin_error":
			category = event.Category
		}
		if category != "" {
			break
		}
	}
	if category == "" {
		return
	}
	clock := &snapshot.Compiled.Metrics.DiagnosticAlertAt
	now := time.Now().Unix()
	previous := clock.Load()
	if previous != 0 && now-previous < 60 {
		return
	}
	if !clock.CompareAndSwap(previous, now) {
		return
	}
	global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.SystemErrorMessageInfo{BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "HomeWaf port diagnostic", Server: c.PortID}, ErrorType: "pipeline_" + category, ErrorMsg: fmt.Sprintf("%s: %s; diagnostic only, no automatic punishment", c.PortID, category)})
}
func (s *WafPipelineStateService) Manage(port uint16, id, action, operator string) error {
	portID := wafpipeline.PortID(port)
	if action == "whitelist" {
		var record *wafpipeline.PunishmentRecord
		for _, r := range s.Punisher.Records(portID) {
			if r.ID == id {
				copy := r
				record = &copy
				break
			}
		}
		if record == nil || record.Prefix == "" {
			return errors.New("IP state record not found")
		}
		config, err := WafPipelineServiceApp.Get(port)
		if err != nil {
			return err
		}
		config.Policy.IPGroup.Entries = append(config.Policy.IPGroup.Entries, wafpipeline.IPEntry{ID: uuid.GenUUID(), Type: "allow", Trust: "trusted", Value: record.Prefix, Remark: "Released from automatic protection"})
		if _, err := WafPipelineServiceApp.Save(PortSaveRequest{Config: config, Operator: operator}); err != nil {
			return err
		}
		action = "release"
	}
	if err := s.Punisher.Manage(portID, id, action); err != nil {
		return err
	}
	WafPipelineServiceApp.audit(operator, port, "punishment_"+action)
	return nil
}
func (s *WafPipelineStateService) Logs(port uint16) ([]model.PipelineLog, error) {
	return s.logs(port, false)
}

// HoneypotLogs filters before limiting, so unrelated traffic cannot hide the
// latest twenty honeypot requests. Existing log retention remains in effect.
func (s *WafPipelineStateService) HoneypotLogs(port uint16) ([]model.PipelineLog, error) {
	return s.logs(port, true)
}

func (s *WafPipelineStateService) logs(port uint16, honeypotOnly bool) ([]model.PipelineLog, error) {
	if global.GWAF_LOCAL_LOG_DB == nil {
		return nil, errors.New("log database unavailable")
	}
	rows := []model.PipelineLog{}
	query := global.GWAF_LOCAL_LOG_DB.Where("port_id = ?", wafpipeline.PortID(port))
	limit := 100
	if honeypotOnly {
		query = query.Where("kind = ? AND (verdict = ? OR (verdict = ? AND reason LIKE ?))", "request", string(wafpipeline.Honeypot), string(wafpipeline.Drop), "honeypot:drop:%")
		limit = 20
	}
	err := query.Order("time_millis DESC").Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// Retain a finite event history; the existing log database rotation also bounds
// the file. Run off the request path alongside punishment persistence.
func (s *WafPipelineStateService) PruneLogs(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	metricsTicker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	defer metricsTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-metricsTicker.C:
			if err := s.PersistMetrics(); err != nil {
				s.MetricsPersistErrors.Add(1)
			}
		case <-ticker.C:
			if db := global.GWAF_LOCAL_LOG_DB; db != nil {
				db.WithContext(ctx).Where("time_millis < ?", time.Now().Add(-30*24*time.Hour).UnixMilli()).Delete(&model.PipelineLog{})
			}
			if db, err := pipelineDB(); err == nil {
				day := time.Now().AddDate(0, 0, -30)
				cutoff := day.Year()*10000 + int(day.Month())*100 + day.Day()
				db.WithContext(ctx).Where("day < ?", cutoff).Delete(&model.PipelineDailyMetrics{})
			}
		}
	}
}

func pipelineStateSummary(rows []wafpipeline.PunishmentRecord) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		out[strings.ToLower(r.Kind)]++
	}
	return out
}

func (s *WafPipelineStateService) LoadMetrics() error {
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	var rows []model.PipelineDailyMetrics
	if err = db.Where("day = ? AND port IN (?)", wafpipeline.Today(), db.Model(&model.PortConfig{}).Select("port")).Limit(wafpipeline.Ports.Limit()).Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		s.MetricsForPort(r.Port).RestoreDaily(wafpipeline.DailyMetrics{Day: r.Day, Requests: r.Requests, Denied: r.Denied, Hits: r.Hits})
	}
	return nil
}
func (s *WafPipelineStateService) MetricsForPort(port uint16) *wafpipeline.PortMetrics {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	if s.metrics == nil {
		s.metrics = map[uint16]*wafpipeline.PortMetrics{}
	}
	m := s.metrics[port]
	if m == nil {
		if len(s.metrics) >= wafpipeline.Ports.Limit() {
			for old := range s.metrics {
				if _, active := wafpipeline.Ports.Get(old); !active {
					delete(s.metrics, old)
				}
			}
		}
		m = &wafpipeline.PortMetrics{}
		if len(s.metrics) < wafpipeline.Ports.Limit() {
			s.metrics[port] = m
		}
	}
	return m
}
func (s *WafPipelineStateService) PersistMetrics() error {
	s.metricsPersistMu.Lock()
	defer s.metricsPersistMu.Unlock()
	if s.metricsSaved == nil {
		s.metricsSaved = map[uint16]wafpipeline.DailyMetrics{}
	}
	rows := []model.PipelineDailyMetrics{}
	ports := wafpipeline.Ports.Ports()
	active := make(map[uint16]bool, len(ports))
	for _, port := range ports {
		active[port] = true
	}
	for port := range s.metricsSaved {
		if !active[port] {
			delete(s.metricsSaved, port)
		}
	}
	for _, port := range ports {
		snapshot, ok := wafpipeline.Ports.Get(port)
		if !ok || snapshot.Compiled == nil || snapshot.Compiled.Metrics == nil {
			continue
		}
		d := snapshot.Compiled.Metrics.Daily()
		if s.metricsSaved[port] == d {
			continue
		}
		rows = append(rows, model.PipelineDailyMetrics{Port: port, Day: d.Day, Requests: d.Requests, Denied: d.Denied, Hits: d.Hits})
	}
	if len(rows) == 0 {
		return nil
	}
	db, err := pipelineDB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "port"}, {Name: "day"}}, DoUpdates: clause.AssignmentColumns([]string{"requests", "denied", "hits"})}).CreateInBatches(rows, 100).Error
	if err == nil {
		for _, r := range rows {
			s.metricsSaved[r.Port] = wafpipeline.DailyMetrics{Day: r.Day, Requests: r.Requests, Denied: r.Denied, Hits: r.Hits}
		}
	}
	return err
}
