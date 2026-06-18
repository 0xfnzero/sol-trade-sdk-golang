package soltradesdk_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/cache"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/pool"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/swqos"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/utils"
	"github.com/gagliardetto/solana-go"
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
	if !ok {
		t.Fatal("expected strategy value for Jito after concurrent writes")
	}
	if value.CuLimit%10 != 0 || value.CuLimit > 90 {
		t.Errorf("unexpected concurrent write result: %d", value.CuLimit)
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

	result, err := p.SubmitWait(func() (interface{}, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected SubmitWait error: %v", err)
	}

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

func TestRootTradingClient_ReturnsExplicitExecutionError(t *testing.T) {
	payer, err := solana.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	client, err := soltradesdk.NewTradingClient(
		context.Background(),
		&payer,
		soltradesdk.NewTradeConfig("http://localhost:8899", nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Buy(context.Background(), soltradesdk.TradeBuyParams{})
	if !errors.Is(err, soltradesdk.ErrTradingExecutionUnavailable) {
		t.Fatalf("expected explicit unavailable error, got %v", err)
	}
}

func TestRootTradingClient_SimpleHelpersExposeConversionWithoutClaimingExecution(t *testing.T) {
	payer, err := solana.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	client, err := soltradesdk.NewTradingClient(
		context.Background(),
		&payer,
		soltradesdk.NewTradeConfig("http://localhost:8899", nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	simple := soltradesdk.SimpleBuyParams{
		DexType:       soltradesdk.DexTypePumpFun,
		PayWith:       soltradesdk.TradeTokenTypeSOL,
		Mint:          solana.NewWallet().PublicKey(),
		Amount:        soltradesdk.BuyExactInput(123),
		AccountPolicy: soltradesdk.AccountPolicyAuto,
	}
	low := client.BuildBuyParamsFromSimple(simple)
	if low.InputTokenAmount != 123 || low.InputTokenType != soltradesdk.TradeTokenTypeSOL {
		t.Fatalf("simple helper conversion mismatch: %+v", low)
	}

	_, err = client.BuySimple(context.Background(), simple)
	if !errors.Is(err, soltradesdk.ErrTradingExecutionUnavailable) {
		t.Fatalf("expected simple execution to keep root boundary, got %v", err)
	}
}

func TestTradeConfigAddsDefaultRPCWhenSwqosConfigured(t *testing.T) {
	config := soltradesdk.NewTradeConfig("https://x", []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeJito, Region: soltradesdk.SwqosRegionFrankfurt, APIKey: "uuid"},
	})

	if len(config.SwqosConfigs) != 2 {
		t.Fatalf("expected 2 swqos configs, got %d", len(config.SwqosConfigs))
	}
	if config.SwqosConfigs[0].Type != soltradesdk.SwqosTypeJito {
		t.Fatalf("expected first route to stay Jito")
	}
	if config.SwqosConfigs[1].Type != soltradesdk.SwqosTypeDefault {
		t.Fatalf("expected default RPC route to be appended")
	}
}

func TestTradeConfigAddsDefaultRPCWhenNoSwqosConfigured(t *testing.T) {
	config := soltradesdk.NewTradeConfig("https://x", nil)
	if len(config.SwqosConfigs) != 1 {
		t.Fatalf("expected default swqos config, got %d", len(config.SwqosConfigs))
	}
	if config.SwqosConfigs[0].Type != soltradesdk.SwqosTypeDefault {
		t.Fatalf("expected default RPC route")
	}
}

func TestTradeConfigFiltersNextBlockBlacklist(t *testing.T) {
	config := soltradesdk.NewTradeConfig("https://x", []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeNextBlock, Region: soltradesdk.SwqosRegionFrankfurt, APIKey: "token"},
	})
	if len(config.SwqosConfigs) != 1 {
		t.Fatalf("expected only fallback default route, got %d", len(config.SwqosConfigs))
	}
	if config.SwqosConfigs[0].Type != soltradesdk.SwqosTypeDefault {
		t.Fatalf("expected NextBlock to be filtered and Default to remain")
	}
}

func TestTradeConfigRustParityDefaults(t *testing.T) {
	config := soltradesdk.NewTradeConfig("https://x", nil)
	if !config.LogEnabled {
		t.Fatalf("expected LogEnabled default true")
	}
	if config.CheckMinTip {
		t.Fatalf("expected CheckMinTip default false")
	}
	if config.MEVProtection {
		t.Fatalf("expected MEVProtection default false")
	}
	if !config.UseSeedOptimize {
		t.Fatalf("expected UseSeedOptimize default true")
	}
	if !config.CreateWsolAtaOnStartup {
		t.Fatalf("expected CreateWsolAtaOnStartup default true")
	}
	if config.SwqosCoresFromEnd {
		t.Fatalf("expected SwqosCoresFromEnd default false")
	}

	built := soltradesdk.NewTradeConfigBuilder("https://x").Build()
	if !built.CreateWsolAtaOnStartup || built.SwqosCoresFromEnd {
		t.Fatalf("builder defaults are not Rust parity: %+v", built)
	}
}

func TestRecommendedSenderThreadCoreIndicesUsesTwoThirdsCap(t *testing.T) {
	got := soltradesdk.RecommendedSenderThreadCoreIndices(10, 6)
	want := []int{2, 3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("expected %d indices, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: expected %d, got %d (%v)", i, want[i], got[i], got)
		}
	}

	fromStart := soltradesdk.RecommendedSenderThreadCoreIndices(10, 6, 0)
	wantFromStart := []int{0, 1, 2, 3}
	for i := range wantFromStart {
		if fromStart[i] != wantFromStart[i] {
			t.Fatalf("from-start index %d: expected %d, got %d (%v)", i, wantFromStart[i], fromStart[i], fromStart)
		}
	}
}

func testHash(seed byte) solana.Hash {
	data := make([]byte, 32)
	for i := range data {
		data[i] = seed
	}
	return solana.HashFromBytes(data)
}

func TestSimpleBuyParamsToTradeBuyParams(t *testing.T) {
	recent := testHash(1)
	nonceHash := testHash(2)
	useMint := solana.NewWallet().PublicKey()

	low := (soltradesdk.SimpleBuyParams{
		DexType:             soltradesdk.DexTypePumpFun,
		PayWith:             soltradesdk.TradeTokenTypeUSDC,
		Mint:                useMint,
		Amount:              soltradesdk.BuyWithMaxInput(10_000),
		RecentBlockhash:     &recent,
		AccountPolicy:       soltradesdk.AccountPolicyHotPathMinimal,
		WaitForAllSubmits:   true,
		DurableNonce:        &soltradesdk.DurableNonceInfo{NonceHash: nonceHash},
		Simulate:            true,
		SlippageBasisPoints: 250,
	}).ToTradeBuyParams()

	if low.InputTokenType != soltradesdk.TradeTokenTypeUSDC || low.Mint != useMint {
		t.Fatalf("basic buy fields not preserved: %+v", low)
	}
	if low.InputTokenAmount != 10_000 {
		t.Fatalf("expected input amount 10000, got %d", low.InputTokenAmount)
	}
	if low.UseExactSolAmount == nil || *low.UseExactSolAmount {
		t.Fatalf("WithMaxInput should set UseExactSolAmount=false")
	}
	if low.CreateInputTokenATA || low.CreateMintATA || low.CloseInputTokenATA {
		t.Fatalf("HotPathMinimal should disable ATA mutations: %+v", low)
	}
	if low.RecentBlockhash != nil || low.DurableNonce == nil {
		t.Fatalf("durable nonce should clear recent blockhash: %+v", low)
	}
	if !low.WaitForAllSubmits || !low.Simulate || low.SlippageBasisPoints != 250 {
		t.Fatalf("execution flags not preserved: %+v", low)
	}
}

func TestSimpleParamConstructorsAndDefaults(t *testing.T) {
	recent := testHash(1)
	mint := solana.NewWallet().PublicKey()

	buy := soltradesdk.NewSimpleBuyParams(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeWSOL,
		mint,
		soltradesdk.BuyWithMaxInput(5_000),
		struct{}{},
		recent,
		nil,
	)
	if buy.AccountPolicy != soltradesdk.AccountPolicyAuto {
		t.Fatalf("expected AccountPolicyAuto, got %v", buy.AccountPolicy)
	}
	if buy.WaitTxConfirmed || buy.WaitForAllSubmits || buy.Simulate {
		t.Fatalf("simple buy defaults should be false: %+v", buy)
	}
	if buy.RecentBlockhash == nil || *buy.RecentBlockhash != recent {
		t.Fatalf("recent blockhash not preserved: %+v", buy.RecentBlockhash)
	}

	sell := soltradesdk.NewSimpleSellParams(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeUSDC,
		mint,
		soltradesdk.SellExactInput(7_000),
		struct{}{},
		recent,
		nil,
	)
	if sell.AccountPolicy != soltradesdk.AccountPolicyAuto {
		t.Fatalf("expected AccountPolicyAuto, got %v", sell.AccountPolicy)
	}
	if sell.WaitTxConfirmed || sell.WaitForAllSubmits || sell.Simulate {
		t.Fatalf("simple sell defaults should be false: %+v", sell)
	}
	if low := sell.ToTradeSellParams(); !low.WithTip {
		t.Fatalf("simple sell should convert to WithTip=true by default")
	}
}

func TestSimpleDurableNonceConstructorsClearRecentBlockhash(t *testing.T) {
	nonce := soltradesdk.DurableNonceInfo{NonceHash: testHash(2)}
	mint := solana.NewWallet().PublicKey()

	buy := soltradesdk.NewSimpleBuyParamsWithDurableNonce(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeSOL,
		mint,
		soltradesdk.BuyExactInput(1_000),
		struct{}{},
		nonce,
		nil,
	)
	if buy.RecentBlockhash != nil || buy.DurableNonce == nil {
		t.Fatalf("buy durable nonce constructor should clear recent blockhash: %+v", buy)
	}

	sell := soltradesdk.NewSimpleSellParamsWithDurableNonce(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeSOL,
		mint,
		soltradesdk.SellExactOutput(500, 1_000),
		struct{}{},
		nonce,
		nil,
	)
	if sell.RecentBlockhash != nil || sell.DurableNonce == nil {
		t.Fatalf("sell durable nonce constructor should clear recent blockhash: %+v", sell)
	}
}

func TestSimpleSettersReturnUpdatedCopies(t *testing.T) {
	recent := testHash(1)
	nonce := soltradesdk.DurableNonceInfo{NonceHash: testHash(2)}
	mint := solana.NewWallet().PublicKey()

	base := soltradesdk.NewSimpleBuyParams(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeWSOL,
		mint,
		soltradesdk.BuyExactInput(1_000),
		struct{}{},
		recent,
		nil,
	)
	buy := base.
		SetSlippageBasisPoints(123).
		SetAccountPolicy(soltradesdk.AccountPolicyCreateMissing).
		SetDurableNonce(nonce).
		SetWaitTxConfirmed(true).
		SetWaitForAllSubmits(true).
		SetSimulate(true).
		SetGrpcRecvUs(456)

	if base.RecentBlockhash == nil || *base.RecentBlockhash != recent {
		t.Fatalf("base should remain unchanged: %+v", base)
	}
	if buy.RecentBlockhash != nil || buy.DurableNonce == nil {
		t.Fatalf("durable nonce setter should clear recent blockhash: %+v", buy)
	}
	if buy.SlippageBasisPoints != 123 || buy.AccountPolicy != soltradesdk.AccountPolicyCreateMissing {
		t.Fatalf("buy setters not preserved: %+v", buy)
	}
	if !buy.WaitTxConfirmed || !buy.WaitForAllSubmits || !buy.Simulate {
		t.Fatalf("buy execution setters not preserved: %+v", buy)
	}
	if buy.GrpcRecvUs == nil || *buy.GrpcRecvUs != 456 {
		t.Fatalf("buy grpc recv timestamp not preserved: %+v", buy.GrpcRecvUs)
	}

	sell := soltradesdk.NewSimpleSellParams(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeSOL,
		mint,
		soltradesdk.SellExactInput(1_000),
		struct{}{},
		recent,
		nil,
	).
		SetSlippageBasisPoints(321).
		SetAccountPolicy(soltradesdk.AccountPolicyAssumePrepared).
		SetWaitTxConfirmed(true).
		SetWaitForAllSubmits(true).
		SetSimulate(true).
		SetWithTip(false).
		SetGrpcRecvUs(654)
	if sell.SlippageBasisPoints != 321 || sell.AccountPolicy != soltradesdk.AccountPolicyAssumePrepared {
		t.Fatalf("sell setters not preserved: %+v", sell)
	}
	if !sell.WaitTxConfirmed || !sell.WaitForAllSubmits || !sell.Simulate {
		t.Fatalf("sell execution setters not preserved: %+v", sell)
	}
	if sell.WithTip == nil || *sell.WithTip {
		t.Fatalf("sell SetWithTip(false) not preserved: %+v", sell.WithTip)
	}
	if sell.GrpcRecvUs == nil || *sell.GrpcRecvUs != 654 {
		t.Fatalf("sell grpc recv timestamp not preserved: %+v", sell.GrpcRecvUs)
	}
}

func TestSimpleBuyExactOutputAndAutoPolicy(t *testing.T) {
	low := (soltradesdk.SimpleBuyParams{
		DexType:       soltradesdk.DexTypePumpFun,
		PayWith:       soltradesdk.TradeTokenTypeSOL,
		Mint:          solana.NewWallet().PublicKey(),
		Amount:        soltradesdk.BuyExactOutput(42, 10_000),
		AccountPolicy: soltradesdk.AccountPolicyAuto,
	}).ToTradeBuyParams()

	if low.InputTokenAmount != 10_000 {
		t.Fatalf("expected max input amount, got %d", low.InputTokenAmount)
	}
	if low.FixedOutputTokenAmount == nil || *low.FixedOutputTokenAmount != 42 {
		t.Fatalf("expected fixed output amount 42, got %v", low.FixedOutputTokenAmount)
	}
	if low.UseExactSolAmount == nil || !*low.UseExactSolAmount {
		t.Fatalf("ExactOutput should set UseExactSolAmount=true")
	}
	if low.CreateInputTokenATA || !low.CreateMintATA || low.CloseInputTokenATA {
		t.Fatalf("Auto buy should create only mint ATA: %+v", low)
	}
}

func TestSimpleSellParamsToTradeSellParams(t *testing.T) {
	low := (soltradesdk.SimpleSellParams{
		DexType:       soltradesdk.DexTypePumpFun,
		ReceiveAs:     soltradesdk.TradeTokenTypeUSDC,
		Mint:          solana.NewWallet().PublicKey(),
		Amount:        soltradesdk.SellExactOutput(7_000, 50_000),
		AccountPolicy: soltradesdk.AccountPolicyAuto,
	}).ToTradeSellParams()

	if low.InputTokenAmount != 50_000 {
		t.Fatalf("expected max input amount, got %d", low.InputTokenAmount)
	}
	if low.FixedOutputTokenAmount == nil || *low.FixedOutputTokenAmount != 7_000 {
		t.Fatalf("expected fixed output amount 7000, got %v", low.FixedOutputTokenAmount)
	}
	if !low.WithTip {
		t.Fatalf("SimpleSellParams should default WithTip=true")
	}
	if !low.CreateOutputTokenATA || low.CloseOutputTokenATA || low.CloseMintTokenATA {
		t.Fatalf("Auto sell should create non-SOL output ATA only: %+v", low)
	}

	noTip := false
	solLow := (soltradesdk.SimpleSellParams{
		DexType:       soltradesdk.DexTypePumpFun,
		ReceiveAs:     soltradesdk.TradeTokenTypeSOL,
		Mint:          solana.NewWallet().PublicKey(),
		Amount:        soltradesdk.SellExactInput(50_000),
		AccountPolicy: soltradesdk.AccountPolicyAuto,
		WithTip:       &noTip,
	}).ToTradeSellParams()
	if solLow.WithTip {
		t.Fatalf("explicit WithTip=false should be preserved")
	}
	if solLow.CreateOutputTokenATA {
		t.Fatalf("Auto sell should not create SOL output ATA")
	}
}

func TestSolamiSwqosFactory(t *testing.T) {
	var hasSolami bool
	for _, typ := range swqos.GetAllSwqosTypes() {
		if typ == soltradesdk.SwqosTypeSolami {
			hasSolami = true
			break
		}
	}
	if !hasSolami {
		t.Fatalf("expected Solami in SWQOS type list")
	}

	client, err := (&swqos.ClientFactory{}).CreateClient(
		soltradesdk.SwqosConfig{Type: soltradesdk.SwqosTypeSolami, Region: soltradesdk.SwqosRegionTokyo},
		"https://rpc.example",
	)
	if err != nil {
		t.Fatalf("create Solami client: %v", err)
	}
	if client.GetSwqosType() != soltradesdk.SwqosTypeSolami {
		t.Fatalf("expected Solami client type, got %v", client.GetSwqosType())
	}
	if client.MinTipSol() != swqos.MinTipSolami {
		t.Fatalf("expected min tip %f, got %f", swqos.MinTipSolami, client.MinTipSol())
	}

	_, err = client.SendTransaction(context.Background(), soltradesdk.TradeTypeBuy, append([]byte{1}, make([]byte, 64)...), false)
	if err == nil || !strings.Contains(err.Error(), "Solami api_token is required") {
		t.Fatalf("expected Solami api_token error, got %v", err)
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
