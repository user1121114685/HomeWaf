package wafpipeline

import (
	"encoding/base32"
	"errors"
	"strings"
)

func NormalizeTOTPSecret(raw string) (string, error) {
	value := strings.TrimRight(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", "")), "=")
	if len(value) < 16 || len(value) > 128 {
		return "", errors.New("动态口令密钥长度无效")
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(value)
	if err != nil || len(decoded) < 10 || len(decoded) > 64 {
		return "", errors.New("动态口令密钥必须是有效Base32")
	}
	return value, nil
}
