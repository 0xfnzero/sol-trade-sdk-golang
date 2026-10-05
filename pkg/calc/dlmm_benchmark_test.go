package calc

import (
	"errors"
	"math/big"
	"testing"
)

func previousDlmmSwapExactIn(pool DlmmPool, bins []DlmmBin, loadedArrays []int64, amount uint64, timestamp int64, swapForY, supportOrders, strict, exhaustive bool) (DlmmQuote, error) {
	empty := DlmmQuote{}
	sp, vp := pool.Static, pool.Variable
	if timestamp < 0 || vp.LastTimestamp < 0 || vp.LastTimestamp > timestamp || pool.ActiveID < -443636 || pool.ActiveID > 443636 || pool.BinStep == 0 || pool.FeeMode > 1 || sp.Power > 18 || sp.ReductionFactor > 10000 {
		return empty, errors.New("invalid DLMM pool/fee/time state")
	}
	loaded := map[int64]bool{}
	lo, hi := int64(0), int64(-1)
	for _, i := range loadedArrays {
		if i < -6338 || i > 6337 || i*70 > 443636 || (i+1)*70 <= -443636 || loaded[i] {
			return empty, errors.New("invalid/duplicate DLMM array")
		}
		if len(loaded) == 0 || i*70 < lo {
			lo = i * 70
		}
		if len(loaded) == 0 || i*70+69 > hi {
			hi = i*70 + 69
		}
		loaded[i] = true
	}
	byID := map[int32]DlmmBin{}
	seen := map[int32]bool{}
	for _, b := range bins {
		if b.BinID < -443636 || b.BinID > 443636 || seen[b.BinID] || !loaded[dlmmArrayIndex(b.BinID)] || b.Price == nil || b.Price.Sign() < 0 || b.Price.BitLen() > 128 {
			return empty, errors.New("invalid/duplicate/unloaded DLMM bin")
		}
		seen[b.BinID] = true
		if b.Price.Sign() == 0 {
			if b.AmountX != 0 || b.AmountY != 0 || b.OpenOrder != 0 || b.ProcessedOrder != 0 {
				return empty, errors.New("zero DLMM price with liquidity")
			}
		} else {
			byID[b.BinID] = b
		}
	}
	ref, index := uint64(vp.Reference), int64(vp.IndexReference)
	elapsed := timestamp - vp.LastTimestamp
	if elapsed >= int64(sp.FilterPeriod) {
		index = int64(pool.ActiveID)
		if elapsed < int64(sp.DecayPeriod) {
			ref = uint64(vp.Volatility) * uint64(sp.ReductionFactor) / 10000
		} else {
			ref = 0
		}
	}
	feeInput := pool.FeeMode == 0 || !swapForY
	step := int32(1)
	if swapForY {
		step = -1
	}
	current, remaining, total, crossed := pool.ActiveID, amount, uint64(0), uint32(0)
	q64 := new(big.Int).Lsh(big.NewInt(1), 64)
	precision := big.NewInt(1000000000)
	for remaining != 0 && current >= -443636 && current <= 443636 {
		if !loaded[dlmmArrayIndex(current)] {
			if exhaustive {
				if int64(current) < lo || int64(current) > hi {
					break
				}
				current += step
				continue
			}
			id := current
			q := DlmmQuote{total, remaining, crossed, false, &id}
			if strict {
				return q, &InsufficientDlmmArrays{q}
			}
			return q, nil
		}
		b, ok := byID[current]
		if !ok {
			current += step
			continue
		}
		reserve := b.AmountX
		if swapForY {
			reserve = b.AmountY
		}
		tiers := []uint64{reserve, 0, 0}
		if supportOrders && ((swapForY && b.AskSide == 0) || (!swapForY && b.AskSide != 0)) {
			tiers[1] = b.ProcessedOrder
			tiers[2] = b.OpenOrder
		}
		if tiers[0] == 0 && tiers[1] == 0 && tiers[2] == 0 {
			current += step
			continue
		}
		delta := index - int64(current)
		if delta < 0 {
			delta = -delta
		}
		vol := ref + uint64(delta)*10000
		if vol > uint64(sp.MaximumVolatility) {
			vol = uint64(sp.MaximumVolatility)
		}
		va := new(big.Int).SetUint64(vol * uint64(pool.BinStep))
		variable := dlmmCeil(new(big.Int).Mul(new(big.Int).Mul(va, va), new(big.Int).SetUint64(uint64(sp.Control))), big.NewInt(100000000000))
		base := new(big.Int).Mul(new(big.Int).SetUint64(uint64(sp.BaseFactor)*uint64(pool.BinStep)*10), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(sp.Power)), nil))
		rate := new(big.Int).Add(base, variable)
		if rate.Cmp(big.NewInt(100000000)) > 0 {
			rate.SetInt64(100000000)
		}
		left := new(big.Int).SetUint64(remaining)
		if feeInput {
			left.Sub(left, dlmmCeil(new(big.Int).Mul(left, rate), precision))
		}
		used, out := new(big.Int), new(big.Int)
		for _, r := range tiers {
			if left.Sign() == 0 {
				break
			}
			numerator, denominator := b.Price, q64
			if swapForY {
				numerator, denominator = q64, b.Price
			}
			needed := dlmmCeil(new(big.Int).Mul(new(big.Int).SetUint64(r), numerator), denominator)
			consumed, produced := new(big.Int), new(big.Int)
			if left.Cmp(needed) >= 0 {
				consumed.Set(needed)
				produced.SetUint64(r)
			} else {
				consumed.Set(left)
				produced.Quo(new(big.Int).Mul(left, denominator), numerator)
			}
			left.Sub(left, consumed)
			used.Add(used, consumed)
			out.Add(out, produced)
		}
		if !feeInput {
			out.Sub(out, dlmmCeil(new(big.Int).Mul(out, rate), precision))
		}
		sum := new(big.Int).Add(new(big.Int).SetUint64(total), out)
		if !sum.IsUint64() {
			return empty, errors.New("DLMM output overflows u64")
		}
		total = sum.Uint64()
		if left.Sign() != 0 {
			if feeInput {
				used = dlmmCeil(new(big.Int).Mul(used, precision), new(big.Int).Sub(precision, rate))
			}
			if !used.IsUint64() || used.Uint64() > remaining {
				return empty, errors.New("DLMM input overconsumption")
			}
			remaining -= used.Uint64()
			crossed++
			current += step
		} else {
			return DlmmQuote{total, 0, crossed + 1, true, nil}, nil
		}
	}
	return DlmmQuote{total, remaining, crossed + 1, true, nil}, nil
}

// Previous function is an algorithm reference, not a historical binary.
func BenchmarkDlmmReview(b *testing.B) {
	pool := DlmmPool{ActiveID: 0, BinStep: 25, Static: DlmmStaticFee{BaseFactor: 20, Power: 2, Control: 100000, MaximumVolatility: 1000000, FilterPeriod: 10, DecayPeriod: 120, ReductionFactor: 5000}, Variable: DlmmVariableFee{LastTimestamp: 100}}
	for _, mode := range []string{"dense", "sparse"} {
		arrays := []int64{}
		bins := []DlmmBin{}
		if mode == "dense" {
			arrays = []int64{-2, -1, 0}
			for i := int32(-100); i <= 0; i++ {
				bins = append(bins, DlmmBin{BinID: i, AmountY: 10, Price: new(big.Int).Lsh(big.NewInt(1), 64)})
			}
		}
		if mode == "sparse" {
			for i := int64(-500); i <= 0; i++ {
				arrays = append(arrays, i)
			}
			bins = []DlmmBin{{BinID: -35000, AmountY: 1000000, Price: new(big.Int).Lsh(big.NewInt(1), 64)}}
		}
		for _, fn := range []struct {
			name string
			swap func(DlmmPool, []DlmmBin, []int64, uint64, int64, bool, bool, bool, bool) (DlmmQuote, error)
		}{{"current", DlmmSwapExactIn}, {"previous_reference", previousDlmmSwapExactIn}} {
			b.Run(mode+"/"+fn.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := fn.swap(pool, bins, arrays, 1000, 115, true, true, true, false); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
