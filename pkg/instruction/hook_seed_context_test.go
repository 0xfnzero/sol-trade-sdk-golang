package instruction

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"testing"
)

func TestExecutedHookSeedContext(t *testing.T) {
	b, e := os.ReadFile("testdata/hook_seed_context_20261009.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct{ Cases []map[string]json.RawMessage }
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	for _, c := range f.Cases {
		str := func(k string) string {
			var v string
			if e := json.Unmarshal(c[k], &v); e != nil {
				t.Fatal(e)
			}
			return v
		}
		pk := func(k string) solana.PublicKey { return solana.MustPublicKeyFromBase58(str(k)) }
		dec := func(v string) []byte {
			b, e := base64.StdEncoding.DecodeString(v)
			if e != nil {
				t.Fatal(e)
			}
			return b
		}
		var ex, expected []string
		json.Unmarshal(c["execute_accounts"], &ex)
		json.Unmarshal(c["expected_accounts"], &expected)
		keys := []solana.PublicKey{}
		for _, k := range ex {
			keys = append(keys, solana.MustPublicKeyFromBase58(k))
		}
		var snapshots map[string]string
		json.Unmarshal(c["account_data"], &snapshots)
		accounts := map[string][]byte{}
		for k, v := range snapshots {
			accounts[k] = dec(v)
		}
		data := dec(str("execute_data"))
		mintData, metaData := dec(str("mint_data")), dec(str("meta_data"))
		invoke := func(meta []byte, data []byte, accounts map[string][]byte) (solana.AccountMetaSlice, error) {
			return ResolveHookAccountsWithContext(pk("hook"), pk("mint"), pk("mint_owner"), mintData, pk("meta"), pk("meta_owner"), meta, keys, data, accounts)
		}
		got, e := invoke(metaData, data, accounts)
		if e != nil {
			t.Fatal(e)
		}
		actual := []string{}
		for _, a := range got {
			actual = append(actual, a.PublicKey.String())
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatal(actual, expected)
		}
		if _, e = invoke(metaData, nil, nil); e == nil {
			t.Fatal("missing context accepted")
		}
		if _, e = invoke(metaData, data, nil); e == nil {
			t.Fatal("missing snapshot accepted")
		}
		if _, e = invoke(metaData, data[:15], accounts); e == nil {
			t.Fatal("invalid Execute data accepted")
		}
		for _, bad := range [][2]int{{86, 255}, {96, 255}, {102, 33}, {88, 31}, {122, 1}, {122, 0}, {123, 255}, {124, 255}, {125, 1}} {
			m := append([]byte(nil), metaData...)
			m[bad[0]] = byte(bad[1])
			if _, e = invoke(m, data, accounts); e == nil {
				t.Fatal("malformed seed accepted", bad)
			}
		}
		changed := append([]byte(nil), data...)
		changed[8]++
		other, e := invoke(metaData, changed, accounts)
		if e != nil || other[2].PublicKey == got[2].PublicKey {
			t.Fatal("amount-dependent PDA not refreshed", e)
		}
	}
}
