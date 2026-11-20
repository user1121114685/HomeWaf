package queue

import (
	"container/list"
	"sync"
)

// Metadata supplies a port/site fairness key and retained-size estimate.
// Priorities are 0 (normal), 1 (important), and 2 (security/critical).
type Metadata interface {
	QueuePriority() int
	QueueKey() string
	QueueSize() int64
}
type item struct {
	value         any
	key           string
	priority      int
	bytes         int64
	global, local *list.Element
}
type keyQueue struct {
	lanes [3]*list.List
	count int
	bytes int64
}
type Queue struct {
	mutex                   sync.Mutex
	lanes                   [3]*list.List
	keys                    map[string]*keyQueue
	count, capacity, perKey int
	bytes, maxBytes         int64
	dropped                 [3]uint64
}

func NewQueue() *Queue { return NewBoundedQueue(8192, 2048, 64<<20) }
func NewBoundedQueue(capacity, perKey int, maxBytes int64) *Queue {
	if capacity < 2 {
		capacity = 8192
	}
	if perKey < 1 {
		perKey = capacity / 4
	}
	if maxBytes < 1024 {
		maxBytes = 64 << 20
	}
	q := &Queue{capacity: capacity, perKey: min(perKey, capacity), maxBytes: maxBytes, keys: map[string]*keyQueue{}}
	for i := range q.lanes {
		q.lanes[i] = list.New()
	}
	return q
}
func (q *Queue) remove(it *item, dropped bool) {
	k := q.keys[it.key]
	q.lanes[it.priority].Remove(it.global)
	k.lanes[it.priority].Remove(it.local)
	q.count--
	q.bytes -= it.bytes
	k.count--
	k.bytes -= it.bytes
	if dropped {
		q.dropped[it.priority]++
	}
	if k.count == 0 {
		delete(q.keys, it.key)
	}
}
func lowest(lanes [3]*list.List, lessThan int) *item {
	for p := 0; p < lessThan; p++ {
		if e := lanes[p].Front(); e != nil {
			return e.Value.(*item)
		}
	}
	return nil
}
func (q *Queue) Enqueue(v any) { q.TryEnqueue(v) }
func (q *Queue) TryEnqueue(v any) bool {
	if safe, ok := v.(interface{ SafeForQueue() any }); ok {
		v = safe.SafeForQueue()
	}
	priority, key, weight := 1, "system", int64(512)
	if meta, ok := v.(Metadata); ok {
		priority, key, weight = meta.QueuePriority(), meta.QueueKey(), meta.QueueSize()
	}
	priority = max(0, min(priority, 2))
	weight = max(weight, 1)
	if len(key) > 128 {
		key = key[:128]
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if weight > q.maxBytes/2 {
		q.dropped[priority]++
		return false
	}
	if k := q.keys[key]; k != nil {
		for k.count >= q.perKey || k.bytes+weight > q.maxBytes/2 {
			victim := lowest(k.lanes, priority)
			if victim == nil {
				q.dropped[priority]++
				return false
			}
			q.remove(victim, true)
		}
	}
	for q.count >= q.capacity || q.bytes+weight > q.maxBytes {
		victim := lowest(q.lanes, priority)
		if victim == nil {
			q.dropped[priority]++
			return false
		}
		q.remove(victim, true)
	}
	k := q.keys[key]
	if k == nil {
		k = &keyQueue{}
		for i := range k.lanes {
			k.lanes[i] = list.New()
		}
		q.keys[key] = k
	}
	it := &item{value: v, key: key, priority: priority, bytes: weight}
	it.global = q.lanes[priority].PushBack(it)
	it.local = k.lanes[priority].PushBack(it)
	q.count++
	q.bytes += weight
	k.count++
	k.bytes += weight
	return true
}
func (q *Queue) Dequeue() (any, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	for p := 2; p >= 0; p-- {
		if e := q.lanes[p].Front(); e != nil {
			it := e.Value.(*item)
			q.remove(it, false)
			return it.value, true
		}
	}
	return nil, false
}
func (q *Queue) Size() int   { q.mutex.Lock(); defer q.mutex.Unlock(); return q.count }
func (q *Queue) Empty() bool { return q.Size() == 0 }
func (q *Queue) Stats() map[string]uint64 {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return map[string]uint64{"queued": uint64(q.count), "capacity": uint64(q.capacity), "bytes": uint64(q.bytes), "max_bytes": uint64(q.maxBytes), "keys": uint64(len(q.keys)), "dropped_low": q.dropped[0], "dropped_medium": q.dropped[1], "dropped_high": q.dropped[2]}
}
