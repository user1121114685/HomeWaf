package wafowasp

import (
	"HomeWaf/innerbean"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/corazawaf/coraza/v3"
)

type closingBody struct {
	io.Reader
	closed bool
}

func (b *closingBody) Close() error { b.closed = true; return nil }
func TestOWASPInspectionPreservesOversizeBodyAndClose(t *testing.T) {
	waf, err := coraza.NewWAF(coraza.NewWAFConfig().WithDirectives("SecRuleEngine On\nSecRequestBodyAccess On\nSecRequestBodyLimit 20000000\nSecRequestBodyLimitAction ProcessPartial"))
	if err != nil {
		t.Fatal(err)
	}
	old := GetBodyInspectLimit()
	defer SetBodyInspectLimit(int(old))
	SetBodyInspectLimit(0)
	tx := waf.NewTransaction()
	defer tx.Close()
	r := httptest.NewRequest("POST", "http://example.test/upload", nil)
	size := maxRequestBodyBytes + 100000
	original := &closingBody{Reader: strings.NewReader(strings.Repeat("x", size))}
	r.Body = original
	w := &WafOWASP{WAF: waf}
	if err = w.processRequestBody(tx, r, &innerbean.WebLog{}); err != nil {
		t.Fatal(err)
	}
	count, err := io.Copy(io.Discard, r.Body)
	if err != nil || count != int64(size) {
		t.Fatalf("stream truncated: %d/%d %v", count, size, err)
	}
	r.Body.Close()
	if !original.closed {
		t.Fatal("original body not closed")
	}
}
