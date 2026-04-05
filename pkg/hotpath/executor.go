package hotpath

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	soltradesdk "github.com/your-org/sol-trade-sdk-go"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// HotPathExecutor executes trades with ZERO RPC calls in the hot path
// All data must be pre-fetched before execution
type HotPathExecutor struct {
	// State management
	state         *HotPathState
	accountCache  *AccountStateCache
	poolCache     *PoolStateCache
	config        *HotPathConfig

	// SWQoS clients for transaction submission
	swqosClients []soltradesdk.SwqosClient

	// Metrics
	metrics *HotPathMetrics
}

// HotPathMetrics tracks execution metrics
type HotPathMetrics struct {
	TotalTrades   atomic.Int64
	SuccessTrades atomic.Int64
	FailedTrades  atomic.Int64
	TotalLatency  atomic.Int64
	AvgLatency    atomic.Int64
}

// NewHotPathExecutor creates a new hot path executor
func NewHotPathExecutor(rpcClient *rpc.Client, config *HotPathConfig) *HotPathExecutor {
	if config == nil {
		config = DefaultHotPathConfig()
	}

	return &HotPathExecutor{
		state:        NewHotPathState(rpcClient, config),
		accountCache: NewAccountStateCache(config),
		poolCache:    NewPoolStateCache(),
		config:       config,
		metrics:      &HotPathMetrics{},
	}
}

// AddSwqosClient adds a SWQoS client for transaction submission
func (e *HotPathExecutor) AddSwqosClient(client soltradesdk.SwqosClient) {
	e.swqosClients = append(e.swqosClients, client)
}

// Start starts background prefetching
func (e *HotPathExecutor) Start(ctx context.Context) error {
	return e.state.Start(ctx)
}

// Stop stops background prefetching
func (e *HotPathExecutor) Stop() {
	e.state.Stop()
}

// GetState returns the hot path state for external access
func (e *HotPathExecutor) GetState() *HotPathState {
	return e.state
}

// GetAccountCache returns the account cache for prefetching
func (e *HotPathExecutor) GetAccountCache() *AccountStateCache {
	return e.accountCache
}

// GetPoolCache returns the pool cache for prefetching
func (e *HotPathExecutor) GetPoolCache() *PoolStateCache {
	return e.poolCache
}

// ExecuteOptions for hot path execution
type ExecuteOptions struct {
	// Submit to all SWQoS clients in parallel
	ParallelSubmit bool
	// Timeout for submission
	Timeout time.Duration
	// Skip blockhash validation (trust prefetched data)
	SkipBlockhashValidation bool
}

// DefaultExecuteOptions returns default options
func DefaultExecuteOptions() ExecuteOptions {
	return ExecuteOptions{
		ParallelSubmit:          true,
		Timeout:                 10 * time.Second,
		SkipBlockhashValidation: false,
	}
}

// ExecuteResult is the result of hot path execution
type ExecuteResult struct {
	Signature     solana.Signature
	Success       bool
	Error         error
	LatencyMs     int64
	SwqosType     soltradesdk.SwqosType
	BlockhashUsed string
}

// Execute executes a pre-signed transaction - NO RPC CALLS
// Transaction must already be signed with valid blockhash
func (e *HotPathExecutor) Execute(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transaction *solana.Transaction,
	opts ExecuteOptions,
) *ExecuteResult {
	start := time.Now()

	// Validate blockhash is still fresh (no RPC, just check cache age)
	if !opts.SkipBlockhashValidation && !e.state.IsDataFresh() {
		return &ExecuteResult{
			Success: false,
			Error:   ErrStaleBlockhash,
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

	// Submit to SWQoS clients
	var result *ExecuteResult
	if opts.ParallelSubmit && len(e.swqosClients) > 1 {
		result = e.executeParallel(ctx, tradeType, txBytes, opts)
	} else {
		result = e.executeSequential(ctx, tradeType, txBytes, opts)
	}

	result.LatencyMs = time.Since(start).Milliseconds()

	// Update metrics
	e.metrics.TotalTrades.Add(1)
	if result.Success {
		e.metrics.SuccessTrades.Add(1)
	} else {
		e.metrics.FailedTrades.Add(1)
	}
	e.metrics.TotalLatency.Add(result.LatencyMs)

	return result
}

// executeParallel submits to all SWQoS clients in parallel - NO RPC
func (e *HotPathExecutor) executeParallel(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	txBytes []byte,
	opts ExecuteOptions,
) *ExecuteResult {
	resultChan := make(chan *ExecuteResult, len(e.swqosClients))
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	for _, client := range e.swqosClients {
		wg.Add(1)
		go func(c soltradesdk.SwqosClient) {
			defer wg.Done()
			sig, err := c.SendTransaction(ctx, tradeType, txBytes, false)
			if err != nil {
				resultChan <- &ExecuteResult{Success: false, Error: err}
				return
			}
			resultChan <- &ExecuteResult{
				Signature: sig,
				Success:   true,
				SwqosType: c.GetSwqosType(),
			}
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
			return result
		}
		lastError = result.Error
	}

	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("all parallel submissions failed: %w", lastError),
	}
}

// executeSequential submits to SWQoS clients one by one - NO RPC
func (e *HotPathExecutor) executeSequential(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	txBytes []byte,
	opts ExecuteOptions,
) *ExecuteResult {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	for _, client := range e.swqosClients {
		sig, err := client.SendTransaction(ctx, tradeType, txBytes, false)
		if err == nil {
			return &ExecuteResult{
				Signature: sig,
				Success:   true,
				SwqosType: client.GetSwqosType(),
			}
		}
	}

	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("all sequential submissions failed"),
	}
}

// BuildTransaction builds a transaction using prefetched data - NO RPC
func (e *HotPathExecutor) BuildTransaction(
	payer solana.PublicKey,
	instructions []solana.Instruction,
	signers []*solana.PrivateKey,
	gasConfig *GasFeeConfig,
) (*solana.Transaction, error) {
	// Get blockhash from cache - NO RPC
	blockhash, lastValidHeight, ok := e.state.GetBlockhash()
	if !ok {
		return nil, ErrStaleBlockhash
	}

	// Build transaction
	var txInstructions []solana.Instruction

	// Add compute budget instructions if gas config provided
	if gasConfig != nil {
		txInstructions = append(txInstructions,
			solana.NewComputeBudgetSetComputeUnitLimitInstruction(gasConfig.ComputeUnitLimit),
			solana.NewComputeBudgetSetComputeUnitPriceInstruction(gasConfig.ComputeUnitPrice),
		)
	}
	txInstructions = append(txInstructions, instructions...)

	tx, err := solana.NewTransaction(
		txInstructions,
		*blockhash,
		solana.TransactionPayer(payer),
		solana.TransactionWithSigners(signers...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	_ = lastValidHeight // Track for validation

	return tx, nil
}

// GasFeeConfig for gas settings
type GasFeeConfig struct {
	ComputeUnitLimit uint64
	ComputeUnitPrice uint64
}

// CreateTradingContext creates a trading context with prefetched data
func (e *HotPathExecutor) CreateTradingContext(payer solana.PublicKey) (*TradingContext, error) {
	return NewTradingContext(e.state, e.accountCache, e.poolCache, payer)
}

// PrefetchAccounts prefetches accounts for hot path access
// Call this BEFORE trading, not during
func (e *HotPathExecutor) PrefetchAccounts(ctx context.Context, rpcClient *rpc.Client, pubkeys []solana.PublicKey) error {
	return e.accountCache.PrefetchAccounts(ctx, rpcClient, pubkeys)
}

// GetMetrics returns execution metrics
func (e *HotPathExecutor) GetMetrics() (total, success, failed int64, avgLatencyMs float64) {
	total = e.metrics.TotalTrades.Load()
	success = e.metrics.SuccessTrades.Load()
	failed = e.metrics.FailedTrades.Load()
	totalLatency := e.metrics.TotalLatency.Load()
	if total > 0 {
		avgLatencyMs = float64(totalLatency) / float64(total)
	}
	return
}

// IsReady checks if executor is ready for hot path execution
func (e *HotPathExecutor) IsReady() bool {
	return e.state.IsDataFresh() && len(e.swqosClients) > 0
}

// WaitForReady blocks until executor is ready or context is done
func (e *HotPathExecutor) WaitForReady(ctx context.Context, checkInterval time.Duration) error {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if e.IsReady() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}
