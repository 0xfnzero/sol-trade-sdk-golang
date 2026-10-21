package main

import (
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/examples/internal/pumpswapreplay"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"strings"
	"testing"
)

func TestGlobalDefaultsPartialCache(t *testing.T) {
	c := trading.DefaultPumpSwapRoutingConfig()
	for _, buy := range []bool{true, false} {
		s, r, e := pumpswapreplay.LoadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", solana.NewWallet().PublicKey(), buy)
		if e != nil {
			t.Fatal(e)
		}
		got, e := c.Prepare(s.Pool, s.TokenMint, s.AssetMint, r)
		if e != nil {
			t.Fatal(e)
		}
		want := uint64(19264527)
		if buy {
			want = 47932572
		}
		if got.Trade.MinimumOutput() != want {
			t.Fatal(got.Trade.MinimumOutput(), want)
		}
	}
}

func TestUSDCStartAndBridgeSlippage(t *testing.T) {
	c := trading.DefaultPumpSwapRoutingConfig()
	s, r, e := pumpswapreplay.LoadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", solana.NewWallet().PublicKey(), true)
	if e != nil {
		t.Fatal(e)
	}
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	slip := uint16(500)
	r.SlippageBps = &slip
	a, e := c.Prepare(s.Pool, s.TokenMint, usdc, r)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Trade.Steps()) != 2 || a.Trade.Steps()[0].InputMint != usdc {
		t.Fatal("USDC detour")
	}
	c.BridgeSlippageBps = 0
	b, e := c.Prepare(s.Pool, s.TokenMint, usdc, r)
	if e != nil {
		t.Fatal(e)
	}
	if b.Trade.MinimumOutput() <= a.Trade.MinimumOutput() {
		t.Fatal("bridge tolerance ignored")
	}
	c.MaximumBridgePriceImpactBps = 10000
	if _, e := c.Prepare(s.Pool, s.TokenMint, usdc, r); e == nil {
		t.Fatal("invalid impact accepted")
	}
}

func TestBridgeImpactRejectsLargeOrderAndAllowsConfiguredTolerance(t *testing.T) {
	s, r, e := pumpswapreplay.LoadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", solana.NewWallet().PublicKey(), true)
	if e != nil {
		t.Fatal(e)
	}
	var hints []subscription.PoolTradeHint
	for _, h := range trading.DefaultPumpSwapRoutingConfig().ConversionPools() {
		if h.Pool.String() == "58oQChx4yWmvKdwLLZzBi4ChoCc2fqCUWBkwMihLYQo2" {
			hints = append(hints, h)
		}
	}
	if len(hints) != 1 {
		t.Fatal("missing AMM fixture")
	}
	limits := &subscription.BridgeLimits{AnchorPool: s.Pool, SlippageBps: 100, MaximumPriceImpactBps: 0}
	_, e = r.Snapshot.PrepareRouteWithBridgeLimits(hints, r.Context, r.UnixTimestamp, r.Payer, 1000000000000, 100, 6, limits)
	if e == nil || !strings.Contains(e.Error(), "impact") {
		t.Fatal("expected impact rejection", e)
	}
	limits.MaximumPriceImpactBps = 9999
	if _, e = r.Snapshot.PrepareRouteWithBridgeLimits(hints, r.Context, r.UnixTimestamp, r.Payer, 1000000000000, 100, 6, limits); e != nil {
		t.Fatal(e)
	}
}
