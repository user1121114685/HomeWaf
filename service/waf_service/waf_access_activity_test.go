package waf_service

import (
	"HomeWaf/cache"
	"HomeWaf/common/queue"
	"HomeWaf/customtype"
	"HomeWaf/enums"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/wafenginecore/accessgate"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func accessActivityFixture(t *testing.T) (*WafAccessSessionService, *gorm.DB, *cache.WafCache) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "activity.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.AccessSession{}, &model.AccessToken{}); err != nil {
		t.Fatal(err)
	}
	store := cache.InitWafCache()
	oldDB, oldCache, oldQueue := global.GWAF_LOCAL_DB, global.GCACHE_WAFCACHE, global.GQEQUE_UPDATE_DB
	global.GWAF_LOCAL_DB, global.GCACHE_WAFCACHE, global.GQEQUE_UPDATE_DB = db, store, queue.NewQueue()
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GCACHE_WAFCACHE, global.GQEQUE_UPDATE_DB = oldDB, oldCache, oldQueue
		store.Close()
		conn.Close()
	})
	return &WafAccessSessionService{}, db, store
}

func applyQueuedActivity(t *testing.T, db *gorm.DB) {
	t.Helper()
	for {
		value, ok := global.GQEQUE_UPDATE_DB.Dequeue()
		if !ok {
			return
		}
		u, ok := value.(innerbean.UpdateModel)
		if !ok {
			t.Fatalf("unexpected update type %T", value)
		}
		if err := db.Model(u.Model).Where(u.Query, u.Args...).Updates(u.Update).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestAccessActivityQueuedCoalescedAndCannotReviveRevocation(t *testing.T) {
	s, db, _ := accessActivityFixture(t)
	now := time.Now().Truncate(time.Second)
	previous := now.Add(-10 * time.Minute)
	row := model.AccessSession{BaseOrm: pipelineORM(), SessionCode: "session-activity", Status: model.AccessStatusValid, LastActiveTime: customtype.JsonTime(previous), ExpireTime: customtype.JsonTime(now.Add(time.Hour))}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for n := 0; n < 64; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.touchSession(row, now, time.Minute) }()
	}
	wg.Wait()
	if global.GQEQUE_UPDATE_DB.Stats()["queued"] != 1 {
		t.Fatal("concurrent requests were not coalesced")
	}
	var stored model.AccessSession
	if err := db.First(&stored, "session_code = ?", row.SessionCode).Error; err != nil {
		t.Fatal(err)
	}
	if !time.Time(stored.LastActiveTime).Equal(previous) {
		t.Fatal("request synchronously wrote activity")
	}
	applyQueuedActivity(t, db)
	if err := db.First(&stored, "session_code = ?", row.SessionCode).Error; err != nil {
		t.Fatal(err)
	}
	if !time.Time(stored.LastActiveTime).Equal(now) {
		t.Fatal("queued activity was not persisted")
	}
	s.touchSession(row, now.Add(2*time.Minute), time.Minute)
	otherWorker := &WafAccessSessionService{}
	otherWorker.touchSession(row, now.Add(time.Minute), time.Minute)
	applyQueuedActivity(t, db)
	if err := db.First(&stored, "session_code = ?", row.SessionCode).Error; err != nil {
		t.Fatal(err)
	}
	if !time.Time(stored.LastActiveTime).Equal(now.Add(2 * time.Minute)) {
		t.Fatal("delayed older update moved activity backwards")
	}
	s.touchSession(row, now.Add(3*time.Minute), time.Minute)
	if err := db.Model(&model.AccessSession{}).Where("session_code = ?", row.SessionCode).Update("status", model.AccessStatusRevoked).Error; err != nil {
		t.Fatal(err)
	}
	applyQueuedActivity(t, db)
	if err := db.First(&stored, "session_code = ?", row.SessionCode).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.AccessStatusRevoked || !time.Time(stored.LastActiveTime).Equal(now.Add(2*time.Minute)) {
		t.Fatal("late touch altered a revoked session")
	}
}

func TestAccessCachedIdleAndAbsoluteExpiryMatchSessionPolicy(t *testing.T) {
	s, db, store := accessActivityFixture(t)
	now := time.Now()
	plain := "isolated-token"
	code := HashAccessCode(plain)
	cfg := &accessgate.Config{IdleTimeout: time.Minute, CachePositiveTTL: time.Minute}
	state := AccessState{SessionCode: "cached-session", Host: "app.test", HostCode: "site-a", ExpireUnix: now.Add(time.Hour).Unix(), SessionExpireUnix: now.Add(time.Hour).Unix(), SessionLastActive: now.Add(-2 * time.Minute)}
	store.SetWithTTlRenewTime(enums.CACHE_ACCESS_TOKEN+code, state, time.Minute)
	if s.ValidateToken(plain, "app.test", "site-a", "192.0.2.1", "", cfg) != nil {
		t.Fatal("cached token bypassed session idle timeout")
	}
	state.SessionLastActive = now.Add(-40 * time.Second)
	store.SetWithTTlRenewTime(enums.CACHE_ACCESS_TOKEN+code, state, time.Minute)
	if s.ValidateToken(plain, "wrong.test", "site-a", "192.0.2.1", "", cfg) != nil {
		t.Fatal("wrong binding accepted")
	}
	if len(s.activity) != 0 {
		t.Fatal("invalid binding refreshed activity")
	}
	expiry, _ := store.GetExpireTime(enums.CACHE_ACCESS_TOKEN + code)
	if s.ValidateToken(plain, "app.test", "site-a", "192.0.2.1", "", cfg) == nil {
		t.Fatal("valid cached visitor rejected")
	}
	after, _ := store.GetExpireTime(enums.CACHE_ACCESS_TOKEN + code)
	if !after.Equal(expiry) {
		t.Fatal("activity extended authorization cache lifetime")
	}
	if s.sessionIdle(state.SessionCode, state.SessionLastActive, now.Add(30*time.Second), time.Minute) {
		t.Fatal("valid business request did not refresh central-session activity")
	}
	state.SessionExpireUnix = now.Add(-time.Second).Unix()
	store.SetWithTTlRenewTime(enums.CACHE_ACCESS_TOKEN+code, state, time.Minute)
	if s.ValidateToken(plain, "app.test", "site-a", "192.0.2.1", "", cfg) != nil {
		t.Fatal("recent activity bypassed absolute expiry")
	}
	row := model.AccessSession{BaseOrm: pipelineORM(), SessionCode: "expired-central", Status: model.AccessStatusValid, LastActiveTime: customtype.JsonTime(now.Add(-2 * time.Minute)), ExpireTime: customtype.JsonTime(now.Add(time.Hour))}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	store.SetWithTTlRenewTime(enums.CACHE_ACCESS_SESSION+row.SessionCode, row, time.Minute)
	if s.loadValidSession(row.SessionCode, cfg) != nil {
		t.Fatal("cached central session ignored idle expiry")
	}
	store.Remove(enums.CACHE_ACCESS_SESSION + row.SessionCode)
	if s.loadValidSession(row.SessionCode, cfg) != nil {
		t.Fatal("database central session ignored idle expiry")
	}
}

func TestAccessActivityCapacityAndFullQueueRemainNonBlocking(t *testing.T) {
	s, db, _ := accessActivityFixture(t)
	global.GQEQUE_UPDATE_DB = queue.NewBoundedQueue(2, 2, 4096)
	if !global.GQEQUE_UPDATE_DB.TryEnqueue("occupied-1") || !global.GQEQUE_UPDATE_DB.TryEnqueue("occupied-2") {
		t.Fatal("failed to fill fixture queue")
	}
	now := time.Now()
	// Prove both full and absent queues have no synchronous database fallback.
	global.GWAF_LOCAL_DB = nil
	for n := 0; n < maxAccessActivity+10; n++ {
		s.touchSession(model.AccessSession{SessionCode: fmt.Sprint(n)}, now, time.Minute)
	}
	if len(s.activity) != maxAccessActivity || s.activityOrder.Len() != maxAccessActivity {
		t.Fatal("activity metadata exceeded quota")
	}
	if global.GQEQUE_UPDATE_DB.Stats()["queued"] != 2 {
		t.Fatal("capacity was bypassed")
	}
	global.GQEQUE_UPDATE_DB = nil
	s.touchToken(model.AccessToken{TokenCode: "no-consumer"}, now)
	global.GWAF_LOCAL_DB = db
}
