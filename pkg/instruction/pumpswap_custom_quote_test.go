package instruction

import (
	"bytes"
	"encoding/binary"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestPumpSwapExplicitMintDirectionAndCustomQuote(t *testing.T) {
	for _, base := range []solana.PublicKey{testPK(22), constants.WSOL_TOKEN_ACCOUNT, constants.USDC_TOKEN_ACCOUNT} {
		p := testPumpSwapParams(func(p *PumpSwapParams) {
			p.BaseMint = base
			p.QuoteMint = testPK(40)
			p.QuoteTokenProgram = constants.TOKEN_PROGRAM_2022
		})
		for _, quoteIn := range []bool{true, false} {
			input, output := p.BaseMint, p.QuoteMint
			if quoteIn {
				input, output = output, input
			}
			// High-level labels cannot override an explicit pool mint direction.
			buy, e := BuildBuyInstructions(&BuildBuyParams{Payer: testPK(99), InputMint: input, OutputMint: output, InputAmount: 10000, SlippageBasisPoints: 100, ProtocolParams: p, UseExactQuoteAmount: true})
			if e != nil {
				t.Fatal(e)
			}
			sell, e := BuildSellInstructions(&BuildSellParams{Payer: testPK(99), InputMint: input, OutputMint: output, InputAmount: 10000, SlippageBasisPoints: 100, ProtocolParams: p})
			if e != nil {
				t.Fatal(e)
			}
			b, _ := buy[0].Data()
			s, _ := sell[0].Data()
			if !bytes.Equal(b, s) {
				t.Fatal("labels changed swap direction or protected output")
			}
			expected := PUMPSWAP_SELL_DISCRIMINATOR
			if quoteIn {
				expected = PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR
			}
			if !bytes.Equal(b[:8], expected) || binary.LittleEndian.Uint64(b[8:16]) != 10000 {
				t.Fatal("wrong signed instruction direction")
			}
			keys := buy[0].Accounts()
			if keys[17].PublicKey != GetCoinCreatorVaultAtaWithTokenProgram(p.CoinCreator, p.QuoteMint, p.QuoteTokenProgram) {
				t.Fatal("wrong creator quote program")
			}
		}
	}
}

func TestPumpSwapRejectsIncompleteAndMismatchedIdentity(t *testing.T) {
	for _, corrupt := range []func(*PumpSwapParams){
		func(p *PumpSwapParams) { p.Pool = solana.PublicKey{} },
		func(p *PumpSwapParams) { p.BaseMint = p.QuoteMint },
		func(p *PumpSwapParams) { p.QuoteTokenProgram = testPK(44) },
		func(p *PumpSwapParams) { p.CoinCreatorVaultAta = testPK(45) },
		func(p *PumpSwapParams) { p.CoinCreatorVaultAuthority = testPK(46) },
	} {
		p := testPumpSwapParams(nil)
		corrupt(p)
		if _, e := BuildBuyInstructions(&BuildBuyParams{InputAmount: 1, ProtocolParams: p}); e == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
	p := testPumpSwapParams(nil)
	if _, e := BuildBuyInstructions(&BuildBuyParams{InputAmount: 10000, ProtocolParams: p, InputMint: testPK(47), OutputMint: p.BaseMint}); e == nil {
		t.Fatal("accepted unconverted funding mint")
	}
	if _, e := BuildBuyInstructions(nil); e == nil {
		t.Fatal("nil request accepted")
	}
	if _, e := BuildSellInstructions(&BuildSellParams{InputAmount: 1}); e == nil {
		t.Fatal("nil pool accepted")
	}
}
