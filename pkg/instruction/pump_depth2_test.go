package instruction

import (
	"encoding/hex"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"testing"
)

func TestPumpDepth2OfficialOracle(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Name              string
			Quote             PumpMultiHop
			Chain             []PumpMultiHop
			Depth, MaxDepth   uint8
			ListedQuoteMints  []solana.PublicKey
			Accounts          map[string]solana.PublicKey
			Params            PumpCreateV2Params
			Data, Transaction string
			Keys              []struct {
				Pubkey           solana.PublicKey
				Signer, Writable bool
			}
		}
	}
	raw, err := os.ReadFile("../../tests/fixtures/pump_upgrade/create_depth2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			remaining, err := DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, c.Chain, c.Depth, c.MaxDepth, c.ListedQuoteMints)
			if err != nil {
				t.Fatal(err)
			}
			// Later whitelisting does not rewrite Q's stored depth or ancestors.
			listed := append(append([]solana.PublicKey(nil), c.ListedQuoteMints...), c.Chain[0].BaseMint)
			expanded, err := DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, c.Chain, c.Depth, c.MaxDepth, listed)
			if err != nil || !reflect.DeepEqual(expanded, remaining) {
				t.Fatal("listed ancestor rejected or chain changed", err)
			}
			ix, err := BuildPumpCreateV2Instruction(c.Accounts, c.Params, remaining)
			if err != nil {
				t.Fatal(err)
			}
			data, err := ix.Data()
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(data) != c.Data {
				t.Fatal("data differs from official SDK")
			}
			if len(ix.Accounts()) != len(c.Keys) {
				t.Fatal("wrong account count")
			}
			for i, m := range ix.Accounts() {
				e := c.Keys[i]
				if m.PublicKey != e.Pubkey || m.IsSigner != e.Signer || m.IsWritable != e.Writable {
					t.Fatalf("account %d differs", i)
				}
			}
			tx, err := solana.TransactionFromBase64(c.Transaction)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.VerifySignatures(); err != nil {
				t.Fatal(err)
			}
			if _, err = DerivePumpCoinQuoteCreateAccounts(c.Accounts["mint"], c.Quote, 1, 2, c.ListedQuoteMints); err == nil {
				t.Fatal("missing chain accepted")
			}
			if _, err = DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, c.Chain, 1, 1, c.ListedQuoteMints); err == nil {
				t.Fatal("depth cap ignored")
			}
			bad := append([]PumpMultiHop(nil), c.Chain...)
			badQuote := c.Quote
			badQuote.Mayhem = true
			if _, err = DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], badQuote, c.Chain, 1, 2, c.ListedQuoteMints); err == nil {
				t.Fatal("mayhem quote accepted")
			}
			// Ancestor mayhem does not disqualify Q for creation.
			bad[0].Mayhem = true
			if _, err = DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, bad, 1, 2, c.ListedQuoteMints); err != nil {
				t.Fatal("ancestor mayhem rejected", err)
			}
			for _, field := range []string{"mint", "token-program"} {
				bad[0] = c.Chain[0]
				if field == "mint" {
					bad[0].BaseMint = c.Accounts["user"]
				} else {
					bad[0].BaseTokenProgram = c.Accounts["program"]
				}
				_, err := DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, bad, 1, 2, c.ListedQuoteMints)
				if err == nil || err.Error() != "discontinuous quote chain" {
					t.Fatalf("%s continuity: %v", field, err)
				}
			}
			if _, err = DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, c.Chain, 5, 255, c.ListedQuoteMints); err == nil || err.Error() != "CurveDepthExceeded" {
				t.Fatal("hard cap ignored", err)
			}
			for _, index := range []int{2, 4} {
				invalid := append([]*solana.AccountMeta(nil), remaining...)
				m := *invalid[index]
				if index == 2 {
					m.IsWritable = true
				} else {
					m.IsSigner = true
				}
				invalid[index] = &m
				if _, err = BuildPumpCreateV2Instruction(c.Accounts, c.Params, invalid); err == nil {
					t.Fatal("invalid account flags accepted")
				}
			}
			bad[0] = c.Chain[0]
			bad[0].BaseMint = c.Accounts["mint"]
			if _, err = DerivePumpCoinQuoteCreateAccountsWithChain(c.Accounts["mint"], c.Quote, bad, 1, 2, c.ListedQuoteMints); err == nil {
				t.Fatal("discontinuous chain accepted")
			}
		})
	}
}
