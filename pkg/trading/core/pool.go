package core

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// TransactionStatus represents the status of a pending transaction
type TransactionStatus int

const (
	// TransactionStatusQueued indicates the transaction is queued
	TransactionStatusQueued TransactionStatus = iota
	// TransactionStatusPending indicates the transaction is pending submission
	TransactionStatusPending
	// TransactionStatusSubmitted indicates the transaction has been submitted
	TransactionStatusSubmitted
	// TransactionStatusConfirmed indicates the transaction is confirmed
	TransactionStatusConfirmed
	// TransactionStatusFailed indicates the transaction failed
	TransactionStatusFailed
	// TransactionStatusExpired indicates the transaction expired
	TransactionStatusExpired
	// TransactionStatusDropped indicates the transaction was dropped
	TransactionStatusDropped
)

func (t TransactionStatus) String() string {
	return [...]string{
		"Queued", "Pending", "Submitted", "Confirmed",
		"Failed", "Expired", "Dropped",
	}[t]
}

// IsTerminal returns true if the status is terminal
func (t TransactionStatus) IsTerminal() bool {
	switch t {
	case TransactionStatusConfirmed, TransactionStatusFailed, TransactionStatusExpired, TransactionStatusDropped:
		return true
	default:
		return false
	}
}

// PendingTransaction represents a transaction in the pool
type PendingTransaction struct {
	// Signature is the transaction signature
	Signature solana.Signature
	// SerializedTransaction is the serialized transaction bytes
	SerializedTransaction []byte
	// Status is the current transaction status
	Status TransactionStatus
	// Priority is the transaction priority (higher = more important)
	Priority int
	// SubmittedAt is when the transaction was submitted
	SubmittedAt time.Time
	// ExpiresAt is when the transaction expires
	ExpiresAt time.Time
	// MaxRetries is the maximum number of submission retries
	MaxRetries int
	// RetryCount is the current retry count
	RetryCount int
	// LastError is the last error encountered
	LastError error
	// Metadata contains additional transaction metadata
	Metadata map[string]interface{}
	// Callback is called when status changes
	Callback func(*PendingTransaction)
	// mu protects concurrent access to mutable fields
	mu sync.RWMutex
}

// SetStatus updates the transaction status and calls the callback
func (pt *PendingTransaction) SetStatus(status TransactionStatus) {
	pt.mu.Lock()
	pt.Status = status
	pt.mu.Unlock()

	if pt.Callback != nil {
		pt.Callback(pt)
	}
}

// SetError sets the last error
func (pt *PendingTransaction) SetError(err error) {
	pt.mu.Lock()
	pt.LastError = err
	pt.mu.Unlock()
}

// GetStatus returns the current status
func (pt *PendingTransaction) GetStatus() TransactionStatus {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.Status
}

// IsExpired returns true if the transaction has expired
func (pt *PendingTransaction) IsExpired() bool {
	return time.Now().After(pt.ExpiresAt)
}

// CanRetry returns true if the transaction can be retried
func (pt *PendingTransaction) CanRetry() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.RetryCount < pt.MaxRetries && !pt.Status.IsTerminal()
}

// IncrementRetry increments the retry count
func (pt *PendingTransaction) IncrementRetry() {
	pt.mu.Lock()
	pt.RetryCount++
	pt.mu.Unlock()
}

// PoolConfig represents configuration for the transaction pool
type PoolConfig struct {
	// MaxSize is the maximum number of transactions in the pool
	MaxSize int
	// MaxPending is the maximum number of pending submissions
	MaxPending int
	// WorkerCount is the number of worker goroutines
	WorkerCount int
	// QueueTimeout is how long a transaction can wait in queue
	QueueTimeout time.Duration
	// SubmissionTimeout is how long to wait for submission
	SubmissionTimeout time.Duration
	// ConfirmationTimeout is how long to wait for confirmation
	ConfirmationTimeout time.Duration
	// RetryInterval is the interval between retries
	RetryInterval time.Duration
	// CleanupInterval is the interval for cleanup tasks
	CleanupInterval time.Duration
	// DefaultPriority is the default transaction priority
	DefaultPriority int
	// EnablePrioritization enables transaction prioritization
	EnablePrioritization bool
	// MaxRetries is the default maximum retries
	MaxRetries int
}

// DefaultPoolConfig returns a default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxSize:              10000,
		MaxPending:           100,
		WorkerCount:          10,
		QueueTimeout:         5 * time.Minute,
		SubmissionTimeout:    30 * time.Second,
		ConfirmationTimeout:  60 * time.Second,
		RetryInterval:        2 * time.Second,
		CleanupInterval:      10 * time.Second,
		DefaultPriority:      0,
		EnablePrioritization: true,
		MaxRetries:           3,
	}
}

// Validate validates the pool configuration
func (c *PoolConfig) Validate() error {
	if c.MaxSize <= 0 {
		return fmt.Errorf("max size must be positive")
	}
	if c.MaxPending <= 0 {
		return fmt.Errorf("max pending must be positive")
	}
	if c.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}
	if c.QueueTimeout <= 0 {
		return fmt.Errorf("queue timeout must be positive")
	}
	return nil
}

// priorityQueue implements a priority queue for transactions
type priorityQueue []*PendingTransaction

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority comes first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// Earlier submission time comes first
	return pq[i].SubmittedAt.Before(pq[j].SubmittedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*PendingTransaction)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

// TransactionPool manages a pool of pending transactions
type TransactionPool struct {
	config       *PoolConfig
	queue        priorityQueue
	queueMu      sync.Mutex
	transactions sync.Map // map[solana.Signature]*PendingTransaction
	pending      int32    // atomic counter for pending submissions
	submitted    int64    // atomic counter for total submitted
	confirmed    int64    // atomic counter for total confirmed
	failed       int64    // atomic counter for total failed
	dropped      int64    // atomic counter for total dropped
	stopCh       chan struct{}
	wg           sync.WaitGroup
	started      atomic.Bool
	submitFunc   func(ctx context.Context, tx []byte) (solana.Signature, error)
}

// NewTransactionPool creates a new transaction pool
func NewTransactionPool(config *PoolConfig, submitFunc func(ctx context.Context, tx []byte) (solana.Signature, error)) (*TransactionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pool config: %w", err)
	}
	if submitFunc == nil {
		return nil, fmt.Errorf("submit function is required")
	}

	pool := &TransactionPool{
		config:     config,
		queue:      make(priorityQueue, 0),
		stopCh:     make(chan struct{}),
		submitFunc: submitFunc,
	}

	heap.Init(&pool.queue)
	return pool, nil
}

// Start starts the transaction pool workers
func (p *TransactionPool) Start() error {
	if !p.started.CompareAndSwap(false, true) {
		return fmt.Errorf("pool already started")
	}

	// Start workers
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start cleanup goroutine
	p.wg.Add(1)
	go p.cleanup()

	return nil
}

// Stop stops the transaction pool
func (p *TransactionPool) Stop() error {
	if !p.started.CompareAndSwap(true, false) {
		return fmt.Errorf("pool not started")
	}

	close(p.stopCh)
	p.wg.Wait()

	// Mark all pending transactions as dropped
	p.transactions.Range(func(key, value interface{}) bool {
		if tx, ok := value.(*PendingTransaction); ok {
			if !tx.Status.IsTerminal() {
				tx.SetStatus(TransactionStatusDropped)
			}
		}
		return true
	})

	return nil
}

// Submit submits a transaction to the pool
func (p *TransactionPool) Submit(
	ctx context.Context,
	serializedTx []byte,
	priority int,
	metadata map[string]interface{},
) (*PendingTransaction, error) {
	if !p.started.Load() {
		return nil, fmt.Errorf("pool not started")
	}

	// Check pool size
	if p.GetSize() >= p.config.MaxSize {
		return nil, fmt.Errorf("pool is full")
	}

	// Create pending transaction
	sig, err := solana.TransactionFromDecoder(bin.NewBinDecoder(serializedTx))
	if err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	tx := &PendingTransaction{
		Signature:             sig.Signatures[0],
		SerializedTransaction: serializedTx,
		Status:                TransactionStatusQueued,
		Priority:              priority,
		SubmittedAt:           time.Now(),
		ExpiresAt:             time.Now().Add(p.config.QueueTimeout),
		MaxRetries:            p.config.MaxRetries,
		Metadata:              metadata,
	}

	// Store in transactions map
	p.transactions.Store(tx.Signature, tx)

	// Add to priority queue
	p.queueMu.Lock()
	heap.Push(&p.queue, tx)
	p.queueMu.Unlock()

	return tx, nil
}

// Get retrieves a transaction by signature
func (p *TransactionPool) Get(sig solana.Signature) (*PendingTransaction, bool) {
	if value, ok := p.transactions.Load(sig); ok {
		if tx, ok := value.(*PendingTransaction); ok {
			return tx, true
		}
	}
	return nil, false
}

// Cancel cancels a pending transaction
func (p *TransactionPool) Cancel(sig solana.Signature) bool {
	if value, ok := p.transactions.Load(sig); ok {
		if tx, ok := value.(*PendingTransaction); ok {
			if !tx.Status.IsTerminal() {
				tx.SetStatus(TransactionStatusDropped)
				return true
			}
		}
	}
	return false
}

// GetSize returns the current pool size
func (p *TransactionPool) GetSize() int {
	count := 0
	p.transactions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// GetQueueSize returns the number of queued transactions
func (p *TransactionPool) GetQueueSize() int {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	return p.queue.Len()
}

// GetPendingCount returns the number of pending submissions
func (p *TransactionPool) GetPendingCount() int {
	return int(atomic.LoadInt32(&p.pending))
}

// GetStats returns pool statistics
func (p *TransactionPool) GetStats() PoolStats {
	return PoolStats{
		Size:      p.GetSize(),
		QueueSize: p.GetQueueSize(),
		Pending:   p.GetPendingCount(),
		Submitted: atomic.LoadInt64(&p.submitted),
		Confirmed: atomic.LoadInt64(&p.confirmed),
		Failed:    atomic.LoadInt64(&p.failed),
		Dropped:   atomic.LoadInt64(&p.dropped),
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	Size      int
	QueueSize int
	Pending   int
	Submitted int64
	Confirmed int64
	Failed    int64
	Dropped   int64
}

// SuccessRate returns the success rate as a percentage
func (s PoolStats) SuccessRate() float64 {
	total := s.Confirmed + s.Failed
	if total == 0 {
		return 0
	}
	return float64(s.Confirmed) / float64(total) * 100
}

// worker processes transactions from the queue
func (p *TransactionPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		// Check pending limit
		if p.GetPendingCount() >= p.config.MaxPending {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Get next transaction from queue
		p.queueMu.Lock()
		if p.queue.Len() == 0 {
			p.queueMu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		tx := heap.Pop(&p.queue).(*PendingTransaction)
		p.queueMu.Unlock()

		// Check if expired
		if tx.IsExpired() {
			tx.SetStatus(TransactionStatusExpired)
			atomic.AddInt64(&p.dropped, 1)
			continue
		}

		// Check if already processed
		if tx.Status != TransactionStatusQueued {
			continue
		}

		// Submit transaction
		p.submitTransaction(tx)
	}
}

// submitTransaction submits a transaction
func (p *TransactionPool) submitTransaction(tx *PendingTransaction) {
	atomic.AddInt32(&p.pending, 1)
	defer atomic.AddInt32(&p.pending, -1)

	tx.SetStatus(TransactionStatusPending)

	ctx, cancel := context.WithTimeout(context.Background(), p.config.SubmissionTimeout)
	defer cancel()

	sig, err := p.submitFunc(ctx, tx.SerializedTransaction)
	if err != nil {
		tx.SetError(err)

		if tx.CanRetry() {
			tx.IncrementRetry()
			time.Sleep(p.config.RetryInterval)

			// Re-queue for retry
			p.queueMu.Lock()
			heap.Push(&p.queue, tx)
			p.queueMu.Unlock()
			tx.SetStatus(TransactionStatusQueued)
		} else {
			tx.SetStatus(TransactionStatusFailed)
			atomic.AddInt64(&p.failed, 1)
		}
		return
	}

	tx.Signature = sig
	tx.SetStatus(TransactionStatusSubmitted)
	atomic.AddInt64(&p.submitted, 1)
}

// cleanup removes expired and terminal transactions
func (p *TransactionPool) cleanup() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.transactions.Range(func(key, value interface{}) bool {
				if tx, ok := value.(*PendingTransaction); ok {
					// Remove terminal transactions older than 1 hour
					if tx.Status.IsTerminal() && time.Since(tx.SubmittedAt) > time.Hour {
						p.transactions.Delete(key)
					}
				}
				return true
			})
		}
	}
}

// List returns a list of all transactions in the pool
func (p *TransactionPool) List() []*PendingTransaction {
	var txs []*PendingTransaction
	p.transactions.Range(func(key, value interface{}) bool {
		if tx, ok := value.(*PendingTransaction); ok {
			txs = append(txs, tx)
		}
		return true
	})
	return txs
}

// ListByStatus returns transactions filtered by status
func (p *TransactionPool) ListByStatus(status TransactionStatus) []*PendingTransaction {
	var txs []*PendingTransaction
	p.transactions.Range(func(key, value interface{}) bool {
		if tx, ok := value.(*PendingTransaction); ok {
			if tx.GetStatus() == status {
				txs = append(txs, tx)
			}
		}
		return true
	})
	return txs
}

// UpdateStatus updates the status of a transaction
func (p *TransactionPool) UpdateStatus(sig solana.Signature, status TransactionStatus) bool {
	if value, ok := p.transactions.Load(sig); ok {
		if tx, ok := value.(*PendingTransaction); ok {
			oldStatus := tx.GetStatus()
			tx.SetStatus(status)

			// Update counters
			if oldStatus != status {
				switch status {
				case TransactionStatusConfirmed:
					atomic.AddInt64(&p.confirmed, 1)
				case TransactionStatusFailed:
					atomic.AddInt64(&p.failed, 1)
				}
			}
			return true
		}
	}
	return false
}
