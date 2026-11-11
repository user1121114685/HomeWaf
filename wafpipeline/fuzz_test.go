package wafpipeline

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func FuzzNormalizedRequestReplay(f *testing.F) {
	f.Add([]byte(`{"a":"family","a":"%252e%252e"}`), "application/json", "/notes", false)
	f.Add([]byte("file=photo%20(1)%23%25%26.jpg"), "application/x-www-form-urlencoded", "/dav/..;/photos", true)
	f.Add([]byte("--test\r\nContent-Disposition: form-data; name=note\r\n\r\nhello\r\n--test--\r\n"), "multipart/form-data; boundary=test", "/upload", false)
	f.Fuzz(func(t *testing.T, raw []byte, contentType, path string, compressed bool) {
		if len(raw) > 65536 || len(contentType) > 512 || len(path) > 4096 {
			return
		}
		payload := raw
		if compressed {
			var encoded bytes.Buffer
			z := gzip.NewWriter(&encoded)
			_, _ = z.Write(raw)
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			payload = encoded.Bytes()
		}
		req, err := http.NewRequest("POST", "http://fixture.invalid/", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.URL.Path = "/" + strings.TrimLeft(path, "/")
		req.Header.Set("Content-Type", contentType)
		if compressed {
			req.Header.Set("Content-Encoding", "gzip")
		}
		v := NewNormalizedView(req, ViewLimits{BodyBytes: 4096, ExpansionRatio: 32, MaxFields: 64, FilePrefixBytes: 256})
		_ = v.ReadBody()
		if len(v.Body) > 4096 || len(v.Fields) > 64 {
			t.Fatal("inspection exceeded configured bounds")
		}
		remaining, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(remaining, payload) {
			t.Fatal("inspection changed the original upstream stream")
		}
		if !strings.HasPrefix(v.Path, "/") || strings.ContainsRune(v.Path, 0) {
			t.Fatal("invalid normalized path")
		}
	})
}

func FuzzTemplateImport(f *testing.F) {
	for _, template := range BuiltinTemplates() {
		data, _ := json.Marshal(template)
		f.Add(data)
	}
	f.Add([]byte(`{"schema_version":1,"id":"unsafe","password_hash":"secret"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		template, err := ImportTemplate(data)
		if err != nil {
			return
		}
		if template.Builtin || len(template.Policy.IPGroup.Entries) != 0 || len(template.Policy.Network.TrustedProxies) != 0 {
			t.Fatal("import escaped trust restrictions")
		}
		if err := ValidateTemplate(template); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ImportTemplate(raw); err != nil {
			t.Fatal("accepted template cannot round-trip", err)
		}
	})
}
