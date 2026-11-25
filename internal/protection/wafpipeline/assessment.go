package wafpipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"
)

type AssessmentSample struct {
	ID        string            `json:"id"`
	Category  string            `json:"category"`
	Kind      string            `json:"kind"` // attack, benign, unknown; historical blocks are not labels.
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	Truncated bool              `json:"truncated"`
	Imported  bool              `json:"imported"`
}
type AssessmentSource struct {
	IP           string `json:"ip"`
	ExpectAccess bool   `json:"expect_access"`
}
type AssessmentInput struct {
	Sources      []AssessmentSource `json:"sources"`
	Samples      []AssessmentSample `json:"samples"`
	Builtins     bool               `json:"builtins"`
	Pressure     int                `json:"pressure"`
	PressureMode string             `json:"pressure_mode,omitempty"`
	PressurePath string             `json:"pressure_path,omitempty"`
}
type AssessmentRow struct {
	ID       string    `json:"id"`
	Category string    `json:"category"`
	Kind     string    `json:"kind"`
	Source   string    `json:"source"`
	ClientIP string    `json:"client_ip"`
	Geo      GeoResult `json:"geo"`
	Expected string    `json:"expected"`
	Outcome  string    `json:"outcome"`
	Result   Result    `json:"result"`
}
type AssessmentCategory struct {
	Category   string `json:"category"`
	Total      int    `json:"total"`
	Blocked    int    `json:"blocked"`
	Challenged int    `json:"challenged"`
	Observed   int    `json:"observed"`
	Missed     int    `json:"missed"`
}
type AssessmentPressure struct {
	Evaluations  int                      `json:"evaluations"`
	Limited      int                      `json:"limited"`
	FirstLimited int                      `json:"first_limited"`
	ElapsedMS    float64                  `json:"elapsed_ms"`
	Lanes        []AssessmentPressureLane `json:"lanes,omitempty"`
}
type AssessmentPressureLane struct {
	Name         string `json:"name"`
	Evaluations  int    `json:"evaluations"`
	Limited      int    `json:"limited"`
	FirstLimited int    `json:"first_limited"`
}
type AssessmentReport struct {
	Evaluations     int                      `json:"evaluations"`
	SourceResults   []AssessmentSourceResult `json:"source_results,omitempty"`
	DirectScore     *int                     `json:"direct_score,omitempty"`
	CamouflageScore int                      `json:"camouflage_score"`
	Target          string                   `json:"target"`
	Revision        int64                    `json:"revision"`
	Score           *int                     `json:"score"`
	NormalTotal     int                      `json:"normal_total"`
	NormalPassed    int                      `json:"normal_passed"`
	Rows            []AssessmentRow          `json:"rows"`
	Categories      []AssessmentCategory     `json:"categories"`
	Pressure        AssessmentPressure       `json:"pressure"`
	ElapsedMS       float64                  `json:"elapsed_ms"`
	Advice          []string                 `json:"advice"`
	Limits          []string                 `json:"limits"`
}

// AssessmentTarget uses only the selected service identity, never an imported Host.
func AssessmentTarget(c PortConfig) string {
	host := "127.0.0.1"
	if c.Policy.Network.ListenIPVersion == "ipv6" {
		host = "::1"
	}
	names := append([]string{}, c.HostNames...)
	for _, route := range c.Routes {
		names = append(names, route.Hosts...)
	}
	if len(names) > 0 {
		host = names[0]
		if strings.HasPrefix(host, "*.") {
			host = "assessment" + host[1:]
		}
	}
	return c.Protocol + "://" + net.JoinHostPort(host, strconv.Itoa(int(c.Port)))
}

func BuiltinAssessmentSamples() []AssessmentSample {
	return []AssessmentSample{
		{ID: "normal-home", Category: "normal", Kind: "benign", Method: "GET", Path: "/"},
		{ID: "normal-asset", Category: "normal", Kind: "benign", Method: "GET", Path: "/favicon.ico"},
		{ID: "sql-query", Category: "sqli", Kind: "attack", Method: "GET", Path: "/?id=1%27%20OR%20%271%27=%271%27--"},
		{ID: "xss-query", Category: "xss", Kind: "attack", Method: "GET", Path: "/?q=%3Cscript%3Ealert(1)%3C/script%3E"},
		{ID: "file-read", Category: "lfi", Kind: "attack", Method: "GET", Path: "/?file=../../../../etc/passwd"},
		{ID: "command", Category: "rce", Kind: "attack", Method: "GET", Path: "/?cmd=cat%20/etc/passwd%3Bid"},
		{ID: "template", Category: "ssti", Kind: "attack", Method: "GET", Path: "/?q=%7B%7B7*7%7D%7D"},
		{ID: "scanner", Category: "scanner", Kind: "attack", Method: "GET", Path: "/.env", Headers: map[string]string{"User-Agent": "sqlmap/1.8"}},
		{ID: "internal-fetch", Category: "ssrf", Kind: "attack", Method: "POST", Path: "/api/fetch", Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"url":"http://169.254.169.254/latest/meta-data/"}`},
	}
}

func assessmentRequest(ctx context.Context, target string, s AssessmentSample, source string) (*http.Request, error) {
	if len(s.ID) > 128 || len(s.Category) > 64 || len(s.Path) > 8192 || len(s.Body) > 65536 || len(s.Headers) > 64 || !strings.HasPrefix(s.Path, "/") || strings.HasPrefix(s.Path, "//") {
		return nil, errors.New("样例超过限制或路径不合法")
	}
	if s.Kind != "attack" && s.Kind != "benign" && s.Kind != "unknown" {
		return nil, errors.New("样例必须标注 attack、benign 或 unknown")
	}
	u, err := url.ParseRequestURI(s.Path)
	if err != nil || u.IsAbs() || u.Host != "" {
		return nil, errors.New("样例路径必须是服务内的相对请求路径")
	}
	method := s.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, target+s.Path, strings.NewReader(s.Body))
	if err != nil {
		return nil, errors.New("样例请求无效")
	}
	bytes := 0
	for k, v := range s.Headers {
		bytes += len(k) + len(v)
		if bytes > 32768 || !httpguts.ValidHeaderFieldName(k) || !httpguts.ValidHeaderFieldValue(v) {
			return nil, errors.New("样例请求头无效或超过 32 KiB")
		}
		if s.Imported {
			switch strings.ToLower(k) {
			case "authorization", "proxy-authorization", "cookie", "x-homewaf-token":
				continue
			}
		}
		switch strings.ToLower(k) {
		case "host", "connection", "content-length", "transfer-encoding":
			continue
		case "origin":
			v = target
		case "referer":
			v = target + "/"
		}
		req.Header.Set(k, v)
	}
	if req.UserAgent() == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 HomeWaf-Assessment")
	}
	req.RemoteAddr = net.JoinHostPort(source, "12345")
	return req, nil
}

func assessmentOutcome(result Result, truncated bool) string {
	if truncated || result.BodyTruncated || result.BodyReadFailure != nil {
		return "incomplete"
	}
	for _, step := range result.Trace {
		if step.Error != "" {
			return "unavailable"
		}
	}
	v := result.Verdict
	if strings.Contains(v.Reason, "unavailable") {
		return "unavailable"
	}
	if !v.Observe {
		switch v.Action {
		case Deny, Drop, Honeypot:
			return "blocked"
		case Challenge:
			return "challenge"
		}
	}
	if len(result.Hits) > 0 {
		return "observe"
	}
	return "passed"
}

// Assess compiles a private preview once. All full-policy evaluations are dry runs;
// the separate pressure lane owns its own QPS/CC stores and has no sink or forwarder.
func (r *Runtime) Assess(ctx context.Context, original *Snapshot, input AssessmentInput) (AssessmentReport, error) {
	started := time.Now()
	out := AssessmentReport{Target: AssessmentTarget(original.Config), Revision: original.Config.Revision, Rows: []AssessmentRow{}, Categories: []AssessmentCategory{}, Advice: []string{}, Limits: []string{
		"本次为隔离规则模拟，不连接源站、不修改服务、不产生封禁；监听可达性、TLS 和公网承载能力需另行验证。",
		"防御分 = 各攻击类别平均防御率 × 正常来源可访问率；拒绝/丢弃/蜜罐计 1，验证挑战计 0.5，仅观察和放行计 0。未标注实录仅展示判定，不推断恶意。",
		"认证挑战只证明认证入口可达；未验证登录后业务、响应检测、外部插件执行和持久处罚。重复请求仅测独立 QPS/CC 计数，不影响防御分。",
	}}
	if len(input.Sources) == 0 || len(input.Sources) > 32 || len(input.Samples) > 256 || input.Pressure < 0 || input.Pressure > 2000 {
		return out, errors.New("来源需 1–32 个、导入样例最多 256 条、重复请求最多 2000 次")
	}
	if input.PressureMode != "" && input.PressureMode != "distributed" && input.PressureMode != "single" && input.PressureMode != "mixed" {
		return out, errors.New("压力方案无效")
	}
	if input.PressurePath == "" {
		input.PressurePath = "/"
	}
	if _, err := assessmentRequest(ctx, out.Target, AssessmentSample{Kind: "benign", Path: input.PressurePath, Method: "GET"}, "127.0.0.1"); err != nil {
		return out, err
	}
	samples := append([]AssessmentSample{}, input.Samples...)
	if input.Builtins {
		samples = append(BuiltinAssessmentSamples(), samples...)
	}
	if input.Builtins {
		for _, source := range input.Sources {
			if source.ExpectAccess {
				samples = append(samples, AssessmentSample{ID: "spoofed-source", Category: "source_spoof", Kind: "benign", Method: "GET", Path: "/", Headers: map[string]string{"X-Forwarded-For": source.IP, "X-Real-IP": source.IP, "Forwarded": "for=" + source.IP}})
				break
			}
		}
	}
	if len(samples) == 0 || len(samples)*len(input.Sources) > 4096 {
		return out, errors.New("请选择样例；单次最多 4096 个来源与样例组合")
	}
	seenSources := map[string]bool{}
	for i, source := range input.Sources {
		ip, err := netip.ParseAddr(source.IP)
		if err != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() {
			return out, errors.New("来源 IP 不合法")
		}
		input.Sources[i].IP = ip.Unmap().String()
		if seenSources[input.Sources[i].IP] {
			return out, errors.New("来源 IP 不得重复")
		}
		seenSources[input.Sources[i].IP] = true
	}
	snap := &Snapshot{Config: CloneConfig(original.Config), Secrets: original.Secrets, Preview: true}
	if err := r.Prepare(snap); err != nil {
		return out, err
	}
	categories := map[string]*AssessmentCategory{}
	unknownGeo := false
	scored := map[string]bool{}
	for _, source := range input.Sources {
		for index, sample := range samples {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			req, err := assessmentRequest(ctx, out.Target, sample, source.IP)
			if err != nil {
				return out, fmt.Errorf("样例 %d: %w", index+1, err)
			}
			ip, err := ExtractClientIP(req, snap.Config.Policy.Network, snap.Compiled.Proxies)
			if err != nil {
				return out, err
			}
			req = WithTrustedExternalScheme(req, snap.Config.Protocol)
			c := NewRequestContext(req, PortID(snap.Config.Port), ip, snap.Config.Policy.RuleSet.View)
			c.DryRun = true
			result := snap.Compiled.Pipeline.Evaluate(c)
			outcome := assessmentOutcome(result, sample.Truncated)
			geo := GeoResult{}
			if r.Geo != nil {
				geo = r.Geo.Lookup(ip)
			}
			if !geo.OK {
				unknownGeo = true
			}
			expected := "unknown"
			if sample.Kind != "unknown" && (sample.Kind == "attack" || !source.ExpectAccess) {
				expected = "block"
			} else if sample.Kind == "benign" {
				expected = "access"
			}
			row := AssessmentRow{ID: sample.ID, Category: sample.Category, Kind: sample.Kind, Source: source.IP, ClientIP: ip.String(), Geo: geo, Expected: expected, Outcome: outcome, Result: result}
			out.Rows = append(out.Rows, row)
			identity := sample
			identity.ID = ""
			raw, _ := json.Marshal(identity)
			key := source.IP + string(raw)
			if scored[key] {
				continue
			}
			scored[key] = true
			if expected == "access" {
				out.NormalTotal++
				if outcome == "passed" || outcome == "observe" || outcome == "challenge" && strings.HasPrefix(result.Verdict.Reason, "gate:") {
					out.NormalPassed++
				}
			}
			if expected != "block" {
				continue
			}
			category := sample.Category
			if !source.ExpectAccess {
				category = "source_boundary"
			} else if category == "" {
				category = "custom"
			}
			if categories[category] == nil {
				categories[category] = &AssessmentCategory{Category: category}
			}
			group := categories[category]
			group.Total++
			switch outcome {
			case "blocked":
				group.Blocked++
			case "challenge":
				group.Challenged++
			case "observe":
				group.Observed++
			default:
				group.Missed++
			}
		}
	}
	security := 0.0
	keys := []string{}
	for key := range categories {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := categories[key]
		out.Categories = append(out.Categories, *group)
		security += (float64(group.Blocked) + 0.5*float64(group.Challenged)) / float64(group.Total)
		if group.Missed+group.Observed > 0 {
			out.Advice = append(out.Advice, fmt.Sprintf("%s 有 %d/%d 条未防御：检查来源范围、规则模式与例外；仅观察不会阻止请求。", AssessmentCategoryName(key), group.Missed+group.Observed, group.Total))
		}
	}
	if len(categories) > 0 && out.NormalTotal > 0 {
		score := int(math.Round(100 * security / float64(len(categories)) * float64(out.NormalPassed) / float64(out.NormalTotal)))
		out.Score = &score
	} else {
		out.Advice = append(out.Advice, "缺少攻击样例或预期允许的正常样例，暂不生成总分。请至少添加一个预期允许的来源。")
	}
	if out.NormalPassed < out.NormalTotal {
		out.Advice = append(out.Advice, "正常访问受阻：优先核对允许地区、来源名单、访问开关及业务路径，避免靠拒绝所有请求获得安全感。")
	}
	if unknownGeo {
		out.Advice = append(out.Advice, "部分来源归属地不可判定，请检查 IPv4/IPv6 数据库；未知归属地不是已验证的地区覆盖。")
	}
	if input.Pressure > 0 {
		pressureStart := time.Now()
		paths := []struct {
			name    string
			sources []AssessmentSource
			count   int
		}{{"多来源突发访问", input.Sources, input.Pressure}}
		if input.PressureMode == "single" {
			paths[0].name = "单来源连续访问"
			paths[0].sources = input.Sources[:1]
		}
		if input.PressureMode == "mixed" {
			var v4, v6 []AssessmentSource
			for _, s := range input.Sources {
				if netip.MustParseAddr(s.IP).Is4() {
					v4 = append(v4, s)
				} else {
					v6 = append(v6, s)
				}
			}
			if len(v4) > 0 && len(v6) > 0 {
				interleaved := []AssessmentSource{}
				for i := 0; i < max(len(v4), len(v6)); i++ {
					interleaved = append(interleaved, v4[i%len(v4)], v6[i%len(v6)])
				}
				count := input.Pressure / 3
				count4, count6 := count, count
				if snap.Config.Policy.Network.ListenIPVersion == "ipv4" {
					count6 = min(5, count)
					interleaved = v4
				}
				if snap.Config.Policy.Network.ListenIPVersion == "ipv6" {
					count4 = min(5, count)
					interleaved = v6
				}
				paths = []struct {
					name    string
					sources []AssessmentSource
					count   int
				}{{"IPv4 单来源连续访问", v4[:1], count4}, {"IPv6 单来源连续访问", v6[:1], count6}, {"已启用协议多来源突发访问", interleaved, input.Pressure - count*2}}
			}
		}
		for _, scenario := range paths {
			laneResult := AssessmentPressureLane{Name: scenario.name}
			controls := NewRulesA(nil).Steps(snap)
			steps := []Step{}
			for _, step := range controls {
				if step.Name == "qps" || step.Name == "cc" {
					steps = append(steps, step)
				}
			}
			lane, _ := New(steps, nil)
			for i := 0; i < scenario.count; i++ {
				if err := ctx.Err(); err != nil {
					return out, err
				}
				source := scenario.sources[i%len(scenario.sources)]
				req, _ := assessmentRequest(ctx, out.Target, AssessmentSample{Kind: "benign", Path: input.PressurePath, Method: "GET"}, source.IP)
				c := NewRequestContext(req, PortID(snap.Config.Port), netip.MustParseAddr(source.IP), snap.Config.Policy.RuleSet.View)
				v := lane.Evaluate(c).Verdict
				out.Pressure.Evaluations++
				laneResult.Evaluations++
				if !v.Observe && (v.Action == Deny || v.Action == Challenge || v.Action == Drop) {
					out.Pressure.Limited++
					laneResult.Limited++
					if laneResult.FirstLimited == 0 {
						laneResult.FirstLimited = i + 1
					}
					if out.Pressure.FirstLimited == 0 {
						out.Pressure.FirstLimited = out.Pressure.Evaluations
					}
				}
			}
			out.Pressure.Lanes = append(out.Pressure.Lanes, laneResult)
		}
		out.Pressure.ElapsedMS = float64(time.Since(pressureStart).Microseconds()) / 1000
		if out.Pressure.Limited == 0 {
			out.Advice = append(out.Advice, "本次重复请求未触发 QPS/CC 限制。检查是否启用、阈值与窗口，并增加受控样本量后复测。")
		}
	}
	out.ElapsedMS = float64(time.Since(started).Microseconds()) / 1000
	out.Evaluations = len(out.Rows)
	return out, nil
}
