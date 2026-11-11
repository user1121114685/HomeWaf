package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	response "HomeWaf/model/response"
	"HomeWaf/wafdb"
	"HomeWaf/wafdb/dialect"
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type screenSource struct {
	DB    *gorm.DB
	Table string
	Name  string
}

var screenCache struct {
	sync.Mutex
	data         *response.ScreenSnapshot
	at           time.Time
	tenant, user string
}

func InvalidateScreenSnapshot() { screenCache.Lock(); screenCache.data = nil; screenCache.Unlock() }

// ScreenSnapshot uses exact millisecond bounds, including archive partitions.
// It counts recorded requests; sampled/off logging is explicitly marked partial.
func GetScreenSnapshot(ctx context.Context) (*response.ScreenSnapshot, error) {
	screenCache.Lock()
	defer screenCache.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if screenCache.data != nil && screenCache.tenant == global.GWAF_TENANT_ID && screenCache.user == global.GWAF_USER_CODE && time.Since(screenCache.at) < 4*time.Second {
		return screenCache.data, nil
	}
	if global.GDATA_CURRENT_CHANGE {
		return nil, errors.New("日志库正在切换，请稍后刷新")
	}
	if global.GWAF_LOCAL_LOG_DB == nil || global.GWAF_LOCAL_DB == nil {
		return nil, errors.New("数据库尚未就绪")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	release, err := wafdb.AcquireLogRead(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	now := time.Now()
	registered := []model.ShareDb{}
	if err := global.GWAF_LOCAL_DB.WithContext(ctx).Where("tenant_id = ? AND user_code = ?", global.GWAF_TENANT_ID, global.GWAF_USER_CODE).Find(&registered).Error; err != nil {
		return nil, fmt.Errorf("无法读取日志分区: %w", err)
	}
	shards := []logShard{{Name: wafdb.LiveLogName(), Live: true, End: now}}
	for _, sh := range registered {
		if sh.FileName == "" || sh.FileName == wafdb.LiveLogName() || sh.IsTableShard() == dialect.Get().IsFileBased() || (sh.DbLogicType != "" && sh.DbLogicType != "log") {
			continue
		}
		start, end := time.Time(sh.StartTime), time.Time(sh.EndTime)
		if (!end.IsZero() && end.Before(now.Add(-24*time.Hour))) || (!start.IsZero() && start.After(now)) {
			continue
		}
		shards = append(shards, logShard{Name: sh.FileName, Start: start, End: end, Registered: sh.Cnt})
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].End.After(shards[j].End) })
	sources := []screenSource{}
	issues := []string{}
	if len(shards) > 32 {
		shards = shards[:32]
		issues = append(issues, "日志分区超过查询上限，结果仅覆盖最近 32 个分区")
	}
	for _, sh := range shards {
		tier := wafdb.ResolveTierTables(sh.Name)
		if tier.Err != nil || tier.DB == nil {
			issues = append(issues, "部分日志分区不可用")
			continue
		}
		if tier.Empty && sh.Registered > 0 {
			issues = append(issues, "部分历史日志分区为空，统计可能不完整")
			continue
		}
		table := tier.Access
		if table == "" {
			table = tier.WebLog
		}
		if table == "" {
			if sh.Registered > 0 {
				issues = append(issues, "部分历史日志分区缺少访问记录")
			}
			continue
		}
		sources = append(sources, screenSource{tier.DB, table, sh.Name})
	}
	if len(sources) == 0 {
		return nil, errors.New("没有可读取的访问日志分区")
	}
	result, err := buildScreenSnapshot(ctx, now, sources, global.GWAF_TENANT_ID, global.GWAF_USER_CODE)
	if err != nil {
		return nil, err
	}
	result.Issues = append(result.Issues, issues...)
	result.LogMode = global.CurrentSettings().GDATA_ACCESS_LOG_MODE
	if result.LogMode != "db" {
		result.Issues = append(result.Issues, "访问日志并非全量记录，统计仅包含已记录的请求")
	}
	result.Partial = len(result.Issues) > 0
	result.QPS = WafStatServiceApp.StatQpsTrendApi(120)
	target, err := GetDashboardTarget()
	if err != nil {
		return nil, err
	}
	result.Target = target
	screenCache.data, screenCache.at, screenCache.tenant, screenCache.user = result, time.Now(), global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	return result, nil
}

func screenQuery(ctx context.Context, source screenSource, tenant, user string, from, to int64) *gorm.DB {
	return source.DB.WithContext(ctx).Table(source.Table).Where("tenant_id = ? AND user_code = ? AND unix_add_time > ? AND unix_add_time <= ?", tenant, user, from, to)
}

func buildScreenSnapshot(ctx context.Context, now time.Time, sources []screenSource, tenant, user string) (*response.ScreenSnapshot, error) {
	from, to := now.Add(-24*time.Hour).UnixMilli(), now.UnixMilli()
	out := &response.ScreenSnapshot{WindowStart: from, WindowEnd: to, Timezone: now.Format("UTC-07:00"), IPs: []response.ScreenIP{}, Regions: []response.WafAnalysisDayStats{}, Events: []response.ScreenEvent{}, Issues: []string{}, VisitorBasis: "按来源 IP 跨站点、跨日志分区去重；不是自然人数"}
	visitors := map[string]bool{}
	attackIPs := map[string]*response.ScreenIP{}
	countries := map[string]int64{}
	seenEvents := map[string]bool{}
	successful := 0
	for _, source := range sources {
		rows, err := screenQuery(ctx, source, tenant, user, from, to).Select("coalesce(src_ip, '') AS src_ip, coalesce(action, '') AS action, coalesce(country, '') AS country, count(*) AS request_count").Group("src_ip, action, country").Rows()
		if err != nil {
			out.Issues = append(out.Issues, "部分日志统计读取失败")
			continue
		}
		for rows.Next() {
			var ip, action, country string
			var count int64
			if err = rows.Scan(&ip, &action, &country, &count); err != nil {
				break
			}
			out.Summary.Visits += count
			parsed := net.ParseIP(strings.TrimSpace(ip))
			if parsed != nil {
				ip = parsed.String()
				visitors[ip] = true
			}
			if action == "阻止" {
				out.Summary.Blocked += count
				if parsed != nil {
					if attackIPs[ip] == nil {
						attackIPs[ip] = &response.ScreenIP{IP: ip, Country: country}
					}
					attackIPs[ip].Count += count
				}
				if country != "" && country != "0" && country != "内网" && country != "Reserved" {
					countries[country] += count
				}
			}
			if len(visitors) > 500000 {
				err = errors.New("IP 基数超过单次查询预算")
				break
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			out.Issues = append(out.Issues, "部分日志统计超时或超过读取预算")
		} else {
			successful++
		}
		events := []response.ScreenEvent{}
		err = screenQuery(ctx, source, tenant, user, from, to).Where("action = ?", "阻止").Select("req_uuid, src_ip, country, province, city, host, rule, action, create_time, unix_add_time").Order("unix_add_time DESC, req_uuid DESC").Limit(12).Scan(&events).Error
		if err != nil {
			out.Issues = append(out.Issues, "部分攻击事件读取失败")
		} else {
			for _, event := range events {
				if !seenEvents[event.ID] {
					seenEvents[event.ID] = true
					out.Events = append(out.Events, event)
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	if successful == 0 {
		return nil, errors.New("访问日志统计暂不可用")
	}
	out.Summary.Visitors = int64(len(visitors))
	out.Summary.AttackIPs = int64(len(attackIPs))
	for _, ip := range attackIPs {
		out.IPs = append(out.IPs, *ip)
	}
	sort.Slice(out.IPs, func(i, j int) bool {
		if out.IPs[i].Count == out.IPs[j].Count {
			return out.IPs[i].IP < out.IPs[j].IP
		}
		return out.IPs[i].Count > out.IPs[j].Count
	})
	if len(out.IPs) > 5 {
		out.IPs = out.IPs[:5]
	}
	for country, count := range countries {
		out.Regions = append(out.Regions, response.WafAnalysisDayStats{Name: country, Value: count})
	}
	sort.Slice(out.Regions, func(i, j int) bool { return out.Regions[i].Value > out.Regions[j].Value })
	sort.Slice(out.Events, func(i, j int) bool {
		if out.Events[i].Time == out.Events[j].Time {
			return out.Events[i].ID > out.Events[j].ID
		}
		return out.Events[i].Time > out.Events[j].Time
	})
	if len(out.Events) > 12 {
		out.Events = out.Events[:12]
	}
	out.Partial = len(out.Issues) > 0
	return out, nil
}

func GetDashboardTarget() (model.DashboardTarget, error) {
	target := model.DashboardTarget{}
	err := global.GWAF_LOCAL_DB.Where("tenant_id = ? AND user_code = ?", global.GWAF_TENANT_ID, global.GWAF_USER_CODE).First(&target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		target = model.DashboardTarget{Mode: "auto", Label: "HomeWaf 防护节点"}
		err = nil
	}
	if err != nil {
		return target, err
	}
	if target.Mode == "coordinates" {
		target.Status, target.HasCoordinates, target.Source = "ready", true, "manual"
		return target, nil
	}
	// Existing logical configurations also upgrade to automatic discovery.
	location := dashboardNodeLocation.snapshot()
	location.Mode, location.Label = "auto", target.Label
	return location, nil
}
