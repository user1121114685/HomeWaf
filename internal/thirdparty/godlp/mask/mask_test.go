package mask

import (
	"HomeWaf/internal/platform/wafsec"
	"HomeWaf/internal/thirdparty/godlp/conf"
	"encoding/hex"
	"testing"
)

func TestPrivacyMaskReturnsKeyedDigest(t *testing.T) {
	mask, err := NewMaskWorker(conf.MaskRuleItem{MaskType: MASKTYPE_ALGO, Value: "HMAC-SHA256"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !wafsec.DataKeyReady() {
		if got, err := mask.Mask("abc"); err != nil || got != "[REDACTED]" {
			t.Fatalf("uninitialized instance disclosed data: %q %v", got, err)
		}
	}
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	got, err := mask.Mask("abc")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(got)
	if err != nil || len(raw) != 32 {
		t.Fatal("expected a SHA-256 sized digest")
	}
	same, err := mask.Mask("abc")
	if err != nil || same != got {
		t.Fatal("inconsistent pseudonym")
	}
	different, err := mask.Mask("def")
	if err != nil || different == got {
		t.Fatal("distinct inputs share a pseudonym")
	}
}
