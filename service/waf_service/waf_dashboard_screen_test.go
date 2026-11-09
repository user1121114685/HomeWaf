package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	response "HomeWaf/model/response"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func TestDashboardRollingWindowAndGlobalVisitors(t *testing.T) {
	live, archive := openAppTestDB(t), openAppTestDB(t)
	for _, db := range []*gorm.DB{live, archive} {
		if err := db.AutoMigrate(&model.AccessLog{}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 11, 9, 18, 30, 0, 500000000, time.FixedZone("CST", 8*3600))
	add := func(db *gorm.DB, id, ip, host, tenant, action, country string, at int64) {
		t.Helper()
		entry := model.AccessLog{LogNarrow: model.LogNarrow{ReqUUID: id, TenantId: tenant, UserCode: "user", HostCode: host, Host: host + ".example", SRC_IP: ip, ACTION: action, COUNTRY: country, UNIX_ADD_TIME: at}}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	from := now.Add(-24 * time.Hour).UnixMilli()
	add(live, "end", "8.8.8.8", "a", "tenant", "阻止", "United States", now.UnixMilli())
	add(live, "same-ip-different-site", "8.8.8.8", "b", "tenant", "放行", "United States", now.Add(-time.Hour).UnixMilli())
	add(archive, "archive", "8.8.8.8", "c", "tenant", "阻止", "United States", from+1)
	add(archive, "v6", "2001:4860:4860::8888", "c", "tenant", "放行", "United States", from+2)
	add(live, "other-ip", "1.1.1.1", "a", "tenant", "阻止", "Australia", now.Add(-2*time.Hour).UnixMilli())
	add(archive, "excluded-start", "9.9.9.9", "a", "tenant", "阻止", "Germany", from)
	add(live, "excluded-future", "7.7.7.7", "a", "tenant", "放行", "China", now.UnixMilli()+1)
	add(live, "excluded-tenant", "6.6.6.6", "a", "other", "放行", "China", now.UnixMilli())
	result, err := buildScreenSnapshot(context.Background(), now, []screenSource{{live, "access_log", "live"}, {archive, "access_log", "archive"}}, "tenant", "user")
	if err != nil {
		t.Fatal(err)
	}
	if result.WindowEnd-result.WindowStart != 86400000 || result.Summary.Visits != 5 || result.Summary.Blocked != 3 || result.Summary.Visitors != 3 || result.Summary.AttackIPs != 2 {
		t.Fatalf("incorrect rolling statistics: %+v", result)
	}
	if len(result.Events) != 3 || result.Events[0].ID != "end" || len(result.IPs) != 2 || result.IPs[0].Count != 2 {
		t.Fatalf("incorrect global ranking/events: %+v", result)
	}
	if result.Partial {
		t.Fatal(result.Issues)
	}
	partial, err := buildScreenSnapshot(context.Background(), now, []screenSource{{live, "access_log", "live"}, {archive, "missing_table", "missing"}}, "tenant", "user")
	if err != nil || !partial.Partial {
		t.Fatalf("missing partitions must be visible: %v %+v", err, partial)
	}
}

func TestDashboardShareLifecycleAndRedaction(t *testing.T) {
	db := openAppTestDB(t)
	if err := db.AutoMigrate(&model.DashboardShare{}, &model.DashboardTarget{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldTenant, oldUser := global.GWAF_LOCAL_DB, global.GWAF_TENANT_ID, global.GWAF_USER_CODE
	global.GWAF_LOCAL_DB, global.GWAF_TENANT_ID, global.GWAF_USER_CODE = db, "screen-tenant", "screen-user"
	t.Cleanup(func() { global.GWAF_LOCAL_DB, global.GWAF_TENANT_ID, global.GWAF_USER_CODE = oldDB, oldTenant, oldUser })
	if _, _, err := CreateDashboardShare("bad", 169); err == nil {
		t.Fatal("unbounded lifetime accepted")
	}
	share, token, err := CreateDashboardShare("test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 47 || share.TokenHash == token || share.Scope != DashboardShareScope {
		t.Fatal("invalid token storage or scope")
	}
	if _, err := ValidateDashboardShare(token); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(share)
	if strings.Contains(string(encoded), token) || strings.Contains(string(encoded), share.TokenHash) {
		t.Fatal("share list leaks credentials")
	}
	if _, err := ValidateDashboardShare("hws_" + strings.Repeat("a", 43)); err == nil {
		t.Fatal("forged token accepted")
	}
	global.GWAF_TENANT_ID = "other"
	if _, err := ValidateDashboardShare(token); err == nil {
		t.Fatal("token escaped tenant")
	}
	global.GWAF_TENANT_ID = "screen-tenant"
	if err := RevokeDashboardShare(share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDashboardShare(token); err == nil {
		t.Fatal("revoked token accepted")
	}
	expired, expiredToken, err := CreateDashboardShare("expired", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.DashboardShare{}).Where("id = ?", expired.ID).Update("expires_at", time.Now().Add(-time.Second).UnixMilli()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDashboardShare(expiredToken); err == nil {
		t.Fatal("expired token accepted")
	}
	source := &response.ScreenSnapshot{Target: model.DashboardTarget{PublicIP: "9.9.9.9"}, IPs: []response.ScreenIP{{IP: "8.8.8.8", Count: 2}}, Events: []response.ScreenEvent{{ID: "private-request-id", IP: "8.8.8.8", Host: "secret.internal", Rule: "secret-rule", Province: "California", City: "Mountain View"}}}
	redacted := RedactScreenSnapshot(source, share.ExpiresAt)
	encoded, _ = json.Marshal(redacted)
	for _, secret := range []string{"8.8.8.8", "9.9.9.9", "secret.internal", "secret-rule", "private-request-id", "Mountain View"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("shared response leaked %s", secret)
		}
	}
	if source.IPs[0].IP != "8.8.8.8" || source.Events[0].Host != "secret.internal" {
		t.Fatal("redaction mutated the admin snapshot")
	}
}
