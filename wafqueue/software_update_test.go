package wafqueue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafsec"
	"HomeWaf/wafupdate"
	"gorm.io/gorm"
)

func TestScheduledSoftwareUpdateReachesSubscribedWebhook(t *testing.T) {
	dir := t.TempDir()
	dialect.Register(&dialect.SQLiteDialect{})
	if err := wafsec.InitDataKey(dir, ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "notify.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.NotifyChannel{}, &model.NotifySubscription{}, &model.NotifyLog{}, &model.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	var deliveries atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".json") {
			json.NewEncoder(w).Encode(map[string]any{"Version": "v9.1.1", "Sha256": make([]byte, 32)})
			return
		}
		if r.URL.Path != "/subscribed" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body struct{ Title string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title != "v9.1.0 → v9.1.1" {
			t.Errorf("invalid release notification %+v %v", body, err)
		}
		deliveries.Add(1)
	}))
	defer endpoint.Close()

	oldDB, oldLogs, oldSource, oldVersion := global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GUPDATE_VERSION_URL, global.GWAF_RELEASE_VERSION
	oldAllow := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB, global.GUPDATE_VERSION_URL, global.GWAF_RELEASE_VERSION = oldDB, oldLogs, oldSource, oldVersion
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = oldAllow })
	})
	global.GWAF_LOCAL_DB, global.GWAF_LOCAL_LOG_DB = db, db
	global.GUPDATE_VERSION_URL, global.GWAF_RELEASE_VERSION = endpoint.URL+"/", "v9.1.0"
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	config, _ := json.Marshal(map[string]string{"url": endpoint.URL + "/subscribed", "body_template": `{"title":"{{.Title}}"}`})
	channel := model.NotifyChannel{BaseOrm: baseorm.BaseOrm{Id: "test-webhook"}, Type: "webhook", ConfigJSON: string(config), Status: 1}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	sub := model.NotifySubscription{BaseOrm: baseorm.BaseOrm{Id: "test-release"}, ChannelId: channel.Id, MessageType: model.MSG_TYPE_SOFTWARE_UPDATE,
		Status: 1, ThrottleMode: model.ThrottleModeRealtime, TitleTemplate: "{{.CurrentVersion}} → {{.NewVersion}}"}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	// No browser or manual check. Advance across the first two due times, reopening
	// the instance data each time as a restarted worker would.
	start := time.Now()
	for _, elapsed := range []time.Duration{0, 48 * time.Hour, 96 * time.Hour, 144 * time.Hour} {
		if err := wafupdate.NewReleaseMonitor(dir).CheckDue(start.Add(elapsed), NotifySoftwareUpdate); err != nil {
			t.Fatal(err)
		}
	}
	var logs []model.NotifyLog
	if err := db.Where("subscription_id = ?", sub.Id).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if deliveries.Load() != 1 || len(logs) != 1 || logs[0].Status != 1 || logs[0].MessageType != model.MSG_TYPE_SOFTWARE_UPDATE {
		t.Fatalf("deliveries=%d logs=%+v", deliveries.Load(), logs)
	}
}
