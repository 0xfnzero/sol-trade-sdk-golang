package instruction

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go/rpc"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCreatorFeeRateUsesOneConfirmedSnapshot(t *testing.T) {
	b, e := os.ReadFile("testdata/cpmm_creator_fee_rust_5_0_7.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		ShareRate uint64 `json:"share_rate"`
		Accounts  []*struct {
			Owner    string
			Data     string `json:"data_base64"`
			Lamports uint64
		}
	}
	if e = json.Unmarshal(b, &cases); e != nil {
		t.Fatal(e)
	}
	c := cases[0]
	poolBytes, _ := base64.StdEncoding.DecodeString(c.Accounts[0].Data)
	pool, _ := DecodeCpmmCollectionPool(poolBytes)
	pda, _ := GetCreatorFeeSharePDA(pool.PoolCreator, pool.AmmConfig)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			ID     any
			Method string
			Params []json.RawMessage
		}
		if e = json.NewDecoder(r.Body).Decode(&req); e != nil {
			t.Error(e)
		}
		if req.Method != "getMultipleAccounts" || len(req.Params) != 2 {
			t.Error("wrong request")
		}
		var keys []string
		json.Unmarshal(req.Params[0], &keys)
		if len(keys) != 2 || keys[0] != pool.AmmConfig.String() || keys[1] != pda.String() {
			t.Error("wrong keys")
		}
		var opts map[string]string
		json.Unmarshal(req.Params[1], &opts)
		if opts["commitment"] != "confirmed" || opts["encoding"] != "base64" {
			t.Error("wrong snapshot opts")
		}
		value := []any{}
		for _, a := range c.Accounts[1:3] {
			if a == nil {
				value = append(value, nil)
			} else {
				value = append(value, map[string]any{"owner": a.Owner, "data": []string{a.Data, "base64"}, "lamports": a.Lamports, "executable": false, "rentEpoch": 0})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"context": map[string]any{"slot": 100}, "value": value}})
	}))
	defer server.Close()
	rate, e := FetchCreatorFeeShareRate(context.Background(), rpc.New(server.URL), pool.PoolCreator, pool.AmmConfig)
	if e != nil || rate != c.ShareRate || calls != 1 {
		t.Fatal(rate, e, calls)
	}
	server.Close()
	if _, e = FetchCreatorFeeShareRate(context.Background(), rpc.New(server.URL), pool.PoolCreator, pool.AmmConfig); e == nil {
		t.Fatal("RPC failure silently defaulted")
	}
}
