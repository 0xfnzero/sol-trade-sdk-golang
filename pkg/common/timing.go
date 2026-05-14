package common

import (
	"sync"
	"sync/atomic"
	"time"
)

// ===== Fast Timing Utilities =====

// nowNs returns current time in nanoseconds using monotonic clock
func nowNs() int64 {
	return time.Now().UnixNano()
}

// nowUs returns current time in microseconds
func nowUs() int64 {
	return time.Now().UnixMicro()
}

// nowMs returns current time in milliseconds
func nowMs() int64 {
	return time.Now().UnixMilli()
}

// NowNs returns current time in nanoseconds (public)
func NowNs() int64 {
	return nowNs()
}

// NowUs returns current time in microseconds (public)
func NowUs() int64 {
	return nowUs()
}

// NowMs returns current time in milliseconds (public)
func NowMs() int64 {
	return nowMs()
}

// ===== Timer =====

// Timer provides high-precision timing for latency measurement
type Timer struct {
	startNs int64
	endNs   int64
	running bool
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{}
}

// Start starts the timer
func (t *Timer) Start() *Timer {
	t.startNs = nowNs()
	t.running = true
	return t
}

// Stop stops the timer and returns elapsed nanoseconds
func (t *Timer) Stop() int64 {
	t.endNs = nowNs()
	t.running = false
	return t.ElapsedNs()
}

// ElapsedNs returns elapsed time in nanoseconds
func (t *Timer) ElapsedNs() int64 {
	if t.running {
		return nowNs() - t.startNs
	}
	return t.endNs - t.startNs
}

// ElapsedUs returns elapsed time in microseconds
func (t *Timer) ElapsedUs() int64 {
	return t.ElapsedNs() / 1000
}

// ElapsedMs returns elapsed time in milliseconds
func (t *Timer) ElapsedMs() int64 {
	return t.ElapsedNs() / 1_000_000
}

// Reset resets the timer
func (t *Timer) Reset() *Timer {
	t.startNs = 0
	t.endNs = 0
	t.running = false
	return t
}

// IsRunning returns true if timer is running
func (t *Timer) IsRunning() bool {
	return t.running
}

// ===== ScopedTimer =====

// ScopedTimer automatically records timing when created and stopped
type ScopedTimer struct {
	startNs  int64
	callback func(int64)
}

// NewScopedTimer creates a timer that calls callback with elapsed time on Stop
func NewScopedTimer(callback func(elapsedNs int64)) *ScopedTimer {
	return &ScopedTimer{
		startNs:  nowNs(),
		callback: callback,
	}
}

// Stop stops the timer and invokes the callback
func (t *ScopedTimer) Stop() int64 {
	elapsed := nowNs() - t.startNs
	if t.callback != nil {
		t.callback(elapsed)
	}
	return elapsed
}

// ===== LatencyHistogram =====

// LatencyBucket represents a histogram bucket
type LatencyBucket struct {
	UpperBound int64 // in microseconds
	Count      uint64
}

// LatencyHistogram tracks latency distribution
type LatencyHistogram struct {
	buckets []LatencyBucket
	counts  []uint64
	sum     atomic.Int64
	count   atomic.Uint64
	min     atomic.Int64
	max     atomic.Int64
	mu      sync.RWMutex
}

// DefaultLatencyBuckets returns default bucket boundaries in microseconds
func DefaultLatencyBuckets() []int64 {
	return []int64{
		10,      // 10us
		25,      // 25us
		50,      // 50us
		100,     // 100us
		250,     // 250us
		500,     // 500us
		1000,    // 1ms
		2500,    // 2.5ms
		5000,    // 5ms
		10000,   // 10ms
		25000,   // 25ms
		50000,   // 50ms
		100000,  // 100ms
		250000,  // 250ms
		500000,  // 500ms
		1000000, // 1s
	}
}

// NewLatencyHistogram creates a new latency histogram with default buckets
func NewLatencyHistogram() *LatencyHistogram {
	return NewLatencyHistogramWithBuckets(DefaultLatencyBuckets())
}

// NewLatencyHistogramWithBuckets creates a histogram with custom buckets
func NewLatencyHistogramWithBuckets(bounds []int64) *LatencyHistogram {
	h := &LatencyHistogram{
		buckets: make([]LatencyBucket, len(bounds)),
		counts:  make([]uint64, len(bounds)+1), // +1 for overflow bucket
	}
	for i, bound := range bounds {
		h.buckets[i] = LatencyBucket{UpperBound: bound}
	}
	h.min.Store(-1) // -1 means unset
	return h
}

// Record records a latency value in nanoseconds
func (h *LatencyHistogram) Record(latencyNs int64) {
	latencyUs := latencyNs / 1000

	h.sum.Add(latencyNs)
	h.count.Add(1)

	// Update min/max
	for {
		oldMin := h.min.Load()
		if oldMin == -1 || latencyNs < oldMin {
			if h.min.CompareAndSwap(oldMin, latencyNs) {
				break
			}
		} else {
			break
		}
	}

	for {
		oldMax := h.max.Load()
		if latencyNs > oldMax {
			if h.max.CompareAndSwap(oldMax, latencyNs) {
				break
			}
		} else {
			break
		}
	}

	// Find bucket
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i, bucket := range h.buckets {
		if latencyUs <= bucket.UpperBound {
			atomic.AddUint64(&h.counts[i], 1)
			return
		}
	}
	// Overflow bucket
	atomic.AddUint64(&h.counts[len(h.buckets)], 1)
}

// Snapshot returns a snapshot of the histogram
func (h *LatencyHistogram) Snapshot() HistogramSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	counts := make([]uint64, len(h.counts))
	for i, c := range h.counts {
		counts[i] = atomic.LoadUint64(&c)
	}

	min := h.min.Load()
	if min == -1 {
		min = 0
	}

	return HistogramSnapshot{
		Buckets: h.buckets,
		Counts:  counts,
		Sum:     h.sum.Load(),
		Count:   h.count.Load(),
		Min:     min,
		Max:     h.max.Load(),
		Mean:    h.mean(),
	}
}

// mean calculates the mean latency
func (h *LatencyHistogram) mean() float64 {
	count := h.count.Load()
	if count == 0 {
		return 0
	}
	return float64(h.sum.Load()) / float64(count)
}

// HistogramSnapshot represents a point-in-time snapshot of the histogram
type HistogramSnapshot struct {
	Buckets []LatencyBucket
	Counts  []uint64
	Sum     int64
	Count   uint64
	Min     int64
	Max     int64
	Mean    float64
}

// Percentile returns the approximate percentile value in nanoseconds
func (s *HistogramSnapshot) Percentile(p float64) int64 {
	if p < 0 || p > 100 || s.Count == 0 {
		return 0
	}

	targetCount := uint64(float64(s.Count) * p / 100.0)
	var cumulative uint64

	for i, count := range s.Counts {
		cumulative += count
		if cumulative >= targetCount {
			if i < len(s.Buckets) {
				return s.Buckets[i].UpperBound * 1000 // Convert us to ns
			}
			return s.Max
		}
	}

	return s.Max
}

// P50 returns the 50th percentile (median)
func (s *HistogramSnapshot) P50() int64 {
	return s.Percentile(50)
}

// P95 returns the 95th percentile
func (s *HistogramSnapshot) P95() int64 {
	return s.Percentile(95)
}

// P99 returns the 99th percentile
func (s *HistogramSnapshot) P99() int64 {
	return s.Percentile(99)
}

// Reset resets all histogram values
func (h *LatencyHistogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.counts {
		h.counts[i] = 0
	}
	h.sum.Store(0)
	h.count.Store(0)
	h.min.Store(-1)
	h.max.Store(0)
}

// ===== LatencyTracker =====

// LatencyTracker tracks latencies with automatic histogram recording
type LatencyTracker struct {
	histogram *LatencyHistogram
	name      string
}

// NewLatencyTracker creates a new latency tracker
func NewLatencyTracker(name string) *LatencyTracker {
	return &LatencyTracker{
		histogram: NewLatencyHistogram(),
		name:      name,
	}
}

// NewLatencyTrackerWithHistogram creates a tracker with existing histogram
func NewLatencyTrackerWithHistogram(name string, hist *LatencyHistogram) *LatencyTracker {
	return &LatencyTracker{
		histogram: hist,
		name:      name,
	}
}

// Track starts tracking a new operation
func (t *LatencyTracker) Track() *Timer {
	return NewTimer().Start()
}

// Record records a latency value
func (t *LatencyTracker) Record(latencyNs int64) {
	t.histogram.Record(latencyNs)
}

// Snapshot returns the current snapshot
func (t *LatencyTracker) Snapshot() HistogramSnapshot {
	return t.histogram.Snapshot()
}

// Name returns the tracker name
func (t *LatencyTracker) Name() string {
	return t.name
}

// ===== Global Timing =====

var (
	globalTimerPool = sync.Pool{
		New: func() interface{} {
			return NewTimer()
		},
	}
)

// AcquireTimer gets a timer from the pool
func AcquireTimer() *Timer {
	return globalTimerPool.Get().(*Timer).Reset()
}

// ReleaseTimer returns a timer to the pool
func ReleaseTimer(t *Timer) {
	if t != nil {
		globalTimerPool.Put(t)
	}
}

// Measure executes a function and returns its execution time
func Measure(fn func()) int64 {
	start := nowNs()
	fn()
	return nowNs() - start
}

// MeasureWithResult executes a function that returns a value and measures execution time
func MeasureWithResult[T any](fn func() T) (T, int64) {
	start := nowNs()
	result := fn()
	return result, nowNs() - start
}
