// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf license-envelope implementation. See LICENSE for licensing scope.
package licensing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
)

const Product = "homewaf"
const MaxDocumentSize = 64 * 1024

// TrustedKeysJSON is supplied at release build time using -ldflags -X.
// An unconfigured build trusts no commercial licenses. Never embed a private key.
var TrustedKeysJSON = "{}"

type Claims struct {
	Version    int       `json:"version"`
	Product    string    `json:"product"`
	LicenseID  string    `json:"license_id"`
	Subject    string    `json:"subject"`
	InstanceID string    `json:"instance_id"`
	Edition    string    `json:"edition"`
	Features   []string  `json:"features"`
	IssuedAt   time.Time `json:"issued_at"`
	NotBefore  time.Time `json:"not_before"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type Envelope struct {
	Version   int    `json:"version"`
	KeyID     string `json:"key_id"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type Keyring map[string]ed25519.PublicKey

var identifier = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

func strictJSON(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func ParseKeyring(text string) (Keyring, error) {
	var encoded map[string]string
	if err := strictJSON([]byte(text), &encoded); err != nil {
		return nil, err
	}
	keys := make(Keyring, len(encoded))
	for id, value := range encoded {
		key, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(key) != ed25519.PublicKeySize || !identifier.MatchString(id) {
			return nil, fmt.Errorf("invalid trusted key %q", id)
		}
		keys[id] = ed25519.PublicKey(key)
	}
	return keys, nil
}

func message(id string, payload []byte) []byte {
	return append([]byte("HomeWaf license v2\n"+id+"\n"), payload...)
}

func validate(c Claims) error {
	if c.Version != 2 || c.Product != Product {
		return errors.New("unsupported license version or product")
	}
	if !identifier.MatchString(c.LicenseID) || !identifier.MatchString(c.InstanceID) || c.Subject == "" || len(c.Subject) > 512 {
		return errors.New("missing or invalid license identity")
	}
	if c.Edition != "commercial" {
		return errors.New("unsupported license edition")
	}
	if c.IssuedAt.IsZero() || c.NotBefore.IsZero() || !c.ExpiresAt.After(c.NotBefore) || c.IssuedAt.After(c.ExpiresAt) {
		return errors.New("invalid license validity interval")
	}
	if len(c.Features) > 256 {
		return errors.New("too many license features")
	}
	seen := make(map[string]bool)
	for _, feature := range c.Features {
		if !identifier.MatchString(feature) || seen[feature] {
			return errors.New("invalid or duplicate license feature")
		}
		seen[feature] = true
	}
	return nil
}

func Sign(c Claims, keyID string, private ed25519.PrivateKey) ([]byte, error) {
	if err := validate(c); err != nil {
		return nil, err
	}
	if !identifier.MatchString(keyID) || len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid signing key")
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(private, message(keyID, payload))
	doc, err := json.Marshal(Envelope{2, keyID, base64.RawURLEncoding.EncodeToString(payload), base64.RawURLEncoding.EncodeToString(signature)})
	if len(doc) > MaxDocumentSize {
		return nil, errors.New("license document too large")
	}
	return doc, err
}

func Verify(document []byte, keys Keyring, instanceID string, now time.Time) (Claims, error) {
	var c Claims
	if len(document) == 0 || len(document) > MaxDocumentSize {
		return c, errors.New("invalid license document size")
	}
	var e Envelope
	if err := strictJSON(document, &e); err != nil {
		return c, fmt.Errorf("invalid license envelope: %w", err)
	}
	if e.Version != 2 || !identifier.MatchString(e.KeyID) {
		return c, errors.New("unsupported license envelope")
	}
	key, ok := keys[e.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return c, errors.New("untrusted signing key")
	}
	payload, err := base64.RawURLEncoding.DecodeString(e.Payload)
	if err != nil {
		return c, errors.New("invalid license payload")
	}
	signature, err := base64.RawURLEncoding.DecodeString(e.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(key, message(e.KeyID, payload), signature) {
		return c, errors.New("license signature verification failed")
	}
	if err := strictJSON(payload, &c); err != nil {
		return Claims{}, fmt.Errorf("invalid signed claims: %w", err)
	}
	if err := validate(c); err != nil {
		return Claims{}, err
	}
	if instanceID == "" || c.InstanceID != instanceID {
		return Claims{}, errors.New("license belongs to another instance")
	}
	if now.Before(c.NotBefore) || c.IssuedAt.After(now.Add(5*time.Minute)) {
		return Claims{}, errors.New("license is not active yet")
	}
	if !now.Before(c.ExpiresAt) {
		return Claims{}, errors.New("license has expired")
	}
	return c, nil
}

func (c Claims) Allows(feature string, now time.Time) bool {
	if now.Before(c.NotBefore) || !now.Before(c.ExpiresAt) {
		return false
	}
	for _, granted := range c.Features {
		if granted == feature {
			return true
		}
	}
	return false
}
