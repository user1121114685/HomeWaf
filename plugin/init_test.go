package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginConfigDistinguishesMissingFromMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.yml")
	if _, err := loadPluginConfigFromFile(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing configuration: %v", err)
	}
	if err := os.WriteFile(path, []byte("plugins: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPluginConfigFromFile(path); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid configuration: %v", err)
	}
}
