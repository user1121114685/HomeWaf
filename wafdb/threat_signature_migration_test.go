package wafdb

import (
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
)

func TestThreatSignatureColumnsAddedWithoutDroppingLegacyChannel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.Exec("CREATE TABLE threat_ip_channel (id TEXT PRIMARY KEY, code TEXT, enable INTEGER, url TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO threat_ip_channel(id,code,enable,url) VALUES ('legacy','old_source',1,'https://legacy.example/feed')").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, field := range []string{"SignatureURL", "FeedID", "SigningKeyID", "SigningPublicKey", "SignedSequence", "SignedDigest", "SignatureExpiresAt"} {
		if !db.Migrator().HasColumn(&model.ThreatIPChannel{}, field) {
			t.Fatalf("missing %s", field)
		}
	}
	var ch model.ThreatIPChannel
	if err = db.First(&ch, "id = ?", "legacy").Error; err != nil {
		t.Fatal(err)
	}
	if ch.Code != "old_source" || ch.Enable != 1 || ch.URL != "https://legacy.example/feed" || ch.SignedSequence != 0 || ch.SigningPublicKey != "" {
		t.Fatal("migration changed legacy data or invented verification")
	}
}
