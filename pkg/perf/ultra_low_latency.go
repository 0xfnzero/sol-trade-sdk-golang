package perf

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// UltraLowLatencyConfig configuration for ULL optimizations
type UltraLowLatencyConfig struct {
	EnableMemoryPool bool
	PoolSize         int
	EnableLockFree   bool
	CacheLineSize    int
	EnableCPUPinning bool
	EnablePrefetch   bool
	PrefetchDistance int
}

// DefaultUltraLowLatencyConfig returns default ULL config
func DefaultUltraLowLatencyConfig() *UltraLowLatencyConfig {
	return &UltraLowLatencyConfig{
		EnableMemoryPool: true,
		PoolSize:         10000,
		EnableLockFree:   true,
		CacheLineSize:    64,
		EnableCPUPinning: true,
		EnablePrefetch:   true,
		PrefetchDistance: 4,
	}
}

// MemoryPool provides a lock-free object pool
type MemoryPool struct {
	pool    chan interface{}
	factory func() interface{}
	reset   func(interface{})
}

// NewMemoryPool creates a memory pool
func NewMemoryPool(size int, factory func() interface{}, reset func(interface{})) *MemoryPool {
	return &MemoryPool{
		pool:    make(chan interface{}, size),
		factory: factory,
		reset:   reset,
	}
}

// Get gets an object from pool
func (p *MemoryPool) Get() interface{} {
	select {
	case obj := <-p.pool:
		return obj
	default:
		return p.factory()
	}
}

// Put returns an object to pool
func (p *MemoryPool) Put(obj interface{}) {
	if p.reset != nil {
		p.reset(obj)
	}
	select {
	case p.pool <- obj:
	default:
		// Pool full, discard
	}
}

// LockFreeQueue provides a lock-free MPMC queue
type LockFreeQueue struct {
	head *lfNode
	tail *lfNode
	len  int64
}

type lfNode struct {
	value interface{}
	next  atomic.Pointer[lfNode]
}

// NewLockFreeQueue creates a lock-free queue
func NewLockFreeQueue() *LockFreeQueue {
	dummy := &lfNode{}
	q := &LockFreeQueue{
		head: dummy,
		tail: dummy,
	}
	return q
}

// Enqueue adds item to queue
func (q *LockFreeQueue) Enqueue(value interface{}) {
	node := &lfNode{value: value}
	for {
		tail := q.tail
		next := tail.next.Load()
		if tail == q.tail {
			if next == nil {
				if tail.next.CompareAndSwap(next, node) {
					atomic.CompareAndSwapPointer(
						(*unsafe.Pointer)(unsafe.Pointer(&q.tail)),
						unsafe.Pointer(tail),
						unsafe.Pointer(node),
					)
					atomic.AddInt64(&q.len, 1)
					return
				}
			} else {
				atomic.CompareAndSwapPointer(
					(*unsafe.Pointer)(unsafe.Pointer(&q.tail)),
					unsafe.Pointer(tail),
					unsafe.Pointer(next),
				)
			}
		}
	}
}

// Dequeue removes item from queue
func (q *LockFreeQueue) Dequeue() (interface{}, bool) {
	for {
		head := q.head
		tail := q.tail
		next := head.next.Load()
		if head == q.head {
			if head == tail {
				if next == nil {
					return nil, false
				}
				atomic.CompareAndSwapPointer(
					(*unsafe.Pointer)(unsafe.Pointer(&q.tail)),
					unsafe.Pointer(tail),
					unsafe.Pointer(next),
				)
			} else {
				value := next.value
				if atomic.CompareAndSwapPointer(
					(*unsafe.Pointer)(unsafe.Pointer(&q.head)),
					unsafe.Pointer(head),
					unsafe.Pointer(next),
				) {
					atomic.AddInt64(&q.len, -1)
					return value, true
				}
			}
		}
	}
}

// Len returns queue length
func (q *LockFreeQueue) Len() int64 {
	return atomic.LoadInt64(&q.len)
}

// LatencyMetrics tracks latency metrics
type LatencyMetrics struct {
	TotalOps    int64
	TotalTimeNs int64
	MinTimeNs   int64
	MaxTimeNs   int64
}

// Record records a latency measurement
func (m *LatencyMetrics) Record(durationNs int64) {
	atomic.AddInt64(&m.TotalOps, 1)
	atomic.AddInt64(&m.TotalTimeNs, durationNs)

	for {
		oldMin := atomic.LoadInt64(&m.MinTimeNs)
		if oldMin == 0 || durationNs < oldMin {
			if atomic.CompareAndSwapInt64(&m.MinTimeNs, oldMin, durationNs) {
				break
			}
		} else {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&m.MaxTimeNs)
		if durationNs > oldMax {
			if atomic.CompareAndSwapInt64(&m.MaxTimeNs, oldMax, durationNs) {
				break
			}
		} else {
			break
		}
	}
}

// Average returns average latency
func (m *LatencyMetrics) Average() int64 {
	opCount := atomic.LoadInt64(&m.TotalOps)
	if opCount == 0 {
		return 0
	}
	return atomic.LoadInt64(&m.TotalTimeNs) / opCount
}

// LatencyOptimizer optimizes for low latency
type LatencyOptimizer struct {
	config  *UltraLowLatencyConfig
	pools   map[string]*MemoryPool
	metrics *LatencyMetrics
	mu      sync.RWMutex
}

// NewLatencyOptimizer creates a latency optimizer
func NewLatencyOptimizer(config *UltraLowLatencyConfig) *LatencyOptimizer {
	if config == nil {
		config = DefaultUltraLowLatencyConfig()
	}

	return &LatencyOptimizer{
		config:  config,
		pools:   make(map[string]*MemoryPool),
		metrics: &LatencyMetrics{},
	}
}

// RegisterPool registers a memory pool
func (o *LatencyOptimizer) RegisterPool(name string, pool *MemoryPool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pools[name] = pool
}

// GetPool gets a registered pool
func (o *LatencyOptimizer) GetPool(name string) *MemoryPool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.pools[name]
}

// OptimizeGC triggers GC optimization
func (o *LatencyOptimizer) OptimizeGC() {
	// Force GC to clean up before critical section
	runtime.GC()
}

// LockOSThread locks goroutine to OS thread
func (o *LatencyOptimizer) LockOSThread() {
	if o.config.EnableCPUPinning {
		runtime.LockOSThread()
	}
}

// UnlockOSThread unlocks goroutine from OS thread
func (o *LatencyOptimizer) UnlockOSThread() {
	if o.config.EnableCPUPinning {
		runtime.UnlockOSThread()
	}
}

// Prefetch hints at data to prefetch
func (o *LatencyOptimizer) Prefetch(addr uintptr) {
	if o.config.EnablePrefetch {
		// In real implementation, use CPU prefetch instructions
		// This is a placeholder
		_ = addr
	}
}

// Measure measures operation latency
func (o *LatencyOptimizer) Measure(start time.Time) {
	duration := time.Since(start).Nanoseconds()
	o.metrics.Record(duration)
}

// GetMetrics returns current metrics
func (o *LatencyOptimizer) GetMetrics() *LatencyMetrics {
	return &LatencyMetrics{
		TotalOps:    atomic.LoadInt64(&o.metrics.TotalOps),
		TotalTimeNs: atomic.LoadInt64(&o.metrics.TotalTimeNs),
		MinTimeNs:   atomic.LoadInt64(&o.metrics.MinTimeNs),
		MaxTimeNs:   atomic.LoadInt64(&o.metrics.MaxTimeNs),
	}
}

// PaddedInt64 is an int64 padded to cache line size
type PaddedInt64 struct {
	_   CacheLinePad
	val int64
	_   CacheLinePad
}

// Load loads value atomically
func (p *PaddedInt64) Load() int64 {
	return atomic.LoadInt64(&p.val)
}

// Store stores value atomically
func (p *PaddedInt64) Store(v int64) {
	atomic.StoreInt64(&p.val, v)
}

// Add adds to value atomically
func (p *PaddedInt64) Add(delta int64) int64 {
	return atomic.AddInt64(&p.val, delta)
}
