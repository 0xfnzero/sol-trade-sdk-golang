package calc

import (
	"errors"
	"math/big"
	"testing"
)

// Test-only pre-optimization price conversion reference, not a release binary.
var previousClmmFactors = []string{"fffcb933bd6fb800", "fff97272373d4000", "fff2e50f5f657000", "ffe5caca7e10f000", "ffcb9843d60f7000", "ff973b41fa98e800", "ff2ea16466c9b000", "fe5dee046a9a3800", "fcbe86c7900bb000", "f987a7253ac65800", "f3392b0822bb6000", "e7159475a2caf000", "d097f3bdfd2f2000", "a9f746462d9f8000", "70d869a156f31c00", "31be135f97ed3200", "9aa508b5b85a500", "5d6af8dedc582c", "2216e584f5fa"}

func previousClmmSqrtPriceAtTick(tick int32) (*big.Int, error) {
	if tick < ClmmMinTick || tick > ClmmMaxTick {
		return nil, errors.New("CLMM tick outside range")
	}
	abs := tick
	if abs < 0 {
		abs = -abs
	}
	ratio := cCopy(clmmQ64)
	for b, s := range previousClmmFactors {
		if abs&(1<<b) != 0 {
			f, _ := new(big.Int).SetString(s, 16)
			ratio.Rsh(cMul(ratio, f), 64)
		}
	}
	if tick > 0 {
		return cDiv(clmmU128, ratio), nil
	}
	return ratio, nil
}
func previousClmmTickAtSqrtPrice(price *big.Int) (int32, error) {
	if price == nil || price.Cmp(clmmMinPrice) < 0 || price.Cmp(clmmMaxPrice) >= 0 {
		return 0, errors.New("CLMM price outside range")
	}
	lo, hi := ClmmMinTick, ClmmMaxTick
	for lo < hi {
		m := lo + (hi-lo+1)/2
		p, _ := previousClmmSqrtPriceAtTick(m)
		if p.Cmp(price) <= 0 {
			lo = m
		} else {
			hi = m - 1
		}
	}
	return lo, nil
}

func TestClmmPreparsedFactorOwnership(t *testing.T) {
	for tick := ClmmMinTick; tick <= ClmmMaxTick; tick += 997 {
		got, e := ClmmSqrtPriceAtTick(tick)
		want, _ := previousClmmSqrtPriceAtTick(tick)
		if e != nil || got.Cmp(want) != 0 {
			t.Fatal(tick, e)
		}
		got.SetInt64(0)
		fresh, _ := ClmmSqrtPriceAtTick(tick)
		if fresh.Cmp(want) != 0 {
			t.Fatal("shared constant mutated", tick)
		}
	}
	for _, tick := range []int32{ClmmMinTick, -1, 0, 1, ClmmMaxTick} {
		got, _ := ClmmSqrtPriceAtTick(tick)
		want, _ := previousClmmSqrtPriceAtTick(tick)
		if got.Cmp(want) != 0 {
			t.Fatal(tick)
		}
	}
}
func BenchmarkClmmPriceReview(b *testing.B) {
	for _, mode := range []string{"tick_to_price", "price_to_tick"} {
		for _, previous := range []bool{false, true} {
			name := "current"
			if previous {
				name = "previous_reference"
			}
			price, _ := ClmmSqrtPriceAtTick(-123456)
			b.Run(mode+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var err error
					if mode == "tick_to_price" {
						if previous {
							_, err = previousClmmSqrtPriceAtTick(-123456)
						} else {
							_, err = ClmmSqrtPriceAtTick(-123456)
						}
					} else {
						if previous {
							_, err = previousClmmTickAtSqrtPrice(price)
						} else {
							_, err = ClmmTickAtSqrtPrice(price)
						}
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
