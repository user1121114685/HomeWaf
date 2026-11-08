// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"
)

func TestSignedLicenseBoundaries(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	c := Claims{Version: 2, Product: Product, LicenseID: "license-1", Subject: "Test license holder", InstanceID: "instance-1", Edition: "commercial", Features: []string{"commercial.use"}, IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(time.Hour)}
	doc, err := Sign(c, "key-1", priv)
	if err != nil {
		t.Fatal(err)
	}
	keys := Keyring{"key-1": pub}
	valid, err := Verify(doc, keys, "instance-1", now)
	if err != nil || !valid.Allows("commercial.use", now) || valid.Allows("ungranted", now) {
		t.Fatalf("valid license rejected: %v", err)
	}
	var envelope Envelope
	if err := json.Unmarshal(doc, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Payload = envelope.Payload[:len(envelope.Payload)-2] + "AA"
	tampered, _ := json.Marshal(envelope)
	for _, tc := range []struct {
		name     string
		doc      []byte
		keys     Keyring
		instance string
		now      time.Time
	}{
		{"empty", nil, keys, "instance-1", now},
		{"unsigned", []byte(`{"version":2}`), keys, "instance-1", now},
		{"tampered", tampered, keys, "instance-1", now},
		{"unknown key", doc, Keyring{}, "instance-1", now},
		{"wrong instance", doc, keys, "instance-2", now},
		{"not yet valid", doc, keys, "instance-1", now.Add(-time.Second)},
		{"expired boundary", doc, keys, "instance-1", c.ExpiresAt},
		{"trailing document", append(append([]byte{}, doc...), []byte(` {}`)...), keys, "instance-1", now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(tc.doc, tc.keys, tc.instance, tc.now); err == nil {
				t.Fatal("invalid license accepted")
			}
		})
	}
	if valid.Allows("commercial.use", c.ExpiresAt) {
		t.Fatal("cached entitlement survived expiry")
	}
}
