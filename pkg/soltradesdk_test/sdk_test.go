package soltradesdk_test

import (
	"sync"
	"testing"
	"time"

	soltradesdk "github.com/your-org/sol-trade-sdk-go"
	"github.com/your-org/sol-trade-sdk-go/pkg/cache"
	"github.com/your-org/sol-trade-sdk-go/pkg/common"
	"github.com/your-org/sol-trade-sdk-go/pkg/pool"
	"github.com/your-org/sol-trade-sdk-go/pkg/utils"
)

// ===== Gas Fee Strategy Tests =====

func TestGasFeeStrategy_Create(t *testing.T) {
	strategy := common.NewGasFeeStrategy()
	if strategy == nil {
		t.Fatal("expected strategy to be created")
	}
}

func TestGasFeeStrategy_SetAndGet(t *testing.T) {
	strategy := common.NewGasFeeStrategy()

	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
		200000, 100000, 0.001,
	)

	value, ok := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	if !ok {
		t.Fatal("expected to get strategy value")
	}

	if value.CuLimit != 200000 {
		t.Errorf("expected CuLimit 200000, got %d", value.CuLimit)
	}

	if value.CuPrice != 100000 {
		t.Errorf("expected CuPrice 100000, got %d", value.CuPrice)
	}

	if value.Tip != 0.001 {
		t.Errorf("expected Tip 0.001, got %f", value.Tip)
	}
}

func TestGasFeeStrategy_Concurrent(t *testing.T) {
	strategy := common.NewGasFeeStrategy()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			strategy.Set(
				soltradesdk.SwqosType(idx%10),
				soltradesdk.TradeTypeBuy,
				common.GasFeeStrategyTypeNormal,
				uint32(idx), uint64(idx), float64(idx)/1000,
			)
		}(i)
	}

	wg.Wait()

	// Verify no race conditions
	value, ok := strategy.Get(soltradesdk.SwqosTypeJito, soltradesdk.TradeTypeBuy, common.GasFeeStrategyTypeNormal)
	if ok && value.CuLimit < 100 {
		t.Error("unexpected concurrent write result")
	}
}

// ===== Bonding Curve Tests =====

func TestBondingCurve_GetBuyPrice(t *testing.T) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
		Complete:             false,
	}

	tokens := curve.GetBuyPrice(1_000_000)
	if tokens == 0 {
		t.Error("expected non-zero token output")
	}
}

func TestBondingCurve_GetSellPrice(t *testing.T) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
		Complete:             false,
	}

	sol := curve.GetSellPrice(1_000_000_000, 100)
	if sol == 0 {
		t.Error("expected non-zero SOL output")
	}
}

func TestBondingCurve_CompleteReturnsZero(t *testing.T) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
		Complete:             true,
	}

	if curve.GetBuyPrice(1_000_000) != 0 {
		t.Error("expected zero tokens for complete curve")
	}

	if curve.GetSellPrice(1_000_000_000, 100) != 0 {
		t.Error("expected zero SOL for complete curve")
	}
}

// ===== Cache Tests =====

func TestLRUCache_Basic(t *testing.T) {
	cache := cache.NewLRUCache(3, time.Minute)

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	if v, ok := cache.Get("a"); !ok || v.(int) != 1 {
		t.Error("expected to get value for 'a'")
	}

	if v, ok := cache.Get("b"); !ok || v.(int) != 2 {
		t.Error("expected to get value for 'b'")
	}

	if v, ok := cache.Get("c"); !ok || v.(int) != 3 {
		t.Error("expected to get value for 'c'")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := cache.NewLRUCache(2, time.Minute)

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // Should evict "a"

	if _, ok := cache.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}

	if v, ok := cache.Get("b"); !ok || v.(int) != 2 {
		t.Error("expected to get value for 'b'")
	}

	if v, ok := cache.Get("c"); !ok || v.(int) != 3 {
		t.Error("expected to get value for 'c'")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	cache := cache.NewLRUCache(10, time.Minute)

	cache.Set("a", 1)
	cache.Get("a") // hit
	cache.Get("b") // miss

	hits, misses, _, size := cache.Stats()

	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}

	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}

	if size != 1 {
		t.Errorf("expected size 1, got %d", size)
	}
}

func TestShardedCache(t *testing.T) {
	cache := cache.NewShardedCache(4, 100, time.Minute)

	for i := 0; i < 100; i++ {
		cache.Set(string(rune(i)), i)
	}

	for i := 0; i < 100; i++ {
		if v, ok := cache.Get(string(rune(i))); !ok || v.(int) != i {
			t.Errorf("expected to get value for key %d", i)
		}
	}
}

// ===== Pool Tests =====

func TestWorkerPool_Submit(t *testing.T) {
	p := pool.NewWorkerPool(4, 100)

	result := p.SubmitWait(func() (interface{}, error) {
		return 42, nil
	})

	if result != 42 {
		t.Errorf("expected result 42, got %v", result)
	}

	p.Close()
}

func TestWorkerPool_Batch(t *testing.T) {
	p := pool.NewWorkerPool(4, 100)

	tasks := make([]pool.Task, 5)
	for i := 0; i < 5; i++ {
		i := i
		tasks[i] = func() (interface{}, error) {
			return i * 2, nil
		}
	}

	results := p.SubmitBatch(tasks)

	for i, result := range results {
		if result.Value.(int) != i*2 {
			t.Errorf("expected result %d, got %v", i*2, result.Value)
		}
	}

	p.Close()
}

func TestRateLimiter(t *testing.T) {
	limiter := pool.NewRateLimiter(100) // 100ms min delay

	// First call should not block
	limiter.Wait()

	// Second call should block
	start := time.Now()
	limiter.Wait()
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond {
		t.Errorf("expected rate limiter to delay, got %v", elapsed)
	}
}

// ===== Utility Tests =====

func TestUtils_LE(t *testing.T) {
	v := uint64(0x0102030405060708)
	b := utils.LE(v)

	if b[0] != 0x08 || b[7] != 0x01 {
		t.Errorf("unexpected little endian bytes: %v", b)
	}
}

func TestUtils_CeilDiv(t *testing.T) {
	tests := []struct {
		a, b, expected uint64
	}{
		{10, 3, 4},
		{9, 3, 3},
		{11, 3, 4},
		{0, 5, 0},
	}

	for _, tt := range tests {
		result := utils.CeilDiv(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("CeilDiv(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestUtils_MinMax(t *testing.T) {
	if utils.Min(5, 10) != 5 {
		t.Error("Min failed")
	}

	if utils.Max(5, 10) != 10 {
		t.Error("Max failed")
	}
}

// ===== Benchmark Tests =====

func BenchmarkLRUCache_Set(b *testing.B) {
	cache := cache.NewLRUCache(10000, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(string(rune(i%10000)), i)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := cache.NewLRUCache(10000, time.Minute)
	for i := 0; i < 10000; i++ {
		cache.Set(string(rune(i)), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(string(rune(i % 10000)))
	}
}

func BenchmarkGasFeeStrategy_Set(b *testing.B) {
	strategy := common.NewGasFeeStrategy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Set(
			soltradesdk.SwqosType(i%10),
			soltradesdk.TradeTypeBuy,
			common.GasFeeStrategyTypeNormal,
			200000, 100000, 0.001,
		)
	}
}

func BenchmarkBondingCurve_GetBuyPrice(b *testing.B) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		curve.GetBuyPrice(1_000_000)
	}
}

func BenchmarkCeilDiv(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.CeilDiv(123456789, 12345)
	}
}
