// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	response2 "HomeWaf/model/response"
	"fmt"
)

type WafAnalysisService struct{}

var WafAnalysisServiceApp = new(WafAnalysisService)

// StatAnalysisDayCountryRangeApi 通过时间获取国家级别的 攻击数 访问数
func (receiver *WafAnalysisService) StatAnalysisDayCountryRangeApi(req request.WafStatsAnalysisDayRangeCountryReq) ([]response2.WafAnalysisDayStats, error) {
	var CountOfRange []response2.WafAnalysisDayStats
	if req.GroupBy != "" && req.GroupBy != "country" && req.GroupBy != "city" {
		return nil, fmt.Errorf("地区统计维度无效")
	}
	query := global.GWAF_LOCAL_STATS_DB.Model(&model.StatsIPCityDay{}).
		Where("day between ? and ? and country<>? and country<>?", req.StartDay, req.EndDay, "0", "内网")
	if req.AttackType != "" {
		query = query.Where("type = ?", req.AttackType)
	}
	if req.GroupBy == "city" {
		query = query.Select("country as name, country, province, city, sum(count) as value").Group("country, province, city")
	} else {
		query = query.Select("country as name, sum(count) as value").Group("country")
	}
	if err := query.Order("sum(count) desc").Scan(&CountOfRange).Error; err != nil {
		return nil, err
	}
	return CountOfRange, nil
}

// AnalysisSpiderApi 爬虫分析
// 数据源是 access_log（分层后所有请求的窄行都在这里）；今天它也只看实时库，
func (receiver *WafAnalysisService) AnalysisSpiderApi(req request.WafAnalysisSpiderReq) []response2.WafAnalysisSpiderResp {
	var CountOfRange []response2.WafAnalysisSpiderResp
	if req.Host == "" {
		global.GWAF_LOCAL_LOG_DB.Model(&model.AccessLog{}).Where("day between ? and ? and  is_bot=1 ",
			req.StartDay, req.EndDay).Select(" guest_identification as Name ,count(1) as Value").
			Group("guest_identification").Order("count(1) desc").Scan(&CountOfRange)

	} else {
		global.GWAF_LOCAL_LOG_DB.Model(&model.AccessLog{}).Where("day between ? and ? and  is_bot=1 and host_code = ? ",
			req.StartDay, req.EndDay, req.Host).
			Select(" guest_identification as Name ,count(1) as Value").
			Group("guest_identification").Order("count(1) desc").Scan(&CountOfRange)
	}
	return CountOfRange
}
