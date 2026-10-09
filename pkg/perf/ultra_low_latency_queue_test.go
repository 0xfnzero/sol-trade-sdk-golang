package perf

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLockFreeQueueFIFOAndNil(t *testing.T) {
	q := NewLockFreeQueue()
	if _, ok := q.Dequeue(); ok {
		t.Fatal("new queue is not empty")
	}
	q.Enqueue(nil)
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}
	if value, ok := q.Dequeue(); !ok || value != nil {
		t.Fatal("nil payload was lost")
	}
	for i := 0; i < 100; i++ {
		if value, ok := q.Dequeue(); !ok || value != i {
			t.Fatalf("FIFO value=%v want=%d", value, i)
		}
	}
	if _, ok := q.Dequeue(); ok || q.Len() != 0 {
		t.Fatal("drained queue is not empty")
	}
}

func TestLockFreeQueueFourProducersFourConsumersExactlyOnce(t *testing.T) {
	const producers, consumers, perProducer = 4, 4, 10000
	const total = producers * perProducer
	q := NewLockFreeQueue()
	seen := make([]atomic.Int32, total)
	var consumed atomic.Int32
	var invalid atomic.Bool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for n := 0; n < perProducer; n++ {
				if ctx.Err() != nil {
					return
				}
				q.Enqueue(p*perProducer + n)
			}
		}(p)
	}
	for c := 0; c < consumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for consumed.Load() < total && ctx.Err() == nil {
				value, ok := q.Dequeue()
				if !ok {
					runtime.Gosched()
					continue
				}
				id, valid := value.(int)
				if !valid || id < 0 || id >= total {
					invalid.Store(true)
					cancel()
					return
				}
				seen[id].Add(1)
				consumed.Add(1)
			}
		}()
	}
	wg.Wait()
	if invalid.Load() || consumed.Load() != total {
		t.Fatalf("invalid=%v consumed=%d want=%d", invalid.Load(), consumed.Load(), total)
	}
	for id := range seen {
		if seen[id].Load() != 1 {
			t.Fatalf("id=%d deliveries=%d", id, seen[id].Load())
		}
	}
	if _, ok := q.Dequeue(); ok || q.Len() != 0 {
		t.Fatal("queue was not completely drained")
	}
	runtime.GC() // Typed links remain valid across GC, without manual reclamation.
	q.Enqueue("after-gc")
	if value, ok := q.Dequeue(); !ok || value != "after-gc" {
		t.Fatal("queue failed after GC")
	}
}

func BenchmarkLockFreeQueueRoundTrip(b *testing.B) {
	q := NewLockFreeQueue()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		value, ok := q.Dequeue()
		if !ok || value != i {
			b.Fatal("round trip failed")
		}
	}
}

func BenchmarkLockFreeQueueFourProducersFourConsumers(b *testing.B) {
	q := NewLockFreeQueue()
	var consumed atomic.Int64
	var producers, consumers sync.WaitGroup
	done := make(chan struct{})
	b.ReportAllocs()
	b.ResetTimer()
	for p := 0; p < 4; p++ {
		producers.Add(1)
		go func(p int) {
			defer producers.Done()
			for i := p; i < b.N; i += 4 {
				q.Enqueue(i)
			}
		}(p)
	}
	for c := 0; c < 4; c++ {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				if _, ok := q.Dequeue(); ok {
					consumed.Add(1)
					continue
				}
				select {
				case <-done:
					return
				default:
					runtime.Gosched()
				}
			}
		}()
	}
	producers.Wait()
	close(done)
	consumers.Wait()
	b.StopTimer()
	if consumed.Load() != int64(b.N) || q.Len() != 0 {
		b.Fatalf("consumed=%d want=%d len=%d", consumed.Load(), b.N, q.Len())
	}
}
