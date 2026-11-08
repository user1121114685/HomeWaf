// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type PluginManifest struct {
	Version   int    `json:"version"`
	KeyID     string `json:"key_id"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

func pluginMessage(keyID, digest string) []byte {
	return []byte("HomeWaf plugin v1\n" + keyID + "\n" + digest + "\n")
}
func SignPlugin(digest, keyID string, key ed25519.PrivateKey) ([]byte, error) {
	b, err := hex.DecodeString(digest)
	if err != nil || len(b) != 32 || !identifier.MatchString(keyID) || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid plugin digest or key")
	}
	digest = hex.EncodeToString(b)
	m := PluginManifest{1, keyID, digest, base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, pluginMessage(keyID, digest)))}
	return json.Marshal(m)
}
func VerifyPlugin(document []byte, digest string, keys Keyring) error {
	if len(document) > MaxDocumentSize {
		return errors.New("plugin manifest too large")
	}
	var m PluginManifest
	if err := strictJSON(document, &m); err != nil {
		return err
	}
	if m.Version != 1 || !identifier.MatchString(m.KeyID) || m.SHA256 != digest {
		return errors.New("plugin digest or version mismatch")
	}
	key := keys[m.KeyID]
	signature, err := base64.RawURLEncoding.DecodeString(m.Signature)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, pluginMessage(m.KeyID, m.SHA256), signature) {
		return errors.New("untrusted plugin signature")
	}
	return nil
}
