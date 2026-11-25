package wafpipeline

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sort"
	"time"
)

type AssessmentSourceResult struct {
	IP          string `json:"ip"`
	Total       int    `json:"total"`
	Blocked     int    `json:"blocked"`
	Challenged  int    `json:"challenged"`
	Observed    int    `json:"observed"`
	Passed      int    `json:"passed"`
	Unavailable int    `json:"unavailable"`
}

// AssessMany bounds memory while a cancellable job covers cities in small batches.
func (r *Runtime) AssessMany(ctx context.Context, snapshot *Snapshot, input AssessmentInput, progress func(int, int)) (AssessmentReport, error) {
	if input.Builtins {
		input.Samples = append(BuiltinAssessmentSamples(), input.Samples...)
		input.Builtins = false
	}
	if len(input.Samples) == 0 || len(input.Samples) > 256 || input.Pressure < 0 || input.Pressure > 2000 {
		return AssessmentReport{}, errors.New("样本需 1–256 条，重复请求需 0–2000 次")
	}
	if len(input.Sources) == 0 || len(input.Sources) > 10000 || len(input.Samples)*len(input.Sources) > 250000 {
		return AssessmentReport{}, errors.New("本轮城市和样本组合超过 250000 次检查上限，请在高级测试中分批执行")
	}
	started := time.Now()
	out := AssessmentReport{Rows: []AssessmentRow{}, Categories: []AssessmentCategory{}, Advice: []string{}}
	categories := map[string]*AssessmentCategory{}
	sources := map[string]*AssessmentSourceResult{}
	seen := map[string]bool{}
	input.Sources = append([]AssessmentSource(nil), input.Sources...)
	for index, source := range input.Sources {
		address, err := netip.ParseAddr(source.IP)
		if err != nil || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
			return out, errors.New("来源 IP 不合法")
		}
		source.IP = address.Unmap().String()
		input.Sources[index].IP = source.IP
		if seen[source.IP] {
			return out, errors.New("来源 IP 不得重复")
		}
		seen[source.IP] = true
	}
	for at := 0; at < len(input.Sources); at += 16 {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		batch := input
		batch.Sources = input.Sources[at:min(at+16, len(input.Sources))]
		batch.Pressure = 0
		report, err := r.Assess(ctx, snapshot, batch)
		if err != nil {
			return out, err
		}
		out.Target, out.Revision, out.Limits = report.Target, report.Revision, report.Limits
		out.NormalTotal += report.NormalTotal
		out.NormalPassed += report.NormalPassed
		out.Evaluations += len(report.Rows)
		for _, group := range report.Categories {
			current := categories[group.Category]
			if current == nil {
				current = &AssessmentCategory{Category: group.Category}
				categories[group.Category] = current
			}
			current.Total += group.Total
			current.Blocked += group.Blocked
			current.Challenged += group.Challenged
			current.Observed += group.Observed
			current.Missed += group.Missed
		}
		for _, row := range report.Rows {
			source := sources[row.Source]
			if source == nil {
				source = &AssessmentSourceResult{IP: row.Source}
				sources[row.Source] = source
			}
			source.Total++
			switch row.Outcome {
			case "blocked":
				source.Blocked++
			case "challenge":
				source.Challenged++
			case "observe":
				source.Observed++
			case "passed":
				source.Passed++
			default:
				source.Unavailable++
			}
			if len(out.Rows) < 128 {
				out.Rows = append(out.Rows, row)
			}
		}
		if progress != nil {
			progress(min(at+16, len(input.Sources)), len(input.Sources))
		}
		// Yield between batches; foreground protection keeps its own counters and admission.
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(2 * time.Millisecond):
		}
	}
	keys := []string{}
	for key := range categories {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	total := 0.0
	for _, key := range keys {
		c := categories[key]
		out.Categories = append(out.Categories, *c)
		total += (float64(c.Blocked) + float64(c.Challenged)*0.5) / float64(c.Total)
		if c.Missed+c.Observed > 0 {
			out.Advice = append(out.Advice, fmt.Sprintf("%s：%d / %d 条未被防御。检查对应规则是否仅观察、关闭或存在例外。", AssessmentCategoryName(key), c.Missed+c.Observed, c.Total))
		}
	}
	if len(categories) > 0 && out.NormalTotal > 0 {
		score := int(math.Round(100 * total / float64(len(categories)) * float64(out.NormalPassed) / float64(out.NormalTotal)))
		out.Score = &score
	}
	if out.NormalPassed < out.NormalTotal {
		out.Advice = append(out.Advice, "部分正常请求受阻，请核对允许来源、认证要求与业务路径。")
	}
	if out.NormalTotal == 0 {
		out.Advice = append(out.Advice, "当前来源中没有可验证的允许来源，暂不生成总分；请核对访问名单与地区库覆盖。")
	}
	for _, inputSource := range input.Sources {
		if source := sources[inputSource.IP]; source != nil {
			out.SourceResults = append(out.SourceResults, *source)
		}
	}
	if input.Pressure > 0 {
		pressureInput := input
		pressureInput.Sources = nil
		// Include both families and their first allowed sources in the pressure lane.
		for _, family := range []string{"v4", "v6"} {
			count := 0
			for _, s := range input.Sources {
				v4 := !containsColon(s.IP)
				if (family == "v4") == v4 {
					pressureInput.Sources = append(pressureInput.Sources, s)
					count++
					if count == 8 {
						break
					}
				}
			}
		}
		pressureInput.Samples = []AssessmentSample{{ID: "pressure-baseline", Category: "normal", Kind: "unknown", Path: "/", Method: "GET"}}
		pressureInput.Builtins = false
		pressureReport, err := r.Assess(ctx, snapshot, pressureInput)
		if err != nil {
			return out, err
		}
		out.Pressure = pressureReport.Pressure
		if out.Pressure.Limited == 0 {
			out.Advice = append(out.Advice, "本轮突发访问未触发限流，请检查限流开关和阈值。")
		}
	}
	out.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	out.Limits = append(out.Limits, "城市来源已分批全部判定；逐条轨迹仅展示前 128 条，各城市和分类统计包含所有完成的检查。")
	return out, nil
}
func containsColon(value string) bool {
	for _, c := range value {
		if c == ':' {
			return true
		}
	}
	return false
}
