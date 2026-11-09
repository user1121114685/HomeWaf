package secretfield_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A subprocess starts without an initialized DEK regardless of test order.
func TestMissingKeyCannotPersistPlaintext(t *testing.T) {
	if os.Getenv("HOMEWAF_SECRET_NO_KEY_TEST") == "1" {
		db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "missing.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := db.DB()
		defer raw.Close()
		if err := db.AutoMigrate(&model.PrivateInfo{}); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.PrivateInfo{BaseOrm: baseorm.BaseOrm{Id: "one"}, PrivateValue: "never-plaintext"}).Error; err == nil {
			t.Fatal("write succeeded without key")
		}
		var n int64
		if err := db.Model(&model.PrivateInfo{}).Count(&n).Error; err != nil || n != 0 {
			t.Fatal("failed write left a row")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMissingKeyCannotPersistPlaintext$")
	cmd.Env = append(os.Environ(), "HOMEWAF_SECRET_NO_KEY_TEST=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("missing-key subprocess: %v\n%s", err, output)
	}
}
