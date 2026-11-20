// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafhttpcore

import "testing"

func TestWafHttpCoreUrlEncode(t *testing.T) {
	const encoded = "family%252Fphotos"
	if got := WafHttpCoreUrlEncode(encoded, 1); got != "family%2Fphotos" {
		t.Fatalf("one decoding pass returned %q", got)
	}
	if got := WafHttpCoreUrlEncode(encoded, 10); got != "family/photos" {
		t.Fatalf("nested URL decoding returned %q", got)
	}
}
