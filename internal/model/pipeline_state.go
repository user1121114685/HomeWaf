package model

// PipelineState stores one atomic punishment record. Runtime reads never query
// this table; the worker periodically replaces the owned set in a transaction.
type PipelineState struct {
	ID       string `gorm:"size:200;primaryKey" json:"id"`
	PortID   string `gorm:"size:32;index" json:"port_id"`
	Kind     string `gorm:"size:32" json:"kind"`
	DataJSON string `gorm:"type:text" json:"-"`
}

func (PipelineState) TableName() string { return "pipeline_state" }

type PipelineDailyMetrics struct {
	Port     uint16 `gorm:"primaryKey"`
	Day      int    `gorm:"primaryKey"`
	Requests uint64
	Denied   uint64
	Hits     uint64
}

func (PipelineDailyMetrics) TableName() string { return "pipeline_daily_metrics" }

type PipelineLog struct {
	ID            string `gorm:"size:64;primaryKey" json:"id"`
	ReqID         string `gorm:"size:64;index" json:"req_id"`
	PortID        string `gorm:"size:32;index:idx_pipeline_log_port_time,priority:1" json:"port_id"`
	TimeMillis    int64  `gorm:"index:idx_pipeline_log_port_time,priority:2" json:"time_ms"`
	Kind          string `gorm:"size:32" json:"kind"`
	ClientIP      string `gorm:"size:64" json:"client_ip"`
	Stage         int    `json:"stage"`
	Verdict       string `gorm:"size:24" json:"verdict"`
	RuleID        string `gorm:"size:128" json:"rule_id"`
	Category      string `gorm:"size:64" json:"category"`
	Severity      string `gorm:"size:16" json:"severity"`
	Observe       bool   `json:"observe"`
	Reason        string `gorm:"size:300" json:"reason"`
	ElapsedMicros int64  `json:"elapsed_us"`
	DetailJSON    string `gorm:"type:text" json:"detail_json"`
}

func (PipelineLog) TableName() string   { return "pipeline_log" }
func (l *PipelineLog) QueueKey() string { return l.PortID }
func (l *PipelineLog) QueuePriority() int {
	if l.Kind == "state" || l.Category != "" || l.Verdict == "deny" || l.Verdict == "honeypot" {
		return 2
	}
	return 0
}
func (l *PipelineLog) QueueSize() int64 { return int64(1024 + len(l.DetailJSON)) }
