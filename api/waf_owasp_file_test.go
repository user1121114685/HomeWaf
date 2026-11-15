package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwaspFilePreviewBoundary(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "rules")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "safe.conf"), []byte("SecAction test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.conf"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	content, err := readOwaspRuleFile(root, "safe.conf")
	if err != nil || string(content) != "SecAction test" {
		t.Fatalf("read: %q %v", content, err)
	}
	for _, path := range []string{"../secret.conf", filepath.Join(dir, "secret.conf"), "private.key", "."} {
		if _, err := readOwaspRuleFile(root, path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "secret.conf"), filepath.Join(root, "escape.conf")); err == nil {
		if _, err := readOwaspRuleFile(root, "escape.conf"); err == nil {
			t.Fatal("escaping symlink accepted")
		}
	}
}
