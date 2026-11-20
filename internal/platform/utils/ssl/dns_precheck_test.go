package ssl

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDNSPropagationCheckDoesNotNestPolling(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("fixture DNS timeout")
	type result struct {
		ok  bool
		err error
	}
	done := make(chan result, 1)
	go func() {
		ok, err := logDNSPropagationCheck("example.com", "_acme-challenge.example.com.", "fixture-txt", func(fqdn, value string) (bool, error) {
			calls.Add(1)
			if fqdn != "_acme-challenge.example.com." || value != "fixture-txt" {
				return false, errors.New("wrong DNS verification input")
			}
			return false, want
		})
		done <- result{ok, err}
	}()
	select {
	case got := <-done:
		if got.ok || !errors.Is(got.err, want) || calls.Load() != 1 {
			t.Fatalf("precheck changed polling/error contract: calls=%d err=%v", calls.Load(), got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("precheck added its own wait loop instead of returning to lego's timeout-controlled poller")
	}
}
