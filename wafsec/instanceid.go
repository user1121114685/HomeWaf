package wafsec

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// InstallationID is a public, stable identifier derived from the private
// installation key. Copying only a database/config/license cannot reproduce it.
// It is not a hardware binding: copying the private key clones the installation.
func InstallationID() (string, error) {
	key, err := currentDataKey()
	if err != nil {
		return "", err
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("HomeWaf public installation identity v1"))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
