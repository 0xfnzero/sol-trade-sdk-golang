package common

import (
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/seed"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestQuoteAwareCurveReconstruction(t *testing.T) {
	mint, creator := [32]byte{2}, [32]byte{3}
	b, err := NewBondingCurveFromDevTradeWithQuoteMint([32]byte{}, mint, creator, 100, 200, false, true, [32]byte(constants.USDC_TOKEN_ACCOUNT))
	if err != nil {
		t.Fatal(err)
	}
	address, _, _ := seed.GetBondingCurvePDA(solana.PublicKey(mint))
	if b.Account != [32]byte(address) || b.VirtualQuoteReserves() != 4292000200 || b.RealQuoteReserves() != 200 || b.TokenTotalSupply != TokenTotalSupply {
		t.Fatal("curve reconstruction mismatch")
	}
	vault, _, err := b.GetCreatorVaultPDA()
	expected, _, _ := seed.FindProgramAddress([][]byte{[]byte("creator-vault"), creator[:]}, constants.PUMPFUN_PROGRAM_ID)
	if err != nil || vault != expected {
		t.Fatal("creator vault mismatch")
	}
	b.WithQuoteMint([32]byte(constants.SOL_TOKEN_ACCOUNT))
	if b.VirtualSolReserves != 30000000200 || b.EffectiveQuoteMint() != [32]byte(constants.WSOL_TOKEN_ACCOUNT) {
		t.Fatal("quote transition")
	}
	b.VirtualSolReserves = 123
	b.WithQuoteMint([32]byte(constants.USDC_TOKEN_ACCOUNT))
	if b.VirtualSolReserves != 123 {
		t.Fatal("changed observed reserves")
	}
	for _, amount := range []struct{ token, quote uint64 }{{InitialRealTokenReserves + 1, 0}, {0, ^uint64(0)}} {
		if _, err := NewBondingCurveFromDevTrade([32]byte{}, mint, creator, amount.token, amount.quote, false, false); err == nil {
			t.Fatal("invalid dev trade accepted")
		}
	}
	b.VirtualSolReserves = ^uint64(0)
	b.RealSolReserves = ^uint64(0)
	b.WithQuoteMint([32]byte(constants.USDC_TOKEN_ACCOUNT))
	if b.VirtualSolReserves != ^uint64(0) {
		t.Fatal("saturation")
	}
	var zero BondingCurveAccount
	if zero.VirtualQuoteReserves() != 0 || zero.TokenTotalSupply != 0 || zero.EffectiveQuoteMint() != [32]byte(constants.WSOL_TOKEN_ACCOUNT) {
		t.Fatal("default mismatch")
	}
}
