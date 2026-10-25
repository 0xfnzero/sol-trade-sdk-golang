package subscription

// Independent buy/sell route. Fixed inputs use preceding protected net credits;
// intermediate residual balances are reported rather than spent from the wallet.
import (
	"errors"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type CachedRouteLeg struct {
	Hint                          PoolTradeHint
	AmountIn, MinimumNetAmountOut uint64
	EstimatedNetAmountOut         *uint64
	Instruction                   solana.Instruction
}
type RouteResidual struct {
	Mint   solana.PublicKey
	Amount uint64
}
type PreparedCachedRoute struct {
	Execution                           instruction.PumpRouteExecution
	Legs                                []CachedRouteLeg
	SetupInstructions, SwapInstructions []solana.Instruction
	MinimumNetAmountOut                 uint64
	EstimatedIntermediateResiduals      []RouteResidual
}

func (s *AccountCacheSnapshot) PrepareRoute(hints []PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int, allowPumpFunNative ...bool) (PreparedCachedRoute, error) {
	return s.prepareRoute(hints, ctx, unixTimestamp, payer, amount, slippageBps, maximumArrays, nil, allowPumpFunNative...)
}

type BridgeLimits struct {
	AnchorPool                         solana.PublicKey
	SlippageBps, MaximumPriceImpactBps uint16
}

func (s *AccountCacheSnapshot) PrepareRouteWithBridgeLimits(hints []PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int, limits *BridgeLimits) (PreparedCachedRoute, error) {
	return s.prepareRoute(hints, ctx, unixTimestamp, payer, amount, slippageBps, maximumArrays, limits)
}
func (s *AccountCacheSnapshot) prepareRoute(hints []PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int, limits *BridgeLimits, allowPumpFunNative ...bool) (PreparedCachedRoute, error) {
	var result PreparedCachedRoute
	if amount == 0 || len(hints) < 1 || len(hints) > 5 || slippageBps >= 10000 || maximumArrays < 1 || maximumArrays > 32 {
		return result, errors.New("invalid cached route request")
	}
	pools := map[solana.PublicKey]bool{}
	mints := []solana.PublicKey{hints[0].InputMint}
	seen := map[solana.PublicKey]bool{hints[0].InputMint: true}
	for i, h := range hints {
		if e := h.validate(); e != nil {
			return result, e
		}
		if pools[h.Pool] {
			return result, errors.New("route reuses a pool and would require changed-state quoting")
		}
		pools[h.Pool] = true
		if seen[h.OutputMint] {
			return result, errors.New("route contains an asset cycle")
		}
		seen[h.OutputMint] = true
		mints = append(mints, h.OutputMint)
		if i > 0 && hints[i-1].OutputMint != h.InputMint {
			return result, errors.New("disconnected route")
		}
	}
	if limits != nil && (limits.SlippageBps >= 10000 || limits.MaximumPriceImpactBps >= 10000) {
		return result, errors.New("invalid bridge limits")
	}
	targetSlippage := slippageBps
	spend := amount
	for _, h := range hints {
		bridge := limits != nil && h.Pool != limits.AnchorPool
		slippageBps = targetSlippage
		if bridge {
			slippageBps = limits.SlippageBps
		}
		p, e := s.get(h.Pool, ctx, nil)
		if e != nil {
			return PreparedCachedRoute{}, e
		}
		var used, estimated, minimum uint64
		var ix solana.Instruction
		switch p.Owner {
		case instruction.PUMPFUN_PROGRAM:
			var r PreparedPumpFun
			r, e = s.preparePumpFunRouteLeg(h, ctx, payer, spend, slippageBps, len(allowPumpFunNative) == 1 && allowPumpFunNative[0])
			used, estimated, minimum, ix = r.Quote.AmountIn, r.Quote.EstimatedNetAmountOut, r.Quote.MinimumNetAmountOut, r.Instruction
		case instruction.PUMPSWAP_PROGRAM:
			var r PreparedPumpSwap
			r, e = s.PreparePumpSwap(h, ctx, payer, spend, slippageBps)
			used, estimated, minimum, ix = r.Quote.AmountIn, r.Quote.AmountOut, r.Quote.MinimumAmountOut, r.Instruction
		case AmmV4Program:
			var r PreparedAmmV4
			r, e = s.PrepareAmmV4(h, ctx, unixTimestamp, payer, spend, slippageBps)
			used, estimated, minimum, ix = r.Quote.AmountIn, r.Quote.AmountOut, r.Quote.MinimumAmountOut, r.Instruction
		case ClmmProgram:
			var q CachedClmmQuote
			_, q, ix, e = s.PrepareClmm(h, ctx, unixTimestamp, payer, spend, slippageBps, maximumArrays)
			used, estimated, minimum = q.AmountIn, q.EstimatedNetAmountOut, q.MinimumNetAmountOut
		case WhirlpoolProgram:
			var q CachedWhirlpoolQuote
			_, q, ix, e = s.PrepareWhirlpool(h, ctx, unixTimestamp, payer, spend, slippageBps, maximumArrays)
			used, estimated, minimum = q.AmountIn, q.EstimatedNetAmountOut, q.MinimumNetAmountOut
		case DlmmProgram:
			var q CachedDlmmQuote
			_, q, ix, e = s.PrepareDlmm(h, ctx, unixTimestamp, payer, spend, slippageBps, maximumArrays)
			used, estimated, minimum = q.AmountIn, q.EstimatedNetAmountOut, q.MinimumNetAmountOut
		case CpmmProgram:
			var r PreparedCpmm
			r, e = s.PrepareCpmm(h, ctx, unixTimestamp, payer, spend, slippageBps)
			used, estimated, minimum, ix = r.Quote.AmountIn, r.Quote.AmountOut, r.Quote.MinimumAmountOut, r.Instruction
		case instruction.BONK_PROGRAM:
			a, state, err := s.LaunchLabCurve(h, ctx)
			if err != nil {
				return PreparedCachedRoute{}, err
			}
			protected, err := calc.QuoteLaunchLabExactIn(state, spend, h.InputMint == a.QuoteMint, slippageBps, 0)
			if err != nil {
				return PreparedCachedRoute{}, err
			}
			q, err := calc.QuoteLaunchLabExactIn(state, spend, h.InputMint == a.QuoteMint, 0, 0)
			if err != nil {
				return PreparedCachedRoute{}, err
			}
			used, estimated, minimum = protected.AmountIn, q.MinimumAmountOut, protected.MinimumAmountOut
			ix, e = instruction.BuildLaunchLabCurveExactIn(a, payer, used, minimum, h.InputMint == a.QuoteMint, 0)
		default:
			return PreparedCachedRoute{}, errors.New("pool protocol has no native cached quote implementation")
		}
		if e != nil {
			return PreparedCachedRoute{}, fmt.Errorf("route pool %s: %w", h.Pool, e)
		}
		if used == 0 || used > spend || minimum == 0 {
			return PreparedCachedRoute{}, errors.New("route has zero output or overconsumes input")
		}
		if bridge {
			probeAmount := max(uint64(1), spend/100)
			probeUsed, reference, err := s.quoteRouteLeg(h, ctx, unixTimestamp, payer, probeAmount, maximumArrays)
			if err != nil {
				return result, err
			}
			ref := &reference
			if ref == nil || *ref == 0 || used != spend || probeUsed != probeAmount {
				return result, errors.New("unusable bridge impact probe")
			}
			lhs := new(big.Int).Mul(new(big.Int).SetUint64(estimated), new(big.Int).SetUint64(probeAmount))
			lhs.Mul(lhs, big.NewInt(10000))
			rhs := new(big.Int).Mul(new(big.Int).SetUint64(*ref), new(big.Int).SetUint64(spend))
			rhs.Mul(rhs, new(big.Int).SetUint64(uint64(10000-limits.MaximumPriceImpactBps)))
			if lhs.Cmp(rhs) < 0 {
				return result, errors.New("bridge price impact exceeds limit")
			}
		}
		result.Legs = append(result.Legs, CachedRouteLeg{Hint: h, AmountIn: used, EstimatedNetAmountOut: &estimated, MinimumNetAmountOut: minimum, Instruction: ix})
		result.SwapInstructions = append(result.SwapInstructions, ix)
		spend = minimum
	}
	for _, m := range mints {
		a, e := s.get(m, ctx, nil)
		if e != nil {
			return PreparedCachedRoute{}, e
		}
		ix, _, e := common.BuildCreateIdempotentATA(payer, payer, m, a.Owner)
		if e != nil {
			return PreparedCachedRoute{}, e
		}
		result.SetupInstructions = append(result.SetupInstructions, ix)
	}
	for i := 0; i < len(result.Legs)-1; i++ {
		a, b := result.Legs[i], result.Legs[i+1]
		if a.EstimatedNetAmountOut == nil || b.AmountIn > *a.EstimatedNetAmountOut {
			return PreparedCachedRoute{}, errors.New("route exceeds estimated credit")
		}
		result.EstimatedIntermediateResiduals = append(result.EstimatedIntermediateResiduals, RouteResidual{a.Hint.OutputMint, *a.EstimatedNetAmountOut - b.AmountIn})
	}
	result.MinimumNetAmountOut = spend
	return result, nil
}

// quoteRouteLeg shares protocol math with preparation, without building instructions.
func (s *AccountCacheSnapshot) quoteRouteLeg(h PoolTradeHint, ctx CacheReadContext, timestamp uint64, payer solana.PublicKey, amount uint64, arrays int) (uint64, uint64, error) {
	a, err := s.Get(h.Pool, ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	switch a.Owner {
	case ClmmProgram:
		q, e := s.QuoteClmm(h, ctx, timestamp, payer, amount, 0, arrays)
		return q.AmountIn, q.EstimatedNetAmountOut, e
	case DlmmProgram:
		q, e := s.QuoteDlmm(h, ctx, timestamp, payer, amount, 0, arrays)
		return q.AmountIn, q.EstimatedNetAmountOut, e
	case WhirlpoolProgram:
		q, e := s.QuoteWhirlpool(h, ctx, timestamp, payer, amount, 0, arrays)
		return q.AmountIn, q.EstimatedNetAmountOut, e
	case AmmV4Program:
		p, e := s.AmmV4(h, ctx, timestamp)
		if e != nil {
			return 0, 0, e
		}
		q, e := QuoteCachedAmmV4ExactIn(p, amount, h.InputMint == p.CoinMint, 0)
		return q.AmountIn, q.AmountOut, e
	case CpmmProgram:
		p, e := s.Cpmm(h, ctx, timestamp)
		if e != nil {
			return 0, 0, e
		}
		q, e := QuoteCachedCpmmExactIn(p, amount, h.InputMint == p.BaseMint, 0)
		return q.AmountIn, q.AmountOut, e
	case instruction.PUMPSWAP_PROGRAM:
		r, e := s.calculatePumpSwap(h, ctx, payer, amount, 0, true)
		return r.Quote.AmountIn, r.Quote.AmountOut, e
	case instruction.PUMPFUN_PROGRAM:
		p, e := s.PumpFun(h, ctx)
		if e != nil {
			return 0, 0, e
		}
		if p.Quote == pumpWSOL {
			return 0, 0, errors.New("PumpFun native quote requires cached trade settlement")
		}
		q, e := QuoteCachedPumpFunExactIn(p, amount, h.InputMint == p.Quote, 0)
		return q.AmountIn, q.EstimatedNetAmountOut, e
	case instruction.BONK_PROGRAM:
		a, p, e := s.LaunchLabCurve(h, ctx)
		if e != nil {
			return 0, 0, e
		}
		q, e := calc.QuoteLaunchLabExactIn(p, amount, h.InputMint == a.QuoteMint, 0, 0)
		return q.AmountIn, q.MinimumAmountOut, e
	default:
		return 0, 0, errors.New("pool protocol has no native cached quote implementation")
	}
}
