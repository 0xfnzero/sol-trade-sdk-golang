package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ConfirmationStatus represents the confirmation status of a transaction
type ConfirmationStatus int

const (
	// ConfirmationStatusUnknown indicates unknown status
	ConfirmationStatusUnknown ConfirmationStatus = iota
	// ConfirmationStatusProcessed indicates the transaction was processed
	ConfirmationStatusProcessed
	// ConfirmationStatusConfirmed indicates the transaction is confirmed
	ConfirmationStatusConfirmed
	// ConfirmationStatusFinalized indicates the transaction is finalized
	ConfirmationStatusFinalized
)

func (c ConfirmationStatus) String() string {
	return [...]string{"Unknown", "Processed", "Confirmed", "Finalized"}[c]
}

// ToRPCCommitment converts to RPC commitment type
func (c ConfirmationStatus) ToRPCCommitment() rpc.CommitmentType {
	switch c {
	case ConfirmationStatusProcessed:
		return rpc.CommitmentProcessed
	case ConfirmationStatusConfirmed:
		return rpc.CommitmentConfirmed
	case ConfirmationStatusFinalized:
		return rpc.CommitmentFinalized
	default:
		return rpc.CommitmentConfirmed
	}
}

// ConfirmationConfig represents configuration for confirmation monitoring
type ConfirmationConfig struct {
	// PollInterval is the interval between status polls
	PollInterval time.Duration
	// Timeout is the maximum time to wait for confirmation
	Timeout time.Duration
	// TargetStatus is the desired confirmation status
	TargetStatus ConfirmationStatus
	// MaxRetries is the maximum number of poll retries
	MaxRetries int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
	// BatchSize is the number of signatures to query in a batch
	BatchSize int
	// EnableWebsocket enables websocket subscription for confirmations
	EnableWebsocket bool
	// WebsocketURL is the websocket URL (empty for same as RPC)
	WebsocketURL string
	// CallbackInterval is the interval for progress callbacks
	CallbackInterval time.Duration
	// SkipStatusCheck skips the initial status check
	SkipStatusCheck bool
}

// DefaultConfirmationConfig returns a default confirmation configuration
func DefaultConfirmationConfig() *ConfirmationConfig {
	return &ConfirmationConfig{
		PollInterval:     500 * time.Millisecond,
		Timeout:          60 * time.Second,
		TargetStatus:     ConfirmationStatusConfirmed,
		MaxRetries:       120,
		RetryDelay:       500 * time.Millisecond,
		BatchSize:        256,
		EnableWebsocket:  false,
		CallbackInterval: 5 * time.Second,
		SkipStatusCheck:  false,
	}
}

// Validate validates the confirmation configuration
func (c *ConfirmationConfig) Validate() error {
	if c.PollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	return nil
}

// ConfirmationResult represents the result of confirmation monitoring
type ConfirmationResult struct {
	// Signature is the transaction signature
	Signature solana.Signature
	// Status is the final confirmation status
	Status ConfirmationStatus
	// Slot is the slot in which the transaction was confirmed
	Slot uint64
	// BlockTime is the block time
	BlockTime *time.Time
	// Confirmations is the number of confirmations received
	Confirmations uint64
	// Err is set if confirmation failed
	Err error
	// ConfirmTime is the time to confirmation
	ConfirmTime time.Duration
	// PollCount is the number of polls performed
	PollCount int
	// RetryCount is the number of retries
	RetryCount int
}

// IsSuccess returns true if confirmation was successful
func (r *ConfirmationResult) IsSuccess() bool {
	return r.Err == nil && (r.Status == ConfirmationStatusConfirmed || r.Status == ConfirmationStatusFinalized)
}

// ConfirmationProgress represents progress during confirmation
type ConfirmationProgress struct {
	// Signature is the transaction signature
	Signature solana.Signature
	// Status is the current status
	Status ConfirmationStatus
	// Slot is the current slot
	Slot uint64
	// Confirmations is the current confirmation count
	Confirmations uint64
	// Elapsed is the elapsed time
	Elapsed time.Duration
	// PollCount is the number of polls
	PollCount int
}

// ConfirmationCallback is called during confirmation progress
type ConfirmationCallback func(progress *ConfirmationProgress)

// ConfirmationMonitor monitors transaction confirmations
type ConfirmationMonitor struct {
	config    *ConfirmationConfig
	rpcClient *rpc.Client
	watched   sync.Map // map[solana.Signature]*watchContext
	callbacks []ConfirmationCallback
	mu        sync.RWMutex
	started   atomic.Bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// watchContext tracks the context of a watched transaction
type watchContext struct {
	signature  solana.Signature
	result     *ConfirmationResult
	config     *ConfirmationConfig
	ctx        context.Context
	cancel     context.CancelFunc
	startTime  time.Time
	pollCount  int
	retryCount int
	lastStatus ConfirmationStatus
}

// NewConfirmationMonitor creates a new confirmation monitor
func NewConfirmationMonitor(config *ConfirmationConfig, rpcClient *rpc.Client) (*ConfirmationMonitor, error) {
	if config == nil {
		config = DefaultConfirmationConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid confirmation config: %w", err)
	}
	if rpcClient == nil {
		return nil, fmt.Errorf("RPC client is required")
	}

	return &ConfirmationMonitor{
		config:    config,
		rpcClient: rpcClient,
		stopCh:    make(chan struct{}),
		callbacks: make([]ConfirmationCallback, 0),
	}, nil
}

// Start starts the confirmation monitor
func (m *ConfirmationMonitor) Start() error {
	if !m.started.CompareAndSwap(false, true) {
		return fmt.Errorf("monitor already started")
	}

	// Start polling goroutine
	m.wg.Add(1)
	go m.pollLoop()

	return nil
}

// Stop stops the confirmation monitor
func (m *ConfirmationMonitor) Stop() error {
	if !m.started.CompareAndSwap(true, false) {
		return fmt.Errorf("monitor not started")
	}

	close(m.stopCh)
	m.wg.Wait()

	// Cancel all watched transactions
	m.watched.Range(func(key, value interface{}) bool {
		if ctx, ok := value.(*watchContext); ok {
			ctx.cancel()
		}
		return true
	})

	return nil
}

// Watch starts watching a transaction for confirmation
func (m *ConfirmationMonitor) Watch(
	ctx context.Context,
	signature solana.Signature,
	configOverrides ...*ConfirmationConfig,
) (*ConfirmationResult, error) {
	if !m.started.Load() {
		return nil, fmt.Errorf("monitor not started")
	}

	config := m.config
	if len(configOverrides) > 0 && configOverrides[0] != nil {
		config = configOverrides[0]
	}

	// Check if already watching
	if _, exists := m.watched.Load(signature); exists {
		return nil, fmt.Errorf("already watching signature: %s", signature)
	}

	watchCtx, cancel := context.WithTimeout(ctx, config.Timeout)

	result := &ConfirmationResult{
		Signature: signature,
		Status:    ConfirmationStatusUnknown,
	}

	watch := &watchContext{
		signature:  signature,
		result:     result,
		config:     config,
		ctx:        watchCtx,
		cancel:     cancel,
		startTime:  time.Now(),
		lastStatus: ConfirmationStatusUnknown,
	}

	m.watched.Store(signature, watch)

	// Start watching in background
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer m.watched.Delete(signature)
		m.watchTransaction(watch)
	}()

	return result, nil
}

// watchTransaction watches a single transaction
func (m *ConfirmationMonitor) watchTransaction(watch *watchContext) {
	defer watch.cancel()

	ticker := time.NewTicker(watch.config.PollInterval)
	defer ticker.Stop()

	callbackTicker := time.NewTicker(watch.config.CallbackInterval)
	defer callbackTicker.Stop()

	for {
		select {
		case <-watch.ctx.Done():
			if watch.result.Err == nil {
				watch.result.Err = fmt.Errorf("confirmation timeout")
			}
			return
		case <-m.stopCh:
			watch.result.Err = fmt.Errorf("monitor stopped")
			return
		case <-ticker.C:
			status, err := m.checkStatus(watch)
			if err != nil {
				watch.retryCount++
				if watch.retryCount > watch.config.MaxRetries {
					watch.result.Err = fmt.Errorf("max retries exceeded: %w", err)
					return
				}
				time.Sleep(watch.config.RetryDelay)
				continue
			}

			watch.pollCount++
			watch.result.PollCount = watch.pollCount
			watch.result.RetryCount = watch.retryCount

			if status != nil {
				m.updateStatus(watch, status)

				// Check if target status reached
				if watch.result.Status >= watch.config.TargetStatus {
					watch.result.ConfirmTime = time.Since(watch.startTime)
					return
				}

				// Check if transaction failed
				if status.Err != nil {
					watch.result.Err = m.transactionError(watch.ctx, watch.signature, status.Err)
					return
				}
			}

		case <-callbackTicker.C:
			m.notifyProgress(watch)
		}
	}
}

func (m *ConfirmationMonitor) transactionError(ctx context.Context, sig solana.Signature, statusErr interface{}) error {
	var zero uint64
	tx, err := m.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		Commitment:                     rpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &zero,
	})
	if err != nil || tx == nil || tx.Meta == nil {
		return soltradesdk.FormatParsedTransactionError(statusErr, nil)
	}
	return soltradesdk.FormatParsedTransactionError(tx.Meta.Err, tx.Meta.LogMessages)
}

// checkStatus checks the confirmation status of a transaction
func (m *ConfirmationMonitor) checkStatus(watch *watchContext) (*rpc.SignatureStatusesResult, error) {
	statuses, err := m.rpcClient.GetSignatureStatuses(watch.ctx, false, watch.signature)
	if err != nil {
		return nil, err
	}

	if len(statuses.Value) == 0 || statuses.Value[0] == nil {
		return nil, fmt.Errorf("no status available")
	}

	return statuses.Value[0], nil
}

// updateStatus updates the confirmation status
func (m *ConfirmationMonitor) updateStatus(watch *watchContext, status *rpc.SignatureStatusesResult) {
	if status.ConfirmationStatus != "" {
		switch status.ConfirmationStatus {
		case rpc.ConfirmationStatusProcessed:
			watch.result.Status = ConfirmationStatusProcessed
		case rpc.ConfirmationStatusConfirmed:
			watch.result.Status = ConfirmationStatusConfirmed
		case rpc.ConfirmationStatusFinalized:
			watch.result.Status = ConfirmationStatusFinalized
		}
	}

	watch.result.Slot = uint64(status.Slot)
	if status.Confirmations != nil {
		watch.result.Confirmations = *status.Confirmations
	}
}

// notifyProgress notifies progress callbacks
func (m *ConfirmationMonitor) notifyProgress(watch *watchContext) {
	progress := &ConfirmationProgress{
		Signature:     watch.signature,
		Status:        watch.result.Status,
		Slot:          watch.result.Slot,
		Confirmations: watch.result.Confirmations,
		Elapsed:       time.Since(watch.startTime),
		PollCount:     watch.pollCount,
	}

	m.mu.RLock()
	callbacks := make([]ConfirmationCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(progress)
	}
}

// pollLoop is the main polling loop for batch status checks
func (m *ConfirmationMonitor) pollLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.batchCheck()
		}
	}
}

// batchCheck performs batch status checks
func (m *ConfirmationMonitor) batchCheck() {
	var signatures []solana.Signature
	var watches []*watchContext

	// Collect signatures to check
	m.watched.Range(func(key, value interface{}) bool {
		if watch, ok := value.(*watchContext); ok {
			signatures = append(signatures, watch.signature)
			watches = append(watches, watch)
		}
		return len(signatures) < m.config.BatchSize
	})

	if len(signatures) == 0 {
		return
	}

	// Batch query
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statuses, err := m.rpcClient.GetSignatureStatuses(ctx, false, signatures...)
	if err != nil {
		return
	}

	// Update results
	for i, status := range statuses.Value {
		if i >= len(watches) || status == nil {
			continue
		}
		m.updateStatus(watches[i], status)
	}
}

// AddCallback adds a confirmation progress callback
func (m *ConfirmationMonitor) AddCallback(callback ConfirmationCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// GetResult retrieves the current result for a watched signature
func (m *ConfirmationMonitor) GetResult(signature solana.Signature) (*ConfirmationResult, bool) {
	if value, ok := m.watched.Load(signature); ok {
		if watch, ok := value.(*watchContext); ok {
			return watch.result, true
		}
	}
	return nil, false
}

// Unwatch stops watching a signature
func (m *ConfirmationMonitor) Unwatch(signature solana.Signature) bool {
	if value, ok := m.watched.Load(signature); ok {
		if watch, ok := value.(*watchContext); ok {
			watch.cancel()
			m.watched.Delete(signature)
			return true
		}
	}
	return false
}

// GetWatchedCount returns the number of watched transactions
func (m *ConfirmationMonitor) GetWatchedCount() int {
	count := 0
	m.watched.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// WaitForConfirmation blocks until a transaction is confirmed
func (m *ConfirmationMonitor) WaitForConfirmation(
	ctx context.Context,
	signature solana.Signature,
	configOverrides ...*ConfirmationConfig,
) (*ConfirmationResult, error) {
	config := m.config
	if len(configOverrides) > 0 && configOverrides[0] != nil {
		config = configOverrides[0]
	}

	watchCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	result := &ConfirmationResult{
		Signature: signature,
		Status:    ConfirmationStatusUnknown,
	}

	watch := &watchContext{
		signature:  signature,
		result:     result,
		config:     config,
		ctx:        watchCtx,
		cancel:     cancel,
		startTime:  time.Now(),
		lastStatus: ConfirmationStatusUnknown,
	}

	m.watchTransaction(watch)

	return result, result.Err
}

// ConfirmBatch waits for multiple transactions to confirm
func (m *ConfirmationMonitor) ConfirmBatch(
	ctx context.Context,
	signatures []solana.Signature,
	configOverrides ...*ConfirmationConfig,
) ([]*ConfirmationResult, error) {
	results := make([]*ConfirmationResult, len(signatures))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, sig := range signatures {
		wg.Add(1)
		go func(idx int, signature solana.Signature) {
			defer wg.Done()

			result, err := m.WaitForConfirmation(ctx, signature, configOverrides...)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
			results[idx] = result
		}(i, sig)
	}

	wg.Wait()
	return results, firstErr
}
