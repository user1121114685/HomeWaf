// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/model/request"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAppUploadPreservesExistingMode(t *testing.T) {
	setupAppDirEnv(t)
	app, err := WafAppServiceApp.AddApi(request.WafAppAddReq{Code: "app-upload-mode", Name: "upload mode", StartCmd: "./server"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		mode os.FileMode
	}{
		{"server", 0750}, {"config.env", 0600}, {"new.txt", 0644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(app.AppDir, tc.name)
			if tc.name != "new.txt" {
				if err := os.WriteFile(path, []byte("old"), tc.mode); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, tc.mode); err != nil {
					t.Fatal(err)
				}
			} else {
				control := filepath.Join(app.AppDir, "control")
				if err := os.WriteFile(control, nil, 0666); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(control)
				if err != nil {
					t.Fatal(err)
				}
				tc.mode = info.Mode().Perm()
				if runtime.GOOS != "windows" {
					tc.mode |= 0100
				}
			}
			if err := WafAppServiceApp.UploadFile(app.Code, tc.name, strings.NewReader("new"), ""); err != nil {
				t.Fatal(err)
			}
			checkAppFile(t, path, "new", tc.mode)
		})
	}
	path := filepath.Join(app.AppDir, "server")
	if err := WafAppServiceApp.UploadFile(app.Code, "server", strings.NewReader("bad"), strings.Repeat("0", 64)); err == nil {
		t.Fatal("invalid hash must fail")
	}
	checkAppFile(t, path, "new", 0750)
	checkAppFileTempCleaned(t, app.AppDir)
}

func TestAppUpgradeRollbackPreservesFileMode(t *testing.T) {
	setupAppDirEnv(t)
	app, err := WafAppServiceApp.AddApi(request.WafAppAddReq{Code: "app-upgrade-mode", Name: "upgrade mode", StartCmd: "./server"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(app.AppDir, "server")
	if err := os.WriteFile(path, []byte("version one"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0750); err != nil {
		t.Fatal(err)
	}
	if err := WafAppServiceApp.UpgradeApp(app.Code, "server", strings.NewReader("version two"), ""); err != nil {
		t.Fatal(err)
	}
	checkAppFile(t, path, "version two", 0750)
	backups, err := WafAppServiceApp.ListBackups(app.Code)
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	checkAppFile(t, filepath.Join(app.AppDir, "backup", backups[0].Filename), "version one", 0750)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := WafAppServiceApp.RollbackApp(app.Code, backups[0].Filename); err != nil {
		t.Fatal(err)
	}
	checkAppFile(t, path, "version one", 0750)
	checkAppFileTempCleaned(t, app.AppDir)
}

func TestAppFileOperationsConfinedToConfiguredRoot(t *testing.T) {
	allowed := setupAppDirEnv(t)
	outside := t.TempDir()
	link := filepath.Join(allowed, "external")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	req := request.WafAppAddReq{Code: "files", Name: "files", StartCmd: "application", AppDir: link}
	if _, err := WafAppServiceApp.AddApi(req); err == nil {
		t.Fatal("accepted app directory outside configured root")
	}
	req.AppDir = ""
	app, err := WafAppServiceApp.AddApi(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(app.AppDir, "backup")); err != nil {
		t.Fatal(err)
	}
	if err := WafAppServiceApp.UploadFile(app.Code, "server", strings.NewReader("original"), ""); err != nil {
		t.Fatal(err)
	}
	if err := WafAppServiceApp.UpgradeApp(app.Code, "server", strings.NewReader("new"), ""); err == nil {
		t.Fatal("wrote backup outside app")
	}
	if err := WafAppServiceApp.RollbackApp(app.Code, "server.20261118120000"); err == nil {
		t.Fatal("read outside backup directory")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("external directory changed", err)
	}
	got, err := os.ReadFile(filepath.Join(app.AppDir, "server"))
	if err != nil || string(got) != "original" {
		t.Fatal("failed upgrade changed application", err)
	}
}
