package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"
)

// ThreatSnapshotPayload authenticates raw legacy subscription bytes before
// parsing. FeedID prevents substitution by another feed sharing the same key.
type ThreatSnapshotPayload struct {
	Schema    int    `json:"schema"`
	FeedID    string `json:"feed_id"`
	Sequence  uint64 `json:"sequence"`
	SHA256    string `json:"sha256"`
	ExpiresAt int64  `json:"expires_at"`
}
type ThreatSnapshotManifest struct {
	ThreatSnapshotPayload
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

func threatSnapshotMessage(payload ThreatSnapshotPayload, keyID string) []byte {
	data, _ := json.Marshal(payload)
	return append([]byte("HomeWaf threat snapshot v1\n"+keyID+"\n"), data...)
}
func validateThreatSnapshot(m ThreatSnapshotManifest) error {
	digest, err := hex.DecodeString(m.SHA256)
	if m.Schema != 1 || !identifier.MatchString(m.FeedID) || !identifier.MatchString(m.KeyID) || m.Sequence == 0 || m.Sequence > math.MaxInt64 || err != nil || len(digest) != 32 || m.ExpiresAt <= 0 {
		return errors.New("invalid threat snapshot metadata")
	}
	return nil
}
func SignThreatSnapshot(payload ThreatSnapshotPayload, keyID string, key ed25519.PrivateKey) ([]byte, error) {
	m := ThreatSnapshotManifest{ThreatSnapshotPayload: payload, KeyID: keyID}
	if err := validateThreatSnapshot(m); err != nil {
		return nil, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("Ed25519 private key required")
	}
	m.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, threatSnapshotMessage(payload, keyID)))
	return json.Marshal(m)
}
func VerifyThreatSnapshot(document []byte, feedID string, keys Keyring, now time.Time) (ThreatSnapshotManifest, error) {
	var m ThreatSnapshotManifest
	if len(document) > 32<<10 || strictJSON(document, &m) != nil {
		return m, errors.New("invalid signed threat snapshot document")
	}
	if err := validateThreatSnapshot(m); err != nil {
		return m, err
	}
	if m.FeedID != feedID || m.ExpiresAt <= now.Unix() || m.ExpiresAt > now.Add(31*24*time.Hour).Unix() {
		return m, errors.New("unexpected feed or expired/excessive signature validity")
	}
	key := keys[m.KeyID]
	sig, err := base64.RawURLEncoding.DecodeString(m.Signature)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, threatSnapshotMessage(m.ThreatSnapshotPayload, m.KeyID), sig) {
		return m, errors.New("untrusted threat snapshot signature")
	}
	return m, nil
}
