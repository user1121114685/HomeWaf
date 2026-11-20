package accessgate

import (
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Shared by the existing Access service and the port gate. Missing users still
// incur the same bcrypt cost as accounts created by HomeWaf.
const DummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func VerifyPassword(hash, password string) bool {
	if hash == "" {
		hash = DummyPasswordHash
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// This short, project-authored list is intentionally shared by account setup
// and failed-login classification. No attempted password is retained.
func WeakPassword(password string) bool {
	switch strings.ToLower(password) {
	case "admin", "password", "123456", "12345678", "123456789", "1234567890", "12345678901", "password123", "password1234", "admin123456", "qwerty12345", "iloveyou123", "qwerty", "letmein", "root", "123456789012":
		return true
	}
	return false
}

func ValidatePassword(username, password string) error {
	if utf8.RuneCountInString(password) < 10 || len(password) > 72 || strings.EqualFold(username, password) {
		return errors.New("口令须至少10个字符、不超过72字节，且不能与用户名相同")
	}
	if WeakPassword(password) {
		return errors.New("不能使用常见弱口令")
	}
	return nil
}
