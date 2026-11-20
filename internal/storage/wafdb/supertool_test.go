package wafdb

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/sqlitedialect"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestResetAccountReplacesHashAndRevokesSessions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reset.db")))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.AutoMigrate(&model.Account{}, &model.TokenInfo{}); err != nil {
		t.Fatal(err)
	}
	account := model.Account{BaseOrm: baseorm.BaseOrm{Id: "account"}, LoginAccount: "admin", LoginPassword: "unusable"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TokenInfo{BaseOrm: baseorm.BaseOrm{Id: "session"}, LoginAccount: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.BcryptHash("recovered-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := resetAccountCredential(db, "admin", hash); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&account).Error; err != nil || account.NeedChangePassword != 1 || !utils.BcryptVerify(account.LoginPassword, "recovered-password") {
		t.Fatal("recovery credential was not persisted", err)
	}
	var sessions int64
	if err := db.Model(&model.TokenInfo{}).Count(&sessions).Error; err != nil || sessions != 0 {
		t.Fatal("old sessions still active", err)
	}
}
