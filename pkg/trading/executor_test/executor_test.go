package trading_test

import (
	"testing"

	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
	"github.com/your-org/sol-trade-sdk-go/pkg/common"
	"github.com/your-org/sol-trade-sdk-go/pkg/trading"
)

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
		200000,
		100000,
		0.001,
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

func TestGasFeeStrategy_GlobalFeeStrategy(t *testing.T) {
	strategy := common.NewGasFeeStrategy()
	strategy.SetGlobalFeeStrategy(200000, 200000, 100000, 100000, 0.001, 0.001)

	// Check Jito buy strategy
	value, ok := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	if !ok {
		t.Fatal("expected to get Jito buy strategy")
	}

	if value.CuLimit != 200000 {
		t.Errorf("expected CuLimit 200000, got %d", value.CuLimit)
	}

	// Check Default (RPC) has no tip
	defaultValue, ok := strategy.Get(
		soltradesdk.SwqosTypeDefault,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	if !ok {
		t.Fatal("expected to get Default buy strategy")
	}

	if defaultValue.Tip != 0 {
		t.Errorf("expected Tip 0 for Default, got %f", defaultValue.Tip)
	}
}

func TestGasFeeStrategy_UpdateBuyTip(t *testing.T) {
	strategy := common.NewGasFeeStrategy()
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
		200000, 100000, 0.001,
	)
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeSell,
		common.GasFeeStrategyTypeNormal,
		200000, 100000, 0.002,
	)

	strategy.UpdateBuyTip(0.005)

	buyValue, _ := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)
	sellValue, _ := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeSell,
		common.GasFeeStrategyTypeNormal,
	)

	if buyValue.Tip != 0.005 {
		t.Errorf("expected buy tip 0.005, got %f", buyValue.Tip)
	}

	if sellValue.Tip != 0.002 {
		t.Errorf("expected sell tip to remain 0.002, got %f", sellValue.Tip)
	}
}

func TestGasFeeStrategy_Delete(t *testing.T) {
	strategy := common.NewGasFeeStrategy()
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
		200000, 100000, 0.001,
	)

	strategy.Delete(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	_, ok := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	if ok {
		t.Error("expected strategy to be deleted")
	}
}

func TestGasFeeStrategy_ConflictResolution(t *testing.T) {
	strategy := common.NewGasFeeStrategy()

	// Set high/low strategies first
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeLowTipHighCuPrice,
		200000, 100000, 0.0005,
	)
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeHighTipLowCuPrice,
		200000, 100000, 0.002,
	)

	// Set Normal strategy (should remove high/low)
	strategy.Set(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
		200000, 100000, 0.001,
	)

	_, lowOk := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeLowTipHighCuPrice,
	)
	_, highOk := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeHighTipLowCuPrice,
	)
	_, normalOk := strategy.Get(
		soltradesdk.SwqosTypeJito,
		soltradesdk.TradeTypeBuy,
		common.GasFeeStrategyTypeNormal,
	)

	if lowOk {
		t.Error("expected low strategy to be deleted")
	}
	if highOk {
		t.Error("expected high strategy to be deleted")
	}
	if !normalOk {
		t.Error("expected normal strategy to exist")
	}
}

func TestBondingCurve_GetBuyPrice(t *testing.T) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
		Complete:             false,
	}

	// Buy with 0.001 SOL
	tokens := curve.GetBuyPrice(1000000)
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

	// Sell 1 million tokens
	sol := curve.GetSellPrice(1000000000, 100)
	if sol == 0 {
		t.Error("expected non-zero SOL output")
	}
}

func TestBondingCurve_CompleteCurveReturnsZero(t *testing.T) {
	curve := &common.BondingCurveAccount{
		VirtualTokenReserves: 1073000000000000,
		VirtualSolReserves:   30000000000,
		RealTokenReserves:    793000000000000,
		Complete:             true,
	}

	tokens := curve.GetBuyPrice(1000000)
	if tokens != 0 {
		t.Error("expected zero tokens for complete curve")
	}

	sol := curve.GetSellPrice(1000000000, 100)
	if sol != 0 {
		t.Error("expected zero SOL for complete curve")
	}
}

func TestTradeExecutor_DefaultOptions(t *testing.T) {
	opts := trading.DefaultExecuteOptions()

	if !opts.WaitConfirmation {
		t.Error("expected WaitConfirmation to be true")
	}

	if opts.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", opts.MaxRetries)
	}

	if opts.ParallelSubmit {
		t.Error("expected ParallelSubmit to be false by default")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := trading.NewRateLimiter(100) // 100ms

	// First call should not block
	limiter.Wait()

	// Second call should block briefly
	// (In production, we'd use a mock timer)
}

func TestMetricsCollector(t *testing.T) {
	collector := &trading.MetricsCollector{}

	collector.RecordTrade(true, 100)
	collector.RecordTrade(true, 200)
	collector.RecordTrade(false, 150)

	total, success, failed, avgLatency := collector.GetStats()

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	if success != 2 {
		t.Errorf("expected success 2, got %d", success)
	}

	if failed != 1 {
		t.Errorf("expected failed 1, got %d", failed)
	}

	expectedAvg := float64(100+200+150) / 3
	if avgLatency != expectedAvg {
		t.Errorf("expected avg latency %f, got %f", expectedAvg, avgLatency)
	}
}
