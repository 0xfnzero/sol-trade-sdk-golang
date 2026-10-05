package calc

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestDammIntegerReview(t *testing.T) {
	data, e := os.ReadFile("testdata/damm_integer_review_20261005.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		A, B, Amount, Fee, Slippage, Wanted, Out, Minimum, Liquidity, Feeout, Needed string
		Buy                                                                          bool
	}
	if e = json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	n := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for k, c := range cases {
		a, b, amount := n(c.A), n(c.B), n(c.Amount)
		q := MeteoraDammV2ComputeSwapAmount(a, b, c.Buy, amount, n(c.Slippage))
		if q.AmountOut != n(c.Out) || q.MinAmountOut != n(c.Minimum) || MeteoraDammV2CalculateLiquidity(a, b) != n(c.Liquidity) {
			t.Fatal(k, q, c)
		}
		i, o := a, b
		if !c.Buy {
			i, o = o, i
		}
		if got := MeteoraDammV2GetAmountOut(amount, i, o, n(c.Fee)); got != n(c.Feeout) {
			t.Fatal(k, got, c.Feeout)
		}
		if got := MeteoraDammV2GetAmountIn(n(c.Wanted), i, o, n(c.Fee)); got != n(c.Needed) {
			t.Fatal(k, got, c.Needed)
		}
		needed := n(c.Needed)
		if needed > 0 && n(c.Wanted) > 0 {
			if MeteoraDammV2GetAmountOut(needed, i, o, n(c.Fee)) < n(c.Wanted) || MeteoraDammV2GetAmountOut(needed-1, i, o, n(c.Fee)) >= n(c.Wanted) {
				t.Fatal("inverse is not minimal", k)
			}
		}
	}
}

func TestDammInverseRoundingReview(t *testing.T) {
	if MeteoraDammV2GetAmountIn(1, 1, 3, 5000) != 2 {
		t.Fatal("combined ceiling underfunded input")
	}
}
