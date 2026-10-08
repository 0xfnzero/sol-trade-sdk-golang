package instruction

import (
	"encoding/base64"
	"encoding/json"
	solana "github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestHookMintTlvTerminator(t *testing.T) {
	data, err := os.ReadFile("testdata/hook_mint_tlv_20261009.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct{ Cases []map[string]json.RawMessage }
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, c := range raw.Cases {
		str := func(k string) string {
			var s string
			if e := json.Unmarshal(c[k], &s); e != nil {
				t.Fatal(e)
			}
			return s
		}
		t.Run(str("name"), func(t *testing.T) {
			pk := func(k string) solana.PublicKey { return solana.MustPublicKeyFromBase58(str(k)) }
			dec := func(s string) []byte {
				v, e := base64.StdEncoding.DecodeString(s)
				if e != nil {
					t.Fatal(e)
				}
				return v
			}
			var active bool
			json.Unmarshal(c["expected_active"], &active)
			var ex, want []string
			json.Unmarshal(c["execute_accounts"], &ex)
			json.Unmarshal(c["expected_accounts"], &want)
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
			got, e := ResolveHookAccountsWithContext(pk("hook"), pk("mint"), pk("mint_owner"), dec(str("mint_data")), pk("meta"), pk("meta_owner"), dec(str("meta_data")), keys, dec(str("execute_data")), accounts)
			if !active {
				if e == nil || !strings.Contains(e.Error(), "Active Hook program mismatch") {
					t.Fatalf("hidden Hook: %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			actual := []string{}
			for _, a := range got {
				actual = append(actual, a.PublicKey.String())
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatal(actual, want)
			}
		})
	}
}
