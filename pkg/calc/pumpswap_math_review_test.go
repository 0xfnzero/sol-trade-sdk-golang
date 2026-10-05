package calc

import (
	"errors"
	"math/big"
	"math/rand"
	"testing"
)

func previousReviewPumpSwapInt(v uint64) *big.Int { return new(big.Int).SetUint64(v) }
func previousReviewPumpSwapDiv(n, d *big.Int, ceil bool) (uint64, error) {
	if d.Sign() == 0 {
		return 0, ErrDivisionByZero
	}
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, d, r)
	if ceil && r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsUint64() {
		return 0, ErrOverflow
	}
	return q.Uint64(), nil
}
func previousReviewPumpSwapReserves(base, quote uint64, virtual *big.Int) (uint64, error) {
	if base == 0 || quote == 0 {
		return 0, ErrInvalidReserves
	}
	effective, err := EffectiveQuoteReserves(quote, virtual)
	if err != nil {
		return 0, err
	}
	if effective == 0 {
		return 0, ErrInvalidReserves
	}
	return effective, nil
}
func previousReviewPumpSwapFeeSum(f PumpSwapFeeBasisPoints) (uint64, error) {
	if err := checkAddOverflow(f.LPFeeBasisPoints, f.ProtocolFeeBasisPoints); err != nil {
		return 0, err
	}
	sum := f.LPFeeBasisPoints + f.ProtocolFeeBasisPoints
	if err := checkAddOverflow(sum, f.CoinCreatorFeeBasisPoints); err != nil {
		return 0, err
	}
	return sum + f.CoinCreatorFeeBasisPoints, nil
}
func previousReviewPumpSwapFees(amount uint64, f PumpSwapFeeBasisPoints) (uint64, uint64, error) {
	lp, err := ComputeFee(amount, f.LPFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	protocol, err := ComputeFee(amount, f.ProtocolFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	creator, err := ComputeFee(amount, f.CoinCreatorFeeBasisPoints)
	if err != nil {
		return 0, 0, err
	}
	if err := checkAddOverflow(lp, protocol); err != nil {
		return 0, 0, err
	}
	sum := lp + protocol
	if err := checkAddOverflow(sum, creator); err != nil {
		return 0, 0, err
	}
	return sum + creator, lp, nil
}

// Compare exact integer and error behavior, including the >128-bit fallback.
func TestPumpSwapFixedWidthDivisionReview(t *testing.T) {
	max := ^uint64(0)
	pairs := [][2]*big.Int{
		{big.NewInt(0), big.NewInt(1)},
		{new(big.Int).SetUint64(max), big.NewInt(1)},
		{new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), new(big.Int).SetUint64(max)},
		{new(big.Int).Lsh(big.NewInt(1), 128), new(big.Int).Lsh(big.NewInt(1), 65)},
		{new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), new(big.Int).Lsh(big.NewInt(1), 65)},
		{big.NewInt(1), big.NewInt(0)},
	}
	// Quotient=max, remainder=d-1: ceil alone overflows.
	pairs = append(pairs, [2]*big.Int{new(big.Int).Add(new(big.Int).Mul(new(big.Int).SetUint64(max), big.NewInt(10000)), big.NewInt(9999)), big.NewInt(10000)})
	rng := rand.New(rand.NewSource(20261005))
	for i := 0; i < 10000; i++ {
		hi, lo, d := rng.Uint64(), rng.Uint64(), rng.Uint64()
		if i%3 == 0 {
			hi %= 10000
			d = 10000
		}
		n := new(big.Int).Lsh(new(big.Int).SetUint64(hi), 64)
		n.Add(n, new(big.Int).SetUint64(lo))
		pairs = append(pairs, [2]*big.Int{n, new(big.Int).SetUint64(d)})
	}
	for i, p := range pairs {
		for _, ceil := range []bool{false, true} {
			n, d := new(big.Int).Set(p[0]), new(big.Int).Set(p[1])
			got, e := pumpSwapDiv(n, d, ceil)
			want, we := previousReviewPumpSwapDiv(n, d, ceil)
			if got != want || !errors.Is(e, we) {
				t.Fatalf("case %d ceil=%v got=%d/%v want=%d/%v", i, ceil, got, e, want, we)
			}
			if n.Cmp(p[0]) != 0 || d.Cmp(p[1]) != 0 {
				t.Fatal("mutated input")
			}
		}
	}
}
func TestPumpSwapFixedWidthFeesReview(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for i := 0; i < 10000; i++ {
		amount := rng.Uint64()
		f := PumpSwapFeeBasisPoints{rng.Uint64(), rng.Uint64(), rng.Uint64()}
		if i%2 == 0 {
			f = PumpSwapFeeBasisPoints{rng.Uint64() % 10001, rng.Uint64() % 10001, rng.Uint64() % 10001}
		}
		got, lp, e := pumpSwapFees(amount, f)
		want, wlp, we := previousReviewPumpSwapFees(amount, f)
		if got != want || lp != wlp || !errors.Is(e, we) {
			t.Fatalf("case %d got=%d/%d/%v want=%d/%d/%v", i, got, lp, e, want, wlp, we)
		}
	}
	for _, amount := range []uint64{0, 1, 9999, 10000, ^uint64(0)} {
		for _, rate := range []uint64{0, 1, 9999, 10000, 10001, ^uint64(0)} {
			f := PumpSwapFeeBasisPoints{rate, 0, 0}
			got, lp, e := pumpSwapFees(amount, f)
			want, wlp, we := previousReviewPumpSwapFees(amount, f)
			if got != want || lp != wlp || !errors.Is(e, we) {
				t.Fatal(amount, rate, got, e, want, we)
			}
		}
	}
}

var reviewPumpSwapMathSink uint64

func BenchmarkPumpSwapMathReview(b *testing.B) {
	n := new(big.Int).Mul(big.NewInt(500000000000), big.NewInt(1000000000000))
	d := big.NewInt(1000000010000)
	f := PumpSwapFeeBasisPoints{20, 5, 5}
	for _, operation := range []string{"floor_division", "three_fees"} {
		for _, previous := range []bool{false, true} {
			name := "current"
			if previous {
				name = "previous_reference"
			}
			b.Run(operation+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var e error
					var result, lp uint64
					if operation == "floor_division" {
						if previous {
							result, e = previousReviewPumpSwapDiv(n, d, false)
						} else {
							result, e = pumpSwapDiv(n, d, false)
						}
					} else {
						amount := uint64(10000000000) + uint64(i&1)
						if previous {
							result, lp, e = previousReviewPumpSwapFees(amount, f)
						} else {
							result, lp, e = pumpSwapFees(amount, f)
						}
					}
					reviewPumpSwapMathSink = result ^ lp
					if e != nil {
						b.Fatal(e)
					}
				}
			})
		}
	}
}
