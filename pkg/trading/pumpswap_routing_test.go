package trading

import (
	"context"
	"encoding/binary"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"reflect"
	"strings"
	"testing"
)

// Synthetic token-quoted bridge built from existing recorded account layouts.
// This verifies configuration propagation, not a claim of on-chain execution.
func pumpSwapPumpFunBridgeFixture(t *testing.T) CachedTradeRequest {
	t.Helper()
	r := cachedFixture(t, "buy", "pumpswap", solana.PublicKey{})
	curve := cachedFixture(t, "buy", "pumpfun_whirlpool", r.Payer)
	anchor := r.Hints[0]
	bridge := curve.Hints[len(curve.Hints)-1]
	cache := &subscription.SubscriptionAccountCache{}
	put := func(key solana.PublicKey, account subscription.CachedAccount) {
		t.Helper()
		account.Slot = r.Context.Slot
		if _, err := cache.Update(key, account); err != nil {
			t.Fatal(err)
		}
	}
	get := func(source CachedTradeRequest, key solana.PublicKey) subscription.CachedAccount {
		t.Helper()
		a, err := source.Snapshot.Get(key, source.Context, nil)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	pool := instruction.DecodePoolAccount(get(r, anchor.Pool).Data)
	if pool == nil {
		t.Fatal("invalid recorded PumpSwap pool")
	}
	for _, key := range []solana.PublicKey{anchor.Pool, anchor.InputMint, anchor.OutputMint,
		pool.PoolBaseTokenAccount, pool.PoolQuoteTokenAccount, instruction.PUMPSWAP_GLOBAL_ACCOUNT, instruction.FEE_CONFIG} {
		put(key, get(r, key))
	}
	for _, key := range []solana.PublicKey{bridge.OutputMint, instruction.PUMPFUN_GLOBAL_ACCOUNT, instruction.PUMPFUN_FEE_CONFIG} {
		put(key, get(curve, key))
	}
	a := get(curve, bridge.Pool)
	copy(a.Data[83:115], anchor.OutputMint[:])
	for offset, n := range map[int]uint64{8: 1_000_000_000_000, 16: 1_000_000_000_000,
		24: 800_000_000_000, 32: 1_000_000_000_000} {
		binary.LittleEndian.PutUint64(a.Data[offset:], n)
	}
	put(bridge.Pool, a)
	put(instruction.GetPumpFunFeeSharingConfigPDA(bridge.OutputMint), subscription.CachedAccount{})
	r.Snapshot = cache.Snapshot()
	r.Hints = []subscription.PoolTradeHint{
		{Pool: bridge.Pool, InputMint: bridge.OutputMint, OutputMint: anchor.OutputMint},
		{Pool: anchor.Pool, InputMint: anchor.OutputMint, OutputMint: anchor.InputMint},
	}
	r.InputMint, r.OutputMint = bridge.OutputMint, anchor.InputMint
	r.Amount, r.NativeInput, r.NativeOutput = 1_000_000, false, false
	return r
}

func TestPumpSwapPreparationAppliesPumpFunVersion(t *testing.T) {
	base := pumpSwapPumpFunBridgeFixture(t)
	for _, side := range []soltradesdk.TradeType{soltradesdk.TradeTypeBuy, soltradesdk.TradeTypeSell} {
		request := base
		request.TradeType = side
		bridge := base.Hints[0]
		token, asset := base.OutputMint, base.InputMint
		sideName := "buy"
		if side == soltradesdk.TradeTypeSell {
			sideName = "sell"
			request.InputMint, request.OutputMint = token, asset
			request.Hints = []subscription.PoolTradeHint{
				{Pool: base.Hints[1].Pool, InputMint: token, OutputMint: bridge.OutputMint},
				{Pool: bridge.Pool, InputMint: bridge.OutputMint, OutputMint: asset},
			}
		}
		search := NewPumpSwapRouteSearch(base.Hints[1].Pool, token, asset, []subscription.PoolTradeHint{bridge})
		config, err := NewPumpSwapRoutingConfig([]subscription.PoolTradeHint{bridge})
		if err != nil {
			t.Fatal(err)
		}
		builders := map[string]func(CachedTradeRequest) (*PreparedPumpSwapTrade, error){
			"explicit": PreparePumpSwapTrade,
			"search": func(r CachedTradeRequest) (*PreparedPumpSwapTrade, error) {
				result, err := PrepareBestPumpSwapTrade(search, r)
				return result.Trade, err
			},
			"config": func(r CachedTradeRequest) (*PreparedPumpSwapTrade, error) {
				result, err := config.Prepare(search.Pool, token, asset, r)
				return result.Trade, err
			},
		}
		for name, build := range builders {
			for _, tc := range []struct {
				name                      string
				request, global, snapshot soltradesdk.PumpFunVersion
				reject                    bool
			}{
				{"request_v2", soltradesdk.PumpFunV2, soltradesdk.PumpFunV3, "", true},
				{"global_v2", "", soltradesdk.PumpFunV2, "", true},
				{"snapshot_v2", "", "", soltradesdk.PumpFunV2, true},
				{"request_override", soltradesdk.PumpFunV3, soltradesdk.PumpFunV2, soltradesdk.PumpFunV2, false},
				{"global_override", "", soltradesdk.PumpFunV3, soltradesdk.PumpFunV2, false},
				{"default_v3", "", "", "", false},
			} {
				t.Run(sideName+"/"+name+"/"+tc.name, func(t *testing.T) {
					r := request
					r.PumpFunVersion = tc.request
					r.TradeConfig = &soltradesdk.TradeConfig{PumpFunVersion: tc.global}
					r.Snapshot = request.Snapshot.WithPumpFunVersion(tc.snapshot)
					p, err := build(r)
					if tc.reject {
						// The bridge quote is Token-2022, which V2 deliberately rejects.
						if err == nil || !strings.Contains(err.Error(), "V2 quote token program") {
							t.Fatalf("V2 ignored: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					index := 0
					disc := []byte{28, 146, 222, 119, 38, 196, 105, 213}
					if side == soltradesdk.TradeTypeSell {
						index = 1
						disc = []byte{225, 247, 80, 30, 213, 179, 132, 136}
					}
					leg := p.route.Legs[index]
					data, err := leg.Instruction.Data()
					if err != nil {
						t.Fatal(err)
					}
					if string(data[:8]) != string(disc) || binary.LittleEndian.Uint64(data[8:]) != leg.AmountIn || binary.LittleEndian.Uint64(data[16:]) != leg.MinimumNetAmountOut {
						t.Fatalf("V3 version or protected amounts changed: %x", data)
					}
				})
			}
		}
	}
}

func TestPumpSwapClientAppliesVersionToPumpFunBridge(t *testing.T) {
	base := pumpSwapPumpFunBridgeFixture(t)
	config := soltradesdk.NewTradeConfigBuilder("http://unused.invalid").PumpFunVersion(soltradesdk.PumpFunV2).Build()
	factory := NewTradeExecutorFactory(NewTradeExecutor(nil, config, nil))
	client := NewTradingClient(factory)
	executor, err := factory.GetExecutor(soltradesdk.DexTypePumpSwap)
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []soltradesdk.TradeType{soltradesdk.TradeTypeBuy, soltradesdk.TradeTypeSell} {
		r := base
		r.TradeType, r.PumpFunVersion = side, ""
		if side == soltradesdk.TradeTypeSell {
			r.Hints = []subscription.PoolTradeHint{
				{Pool: base.Hints[1].Pool, InputMint: base.OutputMint, OutputMint: base.Hints[0].OutputMint},
				{Pool: base.Hints[0].Pool, InputMint: base.Hints[0].OutputMint, OutputMint: base.InputMint},
			}
			r.InputMint, r.OutputMint = base.OutputMint, base.InputMint
		}
		// V3 can prepare this synthetic token-quoted route. The configured V2
		// cannot; it must reject before signing or submitting, never use V3.
		control := r
		control.PumpFunVersion = soltradesdk.PumpFunV3
		if _, err := client.PrepareCachedTrade(control); err != nil {
			t.Fatal(err)
		}
		submit := func(context.Context, []byte, soltradesdk.TradeType) (solana.Signature, error) {
			t.Fatal("unsupported V2 route reached submission")
			return solana.Signature{}, nil
		}
		request := &TradeExecutionRequest{Request: r, Submit: submit}
		for _, direct := range []bool{false, true} {
			if direct && side == soltradesdk.TradeTypeBuy {
				_, err = executor.ExecuteBuy(context.Background(), request)
			} else if direct {
				_, err = executor.ExecuteSell(context.Background(), request)
			} else if side == soltradesdk.TradeTypeBuy {
				_, err = client.Buy(context.Background(), r.DexType, request)
			} else {
				_, err = client.Sell(context.Background(), r.DexType, request)
			}
			if err == nil || !strings.Contains(err.Error(), "V2 quote token program") {
				t.Fatalf("PumpSwap executor ignored global V2: side=%v direct=%v err=%v", side, direct, err)
			}
		}
	}
}

func TestGlobalBridgeRegistryAndUSDCPaths(t *testing.T) {
	c := DefaultPumpSwapRoutingConfig()
	hints := c.ConversionPools()
	if len(hints) != 59 {
		t.Fatal(len(hints))
	}
	reversed := make([]subscription.PoolTradeHint, len(hints))
	for i, h := range hints {
		h.InputMint, h.OutputMint = h.OutputMint, h.InputMint
		reversed[i] = h
	}
	if e := c.ExtendPools(reversed); e != nil || len(c.pools) != 59 {
		t.Fatal(e)
	}
	extra := hints[0]
	extra.Pool = solana.NewWallet().PublicKey()
	bad := hints[0]
	bad.OutputMint = extra.Pool
	if e := c.ExtendPools([]subscription.PoolTradeHint{extra, bad}); e == nil || len(c.pools) != 59 {
		t.Fatal("non-atomic merge")
	}
	if e := c.ExtendPools([]subscription.PoolTradeHint{extra}); e != nil || len(c.pools) != 60 {
		t.Fatal(e)
	}
	// Every USDC path starts at the payer's asset and never revisits it.
	for _, h := range hints[1:] {
		s := NewPumpSwapRouteSearch(solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey(), constants.USDC_TOKEN_ACCOUNT, hints)
		paths, e := pumpConversionPaths(s, constants.USDC_TOKEN_ACCOUNT, h.OutputMint, s.TokenMint)
		if e != nil || len(paths) == 0 {
			t.Fatal(e)
		}
		for _, p := range paths {
			if p[0].InputMint != constants.USDC_TOKEN_ACCOUNT {
				t.Fatal("wrong input")
			}
			for _, leg := range p {
				if leg.OutputMint == constants.USDC_TOKEN_ACCOUNT {
					t.Fatal("cycle")
				}
			}
		}
	}
}

func TestCachedTopologyMatchesDirectSearchAndFailedExtensionIsAtomic(t *testing.T) {
	c := DefaultPumpSwapRoutingConfig()
	before := c.topology
	h := c.pools[0]
	bad := h
	bad.OutputMint = solana.NewWallet().PublicKey()
	if err := c.ExtendPools([]subscription.PoolTradeHint{bad}); err == nil || c.topology != before {
		t.Fatal("failed merge changed graph")
	}
	for _, edge := range c.pools {
		s := NewPumpSwapRouteSearch(solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey(), h.InputMint, c.pools)
		direct, err := pumpConversionPaths(s, h.InputMint, edge.OutputMint, s.TokenMint)
		if err != nil {
			t.Fatal(err)
		}
		cached, err := pumpConversionPathsWithTopology(s, h.InputMint, edge.OutputMint, s.TokenMint, c.topology)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(direct, cached) {
			t.Fatal("cached traversal changed route ordering")
		}
	}
	if c.topology != before {
		t.Fatal("search rebuilt graph")
	}
}
