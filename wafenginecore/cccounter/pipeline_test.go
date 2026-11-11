package cccounter

import (
	"fmt"
	"sync"
	"testing"
)

func TestPeekShortWindowAndConcurrentQuota(t *testing.T) {
	c, advance := newTestCounter(t, 8, 8)
	c.Incr("port", "ip", 1)
	if got := c.Peek("port", "ip", 1).Count; got != 1 {
		t.Fatalf("peek=%d", got)
	}
	advance(1)
	if got := c.Peek("port", "ip", 1).Count; got != 0 {
		t.Fatalf("one-second window retained old count=%d", got)
	}
	if got := c.Incr("port", "ip", 1).Count; got != 1 {
		t.Fatalf("one-second window count=%d", got)
	}
	for i := 0; i < 100; i++ {
		c.Peek("port", fmt.Sprint(i), 60)
	}
	if c.ActiveKeys() != 1 {
		t.Fatal("peek allocated state")
	}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); c.Incr(fmt.Sprint(i), fmt.Sprint(i), 60) }(i)
	}
	wg.Wait()
	if c.ActiveKeys() > 8 {
		t.Fatalf("concurrent reservation exceeded limit: %d", c.ActiveKeys())
	}
}
