package common

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestPinnedRustCurveAccount(t *testing.T) {
	data, err := os.ReadFile("testdata/curve_account_rust_5_0_6.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases  []map[string]string `json:"cases"`
		Layout struct {
			AccountHex string   `json:"account_hex"`
			Reserves   []string `json:"reserves"`
			QuoteHex   string   `json:"quote_hex"`
			CreatorHex string   `json:"creator_hex"`
		} `json:"layout"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	n := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, c := range fixture.Cases {
		t.Run(c["name"], func(t *testing.T) {
			b := &BondingCurveAccount{VirtualTokenReserves: n(c["virtual_token_reserves"]), VirtualSolReserves: n(c["virtual_sol_reserves"]), RealTokenReserves: n(c["real_token_reserves"]), RealSolReserves: n(c["real_sol_reserves"]), TokenTotalSupply: n(c["token_total_supply"])}
			amount, fee := n(c["amount"]), n(c["fee_basis_points"])
			buy, e := b.GetBuyPrice(amount)
			if e != nil {
				t.Fatal(e)
			}
			sell, e := b.GetSellPrice(amount, fee)
			if e != nil {
				t.Fatal(e)
			}
			buyout, e := b.GetBuyOutPrice(amount, fee)
			if e != nil {
				t.Fatal(e)
			}
			final, e := b.GetFinalMarketCapSol(fee)
			if e != nil {
				t.Fatal(e)
			}
			for key, got := range map[string]uint64{"buy": buy, "sell": sell, "market_cap": b.GetMarketCapSol(), "buyout": buyout, "final_market_cap": final} {
				if got != n(c[key]) {
					t.Fatalf("%s = %d want %s", key, got, c[key])
				}
			}
		})
	}
	account, _ := hex.DecodeString(fixture.Layout.AccountHex)
	for _, bytes := range [][]byte{account, append(append([]byte{}, account...), make([]byte, 36)...), account[8:]} {
		b := DecodeBondingCurveAccount(bytes, [32]byte{})
		if b == nil {
			t.Fatal("decode failed")
		}
		for i, got := range []uint64{b.VirtualTokenReserves, b.VirtualSolReserves, b.RealTokenReserves, b.RealSolReserves, b.TokenTotalSupply} {
			if got != n(fixture.Layout.Reserves[i]) {
				t.Fatalf("reserve %d = %d", i, got)
			}
		}
		if hex.EncodeToString(b.QuoteMint[:]) != fixture.Layout.QuoteHex || hex.EncodeToString(b.Creator[:]) != fixture.Layout.CreatorHex || !b.IsCashbackCoin {
			t.Fatal("metadata mismatch")
		}
	}
	for _, bytes := range [][]byte{account[:83], account[8:83]} {
		b := DecodeBondingCurveAccount(bytes, [32]byte{})
		if b == nil || b.RealSolReserves != 456 {
			t.Fatal("legacy offsets")
		}
	}
	for _, index := range []int{0, 48, 81, 82} {
		bad := append([]byte{}, account...)
		bad[index] = 255
		if DecodeBondingCurveAccount(bad, [32]byte{}) != nil {
			t.Fatal("accepted invalid layout")
		}
	}
	for _, length := range []int{0, 74, 82, 84, 106, 107, 108, 114} {
		if DecodeBondingCurveAccount(account[:length], [32]byte{}) != nil {
			t.Fatalf("accepted truncated %d", length)
		}
	}
	complete := &BondingCurveAccount{Complete: true}
	if _, err := complete.GetBuyPriceChecked(0); err == nil {
		t.Fatal("missing completed error")
	}
	if _, err := complete.GetSellPriceChecked(0, 95); err == nil {
		t.Fatal("missing completed error")
	}
}
