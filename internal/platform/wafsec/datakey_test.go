package wafsec

import (
	"path/filepath"
	"testing"
)

// setupDataKey 在临时目录初始化一把全新的 DEK，供各用例使用。
// 因 InitDataKey 用 sync.Once，测试里直接给 dataKey 赋值绕过 Once。
func setupDataKey(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	key, err := loadOrCreateDataKey(filepath.Join(dir, ".keys", "data_key"))
	if err != nil {
		t.Fatalf("loadOrCreateDataKey: %v", err)
	}
	dataKeyMu.Lock()
	dataKey = key
	dataKeyMu.Unlock()
}

func TestDataEncryptRoundTrip(t *testing.T) {
	setupDataKey(t)
	plain := "s3cr3t-TOTP-KEY-你好"
	enc, err := DataEncrypt(plain)
	if err != nil {
		t.Fatalf("DataEncrypt: %v", err)
	}
	if !IsDataKeyCiphertext(enc) {
		t.Fatalf("密文应带 swk1 前缀: %q", enc)
	}
	got, err := DataDecrypt(enc)
	if err != nil {
		t.Fatalf("DataDecrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("往返不一致: got %q want %q", got, plain)
	}
}

func TestDataEncryptEmpty(t *testing.T) {
	setupDataKey(t)
	if enc, _ := DataEncrypt(""); enc != "" {
		t.Fatalf("空串应原样返回空: %q", enc)
	}
	if dec, _ := DataDecrypt(""); dec != "" {
		t.Fatalf("空串应原样返回空: %q", dec)
	}
}

func TestDataEncryptNonceUnique(t *testing.T) {
	setupDataKey(t)
	a, _ := DataEncrypt("same-plaintext")
	b, _ := DataEncrypt("same-plaintext")
	if a == b {
		t.Fatalf("相同明文两次加密应因随机 nonce 得到不同密文")
	}
}

// Runtime must reject ciphertext that does not use the authenticated instance format.
func TestDataDecryptRejectsUnknownFormat(t *testing.T) {
	if _, err := DataDecrypt("unknown-format"); err == nil {
		t.Fatal("unrecognized ciphertext accepted")
	}
}

func TestDataDecryptTamperRejected(t *testing.T) {
	setupDataKey(t)
	enc, _ := DataEncrypt("integrity-protected")
	tampered := enc[:len(enc)-2] + func() string {
		last := enc[len(enc)-2:]
		if last == "AA" {
			return "BB"
		}
		return "AA"
	}()
	if _, err := DataDecrypt(tampered); err == nil {
		t.Fatalf("被篡改的 GCM 密文应解密失败")
	}
}
