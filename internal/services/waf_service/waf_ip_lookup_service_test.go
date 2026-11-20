// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"path/filepath"
	"time"

	"HomeWaf/internal/protection/wafpipeline"
	"testing"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/response"

	sqlite "HomeWaf/internal/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupIPLookupTestDB 建临时库并接管全局 DB，用例结束后还原
func setupIPLookupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "iplookup_test.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(
		&model.IPGroup{}, &model.IPGroupItem{}, &model.FirewallIPBlock{},
	); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	previousPorts := wafpipeline.Ports
	wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](10)
	t.Cleanup(func() { wafpipeline.Ports = previousPorts })
	oldDB, oldTenant, oldUser := global.GWAF_LOCAL_DB, global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	global.GWAF_LOCAL_DB = db
	global.GWAF_TENANT_ID, global.GWAF_USER_CODE = "SamWafCom", "user-uuid-iplookup"
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB = oldDB
		global.GWAF_TENANT_ID, global.GWAF_USER_CODE = oldTenant, oldUser
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// newLookupBase 与 crossdb 套件里的 newBase 同名会撞（那边无 build tag 隔离），故独立命名
func newLookupBase(id string) baseorm.BaseOrm {
	return baseorm.BaseOrm{Id: id, USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID}
}

func publishLookupService(t *testing.T, port uint16, group wafpipeline.IPGroupConfig) {
	t.Helper()
	config := wafpipeline.PortConfig{Name: "Family", Port: port, Policy: wafpipeline.DefaultPolicy()}
	config.Policy.IPGroup = group
	if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) {
		return &wafpipeline.Snapshot{Config: config}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestLookupMatchesCIDR 网段必须能命中：用户填的是 1.2.3.0/24，查 1.2.3.4 得算命中。
// 这条最容易写错成字符串相等，一旦退化就会误报「这个IP不在黑名单里」。
func TestLookupMatchesCIDR(t *testing.T) {
	setupIPLookupTestDB(t)
	publishLookupService(t, 14001, wafpipeline.IPGroupConfig{Entries: []wafpipeline.IPEntry{{Type: "deny", Value: "1.2.3.0/24"}}})

	resp, err := WafIPLookupServiceApp.Lookup("1.2.3.4", nil)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(resp.Hits) != 1 {
		t.Fatalf("期望命中1条，实际 %d 条", len(resp.Hits))
	}
	h := resp.Hits[0]
	if h.Source != srcIPBlack || h.Effect != "block" {
		t.Fatalf("来源/效果不对: %+v", h)
	}
	if h.Matched != "1.2.3.0/24" {
		t.Fatalf("应回显实际命中的那条规则，实际 %q", h.Matched)
	}
	if h.Port != 14001 || h.Scope != "Family (:14001)" {
		t.Fatalf("host_code 应翻成域名，实际 %q", h.Scope)
	}
}

func TestLookupServiceScopeAndExpiry(t *testing.T) {
	setupIPLookupTestDB(t)
	group := wafpipeline.IPGroupConfig{Entries: []wafpipeline.IPEntry{
		{Type: "allow", Value: "10.0.0.1", Trust: "trusted"},
		{Type: "deny", Value: "10.0.0.1", ExpiresAt: time.Now().Add(-time.Minute).Unix()},
	}}
	publishLookupService(t, 14001, group)
	resp, err := WafIPLookupServiceApp.Lookup("10.0.0.1", nil)
	if err != nil || len(resp.Hits) != 1 || resp.Hits[0].Effect != "allow" || resp.Hits[0].Port != 14001 {
		t.Fatalf("service scope or expiry lost: %+v, %v", resp, err)
	}
}

func TestLookupServiceBanUsesCurrentPunisher(t *testing.T) {
	setupIPLookupTestDB(t)
	previous := WafPipelineStateServiceApp
	punisher := wafpipeline.NewPunisher()
	WafPipelineStateServiceApp = &WafPipelineStateService{Punisher: punisher}
	t.Cleanup(func() { WafPipelineStateServiceApp = previous })
	record := wafpipeline.PunishmentRecord{
		ID: "ban|port:14002|2001:db8::/64", Kind: "ban", PortID: "port:14002", Scope: "port",
		Prefix: "2001:db8::/64", Reason: "cc",
	}
	if err := punisher.Restore([]wafpipeline.PunishmentRecord{record}); err != nil {
		t.Fatal(err)
	}
	lookup := func() []response.IPLookupHit {
		t.Helper()
		result, err := WafIPLookupServiceApp.Lookup("2001:db8::1", []string{srcServiceBan})
		if err != nil {
			t.Fatal(err)
		}
		return result.Hits
	}
	if hits := lookup(); len(hits) != 1 || hits[0].Port != 14002 || hits[0].Matched != record.Prefix {
		t.Fatalf("native ban missing owning service or prefix: %+v", hits)
	}
	if err := punisher.Manage(record.PortID, record.ID, "release"); err != nil {
		t.Fatal(err)
	}
	if hits := lookup(); len(hits) != 0 {
		t.Fatalf("released ban still shown: %+v", hits)
	}
}

func TestLookupGroupRefEffect(t *testing.T) {
	db := setupIPLookupTestDB(t)
	db.Create(&model.IPGroup{BaseOrm: newLookupBase("g1"), GroupName: "办公出口", GroupCode: "office"})
	db.Create(&model.IPGroupItem{BaseOrm: newLookupBase("gi1"), GroupCode: "office", Ip: "192.168.1.0/24"})
	publishLookupService(t, 14001, wafpipeline.IPGroupConfig{DenyGroups: []string{"office"}})

	resp, _ := WafIPLookupServiceApp.Lookup("192.168.1.9", nil)
	if len(resp.Hits) != 1 {
		t.Fatalf("期望只命中IP组1条(引用行本身无IP不该重复计)，实际 %+v", resp.Hits)
	}
	if resp.Hits[0].Source != srcIPGroup || resp.Hits[0].Effect != "block" {
		t.Fatalf("被黑名单引用的组应判 block，实际 %+v", resp.Hits[0])
	}

	publishLookupService(t, 14001, wafpipeline.IPGroupConfig{AllowGroups: []string{"office"}, DenyGroups: []string{"office"}})
	resp, _ = WafIPLookupServiceApp.Lookup("192.168.1.9", nil)
	if len(resp.Hits) != 1 || resp.Hits[0].Effect != "block" {
		t.Fatalf("service deny groups must take precedence: %+v", resp.Hits)
	}
}

// TestLookupFirewallExpired 已过期但没清理的封禁记录还躺在表里，
// 不能报成「生效中」，否则用户会照着一条早失效的记录排查半天
func TestLookupFirewallExpired(t *testing.T) {
	db := setupIPLookupTestDB(t)
	db.Create(&model.FirewallIPBlock{
		BaseOrm: newLookupBase("f1"), IP: "8.8.8.8", Status: "active",
		BlockType: "manual", ExpireTime: 1000, // 1970 年，早过期
	})
	db.Create(&model.FirewallIPBlock{
		BaseOrm: newLookupBase("f2"), IP: "9.9.9.9", Status: "active",
		BlockType: "auto", ExpireTime: 0, // 永久
	})

	if resp, _ := WafIPLookupServiceApp.Lookup("8.8.8.8", nil); len(resp.Hits) != 0 {
		t.Fatalf("过期封禁不该算命中，实际 %+v", resp.Hits)
	}
	resp, _ := WafIPLookupServiceApp.Lookup("9.9.9.9", nil)
	if len(resp.Hits) != 1 || resp.Hits[0].Source != srcFirewall {
		t.Fatalf("永久封禁应命中，实际 %+v", resp.Hits)
	}
}

// TestLookupNoHit 干净的库里查任何IP都应返回空命中而不是报错，
// 前端靠这个区分「没查到」和「查询失败」
func TestLookupNoHit(t *testing.T) {
	setupIPLookupTestDB(t)
	resp, err := WafIPLookupServiceApp.Lookup("203.0.113.7", nil)
	if err != nil {
		t.Fatalf("空库查询不该报错: %v", err)
	}
	if len(resp.Hits) != 0 {
		t.Fatalf("期望0条命中，实际 %+v", resp.Hits)
	}
	if resp.Hits == nil {
		t.Fatal("hits 必须是空数组而不是 null，否则前端 .length 会炸")
	}
}

// TestLookupInvalidIP 非法输入要给明确错误，不能当成空结果
func TestLookupInvalidIP(t *testing.T) {
	setupIPLookupTestDB(t)
	if _, err := WafIPLookupServiceApp.Lookup("not-an-ip", nil); err == nil {
		t.Fatal("非法IP应返回错误")
	}
	if _, err := WafIPLookupServiceApp.Lookup("  ", nil); err == nil {
		t.Fatal("空输入应返回错误")
	}
}

// TestLookupSystemLayerFlag 防火墙封禁是内核层丢包，必须标 SystemLayer；
// 前端靠它提示「加白名单也不会通」。漏标就会让用户加完白以为好了，实际还是连不上。
func TestLookupSystemLayerFlag(t *testing.T) {
	db := setupIPLookupTestDB(t)
	db.Create(&model.FirewallIPBlock{
		BaseOrm: newLookupBase("f1"), IP: "5.5.5.5", Status: "active", BlockType: "manual",
	})
	resp, _ := WafIPLookupServiceApp.Lookup("5.5.5.5", nil)
	if len(resp.Hits) != 1 || !resp.Hits[0].SystemLayer {
		t.Fatalf("防火墙封禁应标记为系统层，实际 %+v", resp.Hits)
	}

	// WAF 层的黑名单不该被误标成系统层，否则每次加白都会弹一个无关的警告
	publishLookupService(t, 14001, wafpipeline.IPGroupConfig{Entries: []wafpipeline.IPEntry{{Type: "deny", Value: "6.6.6.6"}}})
	resp2, _ := WafIPLookupServiceApp.Lookup("6.6.6.6", nil)
	if len(resp2.Hits) != 1 || resp2.Hits[0].SystemLayer {
		t.Fatalf("IP黑名单只在WAF层，不该标系统层，实际 %+v", resp2.Hits)
	}
}

// TestLandsOnSystem 落地层判定：只有 system/both 才算落到内核
func TestLandsOnSystem(t *testing.T) {
	for v, want := range map[string]bool{"system": true, "both": true, "waf": false, "": false} {
		if landsOnSystem(v) != want {
			t.Fatalf("landsOnSystem(%q) 应为 %v", v, want)
		}
	}
}

// TestLookupAcceptsCIDR 从「查看IP」列表点进来时传的是网段，
// 不能直接判非法——取代表IP查，并如实说明查的是哪个
func TestLookupAcceptsCIDR(t *testing.T) {
	setupIPLookupTestDB(t)
	publishLookupService(t, 14001, wafpipeline.IPGroupConfig{Entries: []wafpipeline.IPEntry{{Type: "deny", Value: "1.2.3.0/24"}}})

	resp, err := WafIPLookupServiceApp.Lookup("1.2.3.0/24", nil)
	if err != nil {
		t.Fatalf("网段输入不该报错: %v", err)
	}
	if resp.IP != "1.2.3.0" {
		t.Fatalf("应归一到网络地址，实际 %q", resp.IP)
	}
	if resp.QueryNote == "" {
		t.Fatal("归一化了就必须告诉用户查的是哪个IP")
	}
	if len(resp.Hits) != 1 {
		t.Fatalf("应命中该网段本身，实际 %+v", resp.Hits)
	}

	// 单个IP不该产生说明文案，否则每次查询都挂一条噪音
	if r2, _ := WafIPLookupServiceApp.Lookup("1.2.3.9", nil); r2.QueryNote != "" {
		t.Fatalf("单IP不该有归一说明，实际 %q", r2.QueryNote)
	}
	// 区间取起始
	if r3, _ := WafIPLookupServiceApp.Lookup("1.2.3.5-1.2.3.9", nil); r3.IP != "1.2.3.5" {
		t.Fatalf("区间应取起始，实际 %q", r3.IP)
	}
}
