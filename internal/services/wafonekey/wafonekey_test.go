package wafonekey

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/sqlitedialect"
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

func useNginxRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := global.CurrentSettings().NginxConfigRoot
	global.UpdateSettings(func(s *global.RuntimeSettings) { s.NginxConfigRoot = dir })
	t.Cleanup(func() { global.UpdateSettings(func(s *global.RuntimeSettings) { s.NginxConfigRoot = old }) })
	return dir
}

func TestNginxDirConfinesReadsAndWrites(t *testing.T) {
	dir := useNginxRoot(t)
	outside := t.TempDir()
	lookalike := filepath.Join(outside, "server", "panel", "vhost", "nginx")
	if err := os.MkdirAll(lookalike, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, lookalike} {
		if err, _ := OneKeyModifyBt(path); err == nil {
			t.Fatal("modified outside configured root")
		}
	}
	t.Run("linked directory", func(t *testing.T) {
		link := filepath.Join(dir, "linked")
		if err := os.Symlink(outside, link); err != nil {
			t.Skip(err)
		}
		if err, _ := OneKeyModifyBt(link); err == nil {
			t.Fatal("followed link outside configured root")
		}
	})
}
