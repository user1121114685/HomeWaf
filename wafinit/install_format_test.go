package wafinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshInstallFormatMarkerAllowsRestart(t *testing.T) {
	base := t.TempDir()
	if err := EnsureInstallFormat(base); err != nil {
		t.Fatal(err)
	}
	if err := CheckInstallFormat(base); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInstallFormat(base); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFormatRejectsExistingDataWithoutChangingIt(t *testing.T) {
	for _, dir := range []string{"data", ".homewaf-keys"} {
		t.Run(dir, func(t *testing.T) {
			base := t.TempDir()
			path := filepath.Join(base, dir, "fixture")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("operator data"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := EnsureInstallFormat(base); err == nil || !strings.Contains(err.Error(), "fresh installation") {
				t.Fatalf("existing data accepted: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "operator data" {
				t.Fatalf("existing data changed: %q %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(base, "data", FormatMarker)); !os.IsNotExist(err) {
				t.Fatal("refusal left an installation marker")
			}
		})
	}
}
