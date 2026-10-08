package instruction

import (
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestDammSolUsdcExplicitDirections(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	fixed := uint64(1)
	for _, mints := range [][2]solana.PublicKey{{constants.WSOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT}, {constants.USDC_TOKEN_ACCOUNT, constants.WSOL_TOKEN_ACCOUNT}} {
		pool := &MeteoraDammV2Params{Pool: solana.NewWallet().PublicKey(), TokenAMint: mints[0], TokenBMint: mints[1], TokenAVault: solana.NewWallet().PublicKey(), TokenBVault: solana.NewWallet().PublicKey(), TokenAProgram: constants.TOKEN_PROGRAM, TokenBProgram: constants.TOKEN_PROGRAM}
		for _, sell := range []bool{false, true} {
			input, output := constants.WSOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT
			if sell {
				input, output = output, input
			}
			var ixs []solana.Instruction
			var err error
			if sell {
				ixs, err = MeteoraDammV2BuildSellInstructions(&MeteoraDammV2BuildSellParams{Payer: payer, InputMint: input, OutputMint: output, InputAmount: 10000, FixedOutputAmount: &fixed, ProtocolParams: pool})
			} else {
				ixs, err = MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{Payer: payer, InputMint: input, OutputMint: output, InputAmount: 10000, FixedOutputAmount: &fixed, ProtocolParams: pool})
			}
			if err != nil {
				t.Fatal(err)
			}
			swap := ixs[len(ixs)-1]
			accounts := swap.Accounts()
			if !accounts[2].PublicKey.Equals(GetAssociatedTokenAddress(payer, input, constants.TOKEN_PROGRAM)) || !accounts[3].PublicKey.Equals(GetAssociatedTokenAddress(payer, output, constants.TOKEN_PROGRAM)) {
				t.Fatal("wrong directional accounts")
			}
			_, err = MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{Payer: payer, InputMint: input, OutputMint: input, InputAmount: 10000, FixedOutputAmount: &fixed, ProtocolParams: pool})
			if err == nil {
				t.Fatal("same-side pair accepted")
			}
		}
	}
}
