package waf_service

import (
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/protection/wafpipeline"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if err = db.Limit(20001 + wafpipeline.Ports.Limit()).Find(&models).Error; err != nil {
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
		if err := db.WithContext(ctx).Limit(20001 + wafpipeline.Ports.Limit()).Find(&stored).Error; err != nil {
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
	if global.GQEQUE_MESSAGE_DB != nil && change.Event == "ban_created" && r.Kind == "ban" {
		duration := int(r.ExpiresAt - r.CreatedAt)
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.IPBanMessageInfo{
			BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "服务端口自动封禁", Server: r.PortID},
			Ip:              r.Prefix, Reason: r.Reason, Scope: r.PortID, Duration: (duration + 59) / 60, DurationSeconds: duration,
			RemainingSeconds: int(max(0, r.ExpiresAt-time.Now().Unix())), Time: time.Unix(r.CreatedAt, 0).Format(time.RFC3339),
		})
	}
	if global.GQEQUE_MESSAGE_DB != nil && change.Event == "synology_login_lockdown_started" {
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.SystemErrorMessageInfo{
			BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "群晖登录保护", Server: r.PortID},
			ErrorType:       "群晖登录已暂停",
			ErrorMsg:        fmt.Sprintf("%s 累计 %d 次 DSM 认证失败，已暂停该 NAS 所有用户的新登录。恢复时间 %s。管理员可在服务诊断的「当前处罚」中提前解除。", r.PortID, r.Hits, time.Unix(r.ExpiresAt, 0).Format(time.RFC3339)),
			Time:            time.Unix(r.CreatedAt, 0).Format(time.RFC3339),
		})
	}
}
func (s *WafPipelineStateService) LogResult(snapshot *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	s.diagnosticAlert(snapshot, c, result)
	if !c.DryRun && snapshot != nil && !snapshot.Preview && global.GQEQUE_MESSAGE_DB != nil {
		notifyPipelineRule(snapshot, c, result)
	}
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

func notifyPipelineRule(snapshot *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	v := result.Verdict
	for _, hit := range result.Hits {
		outcome := ""
		switch {
		case hit.Category == "tamper_detected":
			outcome = "响应内容与基线不一致"
		case v.RuleID != "" && hit.RuleID == v.RuleID && !hit.Observe && !v.Observe:
			switch v.Action {
			case wafpipeline.Deny, wafpipeline.Drop:
				outcome = "请求被拒绝"
			case wafpipeline.Honeypot:
				outcome = "请求已转入蜜罐"
			case wafpipeline.Challenge:
				outcome = "请求需要验证"
			}
		}
		if outcome == "" {
			continue
		}
		if v.Status > 0 {
			outcome += fmt.Sprintf("（HTTP %d）", v.Status)
		}
		service := strings.TrimSpace(snapshot.Config.Name + " " + c.PortID)
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.RuleMessageInfo{
			BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "命中保护规则", Server: service},
			Domain:          c.Request.Host, RuleInfo: hit.RuleID, Ip: c.ClientIP.String(),
			RequestID: c.ReqID, Time: hit.Time.Format(time.RFC3339), Outcome: outcome,
		})
		return
	}
}

// A fixed atomic value on the existing per-port metrics object bounds both
// memory and notifications. No network or database operation occurs here.
func (s *WafPipelineStateService) diagnosticAlert(snapshot *wafpipeline.Snapshot, c *wafpipeline.RequestContext, result wafpipeline.Result) {
	if c.DryRun || global.GQEQUE_MESSAGE_DB == nil || snapshot == nil || snapshot.Compiled == nil || snapshot.Compiled.Metrics == nil {
		return
	}
	category := ""
	protected, _ := c.Values["protected_source"].(bool)
	var hit wafpipeline.HitEvent
	for _, event := range result.Hits {
		if !event.Observe && (event.Trusted || protected) {
			category = "protected_match"
		}
		switch event.Category {
		case "protected_match", "detector_error", "detector_timeout", "plugin_error":
			category = event.Category
		}
		if category != "" {
			hit = event
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
	at := hit.Time
	if at.IsZero() {
		at = time.Now()
	}
	global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.SystemErrorMessageInfo{
		BaseMessageInfo: innerbean.BaseMessageInfo{OperaType: "HomeWaf port diagnostic", Server: c.PortID},
		ErrorType:       "pipeline_" + category, ErrorMsg: pipelineDiagnosticMessage(snapshot.Config.Name, c, category, hit, result),
		RequestID: c.ReqID, Time: at.Format(time.RFC3339),
	})
}

func pipelineDiagnosticMessage(service string, c *wafpipeline.RequestContext, category string, hit wafpipeline.HitEvent, result wafpipeline.Result) string {
	title := map[string]string{
		"detector_error": "检测器执行失败", "detector_timeout": "检测预算耗尽",
		"protected_match": "受保护来源命中规则", "plugin_error": "插件执行失败",
	}[category]
	var msg strings.Builder
	fmt.Fprintf(&msg, "%s · %s", c.PortID, title)
	if service != "" {
		fmt.Fprintf(&msg, "\n服务 %s", strings.Join(strings.Fields(service), " "))
	}
	if hit.Stage > 0 {
		fmt.Fprintf(&msg, "\n触发阶段 %d", hit.Stage)
	}
	if hit.RuleID != "" {
		fmt.Fprintf(&msg, "\n报告检测项 %s", strings.Join(strings.Fields(hit.RuleID), " "))
	}
	if f := result.BodyReadFailure; f != nil {
		cause := map[bodyinspect.ReadCause]string{
			bodyinspect.ReadTimeout: "读取超时", bodyinspect.RequestCanceled: "请求已取消",
			bodyinspect.PrematureEOF: "正文提前结束", bodyinspect.ConnectionReset: "连接已重置",
			bodyinspect.UnknownError: "原因未确定",
		}[f.Cause]
		fmt.Fprintf(&msg, "\n正文读取失败（%s），已读 %d 字节，耗时 %d ms", cause, f.BytesRead, f.ElapsedMS)
		if f.IdleMS != nil {
			fmt.Fprintf(&msg, "，距上次读到数据 %d ms", *f.IdleMS)
		}
	}
	v := result.Verdict
	if check := result.UnavailableCheck(); check != "" {
		fmt.Fprintf(&msg, "\n%s 检查失败，请求在转发前被拒绝", strings.Join(strings.Fields(check), " "))
	}
	if v.Action != "" {
		actions := map[wafpipeline.Action]string{
			wafpipeline.Allow: "放行", wafpipeline.Bypass: "跳过后续检测", wafpipeline.Continue: "继续处理",
			wafpipeline.Deny: "拒绝", wafpipeline.Drop: "断开连接", wafpipeline.Challenge: "要求验证",
			wafpipeline.Honeypot: "转入蜜罐", wafpipeline.Tarpit: "延迟响应",
		}
		fmt.Fprintf(&msg, "\n最终动作 %s", actions[v.Action])
	}
	if v.Status > 0 {
		fmt.Fprintf(&msg, "（HTTP %d）", v.Status)
	}
	fmt.Fprintf(&msg, "\n请求 ID %s", c.ReqID)
	return msg.String()
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
	query := global.GWAF_LOCAL_LOG_DB.Model(&model.PipelineLog{})
	if port != 0 || !honeypotOnly {
		query = query.Where("port_id = ?", wafpipeline.PortID(port))
	}
	limit := 100
	if honeypotOnly {
		query = query.Where("kind = ? AND verdict = ?", "request", string(wafpipeline.Honeypot))
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
