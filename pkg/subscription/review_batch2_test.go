package subscription

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"os"
	"strings"
	"testing"
)

type batch2Fixture struct {
	Protocol, Pool  string
	InputMint       string `json:"input_mint"`
	OutputMint      string `json:"output_mint"`
	Payer           string
	ExpectedMissing string `json:"expected_missing"`
	Vault           string
	Accounts        []struct{ Pubkey, Owner, Data string }
}

func batch2Snapshot(t *testing.T, name string) (batch2Fixture, *SubscriptionAccountCache, PoolTradeHint) {
	t.Helper()
	b, e := os.ReadFile("testdata/batch2_" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var v batch2Fixture
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	c := &SubscriptionAccountCache{}
	for _, a := range v.Accounts {
		d, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		c.Update(solana.MustPublicKeyFromBase58(a.Pubkey), CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: d, Slot: 100, WriteVersion: 1})
	}
	return v, c, PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(v.InputMint), solana.MustPublicKeyFromBase58(v.OutputMint)}
}
func TestBatch2SparseArrays(t *testing.T) {
	for _, kind := range []string{"clmm", "dlmm"} {
		t.Run(kind, func(t *testing.T) {
			v, c, h := batch2Snapshot(t, "sparse_"+kind)
			var err error
			if kind == "clmm" {
				_, _, _, err = c.Snapshot().PrepareClmm(h, CacheReadContext{Slot: 100, Epoch: 0, MaximumSlotAge: 0}, 1000, solana.MustPublicKeyFromBase58(v.Payer), 100, 100, 8)
			} else {
				_, _, _, err = c.Snapshot().PrepareDlmm(h, CacheReadContext{Slot: 100, Epoch: 0, MaximumSlotAge: 0}, 1000, solana.MustPublicKeyFromBase58(v.Payer), 100, 100, 8)
			}
			if err == nil || !strings.Contains(err.Error(), v.ExpectedMissing) {
				t.Fatalf("did not find distant array: %v", err)
			}
		})
	}
}
func TestBatch2CpmmValidation(t *testing.T) {
	v, c, h := batch2Snapshot(t, "cpmm")
	ctx := CacheReadContext{Slot: 100}
	payer := solana.MustPublicKeyFromBase58(v.Payer)
	if _, e := c.Snapshot().PrepareCpmm(h, ctx, 1000, payer, 1, 100); e == nil || !strings.Contains(e.Error(), "zero protected") {
		t.Fatalf("zero output accepted: %v", e)
	}
	if _, e := c.Snapshot().PrepareCpmm(h, ctx, 1000, payer, 10000, 100); e != nil {
		t.Fatal(e)
	}
	key := solana.MustPublicKeyFromBase58(v.Vault)
	a, e := c.Snapshot().Get(key, ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for i := 32; i < 64; i++ {
		a.Data[i] = 0
	}
	a.WriteVersion++
	c.Update(key, a)
	if _, e = c.Snapshot().Cpmm(h, ctx, 1000); e == nil {
		t.Fatal("wrong vault authority accepted")
	}
}
