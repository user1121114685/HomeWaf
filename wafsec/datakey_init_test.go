package wafsec

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDataKeyInitializationBoundaries(t *testing.T) {
	scenario := os.Getenv("HOMEWAF_DATAKEY_INIT_TEST")
	if scenario == "" {
		for _, name := range []string{"fresh", "database_without_key", "legacy_key", "custom_missing"} {
			t.Run(name, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestDataKeyInitializationBoundaries$")
				cmd.Env = append(os.Environ(), "HOMEWAF_DATAKEY_INIT_TEST="+name)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("init subprocess: %v\n%s", err, output)
				}
			})
		}
		return
	}
	dir := t.TempDir()
	custom := ""
	switch scenario {
	case "database_without_key":
		if err := os.Mkdir(filepath.Join(dir, "data"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data", "local.db"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	case "legacy_key":
		if _, err := loadOrCreateDataKey(filepath.Join(dir, "data", ".keys", "data_key")); err != nil {
			t.Fatal(err)
		}
	case "custom_missing":
		custom = filepath.Join(dir, "missing-key")
	}
	err := InitDataKey(dir, custom)
	if scenario == "fresh" {
		if err != nil || !DataKeyReady() {
			t.Fatalf("new installation: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".homewaf-keys", "data_key")); err != nil {
			t.Fatal(err)
		}
	} else {
		if err == nil || DataKeyReady() {
			t.Fatal("unsafe key replacement accepted")
		}
		if _, err := os.Stat(filepath.Join(dir, ".homewaf-keys", "data_key")); !os.IsNotExist(err) {
			t.Fatal("replacement key created")
		}
	}
}
