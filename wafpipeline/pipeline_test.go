package wafpipeline

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type testSink struct{ hits []HitEvent }

func (s *testSink) Submit(e HitEvent) bool { s.hits = append(s.hits, e); return true }
func TestPipelineOrderingDryRunAndFailure(t *testing.T) {
	sink := &testSink{}
	calls := 0
	p, err := New([]Step{{Number: 1, Name: "access", Inspect: func(c *RequestContext) (Verdict, error) { return Verdict{Action: Continue}, nil }, FailClosed: true}, {Number: 2, Name: "ips", Inspect: func(c *RequestContext) (Verdict, error) { return Verdict{Action: Bypass}, nil }, FailClosed: true}, {Number: 3, Name: "detector", Inspect: func(c *RequestContext) (Verdict, error) { calls++; return Verdict{Action: Deny}, nil }}}, sink)
	if err != nil {
		t.Fatal(err)
	}
	c := NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", netip.MustParseAddr("192.0.2.1"), DefaultViewLimits())
	result := p.Evaluate(c)
	if result.Verdict.Action != Allow || calls != 0 || len(result.Trace) != 3 {
		t.Fatalf("bad trusted flow: %+v calls=%d", result, calls)
	}
	p, _ = New([]Step{{Number: 3, Name: "bad", Inspect: func(c *RequestContext) (Verdict, error) { panic("must not expose panic detail") }}, {Number: 3, Name: "observe", Inspect: func(c *RequestContext) (Verdict, error) {
		c.Hit("sqli", "r1", High, true, "signature matched")
		return Verdict{Action: Deny, Observe: true}, nil
	}}}, sink)
	c = NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", netip.MustParseAddr("192.0.2.1"), DefaultViewLimits())
	c.DryRun = true
	result = p.Evaluate(c)
	if result.Verdict.Action != Allow || len(sink.hits) != 0 || result.Trace[0].Error != "detector panic" {
		t.Fatalf("bad failure/dry-run semantics: %+v", result)
	}
}

type closeBody struct {
	io.Reader
	closed bool
}

func (b *closeBody) Close() error { b.closed = true; return nil }
func TestViewPreservesFullWireBody(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		raw := []byte(strings.Repeat("正常文件内容", 10000))
		wire := raw
		encoding := ""
		if compressed {
			var buf bytes.Buffer
			z := gzip.NewWriter(&buf)
			_, _ = z.Write(raw)
			_ = z.Close()
			wire = buf.Bytes()
			encoding = "gzip"
		}
		original := &closeBody{Reader: bytes.NewReader(wire)}
		r := httptest.NewRequest("POST", "http://test/upload", nil)
		r.Body = original
		r.ContentLength = -1
		r.Header.Set("Content-Encoding", encoding)
		view := NewNormalizedView(r, ViewLimits{BodyBytes: 1024, ExpansionRatio: 2})
		if err := view.ReadBody(); err != nil {
			t.Fatal(err)
		}
		if err := view.ReadBody(); err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(got, wire) {
			t.Fatal("inspection changed upstream bytes")
		}
		_ = r.Body.Close()
		if !original.closed || !view.BodyTruncated {
			t.Fatal("close/truncation contract failed")
		}
	}
}
func TestViewNormalizationAndJSONKeys(t *testing.T) {
	r := httptest.NewRequest("POST", "http://test/a/%252e%252e%3b/x", strings.NewReader(`{"nested":{"key":"%253Ctag%253E","count":123}}`))
	r.Header.Set("Content-Type", "application/json")
	v := NewNormalizedView(r, DefaultViewLimits())
	if err := v.ReadBody(); err != nil {
		t.Fatal(err)
	}
	if v.Path != "/x" {
		t.Fatal(v.Path)
	}
	seen := map[string]bool{}
	for _, f := range v.Fields {
		seen[f.Source+":"+f.Value] = true
	}
	if !seen["json_key:key"] || !seen["json:<tag>"] || !seen["json:123"] {
		t.Fatalf("missing normalized fields: %+v", v.Fields)
	}
}
func TestPortRegistryRejectsInvalidPublicationAndHostSpoof(t *testing.T) {
	r := NewRegistry[string](2)
	a, b := "a", "b"
	_ = r.Publish(5000, func() (*string, error) { return &a, nil })
	_ = r.Publish(5001, func() (*string, error) { return &b, nil })
	if err := r.Publish(5000, func() (*string, error) { return nil, errors.New("invalid") }); err == nil {
		t.Fatal("accepted bad config")
	}
	v, _ := r.Get(5000)
	w, _ := r.Get(5001)
	if *v != "a" || *w != "b" {
		t.Fatal("snapshot changed")
	}
	req := httptest.NewRequest("GET", "http://forged:5001/", nil)
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5000}))
	p, err := LocalPort(req)
	if err != nil || p != 5000 {
		t.Fatal("trusted port came from Host")
	}
}
