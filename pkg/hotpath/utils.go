package hotpath

import (
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter enforces a minimum delay between hot-path submissions.
type RateLimiter struct {
	lastSubmit atomic.Int64
	minDelay   int64
}

// NewRateLimiter creates a minimum-delay limiter. minDelayMs is milliseconds.
func NewRateLimiter(minDelayMs int) *RateLimiter {
	return &RateLimiter{
		minDelay: int64(minDelayMs) * int64(time.Millisecond),
	}
}

// Wait blocks until the configured delay has elapsed since the last caller won.
func (r *RateLimiter) Wait() {
	for {
		last := r.lastSubmit.Load()
		now := time.Now().UnixNano()
		elapsed := now - last

		if elapsed >= r.minDelay {
			if r.lastSubmit.CompareAndSwap(last, now) {
				return
			}
			continue
		}

		time.Sleep(time.Duration(r.minDelay - elapsed))
	}
}

// MetricsCollector tracks hot-path execution counters and latency.
type MetricsCollector struct {
	mu            sync.Mutex
	totalTrades   int64
	successTrades int64
	failedTrades  int64
	totalLatency  int64
}

// RecordTrade records a trade result and latency in milliseconds.
func (m *MetricsCollector) RecordTrade(success bool, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalTrades++
	if success {
		m.successTrades++
	} else {
		m.failedTrades++
	}
	m.totalLatency += latencyMs
}

// GetStats returns total, success, failed, and average latency in milliseconds.
func (m *MetricsCollector) GetStats() (total, success, failed int64, avgLatencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total = m.totalTrades
	success = m.successTrades
	failed = m.failedTrades
	if total > 0 {
		avgLatencyMs = float64(m.totalLatency) / float64(total)
	}
	return
}
