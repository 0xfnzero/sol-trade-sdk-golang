package hotpath

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== HotPathConfig Tests =====

func TestDefaultHotPathConfig(t *testing.T) {
	config := DefaultHotPathConfig()

	assert.Equal(t, 2*time.Second, config.BlockhashRefreshInterval)
	assert.Equal(t, 5*time.Second, config.CacheTTL)
	assert.True(t, config.EnablePrefetch)
}

// ===== PrefetchedData Tests =====

func TestPrefetchedDataIsFresh(t *testing.T) {
	// Fresh data
	data := &PrefetchedData{
		Blockhash:       solana.NewHash(),
		LastValidHeight: 100,
		FetchedAt:       time.Now(),
	}
	assert.True(t, time.Since(data.FetchedAt) <= 5*time.Second)

	// Stale data
	staleData := &PrefetchedData{
		FetchedAt: time.Now().Add(-10 * time.Second),
	}
	assert.False(t, time.Since(staleData.FetchedAt) <= 5*time.Second)
}

// ===== HotPathState Tests =====

func TestHotPathStateGetBlockhash(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	// No data yet
	_, _, ok := state.GetBlockhash()
	assert.False(t, ok)

	// Set data manually
	hash := solana.NewHash()
	data := &PrefetchedData{
		Blockhash:       &hash,
		LastValidHeight: 100,
		FetchedAt:       time.Now(),
	}
	state.currentData.Store(data)

	// Should return data
	retrievedHash, lastValid, ok := state.GetBlockhash()
	assert.True(t, ok)
	assert.Equal(t, &hash, retrievedHash)
	assert.Equal(t, uint64(100), lastValid)
}

func TestHotPathStateIsDataFresh(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	// No data
	assert.False(t, state.IsDataFresh())

	// Fresh data
	hash := solana.NewHash()
	data := &PrefetchedData{
		Blockhash:       &hash,
		LastValidHeight: 100,
		FetchedAt:       time.Now(),
	}
	state.currentData.Store(data)
	assert.True(t, state.IsDataFresh())

	// Stale data
	staleData := &PrefetchedData{
		FetchedAt: time.Now().Add(-10 * time.Second),
	}
	state.currentData.Store(staleData)
	assert.False(t, state.IsDataFresh())
}

func TestHotPathStateMetrics(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	count, errors, _ := state.GetMetrics()
	assert.Equal(t, uint64(0), count)
	assert.Equal(t, uint64(0), errors)
}

// ===== AccountStateCache Tests =====

func TestAccountStateCache(t *testing.T) {
	cache := NewAccountStateCache(DefaultHotPathConfig())

	pubkey := solana.NewWallet().PublicKey()
	data := []byte("test_account_data")

	// Update cache
	cache.Update(pubkey, data, 1000000, solana.SystemProgramID, false, 100)

	// Get from cache
	state, ok := cache.Get(pubkey)
	require.True(t, ok)
	assert.Equal(t, data, state.Data)
	assert.Equal(t, uint64(1000000), state.Lamports)
}

func TestAccountStateCacheStale(t *testing.T) {
	config := &HotPathConfig{CacheTTL: 1 * time.Second}
	cache := NewAccountStateCache(config)

	pubkey := solana.NewWallet().PublicKey()
	cache.Update(pubkey, []byte("data"), 1000, solana.SystemProgramID, false, 100)

	// Should be fresh
	_, ok := cache.Get(pubkey)
	assert.True(t, ok)

	// Wait for TTL
	time.Sleep(1100 * time.Millisecond)

	// Should be stale
	_, ok = cache.Get(pubkey)
	assert.False(t, ok)
}

func TestAccountStateCacheMultiple(t *testing.T) {
	cache := NewAccountStateCache(DefaultHotPathConfig())

	pubkeys := make([]solana.PublicKey, 3)
	for i := 0; i < 3; i++ {
		pubkeys[i] = solana.NewWallet().PublicKey()
		cache.Update(pubkeys[i], []byte("data"), uint64(i*1000), solana.SystemProgramID, false, 100)
	}

	states, missing := cache.GetMultiple(pubkeys)
	assert.Len(t, missing, 0)
	for i, state := range states {
		assert.NotNil(t, state)
		assert.Equal(t, uint64(i*1000), state.Lamports)
	}
}

// ===== PoolStateCache Tests =====

func TestPoolStateCache(t *testing.T) {
	cache := NewPoolStateCache()

	poolAddr := solana.NewWallet().PublicKey()
	state := &PoolState{
		PoolAddress:  poolAddr,
		ReserveA:     1000,
		ReserveB:     2000,
	}

	cache.Update(poolAddr, state)

	retrieved, ok := cache.Get(poolAddr)
	require.True(t, ok)
	assert.Equal(t, uint64(1000), retrieved.ReserveA)
}

func TestPoolStateCacheRawData(t *testing.T) {
	cache := NewPoolStateCache()

	poolAddr := solana.NewWallet().PublicKey()
	rawData := []byte("raw_pool_data")

	cache.UpdateRawData(poolAddr, rawData)

	retrieved, ok := cache.GetRawData(poolAddr)
	require.True(t, ok)
	assert.Equal(t, rawData, retrieved)
}

// ===== TradingContext Tests =====

func TestTradingContext(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	// Set blockhash
	hash := solana.NewHash()
	data := &PrefetchedData{
		Blockhash:       &hash,
		LastValidHeight: 100,
		FetchedAt:       time.Now(),
	}
	state.currentData.Store(data)

	payer := solana.NewWallet().PublicKey()
	ctx, err := NewTradingContext(state, nil, nil, payer)
	require.NoError(t, err)

	assert.Equal(t, payer, ctx.Payer)
	assert.Equal(t, &hash, ctx.Blockhash)
	assert.Equal(t, uint64(100), ctx.LastValidHeight)
}

func TestTradingContextStaleBlockhash(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	payer := solana.NewWallet().PublicKey()
	_, err := NewTradingContext(state, nil, nil, payer)
	assert.ErrorIs(t, err, ErrStaleBlockhash)
}

func TestTradingContextAge(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	hash := solana.NewHash()
	data := &PrefetchedData{
		Blockhash:       &hash,
		LastValidHeight: 100,
		FetchedAt:       time.Now(),
	}
	state.currentData.Store(data)

	payer := solana.NewWallet().PublicKey()
	ctx, _ := NewTradingContext(state, nil, nil, payer)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, ctx.Age() >= 100*time.Millisecond)
}

// ===== RateLimiter Tests =====

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(100) // 100ms

	start := time.Now()
	limiter.Wait()
	limiter.Wait()
	elapsed := time.Since(start)

	assert.True(t, elapsed >= 100*time.Millisecond)
}

// ===== MetricsCollector Tests =====

func TestMetricsCollector(t *testing.T) {
	collector := &MetricsCollector{}

	collector.RecordTrade(true, 100)
	collector.RecordTrade(true, 200)
	collector.RecordTrade(false, 50)

	total, success, failed, avgLatency := collector.GetStats()
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(2), success)
	assert.Equal(t, int64(1), failed)
	assert.Equal(t, float64(350)/3, avgLatency)
}

// ===== Concurrent Access Tests =====

func TestHotPathStateConcurrentAccess(t *testing.T) {
	state := NewHotPathState(nil, DefaultHotPathConfig())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hash := solana.NewHash()
			data := &PrefetchedData{
				Blockhash:       &hash,
				LastValidHeight: uint64(i),
				FetchedAt:       time.Now(),
			}
			state.currentData.Store(data)
			state.GetBlockhash()
			state.IsDataFresh()
		}()
	}
	wg.Wait()
}

func TestAccountStateCacheConcurrentAccess(t *testing.T) {
	cache := NewAccountStateCache(DefaultHotPathConfig())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			pubkey := solana.NewWallet().PublicKey()
			cache.Update(pubkey, []byte("data"), uint64(i), solana.SystemProgramID, false, 100)
		}(i)
		go func() {
			defer wg.Done()
			cache.Get(solana.NewWallet().PublicKey())
		}()
	}
	wg.Wait()
}

// ===== Mock SWQoS Client for Testing =====

type MockSwqosClient struct {
	swqosType    string
	sendTxFunc   func(ctx context.Context, tradeType string, txBytes []byte) (solana.Signature, error)
}

func (m *MockSwqosClient) SendTransaction(ctx context.Context, tradeType interface{}, txBytes []byte, skipPreflight bool) (solana.Signature, error) {
	if m.sendTxFunc != nil {
		return m.sendTxFunc(ctx, tradeType.(string), txBytes)
	}
	return solana.NewSignature(), nil
}

func (m *MockSwqosClient) GetSwqosType() interface{} {
	return m.swqosType
}
