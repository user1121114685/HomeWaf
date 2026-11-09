package wafsec

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// CredentialDigest indexes high-entropy bearer credentials without storing
// their reusable plaintext. The instance key also prevents a database-only
// attacker from computing a valid digest for an arbitrary chosen credential.
func CredentialDigest(plain, purpose string) (string, error) {
	key, err := currentDataKey()
	if err != nil {
		return "", err
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("HomeWaf credential v1/" + purpose + "\x00" + plain))
	return "hwa1:" + hex.EncodeToString(mac.Sum(nil)), nil
}
