// Package transporttest creates negotiated sessions for management handler tests.
package transporttest

import (
	"HomeWaf/internal/platform/wafsec"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func Session(t testing.TB) string {
	t.Helper()
	if err := wafsec.InitCommKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := wafsec.RegisterCommSession(base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
