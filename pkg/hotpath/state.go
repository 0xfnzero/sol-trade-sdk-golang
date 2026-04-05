package hotpath

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// HotPathConfig configures the hot path executor
type HotPathConfig struct {
	// Blockhash refresh interval
	BlockhashRefreshInterval time.Duration
	// Prefetch enabled
	EnablePrefetch bool
	// Cache TTL for prefetched data
	CacheTTL time.Duration
}

// DefaultHotPathConfig returns default configuration
func DefaultHotPathConfig() *HotPathConfig {
	return &HotPathConfig{
		BlockhashRefreshInterval: 2 * time.Second,
		EnablePrefetch:           true,
		CacheTTL:                 5 * time.Second,
	}
}

// PrefetchedData holds all data needed for hot path execution
type PrefetchedData struct {
	Blockhash       *solana.Hash
	LastValidHeight uint64
	FetchedAt       time.Time
	Slot            uint64
}

// HotPathState manages pre-fetched state for hot path execution
// NO RPC calls are made during trading execution
type HotPathState struct {
	config *HotPathConfig

	// Prefetched data - accessed atomically
	currentData atomic.Value // *PrefetchedData

	// Background prefetch control
	prefetchCtx    context.Context
	prefetchCancel context.CancelFunc
	prefetchWg     sync.WaitGroup

	// RPC client for background prefetching only
	rpcClient *rpc.Client

	// Callbacks for state updates
	onBlockhashUpdate func(hash *solana.Hash, lastValidHeight uint64)

	// Metrics
	mu               sync.Mutex
	prefetchCount    uint64
	prefetchErrors   uint64
	lastPrefetchTime time.Time
}

// NewHotPathState creates a new hot path state manager
func NewHotPathState(rpcClient *rpc.Client, config *HotPathConfig) *HotPathState {
	if config == nil {
		config = DefaultHotPathConfig()
	}

	hps := &HotPathState{
		config:    config,
		rpcClient: rpcClient,
	}

	// Initialize with empty data
	hps.currentData.Store(&PrefetchedData{
		FetchedAt: time.Time{}, // Zero time indicates no data
	})

	return hps
}

// Start begins background prefetching
// This should be called BEFORE any hot path execution
func (h *HotPathState) Start(ctx context.Context) error {
	if !h.config.EnablePrefetch {
		return nil
	}

	h.prefetchCtx, h.prefetchCancel = context.WithCancel(ctx)

	// Initial prefetch synchronously
	if err := h.prefetchBlockhash(); err != nil {
		return err
	}

	// Start background prefetch loop
	h.prefetchWg.Add(1)
	go h.prefetchLoop()

	return nil
}

// Stop stops background prefetching
func (h *HotPathState) Stop() {
	if h.prefetchCancel != nil {
		h.prefetchCancel()
	}
	h.prefetchWg.Wait()
}

// prefetchLoop runs in background to keep data fresh
func (h *HotPathState) prefetchLoop() {
	defer h.prefetchWg.Done()

	ticker := time.NewTicker(h.config.BlockhashRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.prefetchCtx.Done():
			return
		case <-ticker.C:
			_ = h.prefetchBlockhash()
		}
	}
}

// prefetchBlockhash fetches latest blockhash - called ONLY in background
func (h *HotPathState) prefetchBlockhash() error {
	ctx, cancel := context.WithTimeout(h.prefetchCtx, 5*time.Second)
	defer cancel()

	result, err := h.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentProcessed)
	if err != nil {
		h.mu.Lock()
		h.prefetchErrors++
		h.mu.Unlock()
		return err
	}

	data := &PrefetchedData{
		Blockhash:       &result.Value.Blockhash,
		LastValidHeight: result.Value.LastValidBlockHeight,
		FetchedAt:       time.Now(),
		Slot:            result.Context.Slot,
	}

	h.currentData.Store(data)

	h.mu.Lock()
	h.prefetchCount++
	h.lastPrefetchTime = time.Now()
	h.mu.Unlock()

	if h.onBlockhashUpdate != nil {
		h.onBlockhashUpdate(data.Blockhash, data.LastValidHeight)
	}

	return nil
}

// GetBlockhash returns the current cached blockhash - NO RPC CALL
// This is safe to call in hot path
func (h *HotPathState) GetBlockhash() (*solana.Hash, uint64, bool) {
	data := h.currentData.Load().(*PrefetchedData)
	if data == nil || data.Blockhash == nil || data.FetchedAt.IsZero() {
		return nil, 0, false
	}

	// Check if data is stale
	if time.Since(data.FetchedAt) > h.config.CacheTTL {
		return nil, 0, false
	}

	return data.Blockhash, data.LastValidHeight, true
}

// GetPrefetchedData returns all prefetched data - NO RPC CALL
func (h *HotPathState) GetPrefetchedData() *PrefetchedData {
	return h.currentData.Load().(*PrefetchedData)
}

// IsDataFresh checks if prefetched data is still valid
func (h *HotPathState) IsDataFresh() bool {
	data := h.currentData.Load().(*PrefetchedData)
	if data == nil || data.FetchedAt.IsZero() {
		return false
	}
	return time.Since(data.FetchedAt) <= h.config.CacheTTL
}

// OnBlockhashUpdate sets callback for blockhash updates
func (h *HotPathState) OnBlockhashUpdate(fn func(hash *solana.Hash, lastValidHeight uint64)) {
	h.onBlockhashUpdate = fn
}

// GetMetrics returns prefetch metrics
func (h *HotPathState) GetMetrics() (count, errors uint64, lastTime time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.prefetchCount, h.prefetchErrors, h.lastPrefetchTime
}

// ===== Account State Cache for Hot Path =====

// AccountState holds cached account state
type AccountState struct {
	Pubkey    solana.PublicKey
	Data      []byte
	Lamports  uint64
	Owner     solana.PublicKey
	Executable bool
	RentEpoch uint64
	FetchedAt time.Time
	Slot      uint64
}

// AccountStateCache caches account states for hot path access
type AccountStateCache struct {
	mu       sync.RWMutex
	accounts map[string]*AccountState
	config   *HotPathConfig
}

// NewAccountStateCache creates a new account state cache
func NewAccountStateCache(config *HotPathConfig) *AccountStateCache {
	return &AccountStateCache{
		accounts: make(map[string]*AccountState),
		config:   config,
	}
}

// Update updates an account state in cache
func (c *AccountStateCache) Update(pubkey solana.PublicKey, data []byte, lamports uint64, owner solana.PublicKey, executable bool, slot uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accounts[pubkey.String()] = &AccountState{
		Pubkey:     pubkey,
		Data:       data,
		Lamports:   lamports,
		Owner:      owner,
		Executable: executable,
		FetchedAt:  time.Now(),
		Slot:       slot,
	}
}

// Get retrieves account state from cache - NO RPC CALL
func (c *AccountStateCache) Get(pubkey solana.PublicKey) (*AccountState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, ok := c.accounts[pubkey.String()]
	if !ok {
		return nil, false
	}

	// Check freshness
	if time.Since(state.FetchedAt) > c.config.CacheTTL {
		return nil, false
	}

	return state, true
}

// GetMultiple retrieves multiple account states - NO RPC CALL
func (c *AccountStateCache) GetMultiple(pubkeys []solana.PublicKey) ([]*AccountState, []int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	states := make([]*AccountState, len(pubkeys))
	missing := make([]int, 0)

	for i, pubkey := range pubkeys {
		state, ok := c.accounts[pubkey.String()]
		if !ok || time.Since(state.FetchedAt) > c.config.CacheTTL {
			missing = append(missing, i)
		} else {
			states[i] = state
		}
	}

	return states, missing
}

// PrefetchAccounts fetches accounts in background
func (c *AccountStateCache) PrefetchAccounts(ctx context.Context, client *rpc.Client, pubkeys []solana.PublicKey) error {
	if len(pubkeys) == 0 {
		return nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.GetMultipleAccountsWithOpts(
		fetchCtx,
		pubkeys,
		&rpc.GetMultipleAccountsOpts{
			Commitment: rpc.CommitmentProcessed,
		},
	)
	if err != nil {
		return err
	}

	for i, account := range result.Value {
		if account == nil {
			continue
		}
		c.Update(pubkeys[i], account.Data.GetBinary(), account.Lamports, account.Owner, account.Executable, result.Context.Slot)
	}

	return nil
}

// ===== Protocol State Cache =====

// PoolState represents cached DEX pool state
type PoolState struct {
	PoolAddress  solana.PublicKey
	AMMId        uint64
	BondingCurve solana.PublicKey
	VaultA       solana.PublicKey
	VaultB       solana.PublicKey
	MintA        solana.PublicKey
	MintB        solana.PublicKey
	ReserveA     uint64
	ReserveB     uint64
	FetchedAt    time.Time
}

// PoolStateCache caches DEX pool states
type PoolStateCache struct {
	mu    sync.RWMutex
	pools map[string]*PoolState

	// Decoded pool data
	poolData map[string][]byte
}

// NewPoolStateCache creates a new pool state cache
func NewPoolStateCache() *PoolStateCache {
	return &PoolStateCache{
		pools:    make(map[string]*PoolState),
		poolData: make(map[string][]byte),
	}
}

// Update updates a pool state
func (c *PoolStateCache) Update(poolAddr solana.PublicKey, state *PoolState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state.FetchedAt = time.Now()
	c.pools[poolAddr.String()] = state
}

// Get retrieves pool state - NO RPC CALL
func (c *PoolStateCache) Get(poolAddr solana.PublicKey) (*PoolState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, ok := c.pools[poolAddr.String()]
	if !ok {
		return nil, false
	}

	return state, true
}

// UpdateRawData updates raw pool data
func (c *PoolStateCache) UpdateRawData(poolAddr solana.PublicKey, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.poolData[poolAddr.String()] = data
}

// GetRawData retrieves raw pool data - NO RPC CALL
func (c *PoolStateCache) GetRawData(poolAddr solana.PublicKey) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.poolData[poolAddr.String()]
	return data, ok
}

// ===== Trading Context =====

// TradingContext holds all state needed for a hot path trade
// Everything is pre-fetched - NO RPC CALLS during trade execution
type TradingContext struct {
	// Blockhash for transaction
	Blockhash       *solana.Hash
	LastValidHeight uint64

	// Account states
	TokenAccounts  map[string]*AccountState
	PoolStates     map[string]*PoolState

	// Creation time for timeout checks
	CreatedAt time.Time

	// Payer public key
	Payer solana.PublicKey
}

// NewTradingContext creates a new trading context from prefetched data
func NewTradingContext(
	hotPathState *HotPathState,
	accountCache *AccountStateCache,
	poolCache *PoolStateCache,
	payer solana.PublicKey,
) (*TradingContext, error) {
	blockhash, lastValidHeight, ok := hotPathState.GetBlockhash()
	if !ok {
		return nil, ErrStaleBlockhash
	}

	return &TradingContext{
		Blockhash:       blockhash,
		LastValidHeight: lastValidHeight,
		TokenAccounts:   make(map[string]*AccountState),
		PoolStates:      make(map[string]*PoolState),
		CreatedAt:       time.Now(),
		Payer:           payer,
	}, nil
}

// AddTokenAccount adds a token account state to context
func (tc *TradingContext) AddTokenAccount(pubkey solana.PublicKey, cache *AccountStateCache) bool {
	state, ok := cache.Get(pubkey)
	if ok {
		tc.TokenAccounts[pubkey.String()] = state
	}
	return ok
}

// AddPool adds a pool state to context
func (tc *TradingContext) AddPool(pubkey solana.PublicKey, cache *PoolStateCache) bool {
	state, ok := cache.Get(pubkey)
	if ok {
		tc.PoolStates[pubkey.String()] = state
	}
	return ok
}

// IsValidForSlot checks if context is still valid
func (tc *TradingContext) IsValidForSlot(currentSlot uint64, blockhashExpirySlots uint64) bool {
	// Check if our blockhash is still valid based on last valid height
	return currentSlot < tc.LastValidHeight-blockhashExpirySlots
}

// Age returns how old the context is
func (tc *TradingContext) Age() time.Duration {
	return time.Since(tc.CreatedAt)
}

// Errors
var (
	ErrStaleBlockhash = fmt.Errorf("blockhash is stale or not available")
	ErrMissingAccount = fmt.Errorf("required account state not in cache")
	ErrContextExpired = fmt.Errorf("trading context has expired")
)
