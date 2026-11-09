package wafsec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCryptoUtil_CreateKeys(t *testing.T) {
	//生成密钥对
	crsa := CryptoUtil{}

	//rsa 密钥文件产生
	prvKey, pubKey := crsa.CreateKeys(4096)
	dir := t.TempDir()

	// 保存私钥和公钥到文件
	err := os.WriteFile(filepath.Join(dir, "private_key.pem"), prvKey, 0600)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(dir, "public_key.pem"), pubKey, 0600)
	if err != nil {
		panic(err)
	}

	if _, err := crsa.File2Bytes(filepath.Join(dir, "public_key.pem")); err != nil {
		t.Fatal(err)
	}
	if _, err := crsa.File2Bytes(filepath.Join(dir, "private_key.pem")); err != nil {
		t.Fatal(err)
	}
}
