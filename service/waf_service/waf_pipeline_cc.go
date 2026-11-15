package waf_service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
)

type PipelineCCRecommendRequest struct {
	Port          uint16 `json:"port"`
	Days          int    `json:"days"`
	WindowSeconds int    `json:"window_seconds"`
	ByPath        bool   `json:"by_path"`
	ExcludeStatic bool   `json:"exclude_static"`
}
type PipelineCCRecommendation struct {
	Supported     bool   `json:"supported"`
	Reason        string `json:"reason"`
	Scope         string `json:"scope"`
	Requests      int    `json:"requests"`
	Clients       int    `json:"clients"`
	From          int64  `json:"from"`
	To            int64  `json:"to"`
	P99           int    `json:"p99"`
	Suggested     int    `json:"suggested"`
	WindowSeconds int    `json:"window_seconds"`
}
type pipelineCCSample struct {
	SrcIP  string `gorm:"column:src_ip"`
	URL    string `gorm:"column:url"`
	Millis int64  `gorm:"column:unix_add_time"`
}

// Recommendations deliberately use a complete bounded range of the active
// partition. Sampled/truncated/unavailable data must never yield a low threshold.
func (s *WafPipelineService) RecommendCC(ctx context.Context, req PipelineCCRecommendRequest) (PipelineCCRecommendation, error) {
	out := PipelineCCRecommendation{WindowSeconds: req.WindowSeconds, Scope: "当前访问日志分区；归档分区未纳入。固定时间桶近似滑动窗口，建议仅供观察调优，不自动启用拦截。"}
	if req.Port == 0 || req.Days < 1 || req.Days > 7 || req.WindowSeconds < 1 || req.WindowSeconds > 3600 {
		return out, fmt.Errorf("端口、天数或统计窗口不合法")
	}
	config, err := s.Get(req.Port)
	if err != nil {
		return out, err
	}
	if global.CurrentSettings().GDATA_ACCESS_LOG_MODE != "db" {
		out.Reason = "需要全量访问日志（access_log_mode=db）。采样或仅安全事件不能可靠估算阈值。"
		return out, nil
	}
	if global.GWAF_LOCAL_LOG_DB == nil {
		return out, fmt.Errorf("访问日志库不可用")
	}
	host := config.HostCode
	if host == "" {
		host = fmt.Sprintf("pipeline-%d", req.Port)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var rows []pipelineCCSample
	from := time.Now().AddDate(0, 0, -req.Days).UnixMilli()
	err = global.GWAF_LOCAL_LOG_DB.WithContext(ctx).Table(model.AccessLogTableName).Select("src_ip, url, unix_add_time").Where("host_code = ? AND unix_add_time >= ?", host, from).Order("unix_add_time ASC").Limit(50001).Find(&rows).Error
	if err != nil {
		return out, fmt.Errorf("访问日志查询失败，请重试或缩小时间范围")
	}
	if len(rows) > 50000 {
		out.Reason = "超过 50000 条查询预算，未计算建议。请缩小天数。"
		return out, nil
	}
	return summarizePipelineCC(rows, req, out), nil
}

func summarizePipelineCC(rows []pipelineCCSample, req PipelineCCRecommendRequest, out PipelineCCRecommendation) PipelineCCRecommendation {
	buckets := map[string]int{}
	clients := map[string]bool{}
	for _, row := range rows {
		ip, err := netip.ParseAddr(row.SrcIP)
		if err != nil {
			out.Reason = "历史日志缺少可靠客户端 IP，未计算建议。"
			return out
		}
		u, err := url.ParseRequestURI(row.URL)
		if err != nil {
			out.Reason = "历史请求路径无法可靠还原，未计算建议。"
			return out
		}
		path := wafpipeline.NormalizePath(u.EscapedPath())
		if req.ExcludeStatic && wafpipeline.IsStaticRequestPath(path) {
			continue
		}
		key := ip.Unmap().String()
		clients[key] = true
		if req.ByPath {
			key += "\x00" + path
		}
		key += fmt.Sprintf("\x00%d", row.Millis/(int64(req.WindowSeconds)*1000))
		buckets[key]++
		out.Requests++
		if out.From == 0 || row.Millis < out.From {
			out.From = row.Millis
		}
		if row.Millis > out.To {
			out.To = row.Millis
		}
	}
	out.Clients = len(clients)
	if out.Requests < 1000 || out.Clients < 20 || len(buckets) == 0 {
		out.Reason = "样本不足：至少需要 1000 条请求和 20 个来源 IP。请保留当前阈值并继续观察。"
		return out
	}
	counts := make([]int, 0, len(buckets))
	for _, count := range buckets {
		counts = append(counts, count)
	}
	sort.Ints(counts)
	out.P99 = counts[int(math.Ceil(float64(len(counts))*.99))-1]
	out.Suggested = min(1000000, max(1, out.P99*3))
	out.Supported = true
	out.Scope += " 包含历史攻击流量；通过挑战的请求和登录前拒绝的请求无法完全剔除，使用前须结合最近命中核对。"
	return out
}

func (s *WafPipelineStateService) CCHits(port uint16) ([]model.PipelineLog, error) {
	if _, err := WafPipelineServiceApp.Get(port); err != nil {
		return nil, err
	}
	if global.GWAF_LOCAL_LOG_DB == nil {
		return nil, fmt.Errorf("日志库不可用")
	}
	rows := []model.PipelineLog{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := global.GWAF_LOCAL_LOG_DB.WithContext(ctx).Where("port_id = ? AND kind = ? AND (category = ? OR rule_id = ? OR reason = ? OR detail_json LIKE ?)", wafpipeline.PortID(port), "request", "cc", "cc", "cc_frequency", `%"category":"cc"%`).Order("time_millis DESC").Limit(200).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("CC 命中读取失败: %s", strings.TrimSpace(err.Error()))
	}
	for i := range rows {
		var result wafpipeline.Result
		if json.Unmarshal([]byte(rows[i].DetailJSON), &result) != nil {
			continue
		}
		for _, hit := range result.Hits {
			if hit.Category == "cc" {
				rows[i].Observe = hit.Observe
				rows[i].Category = "cc"
				rows[i].RuleID = hit.RuleID
				break
			}
		}
		for _, trace := range result.Trace {
			if trace.Check == "cc" {
				rows[i].Verdict = string(trace.Verdict.Action)
				break
			}
		}
	}
	return rows, nil
}
