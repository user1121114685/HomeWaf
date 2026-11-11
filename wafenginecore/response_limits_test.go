package wafenginecore

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type responseLimitBody struct {
	io.Reader
	closed bool
}

func (b *responseLimitBody) Close() error { b.closed = true; return nil }

func TestResponseTransformLimitsPreserveOriginalStreamAndClose(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = io.WriteString(gz, strings.Repeat("x", 8<<20))
	_ = gz.Close()
	for _, tc := range []struct {
		name, encoding string
		wire           []byte
	}{
		{"large", "", bytes.Repeat([]byte("a"), 2<<20)}, {"gzip-bomb", "gzip", compressed.Bytes()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &responseLimitBody{Reader: bytes.NewReader(tc.wire)}
			response := &http.Response{Body: original, Header: http.Header{"Content-Encoding": {tc.encoding}}}
			_, _, err := (&WafEngine{}).getOrgContent(response, false, "auto")
			if !errors.Is(err, errResponseTransformLimit) {
				t.Fatalf("missing resource limit: %v", err)
			}
			got, err := io.ReadAll(response.Body)
			if err != nil || !bytes.Equal(got, tc.wire) {
				t.Fatal("original stream damaged")
			}
			_ = response.Body.Close()
			if !original.closed {
				t.Fatal("original close lost")
			}
		})
	}
}
