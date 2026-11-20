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
	digest, err := instanceDigest("HomeWaf credential v1/"+purpose, plain)
	if err != nil {
		return "", err
	}
	return "hwa1:" + digest, nil
}

// PrivacyDigest yields stable pseudonyms within one HomeWaf installation.
func PrivacyDigest(value string) (string, error) {
	return instanceDigest("HomeWaf response privacy v1", value)
}

func instanceDigest(domain, value string) (string, error) {
	key, err := currentDataKey()
	if err != nil {
		return "", err
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(domain + "\x00"))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
