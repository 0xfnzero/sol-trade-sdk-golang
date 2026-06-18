package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// SubmitMode represents the transaction submission mode
type SubmitMode int

const (
	// SubmitModeSync waits for confirmation before returning
	SubmitModeSync SubmitMode = iota
	// SubmitModeAsync returns immediately after submission
	SubmitModeAsync
	// SubmitModeFireAndForget submits without waiting for any response
	SubmitModeFireAndForget
)

func (s SubmitMode) String() string {
	return [...]string{"Sync", "Async", "FireAndForget"}[s]
}

// ExecutionStatus represents the status of a transaction execution
type ExecutionStatus int

const (
	// ExecutionStatusPending indicates the transaction is pending
	ExecutionStatusPending ExecutionStatus = iota
	// ExecutionStatusSubmitted indicates the transaction has been submitted
	ExecutionStatusSubmitted
	// ExecutionStatusConfirmed indicates the transaction is confirmed
	ExecutionStatusConfirmed
	// ExecutionStatusFinalized indicates the transaction is finalized
	ExecutionStatusFinalized
	// ExecutionStatusFailed indicates the transaction failed
	ExecutionStatusFailed
	// ExecutionStatusTimeout indicates the transaction timed out
	ExecutionStatusTimeout
	// ExecutionStatusCancelled indicates the transaction was cancelled
	ExecutionStatusCancelled
)

func (e ExecutionStatus) String() string {
	return [...]string{
		"Pending", "Submitted", "Confirmed", "Finalized",
		"Failed", "Timeout", "Cancelled",
	}[e]
}

// IsTerminal returns true if the status is terminal (no further changes expected)
func (e ExecutionStatus) IsTerminal() bool {
	switch e {
	case ExecutionStatusFinalized, ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

// ExecutionConfig represents configuration for async trade execution
type ExecutionConfig struct {
	// SubmitMode determines how transactions are submitted
	SubmitMode SubmitMode
	// MaxConcurrentExecutions limits concurrent executions (0 = unlimited)
	MaxConcurrentExecutions int
	// DefaultTimeout is the default execution timeout
	DefaultTimeout time.Duration
	// ConfirmationTimeout is the timeout for waiting confirmation
	ConfirmationTimeout time.Duration
	// RetryEnabled enables automatic retry on failure
	RetryEnabled bool
	// MaxRetries is the maximum number of retries
	MaxRetries int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
	// ParallelSubmission submits to multiple providers in parallel
	ParallelSubmission bool
	// RequiredConfirmations is the number of confirmations required
	RequiredConfirmations uint8
	// CommitmentLevel is the desired commitment level
	CommitmentLevel rpc.CommitmentType
	// PriorityFeeLamports is the priority fee in lamports
	PriorityFeeLamports uint64
	// ComputeUnitLimit is the compute unit limit
	ComputeUnitLimit uint64
	// SkipPreflight skips preflight transaction checks
	SkipPreflight bool
	// PreflightCommitment is the commitment level for preflight
	PreflightCommitment rpc.CommitmentType
	// ReplaceByFee enables replace-by-fee for pending transactions
	ReplaceByFee bool
	// ReplaceByFeeIncrement is the fee increment for RBF
	ReplaceByFeeIncrement uint64
}

// DefaultExecutionConfig returns a default execution configuration
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		SubmitMode:              SubmitModeSync,
		MaxConcurrentExecutions: 100,
		DefaultTimeout:          30 * time.Second,
		ConfirmationTimeout:     60 * time.Second,
		RetryEnabled:            true,
		MaxRetries:              3,
		RetryDelay:              500 * time.Millisecond,
		ParallelSubmission:      true,
		RequiredConfirmations:   1,
		CommitmentLevel:         rpc.CommitmentConfirmed,
		PriorityFeeLamports:     10000,
		ComputeUnitLimit:        200000,
		SkipPreflight:           false,
		PreflightCommitment:     rpc.CommitmentProcessed,
		ReplaceByFee:            false,
		ReplaceByFeeIncrement:   5000,
	}
}

// Validate validates the execution configuration
func (c *ExecutionConfig) Validate() error {
	if c.DefaultTimeout <= 0 {
		return fmt.Errorf("default timeout must be positive")
	}
	if c.ConfirmationTimeout <= 0 {
		return fmt.Errorf("confirmation timeout must be positive")
	}
	if c.RetryEnabled && c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if c.RequiredConfirmations == 0 {
		return fmt.Errorf("required confirmations must be at least 1")
	}
	return nil
}

// ExecutionResult represents the result of an async trade execution
type ExecutionResult struct {
	// Signature is the transaction signature
	Signature solana.Signature
	// Status is the current execution status
	Status ExecutionStatus
	// Error is set if the execution failed
	Error error
	// SubmittedAt is when the transaction was submitted
	SubmittedAt time.Time
	// ConfirmedAt is when the transaction was confirmed
	ConfirmedAt *time.Time
	// FinalizedAt is when the transaction was finalized
	FinalizedAt *time.Time
	// ConfirmationCount is the number of confirmations received
	ConfirmationCount uint8
	// Slot is the slot in which the transaction was confirmed
	Slot uint64
	// BlockTime is the block time of the confirmed transaction
	BlockTime *time.Time
	// RetryCount is the number of retries performed
	RetryCount int
	// ExecutionTimeMs is the total execution time in milliseconds
	ExecutionTimeMs int64
	// ConfirmationTimeMs is the time to confirmation in milliseconds
	ConfirmationTimeMs int64
	// ProviderID identifies which provider submitted the transaction
	ProviderID string
	// RawResponse contains the raw RPC response
	RawResponse interface{}
}

// IsSuccess returns true if the execution was successful
func (r *ExecutionResult) IsSuccess() bool {
	return r.Status == ExecutionStatusConfirmed || r.Status == ExecutionStatusFinalized
}

// IsPending returns true if the execution is still pending
func (r *ExecutionResult) IsPending() bool {
	return !r.Status.IsTerminal()
}

// Duration returns the total duration of the execution
func (r *ExecutionResult) Duration() time.Duration {
	endTime := time.Now()
	if r.FinalizedAt != nil {
		endTime = *r.FinalizedAt
	} else if r.ConfirmedAt != nil {
		endTime = *r.ConfirmedAt
	}
	return endTime.Sub(r.SubmittedAt)
}

// AsyncTradeExecutor handles asynchronous trade execution
type AsyncTradeExecutor struct {
	config     *ExecutionConfig
	rpcClient  *rpc.Client
	semaphore  chan struct{}
	executions sync.Map // map[solana.Signature]*executionContext
	callbacks  []ExecutionCallback
	mu         sync.RWMutex
	started    atomic.Bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// executionContext tracks the context of an ongoing execution
type executionContext struct {
	result      *ExecutionResult
	transaction *solana.Transaction
	config      *ExecutionConfig
	ctx         context.Context
	cancel      context.CancelFunc
	retryCount  int
	submittedTo []string
}

// ExecutionCallback is called when execution status changes
type ExecutionCallback func(result *ExecutionResult)

// NewAsyncTradeExecutor creates a new async trade executor
func NewAsyncTradeExecutor(config *ExecutionConfig, rpcClient *rpc.Client) (*AsyncTradeExecutor, error) {
	if config == nil {
		config = DefaultExecutionConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid execution config: %w", err)
	}

	executor := &AsyncTradeExecutor{
		config:    config,
		rpcClient: rpcClient,
		stopCh:    make(chan struct{}),
		callbacks: make([]ExecutionCallback, 0),
	}

	if config.MaxConcurrentExecutions > 0 {
		executor.semaphore = make(chan struct{}, config.MaxConcurrentExecutions)
	}

	return executor, nil
}

// Start starts the executor background tasks
func (e *AsyncTradeExecutor) Start() error {
	if !e.started.CompareAndSwap(false, true) {
		return fmt.Errorf("executor already started")
	}

	// Start status monitoring goroutine
	e.wg.Add(1)
	go e.monitorExecutions()

	return nil
}

// Stop stops the executor and waits for pending executions
func (e *AsyncTradeExecutor) Stop() error {
	if !e.started.CompareAndSwap(true, false) {
		return fmt.Errorf("executor not started")
	}

	close(e.stopCh)
	e.wg.Wait()

	// Cancel all pending executions
	e.executions.Range(func(key, value interface{}) bool {
		if ctx, ok := value.(*executionContext); ok {
			ctx.cancel()
		}
		return true
	})

	return nil
}

// Submit submits a transaction for async execution
func (e *AsyncTradeExecutor) Submit(
	ctx context.Context,
	transaction *solana.Transaction,
	configOverrides ...*ExecutionConfig,
) (*ExecutionResult, error) {
	if !e.started.Load() {
		return nil, fmt.Errorf("executor not started")
	}

	config := e.config
	if len(configOverrides) > 0 && configOverrides[0] != nil {
		config = configOverrides[0]
	}

	// Acquire semaphore if configured
	if e.semaphore != nil {
		select {
		case e.semaphore <- struct{}{}:
			defer func() { <-e.semaphore }()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.stopCh:
			return nil, fmt.Errorf("executor stopped")
		}
	}

	// Create execution context
	execCtx, cancel := context.WithTimeout(ctx, config.DefaultTimeout)
	defer func() {
		if config.SubmitMode == SubmitModeFireAndForget {
			cancel()
		}
	}()

	result := &ExecutionResult{
		Signature:   transaction.Signatures[0],
		Status:      ExecutionStatusPending,
		SubmittedAt: time.Now(),
	}

	exec := &executionContext{
		result:      result,
		transaction: transaction,
		config:      config,
		ctx:         execCtx,
		cancel:      cancel,
	}

	// Store execution context
	e.executions.Store(result.Signature, exec)

	// Execute based on mode
	switch config.SubmitMode {
	case SubmitModeSync:
		e.executeSync(exec)
		return result, result.Error
	case SubmitModeAsync:
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.executeSync(exec)
		}()
		return result, nil
	case SubmitModeFireAndForget:
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.executeSync(exec)
		}()
		return result, nil
	default:
		return nil, fmt.Errorf("unknown submit mode: %v", config.SubmitMode)
	}
}

// executeSync executes a transaction synchronously
func (e *AsyncTradeExecutor) executeSync(exec *executionContext) {
	defer exec.cancel()

	start := time.Now()
	exec.result.ExecutionTimeMs = 0

	// Serialize transaction
	txBytes, err := exec.transaction.MarshalBinary()
	if err != nil {
		e.updateStatus(exec, ExecutionStatusFailed, fmt.Errorf("failed to serialize transaction: %w", err))
		return
	}

	// Submit transaction with retry logic
	var sig solana.Signature
	for attempt := 0; attempt <= exec.config.MaxRetries; attempt++ {
		if attempt > 0 {
			exec.result.RetryCount = attempt
			time.Sleep(exec.config.RetryDelay)
		}

		sig, err = e.submitTransaction(exec.ctx, txBytes, exec.config)
		if err == nil {
			break
		}

		if !exec.config.RetryEnabled || attempt == exec.config.MaxRetries {
			e.updateStatus(exec, ExecutionStatusFailed, fmt.Errorf("submission failed after %d attempts: %w", attempt+1, err))
			return
		}
	}

	exec.result.Signature = sig
	e.updateStatus(exec, ExecutionStatusSubmitted, nil)

	// Wait for confirmation if required
	if exec.config.SubmitMode == SubmitModeSync || exec.config.RequiredConfirmations > 0 {
		e.waitForConfirmation(exec)
	}

	exec.result.ExecutionTimeMs = time.Since(start).Milliseconds()
}

// submitTransaction submits a transaction to the RPC
func (e *AsyncTradeExecutor) submitTransaction(
	ctx context.Context,
	txBytes []byte,
	config *ExecutionConfig,
) (solana.Signature, error) {
	opts := rpc.TransactionOpts{
		SkipPreflight:       config.SkipPreflight,
		PreflightCommitment: config.PreflightCommitment,
	}

	sig, err := e.rpcClient.SendEncodedTransactionWithOpts(ctx, base64.StdEncoding.EncodeToString(txBytes), opts)
	if err != nil {
		return solana.Signature{}, err
	}

	return sig, nil
}

// waitForConfirmation waits for transaction confirmation
func (e *AsyncTradeExecutor) waitForConfirmation(exec *executionContext) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(exec.config.ConfirmationTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-exec.ctx.Done():
			e.updateStatus(exec, ExecutionStatusTimeout, fmt.Errorf("confirmation timeout"))
			return
		case <-timeout.C:
			e.updateStatus(exec, ExecutionStatusTimeout, fmt.Errorf("confirmation timeout"))
			return
		case <-e.stopCh:
			e.updateStatus(exec, ExecutionStatusCancelled, fmt.Errorf("executor stopped"))
			return
		case <-ticker.C:
			status, err := e.checkConfirmation(exec)
			if err != nil {
				continue
			}

			if status != nil {
				if status.Err != nil {
					e.updateStatus(exec, ExecutionStatusFailed, e.transactionError(exec.ctx, exec.result.Signature, status.Err))
					return
				}

				if status.ConfirmationStatus != "" {
					switch status.ConfirmationStatus {
					case rpc.ConfirmationStatusConfirmed:
						confirmedAt := time.Now()
						exec.result.ConfirmedAt = &confirmedAt
						exec.result.ConfirmationTimeMs = confirmedAt.Sub(exec.result.SubmittedAt).Milliseconds()
						exec.result.Slot = uint64(status.Slot)
						e.updateStatus(exec, ExecutionStatusConfirmed, nil)

						if exec.config.CommitmentLevel == rpc.CommitmentConfirmed {
							return
						}

					case rpc.ConfirmationStatusFinalized:
						finalizedAt := time.Now()
						exec.result.FinalizedAt = &finalizedAt
						exec.result.Slot = uint64(status.Slot)
						e.updateStatus(exec, ExecutionStatusFinalized, nil)
						return
					}
				}
			}
		}
	}
}

func (e *AsyncTradeExecutor) transactionError(ctx context.Context, sig solana.Signature, statusErr interface{}) error {
	var zero uint64
	tx, err := e.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		Commitment:                     rpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &zero,
	})
	if err != nil || tx == nil || tx.Meta == nil {
		return soltradesdk.FormatParsedTransactionError(statusErr, nil)
	}
	return soltradesdk.FormatParsedTransactionError(tx.Meta.Err, tx.Meta.LogMessages)
}

// checkConfirmation checks the confirmation status of a transaction
func (e *AsyncTradeExecutor) checkConfirmation(exec *executionContext) (*rpc.SignatureStatusesResult, error) {
	statuses, err := e.rpcClient.GetSignatureStatuses(exec.ctx, false, exec.result.Signature)
	if err != nil {
		return nil, err
	}

	if len(statuses.Value) == 0 || statuses.Value[0] == nil {
		return nil, fmt.Errorf("no status available")
	}

	return statuses.Value[0], nil
}

// updateStatus updates the execution status and notifies callbacks
func (e *AsyncTradeExecutor) updateStatus(exec *executionContext, status ExecutionStatus, err error) {
	exec.result.Status = status
	exec.result.Error = err

	// Notify callbacks
	e.mu.RLock()
	callbacks := make([]ExecutionCallback, len(e.callbacks))
	copy(callbacks, e.callbacks)
	e.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(exec.result)
	}

	// Clean up if terminal
	if status.IsTerminal() {
		e.executions.Delete(exec.result.Signature)
	}
}

// monitorExecutions monitors ongoing executions
func (e *AsyncTradeExecutor) monitorExecutions() {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			// Clean up stale executions
			e.executions.Range(func(key, value interface{}) bool {
				if exec, ok := value.(*executionContext); ok {
					if exec.result.Status.IsTerminal() {
						e.executions.Delete(key)
					}
				}
				return true
			})
		}
	}
}

// GetExecution retrieves an execution by signature
func (e *AsyncTradeExecutor) GetExecution(sig solana.Signature) (*ExecutionResult, bool) {
	if value, ok := e.executions.Load(sig); ok {
		if exec, ok := value.(*executionContext); ok {
			return exec.result, true
		}
	}
	return nil, false
}

// AddCallback adds a callback for execution status changes
func (e *AsyncTradeExecutor) AddCallback(callback ExecutionCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = append(e.callbacks, callback)
}

// RemoveCallback removes a callback (not implemented - callbacks are append-only)
func (e *AsyncTradeExecutor) RemoveCallback(callback ExecutionCallback) {
	// Callback removal not supported for simplicity
}

// GetPendingCount returns the number of pending executions
func (e *AsyncTradeExecutor) GetPendingCount() int {
	count := 0
	e.executions.Range(func(key, value interface{}) bool {
		if exec, ok := value.(*executionContext); ok {
			if !exec.result.Status.IsTerminal() {
				count++
			}
		}
		return true
	})
	return count
}

// CancelExecution cancels a pending execution
func (e *AsyncTradeExecutor) CancelExecution(sig solana.Signature) bool {
	if value, ok := e.executions.Load(sig); ok {
		if exec, ok := value.(*executionContext); ok {
			if !exec.result.Status.IsTerminal() {
				exec.cancel()
				e.updateStatus(exec, ExecutionStatusCancelled, fmt.Errorf("execution cancelled by user"))
				return true
			}
		}
	}
	return false
}
