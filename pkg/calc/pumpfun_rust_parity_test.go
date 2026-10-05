package calc

import (
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"testing"
)

func TestPumpFunPinnedRust(t *testing.T) {
	data, err := os.ReadFile("testdata/pumpfun_rust_5_0_6.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			VirtualToken string `json:"virtual_token"`
			VirtualQuote string `json:"virtual_quote"`
			RealToken    string `json:"real_token"`
			Amount       string `json:"amount"`
			HasCreator   bool   `json:"has_creator"`
			Buy          string
			Sell         string
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	num := func(s string) uint64 {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}

	wide := func(text string) *big.Int {
		n, ok := new(big.Int).SetString(text, 10)
		if !ok {
			t.Fatal(text)
		}
		return n
	}
	for _, c := range fixture.Cases {
		vt, vq, rt, a := wide(c.VirtualToken), wide(c.VirtualQuote), wide(c.RealToken), num(c.Amount)
		before := new(big.Int).Set(vt)
		if n, e := GetBuyTokenAmountFromSolAmountU128(vt, vq, rt, c.HasCreator, a); e != nil || n != num(c.Buy) {
			t.Fatalf("wide buy got %d want %s err %v", n, c.Buy, e)
		}
		if n, e := GetSellSolAmountFromTokenAmountU128(vt, vq, c.HasCreator, a); e != nil || n != num(c.Sell) {
			t.Fatalf("wide sell got %d want %s err %v", n, c.Sell, e)
		}
		if vt.Cmp(before) != 0 {
			t.Fatal("mutated input")
		}
		if vt.IsUint64() && vq.IsUint64() && rt.IsUint64() {
			if n := GetBuyTokenAmountFromSolAmount(vt.Uint64(), vq.Uint64(), rt.Uint64(), c.HasCreator, a); n != num(c.Buy) {
				t.Fatal("u64 buy differs")
			}
			if n := GetSellSolAmountFromTokenAmount(vt.Uint64(), vq.Uint64(), c.HasCreator, a); n != num(c.Sell) {
				t.Fatal("u64 sell differs")
			}
		}
	}
	if _, err := GetBuyTokenAmountFromSolAmountU128(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1), big.NewInt(1), false, 1); err == nil {
		t.Fatal("outside u128 accepted")
	}
}
