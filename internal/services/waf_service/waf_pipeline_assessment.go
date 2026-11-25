package waf_service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"strings"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/protection/iplocation"
	"HomeWaf/internal/protection/wafenginecore/ipset"
	"HomeWaf/internal/protection/wafpipeline"
)

type AssessmentPlanSource struct {
	iplocation.SampleAddress
	ExpectAccess bool `json:"expect_access"`
}
type AssessmentPlan struct {
	Cities      []iplocation.CityCoverage      `json:"cities"`
	IPv4Enabled bool                           `json:"ipv4_enabled"`
	IPv6Enabled bool                           `json:"ipv6_enabled"`
	Sources     []AssessmentPlanSource         `json:"sources"`
	Samples     []wafpipeline.AssessmentSample `json:"samples"`
	Pressure    int                            `json:"pressure"`
	IPv4        int                            `json:"ipv4"`
	IPv6        int                            `json:"ipv6"`
	Generation  uint64                         `json:"generation"`
	Target      string                         `json:"target"`
	Warnings    []string                       `json:"warnings"`
}

func BuildAssessmentPlan(ctx context.Context, config wafpipeline.PortConfig) (AssessmentPlan, error) {
	plan := AssessmentPlan{Target: wafpipeline.AssessmentTarget(config), Warnings: []string{}}
	manager := global.GIPLOCATION_MANAGER
	if manager == nil {
		return plan, errors.New("IP 数据库尚未就绪，请稍后重试或在 IP 数据库管理中检查状态")
	}
	plan.Generation = manager.Generation()
	plan.IPv4Enabled = config.Policy.Network.ListenIPVersion != "ipv6"
	plan.IPv6Enabled = config.Policy.Network.ListenIPVersion != "ipv4"
	samples, cities, err := manager.SampleCities(ctx, plan.IPv4Enabled, plan.IPv6Enabled)
	if err != nil {
		return plan, err
	}
	plan.Cities = cities
	missing := 0
	for _, city := range cities {
		if city.IPv4 < city.IPv4Target || city.IPv6 < city.IPv6Target {
			missing++
		}
	}
	if missing > 0 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%d 个城市的数据库记录未达到目标数量，已列出具体缺口；缺少的 IP 未编造或重复补齐。", missing))
	}
	patterns := []string{}
	for _, entry := range config.Policy.IPGroup.Entries {
		if entry.Type == "allow" && (entry.ExpiresAt == 0 || entry.ExpiresAt > time.Now().Unix()) {
			patterns = append(patterns, entry.Value)
		}
	}
	group := config.Policy.IPGroup
	if len(group.AllowGroups) > 0 {
		db, err := pipelineDB()
		if err != nil {
			return plan, err
		}
		var rows []model.IPGroupItem
		if err := db.WithContext(ctx).Where("user_code = ? AND tenant_id = ? AND group_code IN ?", global.GWAF_USER_CODE, global.GWAF_TENANT_ID, group.AllowGroups).Order("group_code, ip").Limit(128).Find(&rows).Error; err != nil {
			return plan, err
		}
		for _, row := range rows {
			patterns = append(patterns, row.Ip)
		}
	}
	// Include actual configured exceptions without changing the family ratio.
	used := map[string]bool{}
	for _, s := range samples {
		used[s.IP] = true
	}
	replaced := map[bool]int{}
	for _, pattern := range patterns {
		address, ok := assessmentPatternAddress(pattern)
		if !ok || used[address.String()] || replaced[address.Is4()] >= 2 {
			continue
		}
		if !assessmentExpectedAccess(config, address, manager) {
			continue
		}
		geo := manager.Lookup(address.String())
		family := "IPv6"
		if address.Is4() {
			family = "IPv4"
		}
		replaced[address.Is4()]++
		used[address.String()] = true
		samples = append(samples, iplocation.SampleAddress{IP: address.String(), Country: geo.Country, Province: geo.Province, City: geo.City, ISP: geo.ISP, Family: family})
	}
	allowed4, allowed6 := 0, 0
	for _, sample := range samples {
		address := netip.MustParseAddr(sample.IP)
		allowed := assessmentExpectedAccess(config, address, manager)
		plan.Sources = append(plan.Sources, AssessmentPlanSource{sample, allowed})
		if address.Is4() {
			plan.IPv4++
			if allowed {
				allowed4++
			}
		} else {
			plan.IPv6++
			if allowed {
				allowed6++
			}
		}
	}
	if plan.IPv4 != plan.IPv6 && plan.IPv4Enabled && plan.IPv6Enabled {
		plan.Warnings = append(plan.Warnings, "双栈来源数量因地区库覆盖或访问名单而不同；详情显示实际数量，不将缺失的协议计为已验证。")
	}
	if allowed4 == 0 {
		plan.Warnings = append(plan.Warnings, "本轮未找到预期允许的 IPv4 来源，IPv4 正常访问能力尚未验证。")
	}
	if allowed6 == 0 {
		plan.Warnings = append(plan.Warnings, "本轮未找到预期允许的 IPv6 来源，IPv6 正常访问能力尚未验证。")
	}
	plan.Samples = wafpipeline.GeneratedAssessmentSamples(config)
	for _, source := range plan.Sources {
		if source.ExpectAccess {
			plan.Samples = append(plan.Samples, wafpipeline.AssessmentSample{ID: "spoofed-source", Category: "source_spoof", Kind: "benign", Method: "GET", Path: "/", Headers: map[string]string{"X-Forwarded-For": source.IP, "X-Real-IP": source.IP, "Forwarded": "for=" + source.IP}})
			break
		}
	}
	plan.Pressure = min(1998, ((max(600, config.Policy.RuleSet.QPS.Burst*3, config.Policy.RuleSet.CC.Threshold*3)+5)/6)*6)
	if plan.Generation != manager.Generation() {
		return plan, errors.New("IP 数据库在生成方案时已更新，请重试")
	}
	return plan, nil
}

func assessmentExpectedAccess(config wafpipeline.PortConfig, address netip.Addr, manager *iplocation.Manager) bool {
	group := config.Policy.IPGroup
	ip := net.IP(address.AsSlice())
	allow := false
	for _, entry := range group.Entries {
		if entry.ExpiresAt > 0 && entry.ExpiresAt <= time.Now().Unix() {
			continue
		}
		pattern, err := ipset.ParsePattern(entry.Value)
		if err != nil || !pattern.Match(ip) {
			continue
		}
		if entry.Type == "deny" {
			return false
		}
		allow = true
	}
	for _, code := range group.DenyGroups {
		matcher := ipset.GetGroupMatcher(code)
		if matcher != nil && matcher.Contains(ip) {
			return false
		}
	}
	geo := manager.Lookup(address.String())
	match := func(country, province string) bool {
		return !geo.Unresolved && strings.EqualFold(country, geo.Country) && (province == "" || strings.EqualFold(province, geo.Province))
	}
	if group.BlockOutsideMainland && !address.IsPrivate() && !address.IsLoopback() {
		mainland := match("中国", "") || match("China", "") || match("CN", "")
		for _, outside := range []string{"香港", "澳门", "台湾", "Hong Kong", "Macao", "Taiwan"} {
			if strings.Contains(geo.Province, outside) {
				mainland = false
			}
		}
		if !mainland {
			return false
		}
	}
	for _, region := range group.DeniedRegions {
		if match(region.Country, region.Province) {
			return false
		}
	}
	if allow {
		return true
	}
	for _, code := range group.AllowGroups {
		matcher := ipset.GetGroupMatcher(code)
		if matcher != nil && matcher.Contains(ip) {
			return true
		}
	}
	for _, region := range group.Regions {
		if match(region.Country, region.Province) {
			return true
		}
	}
	return group.UnlistedAction == wafpipeline.Continue || group.UnlistedAction == wafpipeline.Allow || group.UnlistedAction == wafpipeline.Bypass
}

func assessmentPatternAddress(raw string) (netip.Addr, bool) {
	pattern, err := ipset.ParsePattern(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	result := append([]byte{}, pattern.Value...)
	switch pattern.Kind {
	case ipset.KindRange:
		low, high := new(big.Int).SetBytes(pattern.Value), new(big.Int).SetBytes(pattern.End)
		width := new(big.Int).Add(new(big.Int).Sub(high, low), big.NewInt(1))
		value, err := rand.Int(rand.Reader, width)
		if err != nil {
			return netip.Addr{}, false
		}
		value.Add(value, low).FillBytes(result)
	case ipset.KindCIDR, ipset.KindWildcard:
		random := make([]byte, pattern.Width)
		if _, err := rand.Read(random); err != nil {
			return netip.Addr{}, false
		}
		for i := range result {
			result[i] |= random[i] &^ pattern.Mask[i]
		}
	}
	address, ok := netip.AddrFromSlice(result)
	if !ok || address.IsUnspecified() || address.IsMulticast() {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}
