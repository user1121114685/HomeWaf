package waf_service

import (
	"HomeWaf/utils"
	"testing"
)

func TestConsolePasswordRequiresBcrypt(t *testing.T) {
	const password = "example-password"
	hash, err := utils.BcryptHash(password)
	if err != nil {
		t.Fatal(err)
	}
	if !WafAccountServiceApp.VerifyPassword(hash, password) || WafAccountServiceApp.VerifyPassword(hash, "different") {
		t.Fatal("unexpected bcrypt verification result")
	}
	if WafAccountServiceApp.VerifyPassword("ae3066324b61dc241bf38044e6b6cb47", password) {
		t.Fatal("legacy credential authenticated")
	}
}
