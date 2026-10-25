package subscription

import (
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/instruction"
	"github.com/gagliardetto/solana-go"
)

// PrepareOfficialPumpRoute keeps native temporary-account settlement on its existing path.
// Official routes have an aggregate minimum; no per-hop quote is invented.
func (s *AccountCacheSnapshot) PrepareOfficialPumpRoute(hints []PoolTradeHint, ctx CacheReadContext, payer solana.PublicKey, amount uint64, options instruction.PumpRouteOptions, nativeSettlement bool) (*PreparedCachedRoute, error) {
	if len(hints) == 0 {
		return nil, errors.New("Pump route requires hops")
	}
	input, output := hints[0].InputMint, hints[len(hints)-1].OutputMint
	fallback := func() (*PreparedCachedRoute, error) {
		_, err := instruction.SelectPumpRoute(options, payer, input, output, amount, nil)
		return nil, err
	}
	if options.Execution == instruction.PumpRouteSequential || options.MinimumOutputAmount == nil || options.BuybackRecipient == nil || nativeSettlement || len(hints) < 2 || len(hints) > 3 {
		return fallback()
	}
	programs := make([]solana.PublicKey, len(hints))
	for i, h := range hints {
		a, err := s.Get(h.Pool, ctx, nil)
		if err != nil {
			return nil, err
		}
		programs[i] = a.Owner
		if a.Owner != instruction.PUMPFUN_PROGRAM && a.Owner != instruction.PUMPSWAP_PROGRAM {
			return fallback()
		}
	}
	hops := make([]instruction.PumpMultiHop, 0, len(hints))
	for i, h := range hints {
		if i > 0 && hints[i-1].OutputMint != h.InputMint {
			return fallback()
		}
		if programs[i] == instruction.PUMPFUN_PROGRAM {
			state, err := s.PumpFun(h, ctx)
			if err != nil {
				return nil, err
			}
			c := state.Curve
			hops = append(hops, instruction.PumpMultiHop{Venue: "curve", BaseMint: state.Mint, QuoteMint: state.Quote, Address: h.Pool,
				BaseVault:        instruction.GetAssociatedTokenAddress(h.Pool, state.Mint, state.TokenProgram),
				QuoteVault:       instruction.GetAssociatedTokenAddress(h.Pool, state.Quote, state.QuoteTokenProgram),
				BaseTokenProgram: state.TokenProgram, QuoteTokenProgram: state.QuoteTokenProgram,
				Mayhem: c.IsMayhemMode, Cashback: c.IsCashbackCoin, Complete: c.Complete})
		} else {
			state, err := s.PumpSwap(h, ctx)
			if err != nil {
				return nil, err
			}
			if state.BaseTransferFee.BasisPoints != 0 && state.BaseTransferFee.MaximumFee != 0 || state.QuoteTransferFee.BasisPoints != 0 && state.QuoteTransferFee.MaximumFee != 0 {
				return fallback()
			}
			p := state.Pool
			flag := uint8(16)
			if h.InputMint == p.QuoteMint {
				flag = 8
			}
			if state.DisableFlags&flag != 0 {
				return nil, errors.New("PumpSwap direction is disabled")
			}
			recipientValid := false
			for _, k := range state.BuybackFeeRecipients {
				if k == *options.BuybackRecipient {
					recipientValid = true
					break
				}
			}
			if !recipientValid {
				return nil, errors.New("Pump buyback recipient is not in current GlobalConfig")
			}
			hops = append(hops, instruction.PumpMultiHop{Venue: "pool", BaseMint: p.BaseMint, QuoteMint: p.QuoteMint, Address: h.Pool,
				BaseVault: p.PoolBaseTokenAccount, QuoteVault: p.PoolQuoteTokenAccount,
				BaseTokenProgram: state.BaseTokenProgram, QuoteTokenProgram: state.QuoteTokenProgram,
				Mayhem: p.IsMayhemMode, Cashback: p.IsCashbackCoin, Index: p.Index, Creator: p.Creator})
		}
	}
	buy := input == hops[0].QuoteMint
	first, last := hops[0], hops[len(hops)-1]
	currency := last
	inputToken, outputToken := first.BaseTokenProgram, last.QuoteTokenProgram
	if buy {
		currency = first
		inputToken = first.QuoteTokenProgram
		outputToken = last.BaseTokenProgram
	}
	if currency.Venue == "curve" && currency.QuoteMint == solana.WrappedSol {
		return fallback()
	}
	ix, err := instruction.SelectPumpRoute(options, payer, input, output, amount, hops)
	if err != nil || ix == nil {
		return nil, err
	}
	setup := []solana.Instruction{
		instruction.CreateAssociatedTokenAccountIdempotent(payer, payer, input, inputToken),
		instruction.CreateAssociatedTokenAccountIdempotent(payer, payer, output, outputToken),
		instruction.CreateAssociatedTokenAccountIdempotent(payer, *options.BuybackRecipient, currency.QuoteMint, currency.QuoteTokenProgram),
	}
	if err := s.AssertUsable(); err != nil {
		return nil, err
	}
	return &PreparedCachedRoute{Execution: instruction.PumpRouteOfficial, SetupInstructions: setup, SwapInstructions: []solana.Instruction{ix}, MinimumNetAmountOut: *options.MinimumOutputAmount}, nil
}
