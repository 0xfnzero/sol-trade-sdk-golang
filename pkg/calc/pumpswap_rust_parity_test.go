package calc

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strconv"
	"testing"
)

func TestPinnedRustPumpSwap(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Amount   string     `json:"amount"`
			Slippage string     `json:"slippage"`
			Base     string     `json:"base_reserve"`
			Quote    string     `json:"quote_reserve"`
			Virtual  string     `json:"virtual"`
			Fees     []string   `json:"fees"`
			Results  [][]string `json:"results"`
		}
	}
	data, e := os.ReadFile("testdata/pumpswap_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &fixture); e != nil {
		t.Fatal(e)
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for i, c := range fixture.Cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a, s, b, q := parse(c.Amount), parse(c.Slippage), parse(c.Base), parse(c.Quote)
			v, _ := new(big.Int).SetString(c.Virtual, 10)
			f := PumpSwapFeeBasisPoints{LPFeeBasisPoints: parse(c.Fees[0]), ProtocolFeeBasisPoints: parse(c.Fees[1]), CoinCreatorFeeBasisPoints: parse(c.Fees[2])}
			results := make([][]uint64, 4)
			errs := make([]error, 4)
			r0, e := BuyBaseInputInternalWithFees(a, s, b, q, v, f)
			errs[0] = e
			if e == nil {
				results[0] = []uint64{r0.InternalQuoteAmount, r0.UIQuote, r0.MaxQuote}
			}
			r1, e := BuyQuoteInputInternalWithFees(a, s, b, q, v, f)
			errs[1] = e
			if e == nil {
				results[1] = []uint64{r1.Base, r1.InternalQuoteWithoutFees, r1.MaxQuote}
			}
			r2, e := SellBaseInputInternalWithFees(a, s, b, q, v, f)
			errs[2] = e
			if e == nil {
				results[2] = []uint64{r2.UIQuote, r2.MinQuote, r2.InternalQuoteAmountOut}
			}
			r3, e := SellQuoteInputInternalWithFees(a, s, b, q, v, f)
			errs[3] = e
			if e == nil {
				results[3] = []uint64{r3.InternalRawQuote, r3.Base, r3.MinQuote}
			}
			for j, want := range c.Results {
				if want == nil {
					if errs[j] == nil {
						t.Errorf("entry %d: expected error, got %v", j, results[j])
					}
					continue
				}
				if errs[j] != nil {
					t.Errorf("entry %d: %v", j, errs[j])
					continue
				}
				for k, val := range want {
					if results[j][k] != parse(val) {
						t.Errorf("entry %d field %d: got %d want %s", j, k, results[j][k], val)
					}
				}
			}
		})
	}
}
func TestRustCommonBoundarySemantics(t *testing.T) {
	fee, e := ComputeFee(math.MaxUint64, 10000)
	if e != nil || fee != math.MaxUint64 {
		t.Fatal(fee, e)
	}
	ceil, e := CeilDiv(math.MaxUint64, 2)
	if e != nil || ceil != 9223372036854775808 {
		t.Fatal(ceil, e)
	}
	buy, e := CalculateWithSlippageBuy(math.MaxUint64, 100)
	if e != nil || buy != math.MaxUint64 {
		t.Fatal(buy, e)
	}
	sell, e := CalculateWithSlippageSell(math.MaxUint64, 100)
	if e != nil || sell != 18262276632972456099 {
		t.Fatal(sell, e)
	}
	zero, e := CalculateWithSlippageSell(0, math.MaxUint64)
	if e != nil || zero != 0 {
		t.Fatal(zero, e)
	}
	clamped, e := CalculateWithSlippageSell(10000, math.MaxUint64)
	if e != nil || clamped != 1 {
		t.Fatal(clamped, e)
	}
}
