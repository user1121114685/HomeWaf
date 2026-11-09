package wafsec

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

const BoundPrefix = "hwf1:"

// EncryptBound authenticates the field's purpose along with its value. Moving
// ciphertext into a different table/column cannot turn it into a valid secret.
func EncryptBound(plain []byte, purpose string) (string, error) {
	key, err := currentDataKey()
	if err != nil {
		return "", err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plain, []byte("HomeWaf field v1/"+purpose))
	return BoundPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func DecryptBound(encoded, purpose string) ([]byte, error) {
	if !strings.HasPrefix(encoded, BoundPrefix) {
		return nil, errors.New("unencrypted or unsupported secret field")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encoded, BoundPrefix))
	if err != nil {
		return nil, errors.New("invalid secret encoding")
	}
	key, err := currentDataKey()
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("invalid secret length")
	}
	return aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte("HomeWaf field v1/"+purpose))
}
