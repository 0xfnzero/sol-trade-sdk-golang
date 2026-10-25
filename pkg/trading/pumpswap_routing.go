package trading

import (
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/stockcatalog"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/subscription"
	"github.com/gagliardetto/solana-go"
)

// PumpSwapRoutingConfig is instance scoped. Configure before concurrent Prepare calls.
type PumpSwapRoutingConfig struct {
	topology                                       *pumpTopology
	MaximumArrays                                  int
	pools                                          []subscription.PoolTradeHint
	BridgeSlippageBps, MaximumBridgePriceImpactBps uint16
	MaximumConversionHops, MaximumSearchNodes      int
}

// NewPumpSwapRoutingConfig uses only supplied pools (nil means empty).
func NewPumpSwapRoutingConfig(pools []subscription.PoolTradeHint) (*PumpSwapRoutingConfig, error) {
	c := &PumpSwapRoutingConfig{BridgeSlippageBps: 100, MaximumBridgePriceImpactBps: 100, MaximumConversionHops: 2, MaximumSearchNodes: 512, MaximumArrays: 6}
	if err := c.ExtendPools(pools); err != nil {
		return nil, err
	}
	return c, nil
}
func DefaultPumpSwapRoutingConfig() *PumpSwapRoutingConfig {
	key := solana.MustPublicKeyFromBase58
	hints := []subscription.PoolTradeHint{{Pool: key("58oQChx4yWmvKdwLLZzBi4ChoCc2fqCUWBkwMihLYQo2"), InputMint: key("So11111111111111111111111111111111111111112"), OutputMint: key("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")}}
	for _, p := range stockcatalog.ScreenedStockCatalog().Pools {
		hints = append(hints, subscription.PoolTradeHint{Pool: key(p.Pool), InputMint: key(p.QuoteMint), OutputMint: key(p.Mint)})
	}
	c, err := NewPumpSwapRoutingConfig(hints)
	if err != nil {
		panic(err)
	}
	return c
}
func (c *PumpSwapRoutingConfig) ConversionPools() []subscription.PoolTradeHint {
	return append([]subscription.PoolTradeHint(nil), c.pools...)
}

// ExtendPools validates the entire merge before mutation. Reversed SOL/WSOL pairs deduplicate.
func (c *PumpSwapRoutingConfig) ExtendPools(pools []subscription.PoolTradeHint) error {
	merged := append(c.ConversionPools(), pools...)
	result := []subscription.PoolTradeHint{}
	seen := map[solana.PublicKey]subscription.PoolTradeHint{}
	for _, h := range merged {
		h.InputMint = canonicalPumpMint(h.InputMint)
		h.OutputMint = canonicalPumpMint(h.OutputMint)
		if h.Pool.IsZero() || h.InputMint.IsZero() || h.OutputMint.IsZero() || h.InputMint == h.OutputMint {
			return errors.New("invalid bridge identity")
		}
		if old, ok := seen[h.Pool]; ok {
			if !((old.InputMint == h.InputMint && old.OutputMint == h.OutputMint) || (old.InputMint == h.OutputMint && old.OutputMint == h.InputMint)) {
				return errors.New("conflicting bridge identity")
			}
		} else {
			seen[h.Pool] = h
			result = append(result, h)
		}
	}
	if len(result) > 128 {
		return errors.New("at most 128 bridge pools")
	}
	topology, err := buildPumpTopology(result)
	if err != nil {
		return err
	}
	c.topology = topology
	c.pools = result
	return nil
}
func (c *PumpSwapRoutingConfig) Prepare(pool, token, asset solana.PublicKey, r CachedTradeRequest) (PumpSwapRouteSelection, error) {
	if c.MaximumArrays < 1 || c.MaximumArrays > 32 {
		return PumpSwapRouteSelection{}, errors.New("invalid array budget")
	}
	s := PumpSwapRouteSearch{Pool: pool, TokenMint: token, AssetMint: asset, ConversionPools: c.pools}
	s.MaximumConversionHops = c.MaximumConversionHops
	s.MaximumSearchNodes = c.MaximumSearchNodes
	s.BridgeSlippageBps = c.BridgeSlippageBps
	s.MaximumBridgePriceImpactBps = c.MaximumBridgePriceImpactBps
	r.MaximumArrays = c.MaximumArrays
	return prepareBestPumpSwapTradeWithTopology(s, r, c.topology)
}
