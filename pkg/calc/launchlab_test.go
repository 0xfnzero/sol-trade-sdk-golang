package calc

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestLaunchLabRustGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/launchlab_rust_5_0_6.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name        string
			State       map[string]json.RawMessage
			Amount      string
			SlippageBps uint16 `json:"slippage_bps"`
			Buy, Sell   struct {
				AmountIn         string `json:"amount_in"`
				MinimumAmountOut string `json:"minimum_amount_out"`
			}
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, c := range corpus.Cases {
		t.Run(c.Name, func(t *testing.T) {
			n := func(k string) uint64 {
				var v string
				if e := json.Unmarshal(c.State[k], &v); e != nil {
					t.Fatal(e)
				}
				return parse(v)
			}
			fee := func(k string) TokenTransferFee {
				var f struct {
					BasisPoints uint16 `json:"basis_points"`
					MaximumFee  string `json:"maximum_fee"`
				}
				json.Unmarshal(c.State[k], &f)
				return TokenTransferFee{f.BasisPoints, parse(f.MaximumFee)}
			}
			state := LaunchLabQuoteState{VirtualBase: n("virtual_base"), VirtualQuote: n("virtual_quote"), RealBase: n("real_base"), RealQuote: n("real_quote"), TotalBaseSell: n("total_base_sell"), TradeFeeRate: n("trade_fee_rate"), PlatformFeeRate: n("platform_fee_rate"), CreatorFeeRate: n("creator_fee_rate"), BaseTransferFee: fee("base_transfer_fee"), QuoteTransferFee: fee("quote_transfer_fee")}
			for _, buy := range []bool{true, false} {
				want := c.Sell
				if buy {
					want = c.Buy
				}
				got, e := QuoteLaunchLabExactIn(state, parse(c.Amount), buy, c.SlippageBps, 0)
				if e != nil {
					t.Fatal(e)
				}
				if got != (LaunchLabQuote{parse(want.AmountIn), parse(want.MinimumAmountOut)}) {
					t.Fatalf("quote mismatch: %+v expected %+v", got, want)
				}
			}
		})
	}
}
