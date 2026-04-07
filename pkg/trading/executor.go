package trading

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TradeExecutor handles the execution of trades with parallel SWQOS submissions
type TradeExecutor struct {
	rpcClient     *rpc.Client
	swqosClients  []soltradesdk.SwqosClient
	gasStrategy   *soltradesdk.GasFeeStrategy
	config        *soltradesdk.TradeConfig

	// Confirmation polling settings
	confirmationTimeout time.Duration
	confirmationRetry   int
}

// NewTradeExecutor creates a new trade executor
func NewTradeExecutor(
	rpcClient *rpc.Client,
	config *soltradesdk.TradeConfig,
	gasStrategy *soltradesdk.GasFeeStrategy,
) *TradeExecutor {
	return &TradeExecutor{
		rpcClient:          rpcClient,
		swqosClients:       make([]soltradesdk.SwqosClient, 0),
		gasStrategy:        gasStrategy,
		config:             config,
		confirmationTimeout: 30 * time.Second,
		confirmationRetry:   30,
	}
}

// AddSwqosClient adds a SWQOS client to the executor
func (e *TradeExecutor) AddSwqosClient(client soltradesdk.SwqosClient) {
	e.swqosClients = append(e.swqosClients, client)
}

// ExecuteResult represents the result of a trade execution
type ExecuteResult struct {
	Signature       solana.Signature
	Success         bool
	Error           error
	ConfirmationMs  int64
	SubmittedAt     time.Time
	ConfirmedAt     time.Time
	SwqosType       soltradesdk.SwqosType
}

// ExecuteOptions represents options for trade execution
type ExecuteOptions struct {
	WaitConfirmation bool
	MaxRetries        int
	RetryDelayMs      int
	ParallelSubmit    bool
}

// DefaultExecuteOptions returns default execution options
func DefaultExecuteOptions() ExecuteOptions {
	return ExecuteOptions{
		WaitConfirmation: true,
		MaxRetries:       3,
		RetryDelayMs:     100,
		ParallelSubmit:   true,
	}
}

// Execute executes a trade transaction with the given options
func (e *TradeExecutor) Execute(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transaction *solana.Transaction,
	opts ExecuteOptions,
) *ExecuteResult {
	if len(e.swqosClients) == 0 {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("no SWQOS clients configured"),
		}
	}

	// Serialize transaction
	txBytes, err := transaction.MarshalBinary()
	if err != nil {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("failed to serialize transaction: %w", err),
		}
	}

	if opts.ParallelSubmit {
		return e.executeParallel(ctx, tradeType, txBytes, opts)
	}
	return e.executeSequential(ctx, tradeType, txBytes, opts)
}

// executeParallel submits to all SWQOS clients in parallel
func (e *TradeExecutor) executeParallel(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	txBytes []byte,
	opts ExecuteOptions,
) *ExecuteResult {
	resultChan := make(chan *ExecuteResult, len(e.swqosClients))
	var wg sync.WaitGroup

	for _, client := range e.swqosClients {
		wg.Add(1)
		go func(c soltradesdk.SwqosClient) {
			defer wg.Done()
			result := e.submitToClient(ctx, c, tradeType, txBytes, opts)
			resultChan <- result
		}(client)
	}

	// Wait for first success or all failures
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var lastError error
	for result := range resultChan {
		if result.Success {
			// Cancel pending submissions by closing context
			return result
		}
		lastError = result.Error
	}

	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("all parallel submissions failed: %w", lastError),
	}
}

// executeSequential submits to SWQOS clients one by one
func (e *TradeExecutor) executeSequential(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	txBytes []byte,
	opts ExecuteOptions,
) *ExecuteResult {
	for retry := 0; retry < opts.MaxRetries; retry++ {
		for _, client := range e.swqosClients {
			result := e.submitToClient(ctx, client, tradeType, txBytes, opts)
			if result.Success {
				return result
			}
		}

		if retry < opts.MaxRetries-1 {
			time.Sleep(time.Duration(opts.RetryDelayMs) * time.Millisecond)
		}
	}

	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("all sequential submissions failed after %d retries", opts.MaxRetries),
	}
}

// submitToClient submits a transaction to a single SWQOS client
func (e *TradeExecutor) submitToClient(
	ctx context.Context,
	client soltradesdk.SwqosClient,
	tradeType soltradesdk.TradeType,
	txBytes []byte,
	opts ExecuteOptions,
) *ExecuteResult {
	start := time.Now()
	sig, err := client.SendTransaction(ctx, tradeType, txBytes, false)
	if err != nil {
		return &ExecuteResult{
			Success: false,
			Error:   err,
		}
	}

	result := &ExecuteResult{
		Signature:   sig,
		Success:     true,
		SubmittedAt: start,
		SwqosType:   client.GetSwqosType(),
	}

	if opts.WaitConfirmation {
		confirmed, confirmTime := e.waitForConfirmation(ctx, sig)
		result.ConfirmedAt = time.Now()
		result.ConfirmationMs = confirmTime.Milliseconds()
		if !confirmed {
			result.Success = false
			result.Error = fmt.Errorf("transaction failed to confirm")
		}
	}

	return result
}

// waitForConfirmation waits for transaction confirmation
func (e *TradeExecutor) waitForConfirmation(
	ctx context.Context,
	sig solana.Signature,
) (bool, time.Duration) {
	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; i < e.confirmationRetry; i++ {
		select {
		case <-ctx.Done():
			return false, time.Since(start)
		case <-ticker.C:
			status, err := e.rpcClient.GetSignatureStatuses(ctx, true, sig)
			if err != nil {
				continue
			}

			if len(status.Value) > 0 && status.Value[0] != nil {
				if status.Value[0].ConfirmationStatus == rpc.ConfirmationStatusFinalized {
					return true, time.Since(start)
				}
				if status.Value[0].Err != nil {
					return false, time.Since(start)
				}
			}
		}
	}

	return false, time.Since(start)
}

// ExecuteMultiple executes multiple transactions
func (e *TradeExecutor) ExecuteMultiple(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transactions []*solana.Transaction,
	opts ExecuteOptions,
) []*ExecuteResult {
	results := make([]*ExecuteResult, len(transactions))
	var wg sync.WaitGroup

	for i, tx := range transactions {
		wg.Add(1)
		go func(idx int, transaction *solana.Transaction) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, tradeType, transaction, opts)
		}(i, tx)
	}

	wg.Wait()
	return results
}

// ===== Gas Fee Management =====

// GasFeeConfig represents gas fee configuration for a transaction
type GasFeeConfig struct {
	ComputeUnitLimit uint64
	ComputeUnitPrice uint64
	PriorityFee      uint64
}

// GetGasConfig returns gas configuration for a trade type
func (e *TradeExecutor) GetGasConfig(
	swqosType soltradesdk.SwqosType,
	tradeType soltradesdk.TradeType,
	strategyType soltradesdk.GasFeeStrategyType,
) *GasFeeConfig {
	if e.gasStrategy == nil {
		return &GasFeeConfig{
			ComputeUnitLimit: 200000,
			ComputeUnitPrice: 100000,
			PriorityFee:      100000,
		}
	}

	value, ok := e.gasStrategy.Get(swqosType, tradeType, strategyType)
	if !ok {
		return &GasFeeConfig{
			ComputeUnitLimit: 200000,
			ComputeUnitPrice: 100000,
			PriorityFee:      100000,
		}
	}

	return &GasFeeConfig{
		ComputeUnitLimit: uint64(value.CuLimit),
		ComputeUnitPrice: value.CuPrice,
		PriorityFee:      uint64(value.Tip * solana.LAMPORTS_PER_SOL),
	}
}

// ===== Transaction Builder Helper =====

// BuildTransactionOptions represents options for building a transaction
type BuildTransactionOptions struct {
	Payer             solana.PublicKey
	RecentBlockhash   solana.Hash
	Instructions      []solana.Instruction
	Signers           []*solana.PrivateKey
	AddressLookupTables []*solana.AddressLookupTableAccount
	GasConfig         *GasFeeConfig
}

// BuildTransaction builds a transaction from options
func (e *TradeExecutor) BuildTransaction(opts BuildTransactionOptions) (*solana.Transaction, error) {
	// Add compute budget instructions if gas config is provided
	var instructions []solana.Instruction
	if opts.GasConfig != nil {
		instructions = append(instructions,
			solana.NewComputeBudgetSetComputeUnitLimitInstruction(opts.GasConfig.ComputeUnitLimit),
			solana.NewComputeBudgetSetComputeUnitPriceInstruction(opts.GasConfig.ComputeUnitPrice),
		)
	}
	instructions = append(instructions, opts.Instructions...)

	tx, err := solana.NewTransaction(
		instructions,
		opts.RecentBlockhash,
		solana.TransactionPayer(opts.Payer),
		solana.TransactionWithSigners(opts.Signers...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	return tx, nil
}

// ===== Rate Limiting =====

// RateLimiter provides rate limiting for trade execution
type RateLimiter struct {
	lastSubmit atomic.Int64
	minDelay   int64 // nanoseconds
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(minDelayMs int) *RateLimiter {
	return &RateLimiter{
		minDelay: int64(minDelayMs) * int64(time.Millisecond),
	}
}

// Wait blocks until the minimum delay has passed since last submission
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

// ===== Metrics =====

// MetricsCollector collects execution metrics
type MetricsCollector struct {
	mu            sync.Mutex
	totalTrades   int64
	successTrades int64
	failedTrades  int64
	totalLatency  int64
}

// RecordTrade records a trade execution
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

// GetStats returns collected statistics
func (m *MetricsCollector) GetStats() (total, success, failed int64, avgLatencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total = m.totalTrades
	success = m.successTrades
	failed = m.failedTrades
	if m.totalTrades > 0 {
		avgLatencyMs = float64(m.totalLatency) / float64(m.totalTrades)
	}
	return
}
