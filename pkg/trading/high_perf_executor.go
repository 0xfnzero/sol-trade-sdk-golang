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

// HighPerfTradeConfig represents configuration for high-performance trade execution
type HighPerfTradeConfig struct {
	RPCUrl                 string
	SWQoSConfigs           []soltradesdk.SwqosConfig
	GasFeeStrategy         *soltradesdk.GasFeeStrategy
	MaxWorkers             int
	ConfirmationTimeoutMs  int
	ConfirmationRetryCount int
	RateLimitPerSecond     float64
}

// HighPerfTradeResult represents the result of a high-performance trade execution
type HighPerfTradeResult struct {
	Signature          string
	Success            bool
	Error              error
	ConfirmationTimeMs int64
	SubmittedAt        *time.Time
	ConfirmedAt        *time.Time
	SWQoSType          soltradesdk.SwqosType
	Retries            int
}

// BatchTradeResult represents the result of a batch trade execution
type BatchTradeResult struct {
	Results      []*HighPerfTradeResult
	TotalTimeMs  int64
	SuccessCount int
	FailedCount  int
}

// HighPerfExecuteOptions represents options for high-performance trade execution
type HighPerfExecuteOptions struct {
	WaitConfirmation bool
	MaxRetries       int
	RetryDelayMs     int
	ParallelSubmit   bool
	TimeoutMs        int
	Priority         int
	SkipPreflight    bool
}

// DefaultHighPerfExecuteOptions returns default execution options
func DefaultHighPerfExecuteOptions() *HighPerfExecuteOptions {
	return &HighPerfExecuteOptions{
		WaitConfirmation: true,
		MaxRetries:       3,
		RetryDelayMs:     100,
		ParallelSubmit:   true,
		TimeoutMs:        30000,
		Priority:         0,
		SkipPreflight:    false,
	}
}

// HighPerfTradeExecutor implements parallel SWQoS submission with advanced optimization
type HighPerfTradeExecutor struct {
	config     *HighPerfTradeConfig
	rpcClient  *rpc.Client
	clients    map[soltradesdk.SwqosType]soltradesdk.SwqosClient
	gasStrategy *soltradesdk.GasFeeStrategy

	// Worker pool and rate limiting
	workerPool  chan struct{}
	rateLimiter *RateLimiter

	// Caches
	blockhashCache *BlockhashCache
	signatureCache *SignatureCache

	// Metrics
	totalTrades      atomic.Int64
	successfulTrades atomic.Int64
	failedTrades     atomic.Int64
	totalLatencyMs   atomic.Int64

	mu sync.RWMutex
}

// BlockhashCache caches recent blockhashes
type BlockhashCache struct {
	mu        sync.RWMutex
	blockhash solana.Hash
	timestamp time.Time
	ttl       time.Duration
}

// NewBlockhashCache creates a new blockhash cache
func NewBlockhashCache(ttl time.Duration) *BlockhashCache {
	return &BlockhashCache{
		ttl: ttl,
	}
}

// Get retrieves a cached blockhash
func (c *BlockhashCache) Get() (solana.Hash, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if time.Since(c.timestamp) > c.ttl {
		return solana.Hash{}, false
	}
	return c.blockhash, true
}

// Set stores a blockhash in cache
func (c *BlockhashCache) Set(blockhash solana.Hash) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blockhash = blockhash
	c.timestamp = time.Now()
}

// SignatureCache caches trade results by signature
type SignatureCache struct {
	mu    sync.RWMutex
	cache map[string]*HighPerfTradeResult
	max   int
}

// NewSignatureCache creates a new signature cache
func NewSignatureCache(max int) *SignatureCache {
	return &SignatureCache{
		cache: make(map[string]*HighPerfTradeResult),
		max:   max,
	}
}

// Get retrieves a cached result
func (c *SignatureCache) Get(signature string) (*HighPerfTradeResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.cache[signature]
	return result, ok
}

// Set stores a result in cache
func (c *SignatureCache) Set(signature string, result *HighPerfTradeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= c.max {
		// Simple eviction: remove a random entry
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[signature] = result
}

// NewHighPerfTradeExecutor creates a new high-performance trade executor
func NewHighPerfTradeExecutor(config *HighPerfTradeConfig) (*HighPerfTradeExecutor, error) {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 10
	}
	if config.ConfirmationTimeoutMs <= 0 {
		config.ConfirmationTimeoutMs = 30000
	}
	if config.ConfirmationRetryCount <= 0 {
		config.ConfirmationRetryCount = 30
	}
	if config.RateLimitPerSecond <= 0 {
		config.RateLimitPerSecond = 100.0
	}

	executor := &HighPerfTradeExecutor{
		config:      config,
		rpcClient:   rpc.New(config.RPCUrl),
		clients:     make(map[soltradesdk.SwqosType]soltradesdk.SwqosClient),
		workerPool:  make(chan struct{}, config.MaxWorkers),
		rateLimiter: NewRateLimiter(int(1000 / config.RateLimitPerSecond)),
		blockhashCache: NewBlockhashCache(2 * time.Second),
		signatureCache: NewSignatureCache(1000),
	}

	// Initialize gas strategy
	if config.GasFeeStrategy != nil {
		executor.gasStrategy = config.GasFeeStrategy
	}

	// Initialize SWQoS clients
	for _, swqosConfig := range config.SWQoSConfigs {
		client, err := soltradesdk.NewSwqosClient(swqosConfig, config.RPCUrl)
		if err != nil {
			continue
		}
		executor.clients[swqosConfig.Type] = client
	}

	return executor, nil
}

// AddClient adds a new SWQoS client
func (e *HighPerfTradeExecutor) AddClient(config soltradesdk.SwqosConfig) error {
	client, err := soltradesdk.NewSwqosClient(config, e.config.RPCUrl)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.clients[config.Type] = client
	e.mu.Unlock()
	return nil
}

// RemoveClient removes a SWQoS client
func (e *HighPerfTradeExecutor) RemoveClient(swqosType soltradesdk.SwqosType) {
	e.mu.Lock()
	delete(e.clients, swqosType)
	e.mu.Unlock()
}

// GetClient returns a specific SWQoS client
func (e *HighPerfTradeExecutor) GetClient(swqosType soltradesdk.SwqosType) soltradesdk.SwqosClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.clients[swqosType]
}

// Execute executes a trade transaction
func (e *HighPerfTradeExecutor) Execute(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transaction []byte,
	opts *HighPerfExecuteOptions,
) *HighPerfTradeResult {
	if opts == nil {
		opts = DefaultHighPerfExecuteOptions()
	}

	e.mu.RLock()
	clientCount := len(e.clients)
	e.mu.RUnlock()

	if clientCount == 0 {
		return &HighPerfTradeResult{
			Success: false,
			Error:   fmt.Errorf("no SWQoS clients configured"),
		}
	}

	// Rate limit
	e.rateLimiter.Wait()

	if opts.ParallelSubmit {
		return e.executeParallel(ctx, tradeType, transaction, opts)
	}
	return e.executeSequential(ctx, tradeType, transaction, opts)
}

// executeParallel submits to all clients in parallel
func (e *HighPerfTradeExecutor) executeParallel(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transaction []byte,
	opts *HighPerfExecuteOptions,
) *HighPerfTradeResult {
	start := time.Now()

	resultChan := make(chan *HighPerfTradeResult, len(e.clients))
	var wg sync.WaitGroup

	e.mu.RLock()
	clients := make([]soltradesdk.SwqosClient, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	e.mu.RUnlock()

	for _, client := range clients {
		wg.Add(1)
		go func(c soltradesdk.SwqosClient) {
			defer wg.Done()
			result := e.submitToClient(ctx, c, tradeType, transaction, opts)
			resultChan <- result
		}(client)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Wait for first success or collect all failures
	for result := range resultChan {
		if result.Success {
			e.recordSuccess(time.Since(start))
			return result
		}
	}

	e.recordFailure()
	return &HighPerfTradeResult{
		Success:            false,
		Error:              fmt.Errorf("all parallel submissions failed"),
		ConfirmationTimeMs: time.Since(start).Milliseconds(),
	}
}

// executeSequential submits to clients one by one
func (e *HighPerfTradeExecutor) executeSequential(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transaction []byte,
	opts *HighPerfExecuteOptions,
) *HighPerfTradeResult {
	start := time.Now()

	for retry := 0; retry < opts.MaxRetries; retry++ {
		e.mu.RLock()
		clients := make([]soltradesdk.SwqosClient, 0, len(e.clients))
		for _, client := range e.clients {
			clients = append(clients, client)
		}
		e.mu.RUnlock()

		for _, client := range clients {
			result := e.submitToClient(ctx, client, tradeType, transaction, opts)
			if result.Success {
				e.recordSuccess(time.Since(start))
				return result
			}
		}

		if retry < opts.MaxRetries-1 {
			time.Sleep(time.Duration(opts.RetryDelayMs) * time.Millisecond)
		}
	}

	e.recordFailure()
	return &HighPerfTradeResult{
		Success:            false,
		Error:              fmt.Errorf("all submissions failed after %d retries", opts.MaxRetries),
		ConfirmationTimeMs: time.Since(start).Milliseconds(),
		Retries:            opts.MaxRetries,
	}
}

// submitToClient submits a transaction to a single client
func (e *HighPerfTradeExecutor) submitToClient(
	ctx context.Context,
	client soltradesdk.SwqosClient,
	tradeType soltradesdk.TradeType,
	transaction []byte,
	opts *HighPerfExecuteOptions,
) *HighPerfTradeResult {
	start := time.Now()

	sig, err := client.SendTransaction(ctx, tradeType, transaction, opts.WaitConfirmation)
	if err != nil {
		return &HighPerfTradeResult{
			Success:            false,
			Error:              err,
			ConfirmationTimeMs: time.Since(start).Milliseconds(),
			SWQoSType:          client.GetSwqosType(),
		}
	}

	result := &HighPerfTradeResult{
		Signature:          sig.String(),
		Success:            true,
		SubmittedAt:        &start,
		ConfirmationTimeMs: time.Since(start).Milliseconds(),
		SWQoSType:          client.GetSwqosType(),
	}
	confirmedAt := time.Now()
	result.ConfirmedAt = &confirmedAt

	// Cache result
	e.signatureCache.Set(sig.String(), result)

	return result
}

// ExecuteBatch executes multiple transactions
func (e *HighPerfTradeExecutor) ExecuteBatch(
	ctx context.Context,
	tradeType soltradesdk.TradeType,
	transactions [][]byte,
	opts *HighPerfExecuteOptions,
) *BatchTradeResult {
	if opts == nil {
		opts = DefaultHighPerfExecuteOptions()
	}

	start := time.Now()
	results := make([]*HighPerfTradeResult, len(transactions))
	var wg sync.WaitGroup

	for i, tx := range transactions {
		wg.Add(1)
		go func(idx int, txBytes []byte) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, tradeType, txBytes, opts)
		}(i, tx)
	}

	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	return &BatchTradeResult{
		Results:      results,
		TotalTimeMs:  time.Since(start).Milliseconds(),
		SuccessCount: successCount,
		FailedCount:  len(results) - successCount,
	}
}

// GetGasConfig returns gas configuration for a specific scenario
func (e *HighPerfTradeExecutor) GetGasConfig(
	swqosType soltradesdk.SwqosType,
	tradeType soltradesdk.TradeType,
	strategyType soltradesdk.GasFeeStrategyType,
) map[string]interface{} {
	if e.gasStrategy == nil {
		return map[string]interface{}{
			"cu_limit":  uint64(200000),
			"cu_price":  uint64(100000),
			"tip":       0.001,
		}
	}

	value, ok := e.gasStrategy.Get(swqosType, tradeType, strategyType)
	if !ok {
		return map[string]interface{}{
			"cu_limit":  uint64(200000),
			"cu_price":  uint64(100000),
			"tip":       0.001,
		}
	}

	return map[string]interface{}{
		"cu_limit":  uint64(value.CuLimit),
		"cu_price":  value.CuPrice,
		"tip":       value.Tip,
	}
}

// recordSuccess records a successful trade
func (e *HighPerfTradeExecutor) recordSuccess(latency time.Duration) {
	e.totalTrades.Add(1)
	e.successfulTrades.Add(1)
	e.totalLatencyMs.Add(latency.Milliseconds())
}

// recordFailure records a failed trade
func (e *HighPerfTradeExecutor) recordFailure() {
	e.totalTrades.Add(1)
	e.failedTrades.Add(1)
}

// GetMetrics returns executor metrics
func (e *HighPerfTradeExecutor) GetMetrics() map[string]interface{} {
	total := e.totalTrades.Load()
	successful := e.successfulTrades.Load()
	failed := e.failedTrades.Load()
	totalLatency := e.totalLatencyMs.Load()

	var avgLatency float64
	if successful > 0 {
		avgLatency = float64(totalLatency) / float64(successful)
	}

	var successRate float64
	if total > 0 {
		successRate = float64(successful) / float64(total)
	}

	e.mu.RLock()
	clientCount := len(e.clients)
	e.mu.RUnlock()

	return map[string]interface{}{
		"total_trades":     total,
		"successful_trades": successful,
		"failed_trades":    failed,
		"success_rate":     successRate,
		"avg_latency_ms":   avgLatency,
		"clients_count":    clientCount,
	}
}

// Close releases all resources
func (e *HighPerfTradeExecutor) Close() {
	// Close worker pool
	close(e.workerPool)
}

// CreateHighPerfTradeExecutor creates a high-performance trade executor with specified SWQoS types
func CreateHighPerfTradeExecutor(
	rpcURL string,
	swqosTypes []soltradesdk.SwqosType,
	apiKeys map[soltradesdk.SwqosType]string,
) (*HighPerfTradeExecutor, error) {
	configs := make([]soltradesdk.SwqosConfig, len(swqosTypes))
	for i, swqosType := range swqosTypes {
		configs[i] = soltradesdk.SwqosConfig{
			Type:   swqosType,
			ApiKey: apiKeys[swqosType],
		}
	}

	config := &HighPerfTradeConfig{
		RPCUrl:       rpcURL,
		SWQoSConfigs: configs,
	}

	return NewHighPerfTradeExecutor(config)
}
