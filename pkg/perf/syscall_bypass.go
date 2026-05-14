package perf

import (
	"sync"
	"time"
)

// SyscallBypassConfig configuration for syscall bypass
type SyscallBypassConfig struct {
	Enabled       bool
	BatchSize     int
	FlushInterval time.Duration
}

// DefaultSyscallBypassConfig returns default configuration
func DefaultSyscallBypassConfig() *SyscallBypassConfig {
	return &SyscallBypassConfig{
		Enabled:       true,
		BatchSize:     32,
		FlushInterval: 10 * time.Millisecond,
	}
}

// SyscallRequest represents a batched syscall request
type SyscallRequest struct {
	Type      string
	Data      []byte
	Timestamp int64
	Callback  func([]byte, error)
}

// SyscallBypassManager manages batched syscalls
type SyscallBypassManager struct {
	config   *SyscallBypassConfig
	queue    []*SyscallRequest
	mu       sync.RWMutex
	ticker   *time.Ticker
	stopChan chan struct{}
}

// NewSyscallBypassManager creates a new syscall bypass manager
func NewSyscallBypassManager(config *SyscallBypassConfig) *SyscallBypassManager {
	if config == nil {
		config = DefaultSyscallBypassConfig()
	}

	return &SyscallBypassManager{
		config:   config,
		queue:    make([]*SyscallRequest, 0, config.BatchSize),
		stopChan: make(chan struct{}),
	}
}

// Start starts the bypass manager
func (m *SyscallBypassManager) Start() {
	if !m.config.Enabled {
		return
	}

	m.ticker = time.NewTicker(m.config.FlushInterval)
	go m.processLoop()
}

// Stop stops the bypass manager
func (m *SyscallBypassManager) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopChan)
	m.Flush()
}

// Submit submits a syscall request
func (m *SyscallBypassManager) Submit(req *SyscallRequest) {
	if !m.config.Enabled {
		if req.Callback != nil {
			req.Callback(nil, nil)
		}
		return
	}

	m.mu.Lock()
	m.queue = append(m.queue, req)
	shouldFlush := len(m.queue) >= m.config.BatchSize
	m.mu.Unlock()

	if shouldFlush {
		m.Flush()
	}
}

// Flush processes all pending requests
func (m *SyscallBypassManager) Flush() {
	m.mu.Lock()
	if len(m.queue) == 0 {
		m.mu.Unlock()
		return
	}

	batch := m.queue
	m.queue = make([]*SyscallRequest, 0, m.config.BatchSize)
	m.mu.Unlock()

	// Process batch
	for _, req := range batch {
		if req.Callback != nil {
			req.Callback(nil, nil)
		}
	}
}

func (m *SyscallBypassManager) processLoop() {
	for {
		select {
		case <-m.ticker.C:
			m.Flush()
		case <-m.stopChan:
			return
		}
	}
}

// FastTimeProvider provides fast time access
type FastTimeProvider struct {
	cachedTime     int64
	updateInterval time.Duration
	mu             sync.RWMutex
	stopChan       chan struct{}
}

// NewFastTimeProvider creates a fast time provider
func NewFastTimeProvider(updateInterval time.Duration) *FastTimeProvider {
	if updateInterval == 0 {
		updateInterval = time.Millisecond
	}

	f := &FastTimeProvider{
		updateInterval: updateInterval,
		stopChan:       make(chan struct{}),
	}
	f.updateTime()
	go f.updateLoop()
	return f
}

// Now returns cached time in nanoseconds
func (f *FastTimeProvider) Now() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cachedTime
}

// NowMillis returns cached time in milliseconds
func (f *FastTimeProvider) NowMillis() int64 {
	return f.Now() / 1e6
}

// Stop stops the time provider
func (f *FastTimeProvider) Stop() {
	close(f.stopChan)
}

func (f *FastTimeProvider) updateTime() {
	f.mu.Lock()
	f.cachedTime = time.Now().UnixNano()
	f.mu.Unlock()
}

func (f *FastTimeProvider) updateLoop() {
	ticker := time.NewTicker(f.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.updateTime()
		case <-f.stopChan:
			return
		}
	}
}

// IOBuffer provides efficient I/O buffering
type IOBuffer struct {
	buf  []byte
	size int
	pos  int
}

// NewIOBuffer creates a new I/O buffer
func NewIOBuffer(size int) *IOBuffer {
	return &IOBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write writes data to buffer
func (b *IOBuffer) Write(data []byte) (int, error) {
	if b.pos+len(data) > b.size {
		return 0, nil // Buffer full
	}
	copy(b.buf[b.pos:], data)
	b.pos += len(data)
	return len(data), nil
}

// Read reads data from buffer
func (b *IOBuffer) Read(p []byte) (int, error) {
	n := copy(p, b.buf[:b.pos])
	return n, nil
}

// Reset resets the buffer
func (b *IOBuffer) Reset() {
	b.pos = 0
}

// Bytes returns buffer contents
func (b *IOBuffer) Bytes() []byte {
	return b.buf[:b.pos]
}

// Len returns current length
func (b *IOBuffer) Len() int {
	return b.pos
}
