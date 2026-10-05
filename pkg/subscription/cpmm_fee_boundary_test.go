package subscription

import "testing"

func TestCpmmDisabledCreatorRateUint64Boundary(t *testing.T) {
	p := CachedCpmmState{BaseReserve: 1000000, QuoteReserve: 2000000, TradeFeeRate: 2500}
	for _, baseIn := range []bool{true, false} {
		want, err := QuoteCachedCpmmExactIn(p, 100, baseIn, 0)
		if err != nil {
			t.Fatal(err)
		}
		maximum := p
		maximum.CreatorFeeRate = ^uint64(0)
		got, err := QuoteCachedCpmmExactIn(maximum, 100, baseIn, 0)
		if err != nil || got != want {
			t.Fatal("disabled creator rate changed quote", got, want, err)
		}
		maximum.EnableCreatorFee = true
		if _, err := QuoteCachedCpmmExactIn(maximum, 100, baseIn, 0); err == nil {
			t.Fatal("enabled excessive creator rate accepted")
		}
	}
}
