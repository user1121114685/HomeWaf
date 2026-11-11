package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/waftask/threatip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacySignedSubscriptionPreservesCacheAndReplayState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "signed.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	defer connection.Close()
	if err = db.AutoMigrate(&model.ThreatIPChannel{}, &model.ThreatIPSnapshot{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldExclude, oldSettings := global.GWAF_LOCAL_DB, WafThreatIPExcludeServiceApp, global.CurrentSettings()
	global.GWAF_LOCAL_DB = db
	WafThreatIPExcludeServiceApp = &WafThreatIPExcludeService{}
	WafThreatIPExcludeServiceApp.store(&ExcludeSet{})
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	defer func() {
		global.GWAF_LOCAL_DB = oldDB
		WafThreatIPExcludeServiceApp = oldExclude
		global.UpdateSettings(func(c *global.RuntimeSettings) { *c = oldSettings })
	}()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	body := []byte("198.51.100.10\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	var mu sync.RWMutex
	var doc []byte
	var calls atomic.Int64
	set := func(sequence uint64) {
		t.Helper()
		data, err := licensing.SignThreatSnapshot(licensing.ThreatSnapshotPayload{Schema: 1, FeedID: "family", Sequence: sequence, SHA256: digest, ExpiresAt: time.Now().Add(time.Hour).Unix()}, "test", key)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		doc = data
		mu.Unlock()
	}
	set(10)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		mu.RLock()
		defer mu.RUnlock()
		if r.URL.Path == "/manifest" {
			w.Write(doc)
		} else {
			w.Write(body)
		}
	}))
	defer server.Close()
	payload, sha, count, _ := threatip.EncodeSnapshot([]string{"198.51.100.10"})
	ch := model.ThreatIPChannel{BaseOrm: baseorm.BaseOrm{Id: "channel"}, Code: "family", URL: server.URL + "/raw", Enable: 1, ParserType: model.ThreatParserCIDROnly, LandTarget: model.ThreatLandWAF, LandedSha: sha}
	if err = db.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	cache := model.ThreatIPSnapshot{BaseOrm: baseorm.BaseOrm{Id: "original-cache"}, ChannelCode: ch.Code, Payload: payload, Sha256: sha, Count: count}
	if err = db.Create(&cache).Error; err != nil {
		t.Fatal(err)
	}
	s := &WafThreatIPService{signedClient: server.Client()}
	if err = s.SyncChannel(ch); err == nil || calls.Load() != 0 {
		t.Fatal("unsigned legacy subscription contacted the network")
	}
	ch.SignatureURL = server.URL + "/manifest"
	ch.FeedID = "family"
	ch.SigningKeyID = "test"
	ch.SigningPublicKey = base64.RawURLEncoding.EncodeToString(pub)
	if err = db.Save(&ch).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.SyncChannel(ch); err != nil {
		t.Fatal(err)
	}
	if err = db.First(&ch, "id = ?", ch.Id).Error; err != nil {
		t.Fatal(err)
	}
	if ch.SignedSequence != 10 || ch.SignedDigest != digest {
		t.Fatal("authenticated watermark not persisted")
	}
	assertCache := func() {
		t.Helper()
		var saved model.ThreatIPSnapshot
		if err := db.First(&saved, "channel_code = ?", ch.Code).Error; err != nil || saved.Id != cache.Id || saved.Sha256 != sha {
			t.Fatal("last good cache lost")
		}
		var current model.ThreatIPChannel
		db.First(&current, "id = ?", ch.Id)
		if current.SignedSequence != 10 {
			t.Fatal("failed transaction advanced replay watermark")
		}
	}
	assertCache()
	set(9)
	restarted := &WafThreatIPService{signedClient: server.Client()}
	if err = restarted.SyncChannel(ch); err == nil {
		t.Fatal("restart accepted a rolled back feed")
	}
	assertCache()
	if err = db.Callback().Create().Before("gorm:create").Register("reject-threat-cache", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "threat_ip_snapshot" {
			tx.AddError(errors.New("fixture cache write failed"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	auth := licensing.ThreatSnapshotManifest{ThreatSnapshotPayload: licensing.ThreatSnapshotPayload{Schema: 1, Sequence: 11, SHA256: digest, ExpiresAt: time.Now().Add(time.Hour).Unix()}}
	if err = s.saveSignedSnapshot(ch, auth, payload, sha, count, true); err == nil {
		t.Fatal("injected cache error not propagated")
	}
	assertCache()
	db.Callback().Create().Remove("reject-threat-cache")
	if err = db.Model(&ch).Update("enable", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.saveSignedSnapshot(ch, auth, payload, sha, count, true); err == nil {
		t.Fatal("stale enabled configuration committed")
	}
	assertCache()
	before := calls.Load()
	if err = s.SyncChannel(ch); err == nil || calls.Load() != before {
		t.Fatal("disabled subscription contacted network")
	}
}
