package instruction

import (
	"encoding/base64"
	"encoding/json"
	solana "github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"testing"
)

type hookResolverFixture struct {
	Hook, Mint       string
	MintOwner        string `json:"mint_owner"`
	MintData         string `json:"mint_data"`
	Meta             string
	MetaOwner        string   `json:"meta_owner"`
	MetaData         string   `json:"meta_data"`
	ExecuteAccounts  []string `json:"execute_accounts"`
	ExpectedAccounts []string `json:"expected_accounts"`
}

func hookResolve(r hookResolverFixture) (solana.AccountMetaSlice, error) {
	decode := func(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
	keys := []solana.PublicKey{}
	for _, k := range r.ExecuteAccounts {
		keys = append(keys, solana.MustPublicKeyFromBase58(k))
	}
	return ResolveHookAccounts(solana.MustPublicKeyFromBase58(r.Hook), solana.MustPublicKeyFromBase58(r.Mint), solana.MustPublicKeyFromBase58(r.MintOwner), decode(r.MintData), solana.MustPublicKeyFromBase58(r.Meta), solana.MustPublicKeyFromBase58(r.MetaOwner), decode(r.MetaData), keys)
}
func TestWhirlpoolHookExecutedBank(t *testing.T) {
	b, e := os.ReadFile("testdata/whirlpool_hook_bank_20261008.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Cases []struct {
			Accounts        map[string]json.RawMessage
			Amount, Minimum uint64
			Direction       bool
			Resolver        hookResolverFixture
			Expected        struct {
				Data     string
				Accounts []struct {
					Key              string
					Signer, Writable bool
				}
			}
		}
		Rewards []hookResolverFixture
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	for _, c := range f.Cases {
		p := func(k string) solana.PublicKey {
			var s string
			json.Unmarshal(c.Accounts[k], &s)
			return solana.MustPublicKeyFromBase58(s)
		}
		var ts []string
		json.Unmarshal(c.Accounts["tick_arrays"], &ts)
		ticks := []solana.PublicKey{}
		for _, s := range ts {
			ticks = append(ticks, solana.MustPublicKeyFromBase58(s))
		}
		a := WhirlpoolSwapV2Accounts{p("token_program_a"), p("token_program_b"), p("token_authority"), p("whirlpool"), p("mint_a"), p("mint_b"), p("owner_a"), p("vault_a"), p("owner_b"), p("vault_b"), ticks}
		extra, e := hookResolve(c.Resolver)
		if e != nil {
			t.Fatal(e)
		}
		var ha, hb solana.AccountMetaSlice
		if a.MintA.String() == c.Resolver.Mint {
			ha = extra
		} else {
			hb = extra
		}
		ix, e := BuildWhirlpoolSwapV2WithHooks(a, SwapV2Args{Amount: c.Amount, OtherAmountThreshold: c.Minimum, AmountSpecifiedIsInput: true}, c.Direction, ha, hb)
		if e != nil {
			t.Fatal(e)
		}
		d, e := ix.Data()
		if e != nil {
			t.Fatal(e)
		}
		if base64.StdEncoding.EncodeToString(d) != c.Expected.Data {
			t.Fatal("data mismatch")
		}
		metas := ix.Accounts()
		if len(metas) != len(c.Expected.Accounts) {
			t.Fatal("account count")
		}
		for i, m := range metas {
			want := c.Expected.Accounts[i]
			if m.PublicKey.String() != want.Key || m.IsSigner != want.Signer || m.IsWritable != want.Writable {
				t.Fatal("meta mismatch", i)
			}
		}
	}
	for _, r := range f.Rewards {
		got, e := hookResolve(r)
		if e != nil {
			t.Fatal(e)
		}
		keys := []string{}
		for _, m := range got {
			keys = append(keys, m.PublicKey.String())
		}
		if !reflect.DeepEqual(keys, r.ExpectedAccounts) {
			t.Fatal("PDA mismatch")
		}
		for _, mode := range []string{"owner", "mint_owner", "truncated", "count", "seed_index", "encoding", "signer", "padding"} {
			bad := r
			data, _ := base64.StdEncoding.DecodeString(r.MetaData)
			switch mode {
			case "owner":
				bad.MetaOwner = solana.PublicKey{}.String()
			case "mint_owner":
				bad.MintOwner = solana.PublicKey{}.String()
			case "truncated":
				data = data[:len(data)-1]
			case "count":
				data[12] = 255
			case "seed_index":
				data[53] = 255
			case "encoding":
				data[51] = 2
			case "signer":
				data[49] = 1
			case "padding":
				data[55] = 3
			}
			bad.MetaData = base64.StdEncoding.EncodeToString(data)
			if _, e = hookResolve(bad); e == nil {
				t.Fatal("expected rejection", mode)
			}
		}
	}
}
