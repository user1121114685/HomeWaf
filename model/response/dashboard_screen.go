package response

import "HomeWaf/model"

type ScreenSummary struct {
	Visits    int64 `json:"visits"`
	Blocked   int64 `json:"blocked"`
	Visitors  int64 `json:"visitors"`
	AttackIPs int64 `json:"attack_ips"`
}
type ScreenIP struct {
	IP      string `json:"ip"`
	Count   int64  `json:"count"`
	Country string `json:"country"`
}
type ScreenEvent struct {
	ID          string `json:"req_uuid" gorm:"column:req_uuid"`
	IP          string `json:"src_ip" gorm:"column:src_ip"`
	Country     string `json:"country"`
	Province    string `json:"province"`
	City        string `json:"city"`
	Host        string `json:"host"`
	Rule        string `json:"rule"`
	Action      string `json:"action"`
	CreatedTime string `json:"create_time" gorm:"column:create_time"`
	Time        int64  `json:"unix_add_time" gorm:"column:unix_add_time"`
}
type ScreenSnapshot struct {
	WindowStart    int64                 `json:"window_start"`
	WindowEnd      int64                 `json:"window_end"`
	Timezone       string                `json:"timezone"`
	Summary        ScreenSummary         `json:"summary"`
	IPs            []ScreenIP            `json:"ips"`
	Regions        []WafAnalysisDayStats `json:"regions"`
	Events         []ScreenEvent         `json:"events"`
	QPS            WafQpsTrend           `json:"qps"`
	Target         model.DashboardTarget `json:"target"`
	Partial        bool                  `json:"partial"`
	Issues         []string              `json:"issues"`
	LogMode        string                `json:"log_mode"`
	VisitorBasis   string                `json:"visitor_basis"`
	ShareExpiresAt int64                 `json:"share_expires_at,omitempty"`
}
