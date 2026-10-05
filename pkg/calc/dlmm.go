package calc

// Native exact-in DLMM bin walk. Prices are plain Q64.64, never sqrt prices.
// The caller applies current-epoch mint transfer fees outside this pure engine.
import (
	"errors"
	"fmt"
	"math/big"
	"sort"
)

type DlmmStaticFee struct {
	BaseFactor                                 uint16
	Power                                      uint8
	Control, MaximumVolatility                 uint32
	FilterPeriod, DecayPeriod, ReductionFactor uint16
}
type DlmmVariableFee struct {
	Volatility, Reference uint32
	IndexReference        int32
	LastTimestamp         int64
}
type DlmmBin struct {
	BinID                     int32
	AmountX, AmountY          uint64
	Price                     *big.Int
	OpenOrder, ProcessedOrder uint64
	AskSide                   uint8
}
type DlmmPool struct {
	ActiveID int32
	BinStep  uint16
	FeeMode  uint8
	Static   DlmmStaticFee
	Variable DlmmVariableFee
}
type DlmmQuote struct {
	AmountOut, RemainingIn uint64
	BinsCrossed            uint32
	Complete               bool
	MissingBinID           *int32
}
type InsufficientDlmmArrays struct{ Partial DlmmQuote }

func (e *InsufficientDlmmArrays) Error() string {
	return fmt.Sprintf("missing DLMM bin array for bin %d", *e.Partial.MissingBinID)
}
func dlmmArrayIndex(id int32) int64 {
	x := int64(id) / 70
	if id < 0 && id%70 != 0 {
		x--
	}
	return x
}
func dlmmCeil(a, b *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Add(a, new(big.Int).Sub(b, big.NewInt(1))), b)
}
func DlmmSwapExactIn(pool DlmmPool, bins []DlmmBin, loadedArrays []int64, amount uint64, timestamp int64, swapForY, supportOrders, strict, exhaustive bool) (DlmmQuote, error) {
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
	liveIDs := make([]int32, 0, len(bins))
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
			relevant := supportOrders && ((swapForY && b.AskSide == 0) || (!swapForY && b.AskSide != 0))
			reserve := b.AmountX
			if swapForY {
				reserve = b.AmountY
			}
			if reserve != 0 || relevant && (b.ProcessedOrder != 0 || b.OpenOrder != 0) {
				liveIDs = append(liveIDs, b.BinID)
			}
		}
	}
	for i := 1; i < len(liveIDs); i++ {
		if liveIDs[i-1] > liveIDs[i] {
			sort.Slice(liveIDs, func(i, j int) bool { return liveIDs[i] < liveIDs[j] })
			break
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
	variablePrecision := big.NewInt(100000000000)
	control := new(big.Int).SetUint64(uint64(sp.Control))
	baseRate := new(big.Int).Mul(new(big.Int).SetUint64(uint64(sp.BaseFactor)*uint64(pool.BinStep)*10), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(sp.Power)), nil))
	arrayBoundary := func(current int32) int32 {
		array := dlmmArrayIndex(current)
		if swapForY {
			return int32(array*70 - 1)
		}
		return int32((array + 1) * 70)
	}
	// A jump stops at the array edge so the next loop still detects missing
	// arrays, rather than treating unknown liquidity as an empty interval.
	advanceEmpty := func(current int32) int32 {
		position := sort.Search(len(liveIDs), func(i int) bool {
			if swapForY {
				return liveIDs[i] >= current
			}
			return liveIDs[i] > current
		})
		if swapForY {
			position--
		}
		if position >= 0 && position < len(liveIDs) && dlmmArrayIndex(liveIDs[position]) == dlmmArrayIndex(current) {
			return liveIDs[position]
		}
		return arrayBoundary(current)
	}
	for remaining != 0 && current >= -443636 && current <= 443636 {
		if !loaded[dlmmArrayIndex(current)] {
			if exhaustive {
				if int64(current) < lo || int64(current) > hi {
					break
				}
				current = arrayBoundary(current)
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
			current = advanceEmpty(current)
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
			current = advanceEmpty(current)
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
		variable := dlmmCeil(new(big.Int).Mul(new(big.Int).Mul(va, va), control), variablePrecision)
		rate := new(big.Int).Add(baseRate, variable)
		if rate.Cmp(big.NewInt(100000000)) > 0 {
			rate.SetInt64(100000000)
		}
		left := new(big.Int).SetUint64(remaining)
		if feeInput {
			left.Sub(left, dlmmCeil(new(big.Int).Mul(left, rate), precision))
		}
		used, out := new(big.Int), new(big.Int)
		numerator, denominator := b.Price, q64
		if swapForY {
			numerator, denominator = q64, b.Price
		}
		for _, r := range tiers {
			if left.Sign() == 0 {
				break
			}
			if r == 0 {
				continue
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
