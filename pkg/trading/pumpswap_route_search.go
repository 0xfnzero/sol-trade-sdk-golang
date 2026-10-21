package trading

// Bounded route selection from caller-approved conversion pools, without RPC.
import (
	"bytes"
	"errors"
	"fmt"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"sort"
)

type PumpSwapRouteSearch struct {
	Pool, TokenMint, AssetMint                     solana.PublicKey
	ConversionPools                                []subscription.PoolTradeHint
	MaximumConversionHops, MaximumSearchNodes      int
	BridgeSlippageBps, MaximumBridgePriceImpactBps uint16
}

func NewPumpSwapRouteSearch(pool, token, asset solana.PublicKey, hints []subscription.PoolTradeHint) PumpSwapRouteSearch {
	return PumpSwapRouteSearch{pool, token, asset, append([]subscription.PoolTradeHint(nil), hints...), 2, 512, 100, 100}
}

type PumpSwapRouteRejection struct {
	Pools  []solana.PublicKey
	Reason string
}
type PumpSwapRouteSelection struct {
	Trade           *PreparedPumpSwapTrade
	RoutesEvaluated int
	RejectedRoutes  []PumpSwapRouteRejection
}

// pumpConversionPaths counts every partial path and dead end, and refuses an
// incomplete search even after a valid path was found.
func pumpConversionPaths(search PumpSwapRouteSearch, from, to, excluded solana.PublicKey) ([][]subscription.PoolTradeHint, error) {
	return pumpConversionPathsWithTopology(search, from, to, excluded, nil)
}

func pumpConversionPathsWithTopology(search PumpSwapRouteSearch, from, to, excluded solana.PublicKey, graph *pumpTopology) ([][]subscription.PoolTradeHint, error) {
	if len(search.ConversionPools) > 128 || search.MaximumConversionHops < 0 || search.MaximumConversionHops > 3 || search.MaximumSearchNodes < 1 || search.MaximumSearchNodes > 4096 {
		return nil, errors.New("invalid PumpSwap search limits")
	}
	if graph == nil {
		var err error
		graph, err = buildPumpTopology(search.ConversionPools)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := graph.pools[search.Pool]; ok {
		return nil, errors.New("conversion route reuses anchor")
	}
	remaining := search.MaximumSearchNodes
	paths := [][]subscription.PoolTradeHint{}
	seen := map[solana.PublicKey]bool{from: true}
	var walk func(solana.PublicKey, []subscription.PoolTradeHint) error
	walk = func(mint solana.PublicKey, path []subscription.PoolTradeHint) error {
		if remaining == 0 {
			return errors.New("PumpSwap search node budget exhausted")
		}
		remaining--
		if mint == to {
			paths = append(paths, append([]subscription.PoolTradeHint(nil), path...))
			return nil
		}
		if len(path) == search.MaximumConversionHops {
			return nil
		}
		for _, h := range graph.adjacency[mint] {
			var output solana.PublicKey
			if mint == h.InputMint {
				output = h.OutputMint
			} else if mint == h.OutputMint {
				output = h.InputMint
			} else {
				continue
			}
			if seen[output] || output == excluded {
				continue
			}
			reused := false
			for _, old := range path {
				if old.Pool == h.Pool {
					reused = true
					break
				}
			}
			if reused {
				continue
			}
			seen[output] = true
			if err := walk(output, append(path, subscription.PoolTradeHint{Pool: h.Pool, InputMint: mint, OutputMint: output})); err != nil {
				return err
			}
			delete(seen, output)
		}
		return nil
	}
	if err := walk(from, nil); err != nil {
		return nil, err
	}
	return paths, nil
}

// PrepareBestPumpSwapTrade ranks every complete simple path by protected output,
// then fewer hops and raw pool pubkey bytes. Hints never copy observed amounts.
func PrepareBestPumpSwapTrade(search PumpSwapRouteSearch, r CachedTradeRequest) (PumpSwapRouteSelection, error) {
	return prepareBestPumpSwapTradeWithTopology(search, r, nil)
}

func prepareBestPumpSwapTradeWithTopology(search PumpSwapRouteSearch, r CachedTradeRequest, topology *pumpTopology) (PumpSwapRouteSelection, error) {
	var result PumpSwapRouteSelection
	if search.BridgeSlippageBps >= 10000 || search.MaximumBridgePriceImpactBps >= 10000 {
		return result, errors.New("invalid bridge limits")
	}
	if r.Snapshot == nil || r.Amount == 0 || (r.TradeType != soltradesdk.TradeTypeBuy && r.TradeType != soltradesdk.TradeTypeSell) {
		return result, errors.New("invalid PumpSwap search request")
	}
	token, asset := canonicalPumpMint(search.TokenMint), canonicalPumpMint(search.AssetMint)
	if token.IsZero() || asset.IsZero() || token == asset {
		return result, errors.New("invalid PumpSwap traded token or asset")
	}
	a, err := r.Snapshot.Get(search.Pool, r.Context, &instruction.PUMPSWAP_PROGRAM)
	if err != nil {
		return result, err
	}
	pool := instruction.DecodePoolAccount(a.Data)
	if pool == nil {
		return result, errors.New("invalid selected PumpSwap pool")
	}
	quote := pool.BaseMint
	if token == pool.BaseMint {
		quote = pool.QuoteMint
	} else if token != pool.QuoteMint {
		return result, errors.New("traded token not in selected pool")
	}
	pump := subscription.PoolTradeHint{Pool: search.Pool, InputMint: quote, OutputMint: token}
	from, to := asset, quote
	buy := r.TradeType == soltradesdk.TradeTypeBuy
	if !buy {
		pump.InputMint, pump.OutputMint = token, quote
		from, to = quote, asset
	}
	if _, err = r.Snapshot.PumpSwap(pump, r.Context); err != nil {
		return result, err
	}
	paths, err := pumpConversionPathsWithTopology(search, from, to, token, topology)
	if err != nil {
		return result, err
	}
	result.RoutesEvaluated = len(paths)
	for _, path := range paths {
		if buy {
			path = append(path, pump)
			r.InputMint, r.OutputMint = search.AssetMint, search.TokenMint
		} else {
			path = append([]subscription.PoolTradeHint{pump}, path...)
			r.InputMint, r.OutputMint = search.TokenMint, search.AssetMint
		}
		r.Hints = path
		trade, e := preparePumpSwapTradeWithLimits(r, &subscription.BridgeLimits{AnchorPool: search.Pool, SlippageBps: search.BridgeSlippageBps, MaximumPriceImpactBps: search.MaximumBridgePriceImpactBps})
		if e != nil {
			rejected := PumpSwapRouteRejection{Reason: e.Error()}
			for _, h := range path {
				rejected.Pools = append(rejected.Pools, h.Pool)
			}
			result.RejectedRoutes = append(result.RejectedRoutes, rejected)
			continue
		}
		previous := result.Trade
		if previous == nil || trade.minimumOutput > previous.minimumOutput || (trade.minimumOutput == previous.minimumOutput && pumpRouteLess(trade.steps, previous.steps)) {
			result.Trade = trade
		}
	}
	if result.Trade == nil {
		reason := "no connected path"
		if len(result.RejectedRoutes) > 0 {
			reason = result.RejectedRoutes[0].Reason
		}
		return result, fmt.Errorf("no executable PumpSwap route: %s", reason)
	}
	return result, nil
}
func pumpRouteLess(a, b []PumpSwapQuotedStep) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	for i := range a {
		if c := bytes.Compare(a[i].Pool[:], b[i].Pool[:]); c != 0 {
			return c < 0
		}
	}
	return false
}

type pumpTopology struct {
	pools     map[solana.PublicKey]subscription.PoolTradeHint
	adjacency map[solana.PublicKey][]subscription.PoolTradeHint
}

func buildPumpTopology(hints []subscription.PoolTradeHint) (*pumpTopology, error) {
	pools := map[solana.PublicKey]subscription.PoolTradeHint{}
	for _, h := range hints {
		h.InputMint = canonicalPumpMint(h.InputMint)
		h.OutputMint = canonicalPumpMint(h.OutputMint)
		if h.Pool.IsZero() || h.InputMint.IsZero() || h.OutputMint.IsZero() || h.InputMint == h.OutputMint {
			return nil, errors.New("invalid conversion pool identity or selected pool reuse")
		}
		if bytes.Compare(h.InputMint[:], h.OutputMint[:]) > 0 {
			h.InputMint, h.OutputMint = h.OutputMint, h.InputMint
		}
		if old, ok := pools[h.Pool]; ok && old != h {
			return nil, errors.New("conflicting conversion pool mint pairs")
		}
		pools[h.Pool] = h
	}
	ordered := make([]subscription.PoolTradeHint, 0, len(pools))
	for _, h := range pools {
		ordered = append(ordered, h)
	}
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i].Pool[:], ordered[j].Pool[:]) < 0 })
	adjacency := map[solana.PublicKey][]subscription.PoolTradeHint{}
	for _, h := range ordered {
		for _, mint := range []solana.PublicKey{h.InputMint, h.OutputMint} {
			adjacency[mint] = append(adjacency[mint], h)
		}
	}
	return &pumpTopology{pools, adjacency}, nil
}
