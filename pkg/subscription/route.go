package subscription

// Independent buy/sell route. Fixed inputs use preceding protected net credits;
// intermediate residual balances are reported rather than spent from the wallet.
import (
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
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
	Legs                                []CachedRouteLeg
	SetupInstructions, SwapInstructions []solana.Instruction
	MinimumNetAmountOut                 uint64
	EstimatedIntermediateResiduals      []RouteResidual
}

func (s *AccountCacheSnapshot) PrepareRoute(hints []PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int, allowPumpFunNative ...bool) (PreparedCachedRoute, error) {
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
	spend := amount
	for _, h := range hints {
		p, e := s.Get(h.Pool, ctx, nil)
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
			return PreparedCachedRoute{}, e
		}
		if used == 0 || used > spend || minimum == 0 {
			return PreparedCachedRoute{}, errors.New("route has zero output or overconsumes input")
		}
		result.Legs = append(result.Legs, CachedRouteLeg{Hint: h, AmountIn: used, EstimatedNetAmountOut: &estimated, MinimumNetAmountOut: minimum, Instruction: ix})
		result.SwapInstructions = append(result.SwapInstructions, ix)
		spend = minimum
	}
	for _, m := range mints {
		a, e := s.Get(m, ctx, nil)
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
