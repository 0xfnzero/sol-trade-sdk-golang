package instruction

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func TestBuildersMatchSuccessfulMainnetSimulations(t *testing.T) {
	b, e := os.ReadFile("../../tests/fixtures/pump_upgrade/simulated_instructions.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Cases []struct {
			Name, Program, Data string
			Accounts            map[string]string
			Args                []string
			Metas               []struct {
				Pubkey           string
				Signer, Writable bool
			}
		}
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	for _, c := range f.Cases {
		accounts := map[string]solana.PublicKey{}
		for k, v := range c.Accounts {
			accounts[k] = solana.MustPublicKeyFromBase58(v)
		}
		args := make([]uint64, len(c.Args))
		for i, v := range c.Args {
			if _, e = fmt.Sscan(v, &args[i]); e != nil {
				t.Fatal(e)
			}
		}
		ix, e := BuildPumpUpgradeInstruction(c.Name, accounts, args, nil, nil)
		if e != nil {
			t.Fatal(e)
		}
		d, e := ix.Data()
		if e != nil || hex.EncodeToString(d) != c.Data || ix.ProgramID().String() != c.Program {
			t.Fatalf("%s encoding mismatch", c.Name)
		}
		if len(ix.Accounts()) != len(c.Metas) {
			t.Fatal("account count")
		}
		for i, a := range ix.Accounts() {
			m := c.Metas[i]
			if a.PublicKey.String() != m.Pubkey || a.IsSigner != m.Signer || a.IsWritable != m.Writable {
				t.Fatalf("%s role %d", c.Name, i)
			}
		}
	}
}

func TestNativeAliasesMatchOfficialAccounts(t *testing.T) {
	raw, err := os.ReadFile("../../tests/fixtures/pump_upgrade/native_aliases.json")
	if err != nil {
		t.Fatal(err)
	}
	type meta struct {
		Pubkey           string
		Signer, Writable bool
	}
	var f struct {
		Cases []struct {
			Alias                 string
			V3, Buy, Sell, Create []meta
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	key := func(n byte) (p solana.PublicKey) {
		for i := range p {
			p[i] = n
		}
		return
	}
	user, a, b := key(1), key(2), key(3)
	token, token2022 := solana.TokenProgramID, solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	params := func(base, quote, qt solana.PublicKey) PumpCompactAccountParams {
		return PumpCompactAccountParams{User: user, BaseMint: base, QuoteMint: quote, BaseTokenProgram: token2022, QuoteTokenProgram: qt, BuybackRecipient: user}
	}
	hop := func(base, quote, qt solana.PublicKey) PumpMultiHop {
		normalized := compactWSOL
		if quote == a {
			normalized = a
		}
		p, e := DerivePumpV3Accounts(params(base, normalized, qt))
		if e != nil {
			t.Fatal(e)
		}
		return PumpMultiHop{Venue: "curve", BaseMint: base, QuoteMint: quote, Address: p["bonding_curve"], BaseVault: p["associated_base_bonding_curve"], QuoteVault: p["associated_quote_bonding_curve"], BaseTokenProgram: token2022, QuoteTokenProgram: qt}
	}
	check := func(items []*solana.AccountMeta, expected []meta) {
		t.Helper()
		if len(items) != len(expected) {
			t.Fatal("account count mismatch")
		}
		for i, m := range items {
			if m.PublicKey.String() != expected[i].Pubkey || m.IsSigner != expected[i].Signer || m.IsWritable != expected[i].Writable {
				t.Fatalf("account %d differs from official builder", i)
			}
		}
	}
	for _, c := range f.Cases {
		alias := solana.MustPublicKeyFromBase58(c.Alias)
		p := params(a, alias, token)
		accounts, e := DerivePumpV3Accounts(p)
		if e != nil {
			t.Fatal(e)
		}
		ix, e := BuildPumpUpgradeInstruction("pump_buy_v3", accounts, []uint64{7, 9}, nil, nil)
		if e != nil {
			t.Fatal(e)
		}
		check(ix.Accounts(), c.V3)
		parent, child := hop(a, alias, token), hop(b, a, token2022)
		for _, route := range []struct {
			Hops          []PumpMultiHop
			Input, Output solana.PublicKey
			Expected      []meta
		}{{[]PumpMultiHop{parent, child}, alias, b, c.Buy}, {[]PumpMultiHop{child, parent}, b, alias, c.Sell}} {
			beforeFirst, beforeLast := route.Hops[0], route.Hops[1]
			accounts, remaining, e := DerivePumpMultiHopAccounts(user, route.Input, route.Output, user, route.Hops, false)
			if e != nil {
				t.Fatal(e)
			}
			ix, e := BuildPumpUpgradeInstruction("pump_amm_multi_hop_swap", accounts, []uint64{7, 9}, nil, remaining)
			if e != nil {
				t.Fatal(e)
			}
			check(ix.Accounts(), route.Expected)
			if route.Hops[0] != beforeFirst || route.Hops[1] != beforeLast {
				t.Fatal("input state mutated")
			}
		}
		remaining, e := DerivePumpCoinQuoteCreateAccounts(b, parent, 0, 1, nil)
		if e != nil {
			t.Fatal(e)
		}
		check(remaining, c.Create)
		actual, e := DerivePumpSwapV2Accounts(p, user, a, b)
		if e != nil {
			t.Fatal(e)
		}
		expected, e := DerivePumpSwapV2Accounts(params(a, compactWSOL, token), user, a, b)
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range expected {
			if actual[k] != v {
				t.Fatalf("AMM alias mismatch %s", k)
			}
		}
	}
}
