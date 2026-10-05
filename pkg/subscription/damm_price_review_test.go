package subscription

import (
	"math/big"
	"math/rand"
	"testing"
)

func previousReviewDammU128(value [16]byte) *big.Int {
	var b [16]byte
	for i := range value {
		b[15-i] = value[i]
	}
	return new(big.Int).SetBytes(b[:])
}
func TestDammFixedWidthCompareReview(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for i := 0; i < 10000; i++ {
		var a, b [16]byte
		rng.Read(a[:])
		rng.Read(b[:])
		if i%3 == 0 {
			b = a
		}
		if dammCompareU128(a, b) != previousReviewDammU128(a).Cmp(previousReviewDammU128(b)) {
			t.Fatal(i, a, b)
		}
	}
}

var dammReviewCompareSink int

func BenchmarkDammPriceValidationReview(b *testing.B) {
	price, min, max, liq := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 1}, [16]byte{1}, [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, [16]byte{0, 1}
	for _, previous := range []bool{false, true} {
		name := "current"
		if previous {
			name = "previous_reference"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				price[0] = byte(i)
				if previous {
					p, n, x, l := previousReviewDammU128(price), previousReviewDammU128(min), previousReviewDammU128(max), previousReviewDammU128(liq)
					dammReviewCompareSink = p.Cmp(n) + p.Cmp(x) + l.Sign() + n.Sign()
				} else {
					nonzero, minnonzero := 0, 0
					if liq != [16]byte{} {
						nonzero = 1
					}
					if min != [16]byte{} {
						minnonzero = 1
					}
					dammReviewCompareSink = dammCompareU128(price, min) + dammCompareU128(price, max) + nonzero + minnonzero
				}
			}
		})
	}
}
