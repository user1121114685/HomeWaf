package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/protection/iplocation"
	"HomeWaf/internal/protection/wafenginecore/ipset"
	"HomeWaf/internal/protection/wafpipeline"
	"context"
	"net/netip"
	"os"
	"testing"
	"time"
)

func TestAssessmentAutomaticPlanRunsAllCitiesAndScopesGroups(t *testing.T) {
	db := setupIPGroupTestDB(t)
	manager := iplocation.NewManager()
	defer manager.Close()
	v4, err := os.ReadFile("../../../cmd/homewaf/exedata/ip2region.xdb")
	if err != nil {
		t.Fatal(err)
	}
	v6, err := os.ReadFile("../../../cmd/homewaf/exedata/ip2region_v6.xdb")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.LoadV4Ip2Region(v4, iplocation.FormatOpenSource); err != nil {
		t.Fatal(err)
	}
	if err := manager.LoadV6Ip2Region(v6, iplocation.FormatOpenSource); err != nil {
		t.Fatal(err)
	}
	previous := global.GIPLOCATION_MANAGER
	global.GIPLOCATION_MANAGER = manager
	defer func() { global.GIPLOCATION_MANAGER = previous }()
	for _, row := range []model.IPGroupItem{
		{BaseOrm: baseorm.BaseOrm{Id: "ours", USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID}, GroupCode: "assessment-scope", Ip: "10.88.1.25"},
		{BaseOrm: baseorm.BaseOrm{Id: "other", USER_CODE: "other", Tenant_ID: "other"}, GroupCode: "assessment-scope", Ip: "10.99.1.25"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ipset.UpsertGroupMatcher("assessment-scope", "", ipset.BuildMatchSet([]string{"10.88.1.25"}))
	config := wafpipeline.PortConfig{Schema: 1, Port: 15000, Protocol: "http", Upstream: "http://127.0.0.1:18080", Policy: wafpipeline.DefaultPolicy()}
	config.Policy.IPGroup.AllowGroups = []string{"assessment-scope"}
	config.Policy.IPGroup.Regions = []wafpipeline.Region{{Country: "中国", Province: "重庆市"}}
	config.Policy.IPGroup.UnlistedAction = wafpipeline.Deny
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plan, err := BuildAssessmentPlan(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	input := wafpipeline.AssessmentInput{Samples: plan.Samples, Pressure: plan.Pressure, PressureMode: "mixed"}
	for _, source := range plan.Sources {
		if source.IP == "10.99.1.25" {
			t.Fatal("cross-tenant source leaked")
		}
		if source.IP == "10.88.1.25" {
			found = source.ExpectAccess
		}
		input.Sources = append(input.Sources, wafpipeline.AssessmentSource{IP: source.IP, ExpectAccess: source.ExpectAccess})
	}
	if !found {
		t.Fatal("selected shared allow group missing")
	}
	geo := wafpipeline.NewGeoCache(func(ip netip.Addr) wafpipeline.GeoResult {
		g := manager.Lookup(ip.String())
		return wafpipeline.GeoResult{Country: g.Country, Province: g.Province, City: g.City, ISP: g.ISP, OK: !g.Unresolved}
	}, manager.Generation)
	controls := wafpipeline.NewRulesA(wafpipeline.NewGate(nil))
	engine := wafpipeline.DefaultRuleEngine()
	runtime := &wafpipeline.Runtime{Geo: geo, RuleSteps: func(s *wafpipeline.Snapshot) []wafpipeline.Step { return engine.WithControls(s, controls.Steps(s)) }}
	result, err := runtime.AssessMany(ctx, &wafpipeline.Snapshot{Config: config}, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Evaluations != len(plan.Sources)*len(plan.Samples) || result.Score == nil || result.NormalTotal == 0 {
		t.Fatalf("incomplete automatic report: checks=%d score=%v normals=%d", result.Evaluations, result.Score, result.NormalTotal)
	}
	t.Logf("cities=%d v4=%d v6=%d samples=%d checks=%d direct_score=%d elapsed_ms=%.0f", len(plan.Cities), plan.IPv4, plan.IPv6, len(plan.Samples), result.Evaluations, *result.Score, result.ElapsedMS)
}
