package instruction

import (
	"encoding/binary"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestMeteoraDammV2AccountLayoutOptions(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	pool := solana.NewWallet().PublicKey()
	referral := solana.NewWallet().PublicKey()
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	mint := solana.NewWallet().PublicKey()
	tokenProg := solana.TokenProgramID
	fixed := uint64(123)

	base := &MeteoraDammV2Params{
		Pool:          pool,
		TokenAMint:    wsol,
		TokenBMint:    mint,
		TokenAVault:   solana.NewWallet().PublicKey(),
		TokenBVault:   solana.NewWallet().PublicKey(),
		TokenAProgram: tokenProg,
		TokenBProgram: tokenProg,
	}

	buy := &MeteoraDammV2BuildBuyParams{
		Payer:               payer,
		InputMint:           wsol,
		OutputMint:          mint,
		InputAmount:         1_000_000,
		FixedOutputAmount:   &fixed,
		CreateInputMintAta:  false,
		CreateOutputMintAta: false,
		ProtocolParams:      base,
	}

	ixs, err := MeteoraDammV2BuildBuyInstructions(buy)
	if err != nil {
		t.Fatal(err)
	}
	swap := ixs[len(ixs)-1]
	if len(swap.Accounts()) != 14 {
		t.Fatalf("default accounts want 14, got %d", len(swap.Accounts()))
	}
	if swap.Accounts()[11].PublicKey != METEORA_DAMM_V2_PROGRAM || swap.Accounts()[11].IsWritable || swap.Accounts()[8].IsWritable {
		t.Fatal("no-referral slot or payer privileges differ from Rust")
	}
	data, err := swap.Data()
	if err != nil {
		t.Fatal(err)
	}
	if data[24] != MeteoraDammV2SwapModePartialFill {
		t.Fatalf("default swap mode")
	}

	base.ReferralTokenAccount = &referral
	base.IncludeRateLimiterSysvar = true
	exactOut := MeteoraDammV2SwapModeExactOut
	base.SwapMode = &exactOut
	ixs, err = MeteoraDammV2BuildBuyInstructions(buy)
	if err != nil {
		t.Fatal(err)
	}
	swap = ixs[len(ixs)-1]
	accounts := swap.Accounts()
	if len(accounts) != 15 {
		t.Fatalf("referral+sysvar accounts want 15, got %d", len(accounts))
	}
	// referral before event authority / program
	if !accounts[11].PublicKey.Equals(referral) || !accounts[11].IsWritable {
		t.Fatalf("referral should be at index 11 writable")
	}
	if !accounts[14].PublicKey.Equals(METEORA_DAMM_V2_SYSVAR_INSTRUCTIONS) {
		t.Fatalf("sysvar should be last")
	}
	data, err = swap.Data()
	if err != nil {
		t.Fatal(err)
	}
	if data[24] != MeteoraDammV2SwapModeExactOut {
		t.Fatalf("exact-out mode")
	}
	amount0 := binary.LittleEndian.Uint64(data[8:16])
	amount1 := binary.LittleEndian.Uint64(data[16:24])
	if amount0 != fixed || amount1 != 1_000_000 {
		t.Fatalf("exact-out amount order: got %d,%d", amount0, amount1)
	}
}

func TestMeteoraRejectsInvalidExplicitMode(t *testing.T) {
	invalid := uint8(255)
	minimum := uint64(1)
	p := &MeteoraDammV2Params{TokenAMint: solana.WrappedSol, TokenBMint: solana.NewWallet().PublicKey(), SwapMode: &invalid}
	_, err := MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{InputAmount: 10000, InputMint: p.TokenAMint, OutputMint: p.TokenBMint, ProtocolParams: p, FixedOutputAmount: &minimum})
	if err == nil {
		t.Fatal("invalid explicit mode accepted")
	}
}
