package queue

import "testing"

type entry struct {
	port string
	p    int
	size int64
}

func (e entry) QueuePriority() int { return e.p }
func (e entry) QueueKey() string   { return e.port }
func (e entry) QueueSize() int64   { return e.size }
func TestBoundedPriorityFairnessAndByteBudget(t *testing.T) {
	q := NewBoundedQueue(4, 2, 2048)
	for range 2 {
		if !q.TryEnqueue(entry{"a", 0, 200}) {
			t.Fatal("unexpected reject")
		}
	}
	if q.TryEnqueue(entry{"a", 0, 200}) {
		t.Fatal("port quota missing")
	}
	for range 2 {
		if !q.TryEnqueue(entry{"b", 0, 200}) {
			t.Fatal("other port starved")
		}
	}
	if !q.TryEnqueue(entry{"a", 2, 200}) {
		t.Fatal("critical event did not replace low priority")
	}
	v, _ := q.Dequeue()
	if v.(entry).p != 2 {
		t.Fatal("critical event not drained first")
	}
	for i := 0; i < 1000000; i++ {
		q.TryEnqueue(entry{"flood", 0, 200})
	}
	st := q.Stats()
	if st["queued"] > 4 || st["bytes"] > 2048 || st["dropped_low"] == 0 || st["keys"] > 4 {
		t.Fatalf("unbounded queue: %v", st)
	}
	if q.TryEnqueue(entry{"huge", 2, 2049}) {
		t.Fatal("byte limit missing")
	}
	for !q.Empty() {
		q.Dequeue()
	}
	if q.Stats()["keys"] != 0 || q.Stats()["bytes"] != 0 {
		t.Fatal("queue retained empty keys")
	}
}
