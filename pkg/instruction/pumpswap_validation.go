package instruction

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

// ValidateAccounts rejects incomplete log-only identities before constructing a swap.
// Enrich those identities with a verified pool snapshot first.
func (p *PumpSwapParams) ValidateAccounts() error {
	if p == nil {
		return fmt.Errorf("missing PumpSwap parameters")
	}
	for _, field := range []struct {
		name string
		key  solana.PublicKey
	}{
		{"pool", p.Pool}, {"base mint", p.BaseMint}, {"quote mint", p.QuoteMint},
		{"base vault", p.PoolBaseTokenAccount}, {"quote vault", p.PoolQuoteTokenAccount},
		{"creator vault authority", p.CoinCreatorVaultAuthority}, {"creator vault ATA", p.CoinCreatorVaultAta},
	} {
		if field.key.IsZero() {
			return fmt.Errorf("incomplete PumpSwap account snapshot: missing %s", field.name)
		}
	}
	if p.BaseMint == p.QuoteMint {
		return fmt.Errorf("identical PumpSwap pool mints")
	}
	if p.PoolBaseTokenAccount == p.PoolQuoteTokenAccount {
		return fmt.Errorf("PumpSwap pool token vaults must differ")
	}
	if !p.CoinCreatorKnown && p.CoinCreator.IsZero() {
		return fmt.Errorf("PumpSwap requires a known coin creator from a complete pool snapshot")
	}
	for _, recipient := range []*solana.PublicKey{p.ProtocolFeeRecipientOverride, p.ProtocolExtraFeeRecipientOverride} {
		if recipient != nil && recipient.IsZero() {
			return fmt.Errorf("PumpSwap fee recipient override must not be zero")
		}
	}
	for _, program := range []solana.PublicKey{p.BaseTokenProgram, p.QuoteTokenProgram} {
		if program != constants.TOKEN_PROGRAM && program != constants.TOKEN_PROGRAM_2022 {
			return fmt.Errorf("unsupported PumpSwap token program")
		}
	}
	authority := GetCoinCreatorVaultAuthority(p.CoinCreator)
	if p.CoinCreatorVaultAuthority != authority || p.CoinCreatorVaultAta != GetAssociatedTokenAddress(authority, p.QuoteMint, p.QuoteTokenProgram) {
		return fmt.Errorf("PumpSwap creator vault does not match creator and quote mint/program")
	}
	return nil
}

func pumpSwapDirection(p *PumpSwapParams, input, output solana.PublicKey, buy bool) (bool, error) {
	// Preserve the legacy default for callers omitting both endpoints. Explicit
	// endpoints always decide direction, including pools whose base is WSOL.
	if input.IsZero() && output.IsZero() {
		quoteFunding := p.QuoteMint == constants.WSOL_TOKEN_ACCOUNT || p.QuoteMint == constants.USDC_TOKEN_ACCOUNT
		if p.BaseMint != constants.WSOL_TOKEN_ACCOUNT && p.BaseMint != constants.USDC_TOKEN_ACCOUNT {
			quoteFunding = true
		}
		if buy {
			return quoteFunding, nil
		}
		return !quoteFunding, nil
	}
	if input == constants.SOL_TOKEN_ACCOUNT {
		input = constants.WSOL_TOKEN_ACCOUNT
	}
	if output == constants.SOL_TOKEN_ACCOUNT {
		output = constants.WSOL_TOKEN_ACCOUNT
	}
	if input == p.QuoteMint && output == p.BaseMint {
		return true, nil
	}
	if input == p.BaseMint && output == p.QuoteMint {
		return false, nil
	}
	return false, fmt.Errorf("PumpSwap requested mints do not match pool; provide an explicit conversion route")
}
