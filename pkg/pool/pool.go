package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ===== Worker Pool =====

// Task represents a unit of work
type Task func() (interface{}, error)

// Result represents the result of a task
type Result struct {
	Value interface{}
	Error error
}

// WorkerPool manages a pool of workers for parallel execution
type WorkerPool struct {
	taskQueue  chan Task
	resultChan chan Result
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	active     int64
	tasksDone  int64
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &WorkerPool{
		taskQueue:  make(chan Task, queueSize),
		resultChan: make(chan Result, queueSize),
		workers:    workers,
		ctx:        ctx,
		cancel:     cancel,
	}
	p.start()
	return p
}

// Start starts the worker pool
func (p *WorkerPool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker processes tasks from the queue
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			atomic.AddInt64(&p.active, 1)
			result, err := task()
			atomic.AddInt64(&p.tasksDone, 1)
			atomic.AddInt64(&p.active, -1)
			p.resultChan <- Result{Value: result, Error: err}
		}
	}
}

// Submit submits a task to the pool
func (p *WorkerPool) Submit(task Task) error {
	select {
	case p.taskQueue <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// SubmitWait submits a task and waits for the result
func (p *WorkerPool) SubmitWait(task Task) (interface{}, error) {
	if err := p.Submit(task); err != nil {
		return nil, err
	}
	result := <-p.resultChan
	return result.Value, result.Error
}

// SubmitBatch submits multiple tasks and returns results
func (p *WorkerPool) SubmitBatch(tasks []Task) []Result {
	results := make([]Result, len(tasks))
	for i, task := range tasks {
		if err := p.Submit(task); err != nil {
			results[i] = Result{Error: err}
			continue
		}
	}
	for i := range results {
		if results[i].Error == nil {
			results[i] = <-p.resultChan
		}
	}
	return results
}

// Close shuts down the worker pool
func (p *WorkerPool) Close() {
	p.cancel()
	p.wg.Wait()
	close(p.taskQueue)
	close(p.resultChan)
}

// Stats returns pool statistics
func (p *WorkerPool) Stats() (active, done int64) {
	return atomic.LoadInt64(&p.active), atomic.LoadInt64(&p.tasksDone)
}

// Error definitions
var ErrQueueFull = &PoolError{Code: 1, Message: "task queue is full"}

type PoolError struct {
	Code    int
	Message string
}

func (e *PoolError) Error() string { return e.Message }

// ===== Connection Pool =====

// Connection represents a generic connection
type Connection interface {
	Close() error
	IsAlive() bool
}

// ConnectionFactory creates new connections
type ConnectionFactory func() (Connection, error)

// ConnectionPool manages a pool of connections
type ConnectionPool struct {
	factory   ConnectionFactory
	pool      chan Connection
	maxSize   int
	mu        sync.Mutex
	created   int
	waiting   int64
	createdAt map[Connection]time.Time
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(factory ConnectionFactory, maxSize int) *ConnectionPool {
	return &ConnectionPool{
		factory:   factory,
		pool:      make(chan Connection, maxSize),
		maxSize:   maxSize,
		createdAt: make(map[Connection]time.Time),
	}
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get() (Connection, error) {
	select {
	case conn := <-p.pool:
		if conn.IsAlive() {
			return conn, nil
		}
		p.mu.Lock()
		delete(p.createdAt, conn)
		p.created--
		p.mu.Unlock()
	default:
	}

	p.mu.Lock()
	if p.created < p.maxSize {
		conn, err := p.factory()
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		p.created++
		p.createdAt[conn] = time.Now()
		p.mu.Unlock()
		return conn, nil
	}
	p.mu.Unlock()

	// Wait for available connection
	atomic.AddInt64(&p.waiting, 1)
	defer atomic.AddInt64(&p.waiting, -1)

	select {
	case conn := <-p.pool:
		return conn, nil
	case <-time.After(30 * time.Second):
		return nil, ErrConnectionTimeout
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn Connection) {
	if !conn.IsAlive() {
		p.mu.Lock()
		delete(p.createdAt, conn)
		p.created--
		p.mu.Unlock()
		return
	}

	select {
	case p.pool <- conn:
	default:
		conn.Close()
		p.mu.Lock()
		delete(p.createdAt, conn)
		p.created--
		p.mu.Unlock()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.pool)
	for conn := range p.pool {
		conn.Close()
		delete(p.createdAt, conn)
	}
	p.created = 0
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() (created, waiting, available int) {
	p.mu.Lock()
	created = p.created
	p.mu.Unlock()
	waiting = int(atomic.LoadInt64(&p.waiting))
	available = len(p.pool)
	return
}

var ErrConnectionTimeout = &PoolError{Code: 2, Message: "connection timeout"}

// ===== Rate Limiter Pool =====

// RateLimiterPool manages rate limiters for different keys
type RateLimiterPool struct {
	limiters sync.Map
	rate     int
	burst    int
}

// NewRateLimiterPool creates a new rate limiter pool
func NewRateLimiterPool(rate, burst int) *RateLimiterPool {
	return &RateLimiterPool{
		rate:  rate,
		burst: burst,
	}
}

type rateLimiter struct {
	mu        sync.Mutex
	tokens    int
	lastCheck time.Time
	rate      int
	burst     int
}

// Allow checks if the request is allowed for the given key
func (p *RateLimiterPool) Allow(key string) bool {
	v, _ := p.limiters.LoadOrStore(key, &rateLimiter{
		tokens:    p.burst,
		lastCheck: time.Now(),
		rate:      p.rate,
		burst:     p.burst,
	})
	limiter := v.(*rateLimiter)
	return limiter.allow()
}

func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastCheck)
	r.lastCheck = now

	// Add tokens based on elapsed time
	r.tokens += int(elapsed.Seconds() * float64(r.rate))
	if r.tokens > r.burst {
		r.tokens = r.burst
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}
