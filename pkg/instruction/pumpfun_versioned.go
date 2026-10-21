package instruction

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

func pumpFunVersionedBuy(p *PumpFunBuildBuyParams) ([]solana.Instruction, error) {
	switch p.ProtocolParams.Version {
	case constants.PumpFunV3:
		amount, fixed := p.InputAmount, p.FixedOutputAmount
		if !p.UseExactSolAmount && fixed == nil {
			if !pumpFunIsSolQuoteMint(pumpFunEffectiveQuoteMint(p.ProtocolParams)) {
				return nil, fmt.Errorf("token-quoted V3 maximum-input buys require an explicit fixed output")
			}
			c := p.ProtocolParams.BondingCurve
			if c == nil || amount == 0 || p.SlippageBasisPoints > 9999 {
				return nil, fmt.Errorf("invalid PumpFun state, amount or slippage")
			}
			creator := pumpFunUsablePubkey(pumpFunEffectiveCreator(p.ProtocolParams))
			output := calc.GetBuyTokenAmountFromSolAmount(c.VirtualTokenReserves, c.VirtualSolReserves, c.RealTokenReserves, creator, amount)
			if output >= c.RealTokenReserves {
				return nil, fmt.Errorf("graduating V3 buy requires an explicit output limit")
			}
			var err error
			amount, err = calc.CalculateWithSlippageBuy(amount, p.SlippageBasisPoints)
			if err != nil {
				return nil, err
			}
			fixed = &output
		}
		return BuildPumpFunV3Instructions(p.Payer, p.OutputMint, p.InputMint, amount, p.SlippageBasisPoints, p.ProtocolParams, true, fixed, p.CreateOutputMintAta, p.CreateInputMintAta, false)
	case constants.PumpFunV2:
		return PumpFunBuildBuyV2Instructions(p)
	case constants.PumpFunLegacy:
		if !pumpFunIsSolQuoteMint(pumpFunEffectiveQuoteMint(p.ProtocolParams)) {
			return nil, fmt.Errorf("PumpFun V1 requires native SOL")
		}
		if !p.InputMint.IsZero() && p.InputMint != constants.SOL_TOKEN_ACCOUNT {
			return nil, fmt.Errorf("PumpFun V1 buys settle wallet SOL; use cached trade preparation for token settlement")
		}
		copyParams, protocol := *p, *p.ProtocolParams
		protocol.Version = ""
		copyParams.ProtocolParams = &protocol
		return PumpFunBuildBuyInstructions(&copyParams)
	default:
		return nil, fmt.Errorf("unknown PumpFun version %q", p.ProtocolParams.Version)
	}
}

func pumpFunVersionedSell(p *PumpFunBuildSellParams) ([]solana.Instruction, error) {
	switch p.ProtocolParams.Version {
	case constants.PumpFunV3:
		return BuildPumpFunV3Instructions(p.Payer, p.InputMint, p.OutputMint, p.InputAmount, p.SlippageBasisPoints, p.ProtocolParams, false, p.FixedOutputAmount, p.CreateOutputMintAta, false, p.CloseInputMintAta)
	case constants.PumpFunV2:
		return PumpFunBuildSellV2Instructions(p)
	case constants.PumpFunLegacy:
		if !pumpFunIsSolQuoteMint(pumpFunEffectiveQuoteMint(p.ProtocolParams)) {
			return nil, fmt.Errorf("PumpFun V1 requires native SOL")
		}
		if !p.OutputMint.IsZero() && p.OutputMint != constants.SOL_TOKEN_ACCOUNT {
			return nil, fmt.Errorf("PumpFun V1 sells settle wallet SOL; use cached trade preparation for token settlement")
		}
		copyParams, protocol := *p, *p.ProtocolParams
		protocol.Version = ""
		copyParams.ProtocolParams = &protocol
		return PumpFunBuildSellInstructions(&copyParams)
	default:
		return nil, fmt.Errorf("unknown PumpFun version %q", p.ProtocolParams.Version)
	}
}

// BuildPumpFunV3Instructions consumes caller-owned state only; it never fetches RPC data.
// Native curves settle wallet SOL. Token-quoted curves require verified zero-fee snapshots.
func BuildPumpFunV3Instructions(payer, mint, settlement solana.PublicKey, amount, slippage uint64, p *PumpFunParams, buy bool, fixed *uint64, createOutput, createInput, closeInput bool) ([]solana.Instruction, error) {
	if p == nil || p.BondingCurve == nil {
		return nil, fmt.Errorf("missing PumpFun state")
	}
	c := p.BondingCurve
	quote := pumpFunEffectiveQuoteMint(p)
	native := pumpFunIsSolQuoteMint(quote)
	if (!native && settlement != quote) || (native && settlement != constants.SOL_TOKEN_ACCOUNT) {
		return nil, fmt.Errorf("PumpFun V3 quote endpoint mismatch; native curves settle wallet SOL, not WSOL tokens")
	}
	if c.Account != GetBondingCurvePDA(mint) || c.Complete || c.IsCashbackCoin {
		return nil, fmt.Errorf("PumpFun V3 requires matching active non-cashback curve")
	}
	quoteProgram := p.QuoteTokenProgram
	if native {
		quoteProgram = solana.TokenProgramID
	}
	validProgram := func(k solana.PublicKey) bool { return k == solana.TokenProgramID || k == solana.Token2022ProgramID }
	if !validProgram(p.TokenProgram) || !validProgram(quoteProgram) {
		return nil, fmt.Errorf("PumpFun V3 requires actual mint token programs")
	}
	if !native && (p.BaseTransferFeeBps == nil || p.QuoteTransferFeeBps == nil || p.MinimumOutputAmount == nil) {
		return nil, fmt.Errorf("token-quoted V3 requires minimum output and verified zero transfer fees")
	}
	if (p.BaseTransferFeeBps != nil && *p.BaseTransferFeeBps != 0) || (p.QuoteTransferFeeBps != nil && *p.QuoteTransferFeeBps != 0) {
		return nil, fmt.Errorf("PumpFun V3 transfer-fee mints unsupported")
	}
	if amount == 0 || slippage > 9999 {
		return nil, fmt.Errorf("invalid amount or slippage")
	}
	minimum := p.MinimumOutputAmount
	if fixed != nil {
		minimum = fixed
	}
	if minimum == nil {
		creator := pumpFunUsablePubkey(pumpFunEffectiveCreator(p))
		var estimated uint64
		if buy {
			estimated = calc.GetBuyTokenAmountFromSolAmount(c.VirtualTokenReserves, c.VirtualSolReserves, c.RealTokenReserves, creator, amount)
			if estimated >= c.RealTokenReserves {
				return nil, fmt.Errorf("graduating V3 buy requires explicit minimum")
			}
		} else {
			estimated = calc.GetSellSolAmountFromTokenAmount(c.VirtualTokenReserves, c.VirtualSolReserves, creator, amount)
		}
		value, err := calc.CalculateWithSlippageSell(estimated, slippage)
		if err != nil {
			return nil, err
		}
		minimum = &value
	}
	if *minimum == 0 {
		return nil, fmt.Errorf("PumpFun V3 requires nonzero minimum")
	}
	recipient := p.BuybackRecipient
	if recipient.IsZero() {
		recipient = GetPumpFunBuybackFeeRecipientRandom()
	}
	accounts, err := DerivePumpV3Accounts(PumpCompactAccountParams{User: payer, BaseMint: mint, QuoteMint: quote, BaseTokenProgram: p.TokenProgram, QuoteTokenProgram: quoteProgram, BuybackRecipient: recipient})
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{}
	if buy && createOutput {
		out = append(out, CreateAssociatedTokenAccountIdempotent(payer, payer, mint, p.TokenProgram))
	}
	if !native && ((buy && createInput) || (!buy && createOutput)) {
		out = append(out, CreateAssociatedTokenAccountIdempotent(payer, payer, quote, quoteProgram))
	}
	var swap solana.Instruction
	partial := false
	if buy && fixed != nil {
		swap, err = BuildPumpBuyV3Instruction(accounts, *fixed, amount, &partial)
	} else if buy {
		swap, err = BuildPumpBuyExactQuoteInV3Instruction(accounts, amount, *minimum, &partial)
	} else {
		swap, err = BuildPumpSellV3Instruction(accounts, amount, *minimum)
	}
	if err != nil {
		return nil, err
	}
	out = append(out, swap)
	if !buy && (closeInput || (p.CloseTokenAccountWhenSell != nil && *p.CloseTokenAccountWhenSell)) {
		close := token.NewCloseAccountInstruction(GetAssociatedTokenAddress(payer, mint, p.TokenProgram), payer, payer, nil).Build()
		close.SetProgramID(p.TokenProgram)
		out = append(out, close)
	}
	return out, nil
}
