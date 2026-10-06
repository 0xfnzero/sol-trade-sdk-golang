package instruction

import (
	"testing"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

func TestPumpFunExplicitTokenProgramAccountDerivation(t *testing.T) {
	for _, mint := range []solana.PublicKey{
		solana.MustPublicKeyFromBase58("GqpCGjif7WDNgLww8vKGgf5daW5dVq39eFRTdmfhpump"),
		testPK(2),
	} {
		for _, program := range []solana.PublicKey{constants.TOKEN_PROGRAM, constants.TOKEN_PROGRAM_2022, {}} {
			for _, quote := range []solana.PublicKey{constants.SOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT} {
				for _, buy := range []bool{true, false} {
					name := mint.String() + "/" + program.String() + "/" + quote.String()
					if buy {
						name += "/buy"
					} else {
						name += "/sell"
					}
					t.Run(name, func(t *testing.T) {
						p := testPumpFunParams(quote)
						p.TokenProgram = program
						payer := testPK(42)
						var ixs []solana.Instruction
						var err error
						if buy {
							ixs, err = PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{
								Payer: payer, InputMint: quote, OutputMint: mint, InputAmount: 10000,
								ProtocolParams: p, CreateOutputMintAta: true, UseExactSolAmount: true,
							})
						} else {
							ixs, err = PumpFunBuildSellInstructions(&PumpFunBuildSellParams{
								Payer: payer, InputMint: mint, OutputMint: quote, InputAmount: 10000, ProtocolParams: p,
							})
						}
						if err != nil {
							t.Fatal(err)
						}
						wantProgram := program
						if wantProgram.IsZero() {
							wantProgram = constants.TOKEN_PROGRAM_2022
						}
						programIndex, curveIndex, userIndex := 3, 11, 14
						if quote == constants.SOL_TOKEN_ACCOUNT {
							programIndex, curveIndex, userIndex = 8, 4, 5
							if !buy {
								programIndex = 9
							}
						}
						found := false
						for _, ix := range ixs {
							a := ix.Accounts()
							if ix.ProgramID() == constants.ASSOCIATED_TOKEN_PROGRAM_ID {
								if a[1].PublicKey != GetAssociatedTokenAddress(payer, mint, wantProgram) || a[5].PublicKey != wantProgram {
									t.Fatal("ATA creation uses the wrong token program")
								}
							}
							if ix.ProgramID() != PUMPFUN_PROGRAM {
								continue
							}
							found = true
							if a[programIndex].PublicKey != wantProgram ||
								a[curveIndex].PublicKey != GetAssociatedTokenAddress(GetBondingCurvePDA(mint), mint, wantProgram) ||
								a[userIndex].PublicKey != GetAssociatedTokenAddress(payer, mint, wantProgram) {
								t.Fatal("swap token program and derived ATAs do not match the supplied mint owner")
							}
						}
						if !found {
							t.Fatal("missing swap instruction")
						}
					})
				}
			}
		}
	}
}
