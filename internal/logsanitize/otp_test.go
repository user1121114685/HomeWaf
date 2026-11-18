package logsanitize

import (
	"strings"
	"testing"
)

func TestDSMAuthEnvelopesRedacted(t *testing.T) {
	for _, raw := range []string{
		"api=SYNO.API.Auth&__cIpHeRtExT=private-envelope&ik_message=private-state",
		`{"api":"SYNO.API.Auth","__cIpHeRtExT":{"rsa":"private-envelope"},"ik_message":"private-state"}`,
	} {
		if out := Body(raw); strings.Contains(out, "private-") || !strings.Contains(out, "SYNO.API.Auth") {
			t.Fatal("DSM login payload was not selectively redacted")
		}
	}
}

func TestDynamicCodesAreRedactedWithoutMaskingUnrelatedNames(t *testing.T) {
	out := Query("otp=123456&totp_code=234567&totp_secret=JBSWY3DPEHPK3PXP&hotpot=family")
	for _, secret := range []string{"123456", "234567", "JBSWY3DPEHPK3PXP"} {
		if strings.Contains(out, secret) {
			t.Fatal("dynamic credential leaked")
		}
	}
	if !strings.Contains(out, "hotpot=family") {
		t.Fatal("unrelated field was classified as OTP")
	}
}
