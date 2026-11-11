package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"bytes"
	"io"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestUploadOverLimitPassPreservesEntireStream(t *testing.T) {
	buf, ct := buildMultipart(t, "file", "backup.bin", "application/octet-stream", bytes.Repeat([]byte("b"), 256<<10))
	raw := append([]byte(nil), buf.Bytes()...)
	r := httptest.NewRequest("POST", "/upload", bytes.NewReader(raw))
	r.Header.Set("Content-Type", ct)
	h := &wafenginmodel.HostSafe{Host: model.Hosts{UploadSecurityJSON: `{"is_enable":1,"check_ext":1,"check_content":1,"check_magic":1,"check_size":1,"max_size_kb":1,"over_limit_action":"pass"}`}}
	result := (&WafEngine{}).CheckUpload(r, &innerbean.WebLog{}, url.Values{}, h, h)
	if result.IsBlock {
		t.Fatal("pass policy ignored")
	}
	got, err := io.ReadAll(r.Body)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("pass truncated upload: %d/%d err=%v", len(got), len(raw), err)
	}
}
