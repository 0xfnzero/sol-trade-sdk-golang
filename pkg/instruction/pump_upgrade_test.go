package instruction

import (
	"encoding/hex"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"testing"
)

func TestPumpCreateOfficialEncoding(t *testing.T) {
	raw, e := os.ReadFile("../../tests/fixtures/pump_upgrade/create.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Data     string
		Accounts []struct {
			Name             string
			Writable, Signer bool
		}
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}
	accounts := map[string]solana.PublicKey{}
	for i, a := range f.Accounts {
		var k solana.PublicKey
		for j := range k {
			k[j] = byte(i + 1)
		}
		accounts[a.Name] = k
	}
	var creator solana.PublicKey
	for i := range creator {
		creator[i] = 9
	}
	ix, e := BuildPumpCreateV2Instruction(accounts, PumpCreateV2Params{Name: "测试", Symbol: "Q", URI: "https://example.com/q", Creator: creator, CreatorFeeBps: 25, HolderReward: true}, nil)
	if e != nil {
		t.Fatal(e)
	}
	data, e := ix.Data()
	if e != nil || hex.EncodeToString(data) != f.Data {
		t.Fatal("official encoding mismatch", e)
	}
	for i, a := range f.Accounts {
		m := ix.Accounts()[i]
		if m.IsSigner != a.Signer || m.IsWritable != a.Writable {
			t.Fatal("flags mismatch")
		}
	}
}
func TestNestedCurveRoute(t *testing.T) {
	user, a, b := solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()
	token := solana.TokenProgramID
	hop := func(base, quote solana.PublicKey) PumpMultiHop {
		p, e := DerivePumpV3Accounts(PumpCompactAccountParams{User: user, BaseMint: base, QuoteMint: quote, BaseTokenProgram: token, QuoteTokenProgram: token, BuybackRecipient: user})
		if e != nil {
			t.Fatal(e)
		}
		return PumpMultiHop{Venue: "curve", BaseMint: base, QuoteMint: quote, Address: p["bonding_curve"], BaseVault: p["associated_base_bonding_curve"], QuoteVault: p["associated_quote_bonding_curve"], BaseTokenProgram: token, QuoteTokenProgram: token}
	}
	hops := []PumpMultiHop{hop(a, compactWSOL), hop(b, a)}
	accounts, remaining, e := DerivePumpMultiHopAccounts(user, compactWSOL, b, user, hops, false)
	if e != nil || len(accounts) != 16 || len(remaining) != 10 {
		t.Fatal(e)
	}
	hops[1].Cashback = true
	if _, _, e = DerivePumpMultiHopAccounts(user, compactWSOL, b, user, hops, false); e == nil {
		t.Fatal("cashback accepted")
	}
	hops[0], hops[1] = hops[1], hops[0]
	if _, _, e = DerivePumpMultiHopAccounts(user, compactWSOL, b, user, hops, false); e == nil {
		t.Fatal("discontinuous route accepted")
	}
}

func TestOfficialQuoteControl(t *testing.T) {
	raw, e := os.ReadFile("../../tests/fixtures/pump_upgrade/quote_control.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct{ Data, Admin, ReservesAdmin, Mint string }
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}
	data, _ := hex.DecodeString(f.Data)
	c, e := DecodePumpQuoteControl(data)
	if e != nil || c.Admin.String() != f.Admin || c.ReservesAdmin.String() != f.ReservesAdmin || c.Mints[0].Mint.String() != f.Mint || c.Mints[0].InitialVirtualQuoteReserves != 321 {
		t.Fatal("official QuoteControl mismatch", e)
	}
	if _, e = DecodePumpQuoteControl(data[:len(data)-1]); e == nil {
		t.Fatal("truncation accepted")
	}
}
