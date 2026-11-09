package secretfield_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/wafsec"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func secretDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secrets.db")
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(path)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := db.DB()
	t.Cleanup(func() { _ = raw.Close() })
	if err := db.AutoMigrate(&model.SslConfig{}, &model.SslOrder{}, &model.Hosts{}, &model.PrivateInfo{}, &model.NotifyChannel{}, &model.HttpAuthBase{}); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func TestProtectedFieldsCreateUpdateRead(t *testing.T) {
	db, path := secretDB(t)
	const private = "fixture-private-key-that-must-never-appear-in-sqlite"
	base := baseorm.BaseOrm{Id: "one"}
	for _, row := range []interface{}{
		&model.SslConfig{BaseOrm: base, KeyContent: private},
		&model.SslOrder{BaseOrm: base, ApplyKey: private, ResultPrivateKey: []byte(private)},
		&model.Hosts{BaseOrm: base, Keyfile: private},
		&model.PrivateInfo{BaseOrm: base, PrivateValue: private},
		&model.NotifyChannel{BaseOrm: base, WebhookURL: private, Secret: private, AccessToken: private, ConfigJSON: private},
		&model.HttpAuthBase{BaseOrm: base, Password: private},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, update := range []interface{}{
		map[string]interface{}{"KeyContent": private + "-map"},
		map[string]interface{}{"key_content": private + "-column"},
		model.SslConfig{KeyContent: private + "-struct"},
	} {
		if err := db.Model(model.SslConfig{}).Where("id = ?", base.Id).Updates(update).Error; err != nil {
			t.Fatal(err)
		}
	}
	var config model.SslConfig
	if err := db.First(&config, "id = ?", base.Id).Error; err != nil || config.KeyContent != private+"-struct" {
		t.Fatalf("decrypted config mismatch: %v", err)
	}
	if err := db.Save(&config).Error; err != nil {
		t.Fatal(err)
	}
	if config.KeyContent != private+"-struct" {
		t.Fatal("Save changed the in-memory secret")
	}
	var order model.SslOrder
	if err := db.First(&order).Error; err != nil || string(order.ResultPrivateKey) != private {
		t.Fatalf("byte field roundtrip: %v", err)
	}
	if err := db.Model(model.SslOrder{}).Where("id = ?", base.Id).Update("ResultPrivateKey", []byte(private+"-updated")).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&order).Error; err != nil || string(order.ResultPrivateKey) != private+"-updated" {
		t.Fatalf("byte map update: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte(private)) {
		t.Fatalf("secret persisted as plaintext: %v", err)
	}
}

func TestProtectedFieldsRejectTamperingAndPlaintext(t *testing.T) {
	db, _ := secretDB(t)
	config := model.SslConfig{BaseOrm: baseorm.BaseOrm{Id: "one"}, KeyContent: "secret"}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	var encoded string
	if err := db.Table("ssl_configs").Select("key_content").Row().Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, wafsec.BoundPrefix) {
		t.Fatal("not encrypted")
	}
	for _, invalid := range []string{"injected plaintext", encoded[:len(encoded)-5], wafsec.DataKeyPrefixV1 + "legacy"} {
		if err := db.Exec("UPDATE ssl_configs SET key_content = ?", invalid).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&config).Error; err == nil || config.KeyContent != "" {
			t.Fatal("invalid secret accepted or old plaintext retained")
		}
	}
	if err := db.Create(&model.PrivateInfo{BaseOrm: baseorm.BaseOrm{Id: "one"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE private_infos SET private_value = ?", encoded).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&model.PrivateInfo{}).Error; err == nil {
		t.Fatal("cross-column ciphertext substitution accepted")
	}
}
