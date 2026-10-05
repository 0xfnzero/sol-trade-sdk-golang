package seed_test

import (
	"bytes"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/security"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/seed"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestPublicPdaParity(t *testing.T) {
	for name, pk := range map[string]solana.PublicKey{"pumpfun": constants.PUMPFUN_PROGRAM_ID, "pumpswap": constants.PUMPSWAP_PROGRAM_ID, "raydium": constants.RAYDIUM_CPMM_PROGRAM_ID, "meteora": constants.METEORA_DAMM_V2_PROGRAM_ID} {
		if e := security.ValidateProgramID(pk.String(), name); e != nil {
			t.Fatal(e)
		}
	}
	if constants.PUMPFUN_PROGRAM_ID != instruction.PUMPFUN_PROGRAM || seed.PumpFunProgramID != instruction.PUMPFUN_PROGRAM {
		t.Fatal("program mismatch")
	}
	if constants.PUMPSWAP_PROGRAM_ID != seed.PumpSwapProgramID {
		t.Fatal("program mismatch")
	}
	if !bytes.Equal(constants.BUY_DISCRIMINATOR[:], instruction.PumpFunBuyDiscriminator) || !bytes.Equal(constants.SELL_DISCRIMINATOR[:], instruction.PumpFunSellDiscriminator) {
		t.Fatal("discriminator mismatch")
	}
	global, _, e := seed.GetGlobalAccountPDA()
	if e != nil || global != instruction.PUMPFUN_GLOBAL_ACCOUNT || instruction.GetGlobalAccount() != global {
		t.Fatal("global mismatch")
	}
	event, _, e := seed.GetEventAuthorityPDA()
	if e != nil || event != instruction.PUMPFUN_EVENT_AUTHORITY || instruction.GetEventAuthority() != event {
		t.Fatal("event mismatch")
	}
	curve, _, e := seed.GetBondingCurvePDA(constants.USDC_TOKEN_ACCOUNT)
	if e != nil || curve != instruction.GetBondingCurvePDA(constants.USDC_TOKEN_ACCOUNT) {
		t.Fatal("curve mismatch")
	}
	if len(constants.MAYHEM_FEE_RECIPIENTS) != 8 {
		t.Fatal("incomplete mayhem recipients")
	}
	if e = security.ValidateProgramID("6EF8rrecthR5Dkzon8Nwu78hRvfCKopJFfWcCzNfXt3D", "pumpfun"); e == nil {
		t.Fatal("accepted wrong address")
	}
	creator, base, quote := constants.USDC_TOKEN_ACCOUNT, constants.WSOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT
	expected, bump, e := solana.FindProgramAddress([][]byte{[]byte("pool"), {1, 1}, creator[:], base[:], quote[:]}, constants.PUMPSWAP_PROGRAM_ID)
	if e != nil {
		t.Fatal(e)
	}
	got, gotBump, e := seed.GetPumpSwapPoolPDA(base, quote, 257, creator)
	if e != nil || got != expected || gotBump != bump || instruction.GetPoolPDA(base, quote, 257, creator) != expected {
		t.Fatal("pool seeds mismatch")
	}
	if _, _, e = seed.GetFeeRecipientPDA(false); e == nil {
		t.Fatal("invented fee recipient PDA")
	}
	if _, _, e = seed.GetMeteoraPoolPDA(base, quote); e == nil {
		t.Fatal("invented two-mint DAMM pool")
	}
}
