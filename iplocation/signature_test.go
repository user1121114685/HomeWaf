package iplocation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignedDatasetRejectsEmptyTrustTamperAndForeignURL(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	keys, _ := json.Marshal(map[string]string{"fixture": base64.RawURLEncoding.EncodeToString(pub)})
	cfg := UpgradeConfig{Enabled: true, RequireSignature: true, UpdateVersionURL: "https://dataset.example/", TrustedKeysJSON: string(keys)}
	m := &remoteManifest{KeyID: "fixture", Version: "1", Files: map[string]remoteFile{"ip2region_v4": {URL: "https://dataset.example/v4.xdb", SHA256: strings.Repeat("a", 64), Size: 1024}}}
	m.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, datasetSigningMessage(m)))
	if err := verifyDatasetManifest(m, cfg); err != nil {
		t.Fatal(err)
	}
	empty := cfg
	empty.TrustedKeysJSON = "{}"
	if verifyDatasetManifest(m, empty) == nil {
		t.Fatal("empty keys accepted")
	}
	m.Version = "2"
	if verifyDatasetManifest(m, cfg) == nil {
		t.Fatal("tampered manifest accepted")
	}
	if checkSignedDatasetURL("https://other.example/v4.xdb", cfg) == nil {
		t.Fatal("foreign download accepted")
	}
	cfg.Enabled = false
	if checkSignedDatasetURL("https://dataset.example/v4.xdb", cfg) == nil {
		t.Fatal("disabled update accepted")
	}
}
