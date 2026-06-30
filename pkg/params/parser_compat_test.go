package params

import (
	"testing"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

func TestNewPumpFunParamsFromParserTradeUsesQuoteReserves(t *testing.T) {
	p, err := NewPumpFunParamsFromParserTrade(PumpFunParserTradeEvent{
		QuoteMint:            constants.USDC_TOKEN_ACCOUNT.String(),
		TokenProgram:         constants.TOKEN_PROGRAM.String(),
		VirtualTokenReserves: 1_000_000,
		VirtualSolReserves:   30_000_000_000,
		VirtualQuoteReserves: 4_292_000_000,
		RealTokenReserves:    900_000,
		RealSolReserves:      20_000_000_000,
		RealQuoteReserves:    123_456,
		IsCashbackCoin:       true,
	})
	if err != nil {
		t.Fatalf("NewPumpFunParamsFromParserTrade error: %v", err)
	}
	if !p.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT) {
		t.Fatalf("quote mint = %s", p.QuoteMint)
	}
	if p.BondingCurve.VirtualSolReserves != 4_292_000_000 {
		t.Fatalf("virtual quote reserve not mapped: %d", p.BondingCurve.VirtualSolReserves)
	}
	if p.BondingCurve.RealSolReserves != 123_456 {
		t.Fatalf("real quote reserve not mapped: %d", p.BondingCurve.RealSolReserves)
	}
	if !p.BondingCurve.IsCashbackCoin {
		t.Fatal("cashback flag not mapped")
	}
}

func TestNewPumpFunParamsFromParserTradePreservesZeroQuoteReserves(t *testing.T) {
	p, err := NewPumpFunParamsFromParserTrade(PumpFunParserTradeEvent{
		QuoteMint:            constants.USDC_TOKEN_ACCOUNT.String(),
		TokenProgram:         constants.TOKEN_PROGRAM.String(),
		VirtualTokenReserves: 1_000_000,
		VirtualSolReserves:   30_000_000_000,
		VirtualQuoteReserves: 0,
		RealTokenReserves:    900_000,
		RealSolReserves:      20_000_000_000,
		RealQuoteReserves:    0,
	})
	if err != nil {
		t.Fatalf("NewPumpFunParamsFromParserTrade error: %v", err)
	}
	if p.BondingCurve.VirtualSolReserves != 0 {
		t.Fatalf("virtual quote reserve should remain zero: %d", p.BondingCurve.VirtualSolReserves)
	}
	if p.BondingCurve.RealSolReserves != 0 {
		t.Fatalf("real quote reserve should remain zero: %d", p.BondingCurve.RealSolReserves)
	}
}

func TestNewPumpFunParamsFromParserTradeSolscanSolUsesLegacyReserves(t *testing.T) {
	p, err := NewPumpFunParamsFromParserTrade(PumpFunParserTradeEvent{
		QuoteMint:            constants.SOL_TOKEN_ACCOUNT.String(),
		TokenProgram:         constants.TOKEN_PROGRAM.String(),
		VirtualTokenReserves: 1_000_000,
		VirtualSolReserves:   30_123_456_789,
		VirtualQuoteReserves: 0,
		RealTokenReserves:    900_000,
		RealSolReserves:      123_456_789,
		RealQuoteReserves:    0,
	})
	if err != nil {
		t.Fatalf("NewPumpFunParamsFromParserTrade error: %v", err)
	}
	if !p.QuoteMint.IsZero() {
		t.Fatalf("SOL sentinel should keep legacy layout quote mint, got %s", p.QuoteMint)
	}
	if p.BondingCurve.VirtualSolReserves != 30_123_456_789 {
		t.Fatalf("virtual sol reserve not preserved: %d", p.BondingCurve.VirtualSolReserves)
	}
	if p.BondingCurve.RealSolReserves != 123_456_789 {
		t.Fatalf("real sol reserve not preserved: %d", p.BondingCurve.RealSolReserves)
	}
}

func TestNewPumpSwapParamsFromParserTradeUsesCreatorVaultAccounts(t *testing.T) {
	vault := solana.NewWallet().PublicKey()
	authority := solana.NewWallet().PublicKey()
	p, err := NewPumpSwapParamsFromParserTrade(PumpSwapParserTradeEvent{
		Pool:                      solana.NewWallet().PublicKey().String(),
		BaseMint:                  solana.NewWallet().PublicKey().String(),
		QuoteMint:                 constants.USDC_TOKEN_ACCOUNT.String(),
		PoolBaseTokenAccount:      solana.NewWallet().PublicKey().String(),
		PoolQuoteTokenAccount:     solana.NewWallet().PublicKey().String(),
		PoolBaseTokenReserves:     10,
		PoolQuoteTokenReserves:    20,
		CoinCreatorVaultATA:       vault.String(),
		CoinCreatorVaultAuthority: authority.String(),
		BaseTokenProgram:          constants.TOKEN_PROGRAM.String(),
		QuoteTokenProgram:         constants.TOKEN_PROGRAM.String(),
	})
	if err != nil {
		t.Fatalf("NewPumpSwapParamsFromParserTrade error: %v", err)
	}
	if !p.CoinCreatorVaultATA.Equals(vault) {
		t.Fatalf("vault = %s", p.CoinCreatorVaultATA)
	}
	if !p.CoinCreatorVaultAuth.Equals(authority) {
		t.Fatalf("authority = %s", p.CoinCreatorVaultAuth)
	}
}

func TestNewPumpSwapParamsFromParserTradeUsesFeeBasisPoints(t *testing.T) {
	creator := solana.NewWallet().PublicKey()
	p, err := NewPumpSwapParamsFromParserTrade(PumpSwapParserTradeEvent{
		Pool:                      solana.NewWallet().PublicKey().String(),
		BaseMint:                  solana.NewWallet().PublicKey().String(),
		QuoteMint:                 constants.USDC_TOKEN_ACCOUNT.String(),
		PoolBaseTokenAccount:      solana.NewWallet().PublicKey().String(),
		PoolQuoteTokenAccount:     solana.NewWallet().PublicKey().String(),
		PoolBaseTokenReserves:     10,
		PoolQuoteTokenReserves:    20,
		CoinCreatorVaultATA:       solana.NewWallet().PublicKey().String(),
		CoinCreatorVaultAuthority: solana.NewWallet().PublicKey().String(),
		BaseTokenProgram:          constants.TOKEN_PROGRAM.String(),
		QuoteTokenProgram:         constants.TOKEN_PROGRAM.String(),
		CoinCreator:               creator.String(),
		CashbackFeeBasisPoints:    4,
		LPFeeBasisPoints:          20,
		ProtocolFeeBasisPoints:    5,
		CoinCreatorFeeBasisPoints: 75,
	})
	if err != nil {
		t.Fatalf("NewPumpSwapParamsFromParserTrade error: %v", err)
	}
	if !p.CoinCreator.Equals(creator) || !p.CoinCreatorKnown {
		t.Fatalf("coin creator not mapped")
	}
	if p.CashbackFeeBasisPoints != 4 {
		t.Fatalf("cashback fee bps = %d", p.CashbackFeeBasisPoints)
	}
	if p.FeeBasisPoints == nil {
		t.Fatal("fee basis points not mapped")
	}
	if p.FeeBasisPoints.LPFeeBasisPoints != 20 ||
		p.FeeBasisPoints.ProtocolFeeBasisPoints != 5 ||
		p.FeeBasisPoints.CoinCreatorFeeBasisPoints != 75 {
		t.Fatalf("fee bps = %+v", *p.FeeBasisPoints)
	}
}
