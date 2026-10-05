package common

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/seed"
	"github.com/gagliardetto/solana-go"
)

// NormalizeQuoteMint uses WSOL for the legacy native-SOL selector.
func NormalizeQuoteMint(mint [32]byte) [32]byte {
	if mint == [32]byte{} || mint == [32]byte(constants.SOL_TOKEN_ACCOUNT) {
		return [32]byte(constants.WSOL_TOKEN_ACCOUNT)
	}
	return mint
}
func InitialVirtualQuoteReservesForQuoteMint(mint [32]byte) uint64 {
	if mint == [32]byte(constants.USDC_TOKEN_ACCOUNT) {
		return 4292000000
	}
	return InitialVirtualSolReserves
}
func (b *BondingCurveAccount) EffectiveQuoteMint() [32]byte { return NormalizeQuoteMint(b.QuoteMint) }
func (b *BondingCurveAccount) VirtualQuoteReserves() uint64 { return b.VirtualSolReserves }
func (b *BondingCurveAccount) RealQuoteReserves() uint64    { return b.RealSolReserves }
func curveSaturatingAdd(a, b uint64) uint64 {
	if b > ^uint64(0)-a {
		return ^uint64(0)
	}
	return a + b
}
func (b *BondingCurveAccount) WithQuoteMint(mint [32]byte) *BondingCurveAccount {
	normalized := NormalizeQuoteMint(mint)
	previous := InitialVirtualQuoteReservesForQuoteMint(b.EffectiveQuoteMint())
	if b.VirtualSolReserves == curveSaturatingAdd(previous, b.RealSolReserves) {
		b.VirtualSolReserves = curveSaturatingAdd(InitialVirtualQuoteReservesForQuoteMint(normalized), b.RealSolReserves)
	}
	b.QuoteMint = normalized
	return b
}
func (b *BondingCurveAccount) GetCreatorVaultPDA() (solana.PublicKey, uint8, error) {
	return seed.FindProgramAddress([][]byte{[]byte("creator-vault"), b.Creator[:]}, constants.PUMPFUN_PROGRAM_ID)
}

// NewBondingCurveFromTradeWithQuoteMint reconstructs observed reserves, not current fee configuration.
func NewBondingCurveFromTradeWithQuoteMint(account, mint, creator [32]byte, virtualToken, virtualQuote, realToken, realQuote uint64, mayhem, cashback bool, quoteMint [32]byte) (*BondingCurveAccount, error) {
	if account == [32]byte{} {
		p, _, err := seed.GetBondingCurvePDA(solana.PublicKey(mint))
		if err != nil {
			return nil, err
		}
		account = [32]byte(p)
	}
	return &BondingCurveAccount{Account: account, Creator: creator, VirtualTokenReserves: virtualToken, VirtualSolReserves: virtualQuote, RealTokenReserves: realToken, RealSolReserves: realQuote, TokenTotalSupply: TokenTotalSupply, IsMayhemMode: mayhem, IsCashbackCoin: cashback, QuoteMint: NormalizeQuoteMint(quoteMint)}, nil
}
func NewBondingCurveFromTrade(account, mint, creator [32]byte, virtualToken, virtualSol, realToken, realSol uint64, mayhem, cashback bool) (*BondingCurveAccount, error) {
	return NewBondingCurveFromTradeWithQuoteMint(account, mint, creator, virtualToken, virtualSol, realToken, realSol, mayhem, cashback, [32]byte{})
}
func NewBondingCurveFromDevTradeWithQuoteMint(account, mint, creator [32]byte, token, quote uint64, mayhem, cashback bool, quoteMint [32]byte) (*BondingCurveAccount, error) {
	normalized := NormalizeQuoteMint(quoteMint)
	initial := InitialVirtualQuoteReservesForQuoteMint(normalized)
	if token > InitialRealTokenReserves || quote > ^uint64(0)-initial {
		return nil, fmt.Errorf("Invalid dev trade reserves")
	}
	return NewBondingCurveFromTradeWithQuoteMint(account, mint, creator, InitialVirtualTokenReserves-token, initial+quote, InitialRealTokenReserves-token, quote, mayhem, cashback, normalized)
}
func NewBondingCurveFromDevTrade(account, mint, creator [32]byte, token, sol uint64, mayhem, cashback bool) (*BondingCurveAccount, error) {
	return NewBondingCurveFromDevTradeWithQuoteMint(account, mint, creator, token, sol, mayhem, cashback, [32]byte{})
}
