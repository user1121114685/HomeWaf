package main

import (
	"HomeWaf/internal/licensing"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOfflineThreatSnapshotSigningCommand(t *testing.T) {
	dir := t.TempDir()
	old := os.Args
	defer func() { os.Args = old }()
	key, pub := filepath.Join(dir, "private.pem"), filepath.Join(dir, "public.json")
	os.Args = []string{"homewaf-license", "keygen", "--key", key, "--key-id", "fixture", "--out", pub}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	source, out := filepath.Join(dir, "feed.txt"), filepath.Join(dir, "feed.sig.json")
	body := []byte("198.51.100.10\n")
	if err := os.WriteFile(source, body, 0600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"homewaf-license", "sign-threat-snapshot", "--key", key, "--key-id", "fixture", "--snapshot", source, "--feed-id", "family", "--sequence", "42", "--valid-for", "24h", "--out", out}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	keysJSON, _ := os.ReadFile(pub)
	keys, err := licensing.ParseKeyring(string(keysJSON))
	if err != nil {
		t.Fatal(err)
	}
	m, err := licensing.VerifyThreatSnapshot(doc, "family", keys, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if m.Sequence != 42 || m.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("signing command did not authenticate exact file bytes")
	}
	if err := run(); err == nil {
		t.Fatal("signing command overwrote an existing manifest")
	}
}
