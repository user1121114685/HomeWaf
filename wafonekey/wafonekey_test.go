package wafonekey

import (
	"HomeWaf/global"
	"HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestOneKeyModifyAndRestore(t *testing.T) {
	dir := useNginxRoot(t)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "log.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.OneKeyMod{}); err != nil {
		t.Fatal(err)
	}
	old := global.GWAF_LOCAL_LOG_DB
	global.GWAF_LOCAL_LOG_DB = db
	defer func() { global.GWAF_LOCAL_LOG_DB = old }()

	before := "server {\n  listen 80;\n  listen 8080;\n  server_name family.example;\n}\n"
	after := "server {\n  listen 81;\n  listen 8080;\n  server_name family.example;\n}\n"
	path := filepath.Join(dir, "site.conf")
	if err := os.WriteFile(path, []byte(before), 0640); err != nil {
		t.Fatal(err)
	}
	if rows, err := ScanNginxDir(""); err != nil || len(rows) == 0 {
		t.Fatalf("scan: %v, %v", rows, err)
	}
	if err, _ := OneKeyModifyBt(""); err != nil {
		t.Fatal(err)
	}
	var row model.OneKeyMod
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != after || row.AfterContent != after || row.BeforeContent != before {
		t.Fatalf("change or backup mismatch: %v", err)
	}
	if err := RestoreOneKeyMod(row.Id); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != before {
		t.Fatalf("restore mismatch: %v", err)
	}
	if err := RestoreOneKeyMod(row.Id); err == nil {
		t.Fatal("accepted already restored record")
	}
}
