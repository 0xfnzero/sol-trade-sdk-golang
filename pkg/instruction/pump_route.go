package instruction

import (
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

// PumpRouteExecution is independent of the Pump.fun single-trade version.
type PumpRouteExecution string

const (
	PumpRouteAuto       PumpRouteExecution = "auto"
	PumpRouteOfficial   PumpRouteExecution = "official"
	PumpRouteSequential PumpRouteExecution = "sequential"
)

type PumpRouteOptions struct {
	Execution PumpRouteExecution
	// Fresh final recipient credit. Sequential hop quotes are not official quotes.
	MinimumOutputAmount *uint64
	BuybackRecipient    *solana.PublicKey
}

// SelectPumpRoute returns nil for sequential execution. No RPC or submission retry.
func SelectPumpRoute(options PumpRouteOptions, user, input, output solana.PublicKey, amount uint64, hops []PumpMultiHop) (solana.Instruction, error) {
	execution := options.Execution
	if execution == "" {
		execution = PumpRouteAuto
	}
	if execution != PumpRouteAuto && execution != PumpRouteOfficial && execution != PumpRouteSequential {
		return nil, errors.New("unknown Pump route execution")
	}
	if amount == 0 {
		return nil, errors.New("Pump route input must be nonzero")
	}
	if execution == PumpRouteSequential {
		return nil, nil
	}
	if options.MinimumOutputAmount != nil && *options.MinimumOutputAmount == 0 {
		return nil, errors.New("official Pump route minimum must be nonzero")
	}
	if options.BuybackRecipient != nil && options.BuybackRecipient.IsZero() {
		return nil, errors.New("missing Pump buyback recipient")
	}
	if execution == PumpRouteAuto && (len(hops) < 2 || len(hops) > 3 || options.MinimumOutputAmount == nil || options.BuybackRecipient == nil) {
		return nil, nil
	}
	candidate := func() (solana.Instruction, error) {
		if len(hops) < 2 || len(hops) > 3 {
			return nil, errors.New("official Pump routes require two or three supported hops")
		}
		if options.MinimumOutputAmount == nil || options.BuybackRecipient == nil {
			return nil, errors.New("provide an official final minimum and current buyback recipient")
		}
		if user.IsZero() || input == output {
			return nil, errors.New("invalid Pump route endpoints")
		}
		seen := map[solana.PublicKey]bool{input: true}
		current := input
		var previous solana.PublicKey
		for _, h := range hops {
			for _, p := range []solana.PublicKey{h.BaseTokenProgram, h.QuoteTokenProgram} {
				if p != constants.TOKEN_PROGRAM && p != constants.TOKEN_PROGRAM_2022 {
					return nil, errors.New("unknown Pump route token program")
				}
			}
			next, tokenIn, tokenOut := h.QuoteMint, h.BaseTokenProgram, h.QuoteTokenProgram
			if current == h.QuoteMint {
				next, tokenIn, tokenOut = h.BaseMint, h.QuoteTokenProgram, h.BaseTokenProgram
			} else if current != h.BaseMint {
				return nil, errors.New("disconnected Pump route")
			}
			if !previous.IsZero() && previous != tokenIn {
				return nil, errors.New("inconsistent intermediate token program")
			}
			if seen[next] {
				return nil, errors.New("Pump route contains an asset cycle")
			}
			if h.Address.IsZero() || h.BaseVault.IsZero() || h.QuoteVault.IsZero() || h.BaseVault == h.QuoteVault {
				return nil, errors.New("incomplete Pump route venue")
			}
			seen[next] = true
			current, previous = next, tokenOut
		}
		accounts, remaining, err := DerivePumpMultiHopAccounts(user, input, output, *options.BuybackRecipient, hops, false)
		if err != nil {
			return nil, err
		}
		return BuildPumpAmmMultiHopSwapInstruction(accounts, amount, *options.MinimumOutputAmount, remaining)
	}
	ix, err := candidate()
	if err != nil && execution == PumpRouteAuto {
		return nil, nil
	}
	return ix, err
}
