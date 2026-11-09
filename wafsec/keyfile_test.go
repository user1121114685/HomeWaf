package wafsec

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestConcurrentKeyCreationNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "data_key")
	var wg sync.WaitGroup
	keys, errs := make([][]byte, 16), make([]error, 16)
	for i := range keys {
		wg.Add(1)
		go func(i int) { defer wg.Done(); keys[i], errs[i] = loadOrCreateDataKey(path) }(i)
	}
	wg.Wait()
	for i := range keys {
		if errs[i] != nil || len(keys[i]) != 32 || !bytes.Equal(keys[0], keys[i]) {
			t.Fatalf("worker %d did not load the single published key: %v", i, errs[i])
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary key leaked: %v", err)
	}
	if err := os.WriteFile(path, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateDataKey(path); err == nil {
		t.Fatal("corrupt key was replaced")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "corrupted" {
		t.Fatal("original file modified")
	}
}

func TestKeyFileRejectsSymlinkAndBroadUnixMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if _, err := loadOrCreateDataKey(path); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err == nil {
		if _, err := loadOrCreateDataKey(link); err == nil {
			t.Fatal("symlink key accepted")
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadOrCreateDataKey(path); err == nil {
			t.Fatal("world-readable key accepted")
		}
	}
}

func TestBoundSecretsRejectWrongKeyAndPurpose(t *testing.T) {
	setupDataKey(t)
	firstID, err := InstallationID()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncryptBound([]byte("fixture"), "ssl_configs/key_content")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptBound(encoded, "private_infos/private_value"); err == nil {
		t.Fatal("cross-purpose substitution accepted")
	}
	setupDataKey(t)
	secondID, err := InstallationID()
	if err != nil || firstID == secondID {
		t.Fatal("different private installation keys produced the same identity")
	}
	if _, err := DecryptBound(encoded, "ssl_configs/key_content"); err == nil {
		t.Fatal("wrong key accepted")
	}
}
