package wafenginecore

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyTamperTasksBoundAllocationRecoverAndDrain(t *testing.T) {
	e := newLegacyTamperExecutor(2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.run(ctx); close(done) }()
	<-e.started
	entered, release := make(chan struct{}), make(chan struct{})
	var built, completed atomic.Int64
	if !e.submit(func() func() { built.Add(1); return func() { close(entered); <-release; completed.Add(1) } }) {
		t.Fatal("first task rejected")
	}
	<-entered
	if !e.submit(func() func() { built.Add(1); return func() { panic("fixture") } }) || !e.submit(func() func() { built.Add(1); return func() { completed.Add(1) } }) {
		t.Fatal("bounded tasks unexpectedly rejected")
	}
	if e.submit(func() func() { built.Add(1); return func() {} }) || built.Load() != 3 {
		t.Fatal("full queue still allocated a task")
	}
	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker failed to drain")
	}
	if completed.Load() != 2 || e.failed != 1 || len(e.queue) != 0 || e.active {
		t.Fatal("panic or shutdown lost worker invariants")
	}
	if e.submit(func() func() { t.Error("stopped worker allocated task"); return func() {} }) {
		t.Fatal("stopped worker accepted work")
	}
}

type tamperWireBody struct {
	*bytes.Reader
	closed bool
}

func (b *tamperWireBody) Close() error { b.closed = true; return nil }
func TestLegacyTamperDecodingPreservesOverLimitCompressedStream(t *testing.T) {
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	_, _ = io.WriteString(gz, strings.Repeat("x", (1<<20)+1))
	_ = gz.Close()
	original := append([]byte{}, encoded.Bytes()...)
	body := &tamperWireBody{Reader: bytes.NewReader(original)}
	resp := &http.Response{Header: http.Header{"Content-Encoding": []string{"gzip"}}, Body: body, ContentLength: int64(len(original))}
	if _, err := readDecompressedBody(resp); err == nil {
		t.Fatal("over-limit decompression accepted")
	}
	replayed, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(replayed, original) {
		t.Fatal("bounded inspection altered compressed original", err)
	}
	_ = resp.Body.Close()
	if !body.closed {
		t.Fatal("original body was not closed")
	}
}
