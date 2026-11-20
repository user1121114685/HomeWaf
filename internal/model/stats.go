// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package model

import (
	"HomeWaf/internal/model/baseorm"
)

/*
*
整体统计
*/
type StatsTotal struct {
	baseorm.BaseOrm
	AttackCount        int `json:"attack_count"`         //攻击数量
	VisitCount         int `json:"visit_count"`          //访问数量
	HistoryAttackCount int `json:"history_attack_count"` //历史攻击数量
	HistoryVisitCount  int `json:"history_visit_count"`  //历史访问数量
}

/*
*
按天统计和不同类型统计
*/
type StatsDay struct {
	baseorm.BaseOrm
	HostCode string `gorm:"size:64" json:"host_code"` //网站唯一码（主要键）
	Day      int    `json:"day"`                      //年月日（主要键）
	Host     string `gorm:"size:255" json:"host"`     //域名
	Type     string `gorm:"size:32" json:"type"`      //类型 放行,阻止
	Count    int    `json:"count"`                    //数量
}

/*
*
按天统计和不同类型统计IP
*/
type StatsIPDay struct {
	baseorm.BaseOrm
	HostCode string `gorm:"size:64" json:"host_code"` //网站唯一码（主要键）
	Day      int    `json:"day"`                      //年月日（主要键）
	Host     string `gorm:"size:255" json:"host"`     //域名
	IP       string `gorm:"size:64" json:"ip"`        //ip
	Type     string `gorm:"size:32" json:"type"`      //类型 放行,阻止
	Count    int    `json:"count"`                    //数量
}

/*
*
按天统计和不同类型统计城市
*/
type StatsIPCityDay struct {
	baseorm.BaseOrm
	HostCode string `gorm:"size:64" json:"host_code"` //网站唯一码（主要键）
	Day      int    `json:"day"`                      //年月日（主要键）
	Host     string `gorm:"size:255" json:"host"`     //域名
	Country  string `gorm:"size:100" json:"country"`  //国家
	Province string `gorm:"size:100" json:"province"` //省份
	City     string `gorm:"size:100" json:"city"`     //城市
	Type     string `gorm:"size:32" json:"type"`      //类型 放行,阻止
	Count    int    `json:"count"`                    //数量
}

/*
*
天数对应的数量[临时]
*/
type StatsDayCount struct {
	Day   int   `json:"day"`   //年月日
	Count int64 `json:"count"` //数量
}

/*
*
域名对应的数量[临时]
*/
type StatsIPCount struct {
	IP       string `json:"ip"`        //ip
	IPBelong string `json:"ip_belong"` //归属地
	Count    int64  `json:"count"`     //数量
}

/*
*
域名对应的数量丰富的标签内容
*/
type StatsIPCountMore struct {
	IP       string  `json:"ip"`        //ip
	IPBelong string  `json:"ip_belong"` //归属地
	Country  string  `json:"country"`
	Province string  `json:"province"`
	City     string  `json:"city"`
	IPTag    []IPTag `json:"ip_tags"` //IP标签
	Count    int64   `json:"count"`   //数量
}

/*
*
按天统计站点综合数据（天级聚合，永久保留）
*/
type StatsSiteDay struct {
	baseorm.BaseOrm
	HostCode       string `gorm:"size:64" json:"host_code"` //网站唯一码（主要键）
	Day            int    `json:"day"`                      //年月日（主要键）如 20260310
	Host           string `gorm:"size:255" json:"host"`     //域名
	TotalCount     int64  `json:"total_count"`              //总请求数(PV)
	AttackCount    int64  `json:"attack_count"`             //攻击拦截数
	NormalCount    int64  `json:"normal_count"`             //正常放行数
	TrafficIn      int64  `json:"traffic_in"`               //入站流量(bytes)
	TrafficOut     int64  `json:"traffic_out"`              //出站流量(bytes)
	TotalTimeSpent int64  `json:"total_time_spent"`         //总响应时间(ms)，用于计算平均值
}

/*
*
按小时统计站点综合数据（小时级聚合，仅保留最近3天）
*/
type StatsSiteHour struct {
	baseorm.BaseOrm
	HostCode       string `gorm:"size:64" json:"host_code"` //网站唯一码（主要键）
	HourTime       int64  `json:"hour_time"`                //整点unix时间戳(秒)（主要键）
	Host           string `gorm:"size:255" json:"host"`     //域名
	TotalCount     int64  `json:"total_count"`              //总请求数
	AttackCount    int64  `json:"attack_count"`             //攻击拦截数
	NormalCount    int64  `json:"normal_count"`             //正常放行数
	TrafficIn      int64  `json:"traffic_in"`               //入站流量(bytes)
	TrafficOut     int64  `json:"traffic_out"`              //出站流量(bytes)
	TotalTimeSpent int64  `json:"total_time_spent"`         //总响应时间(ms)
}
