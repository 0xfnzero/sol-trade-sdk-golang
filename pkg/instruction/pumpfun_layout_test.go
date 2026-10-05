package instruction

import (
	"bytes"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
	"os"
	"testing"
)

func TestPinnedPumpFunLayoutMatrix(t *testing.T) {
	data, err := os.ReadFile("testdata/pumpfun_layout_rust_5_0_6.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Quote               string
			Curve               string `json:"curve_quote"`
			Settlement          string
			V2                  bool
			Error               bool
			StrictEndpointError bool `json:"strict_endpoint_error"`
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	keys := map[string]solana.PublicKey{"default": {}, "SOL": constants.SOL_TOKEN_ACCOUNT, "WSOL": constants.WSOL_TOKEN_ACCOUNT, "USDC": constants.USDC_TOKEN_ACCOUNT}
	for _, c := range corpus.Cases {
		for _, buy := range []bool{true, false} {
			p := testPumpFunParams(keys[c.Quote])
			p.BondingCurve.QuoteMint = keys[c.Curve]
			var ixs []solana.Instruction
			var err error
			if buy {
				ixs, err = PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{Payer: testPK(42), InputMint: keys[c.Settlement], OutputMint: testPK(2), InputAmount: 10000, ProtocolParams: p, UseExactSolAmount: true})
			} else {
				ixs, err = PumpFunBuildSellInstructions(&PumpFunBuildSellParams{Payer: testPK(42), InputMint: testPK(2), OutputMint: keys[c.Settlement], InputAmount: 10000, ProtocolParams: p})
			}
			if c.Error || c.StrictEndpointError {
				if err == nil {
					t.Fatal("expected quote mismatch", c, buy)
				}
				continue
			}
			if err != nil {
				t.Fatal(c, buy, err)
			}
			want := PumpFunSellDiscriminator
			if buy {
				want = PumpFunBuyExactSolInDiscriminator
			}
			if c.V2 {
				if buy {
					want = PumpFunBuyExactQuoteInV2Discriminator
				} else {
					want = PumpFunSellV2Discriminator
				}
			}
			found := false
			for _, ix := range ixs {
				if ix.ProgramID() == PUMPFUN_PROGRAM {
					d, err := ix.Data()
					if err != nil || !bytes.Equal(d[:8], want) {
						t.Fatal(c, buy, d, err)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing swap")
			}
		}
	}
}
