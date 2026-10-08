package instruction

import (
	"encoding/hex"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestNativePumpTwoHopBankWire(t *testing.T) {
	b, e := os.ReadFile("testdata/pump_routes_bank_20261009.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Cases []struct {
			Name, User, Program, Data string
			Input                     string `json:"input_mint"`
			Output                    string `json:"output_mint"`
			Buyback                   string `json:"buyback_recipient"`
			Hops                      []map[string]any
			Args, Accounts            []string
		}
	}
	if e = json.Unmarshal(b, &fixture); e != nil {
		t.Fatal(e)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			hops := make([]PumpMultiHop, 0, len(c.Hops))
			for _, h := range c.Hops {
				pk := func(k string) solana.PublicKey { return solana.MustPublicKeyFromBase58(h[k].(string)) }
				hops = append(hops, PumpMultiHop{Venue: h["venue"].(string), BaseMint: pk("base_mint"), QuoteMint: pk("quote_mint"), Address: pk("address"), BaseVault: pk("base_vault"), QuoteVault: pk("quote_vault"), BaseTokenProgram: pk("base_token_program"), QuoteTokenProgram: pk("quote_token_program"), Creator: pk("creator")})
			}
			fixed, remaining, e := DerivePumpMultiHopAccounts(solana.MustPublicKeyFromBase58(c.User), solana.MustPublicKeyFromBase58(c.Input), solana.MustPublicKeyFromBase58(c.Output), solana.MustPublicKeyFromBase58(c.Buyback), hops, false)
			if e != nil {
				t.Fatal(e)
			}
			args := []uint64{}
			for _, a := range c.Args {
				n, e := strconv.ParseUint(a, 10, 64)
				if e != nil {
					t.Fatal(e)
				}
				args = append(args, n)
			}
			ix, e := BuildPumpUpgradeInstruction("pump_amm_multi_hop_swap", fixed, args, nil, remaining)
			if e != nil {
				t.Fatal(e)
			}
			data, e := ix.Data()
			if e != nil {
				t.Fatal(e)
			}
			var keys []string
			for _, a := range ix.Accounts() {
				keys = append(keys, a.PublicKey.String())
			}
			if ix.ProgramID().String() != c.Program || hex.EncodeToString(data) != c.Data || !reflect.DeepEqual(keys, c.Accounts) {
				t.Fatal("native instruction differs from executed bank wire")
			}
		})
	}
}
