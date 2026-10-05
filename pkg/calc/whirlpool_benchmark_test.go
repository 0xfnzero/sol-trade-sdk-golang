package calc

import (
	"errors"
	"math/big"
	"sort"
	"testing"
)

func previousWhirlpoolSwapExactIn(pool ClmmPool, ticks []ClmmTick, arrayStarts []int32, amount, timestamp uint64, down bool, adaptive *WhirlpoolAdaptiveFee, limit *big.Int) (WhirlpoolSwapResult, error) {
	var empty WhirlpoolSwapResult
	if amount == 0 || pool.TickSpacing == 0 || pool.FeeRate > 65535 || !cValid(pool.SqrtPrice, 128) || !cValid(pool.Liquidity, 128) || pool.SqrtPrice.Cmp(whirlpoolMinPrice) < 0 || pool.SqrtPrice.Cmp(whirlpoolMaxPrice) > 0 || pool.TickCurrent < ClmmMinTick || pool.TickCurrent > ClmmMaxTick {
		return empty, errors.New("invalid Whirlpool pool/input")
	}
	starts := append([]int32(nil), arrayStarts...)
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	step := int32(pool.TickSpacing) * 88
	if len(starts) < 1 || len(starts) > 6 {
		return empty, errors.New("invalid Whirlpool tick array sequence")
	}
	for i, s := range starts {
		if s%step != 0 || i > 0 && s-starts[i-1] != step {
			return empty, errors.New("invalid Whirlpool tick array sequence")
		}
	}
	lower, upper := starts[0], starts[len(starts)-1]+step-1
	if lower < ClmmMinTick {
		lower = ClmmMinTick
	}
	if upper > ClmmMaxTick {
		upper = ClmmMaxTick
	}
	if limit == nil || limit.Sign() == 0 {
		limit = whirlpoolMaxPrice
		if down {
			limit = whirlpoolMinPrice
		}
	}
	if !cValid(limit, 128) || limit.Cmp(whirlpoolMinPrice) < 0 || limit.Cmp(whirlpoolMaxPrice) > 0 || (down && limit.Cmp(pool.SqrtPrice) >= 0) || (!down && limit.Cmp(pool.SqrtPrice) <= 0) {
		return empty, errors.New("invalid Whirlpool price limit")
	}
	seen := map[int32]bool{}
	signedBound := new(big.Int).Lsh(big.NewInt(1), 127)
	for _, t := range ticks {
		if seen[t.Tick] || t.Tick < lower || t.Tick > upper || t.Tick%int32(pool.TickSpacing) != 0 || t.LiquidityNet == nil || t.LiquidityNet.Cmp(new(big.Int).Neg(signedBound)) < 0 || t.LiquidityNet.Cmp(signedBound) >= 0 {
			return empty, errors.New("invalid or duplicate Whirlpool tick")
		}
		seen[t.Tick] = true
	}
	group, reference := int64(0), int64(0)
	volatilityReference, maximum, control := uint64(0), uint64(0), uint64(0)
	groupSize := int64(1)
	var lowerGroup, upperGroup *int64
	if adaptive != nil {
		a := adaptive
		if a.TickGroupSize == 0 || a.ReductionFactor > 10000 || a.VolatilityReference > a.MaximumVolatility || uint64(a.MaximumVolatility)*uint64(a.TickGroupSize) > 0xffffffff {
			return empty, errors.New("invalid Whirlpool adaptive fee")
		}
		last := a.LastReferenceTimestamp
		if a.LastMajorSwapTimestamp > last {
			last = a.LastMajorSwapTimestamp
		}
		if timestamp < last {
			return empty, errors.New("Whirlpool timestamp predates adaptive reference")
		}
		groupSize = int64(a.TickGroupSize)
		group = int64(cFloor(pool.TickCurrent, int32(a.TickGroupSize)))
		reference = int64(a.ReferenceGroup)
		volatilityReference = uint64(a.VolatilityReference)
		maximum = uint64(a.MaximumVolatility)
		control = uint64(a.ControlFactor)
		if timestamp-a.LastReferenceTimestamp > 3600 || timestamp-last >= uint64(a.DecayPeriod) {
			reference = group
			volatilityReference = 0
		} else if timestamp-last >= uint64(a.FilterPeriod) {
			reference = group
			volatilityReference = uint64(a.Volatility) * uint64(a.ReductionFactor) / 10000
		}
		if volatilityReference > maximum {
			return empty, errors.New("invalid Whirlpool volatility reference")
		}
		distance := int64((maximum - volatilityReference + 9999) / 10000)
		lg, ug := reference-distance, reference+distance
		if lg*groupSize > int64(ClmmMinTick) {
			lowerGroup = &lg
		}
		if (ug+1)*groupSize < int64(ClmmMaxTick) {
			upperGroup = &ug
		}
	}
	current, liquidity := cCopy(pool.SqrtPrice), cCopy(pool.Liquidity)
	tick := pool.TickCurrent
	remaining, output, fees := amount, uint64(0), uint64(0)
	minimumFee, maximumFee := pool.FeeRate, pool.FeeRate
	first := true
	for iter := 0; iter < 8192; iter++ {
		if remaining == 0 || current.Cmp(limit) == 0 {
			return WhirlpoolSwapResult{amount - remaining, output, fees, minimumFee, maximumFee, cCopy(current), tick, cCopy(liquidity)}, nil
		}
		if (down && tick < lower) || (!down && tick >= upper) {
			return empty, errors.New("Whirlpool quote requires more tick arrays")
		}
		var next *ClmmTick
		for i := range ticks {
			t := &ticks[i]
			if ((down && t.Tick <= tick) || (!down && t.Tick > tick)) && (next == nil || (down && t.Tick > next.Tick) || (!down && t.Tick < next.Tick)) {
				next = t
			}
		}
		nextIndex := upper
		if down {
			nextIndex = lower
		}
		if next != nil {
			nextIndex = next.Tick
		}
		nextPrice, e := WhirlpoolSqrtPriceAtTick(nextIndex)
		if e != nil {
			return empty, e
		}
		target := cMin(nextPrice, limit)
		if down {
			target = cMax(nextPrice, limit)
		}
		fee := pool.FeeRate
		bound := cCopy(target)
		skipped := false
		if adaptive != nil {
			distance := reference - group
			if distance < 0 {
				distance = -distance
			}
			volatility := volatilityReference + uint64(distance)*10000
			if volatility > maximum {
				volatility = maximum
			}
			crossed := llN(volatility * uint64(groupSize))
			extra := llCeil(cMul(llN(control), cMul(crossed, crossed)), llN(10000000000000))
			if extra.Cmp(llN(100000)) > 0 {
				extra = llN(100000)
			}
			f := cAdd(llN(uint64(fee)), extra)
			if f.Cmp(llN(100000)) > 0 {
				fee = 100000
			} else {
				fee = uint32(f.Uint64())
			}
			skipped = control == 0 || liquidity.Sign() == 0
			if !skipped && lowerGroup != nil && group < *lowerGroup {
				skipped = true
				if !down {
					p, e := WhirlpoolSqrtPriceAtTick(int32(*lowerGroup * groupSize))
					if e != nil {
						return empty, e
					}
					bound = cMin(target, p)
				}
			} else if !skipped && upperGroup != nil && group > *upperGroup {
				skipped = true
				if down {
					p, e := WhirlpoolSqrtPriceAtTick(int32((*upperGroup + 1) * groupSize))
					if e != nil {
						return empty, e
					}
					bound = cMax(target, p)
				}
			} else if !skipped {
				g := group
				if !down {
					g++
				}
				boundary := g * groupSize
				if boundary < int64(ClmmMinTick) {
					boundary = int64(ClmmMinTick)
				}
				if boundary > int64(ClmmMaxTick) {
					boundary = int64(ClmmMaxTick)
				}
				p, e := WhirlpoolSqrtPriceAtTick(int32(boundary))
				if e != nil {
					return empty, e
				}
				bound = cMin(target, p)
				if down {
					bound = cMax(target, p)
				}
			}
		}
		if first || fee < minimumFee {
			minimumFee = fee
		}
		if first || fee > maximumFee {
			maximumFee = fee
		}
		first = false
		old := cCopy(current)
		r, e := ClmmSwapStep(current, bound, liquidity, remaining, fee, down)
		if e != nil {
			return empty, e
		}
		used := cAdd(llN(r.AmountIn), llN(r.Fee))
		left := cSub(llN(remaining), used)
		out := cAdd(llN(output), llN(r.AmountOut))
		feeSum := cAdd(llN(fees), llN(r.Fee))
		if !cValid(left, 64) || !cValid(out, 64) || !cValid(feeSum, 64) {
			return empty, errors.New("Whirlpool amount overflow")
		}
		remaining, output, fees = left.Uint64(), out.Uint64(), feeSum.Uint64()
		current = r.SqrtPrice
		if current.Cmp(nextPrice) == 0 {
			if next != nil {
				change := cCopy(next.LiquidityNet)
				if down {
					change.Neg(change)
				}
				liquidity = cAdd(liquidity, change)
				if !cValid(liquidity, 128) {
					return empty, errors.New("Whirlpool liquidity crossing overflow")
				}
			}
			tick = nextIndex
			if down {
				tick--
			}
		} else if current.Cmp(old) != 0 {
			tick, e = WhirlpoolTickAtSqrtPrice(current)
			if e != nil {
				return empty, e
			}
		}
		if adaptive != nil {
			if skipped {
				ti := nextIndex
				if current.Cmp(nextPrice) != 0 {
					ti, e = WhirlpoolTickAtSqrtPrice(current)
					if e != nil {
						return empty, e
					}
				}
				p, e := WhirlpoolSqrtPriceAtTick(ti)
				if e != nil {
					return empty, e
				}
				onBoundary := int64(ti)%groupSize == 0 && current.Cmp(p) == 0
				lastGroup := int64(cFloor(ti, int32(groupSize)))
				if !down && onBoundary {
					lastGroup--
				}
				if (down && lastGroup < group) || (!down && lastGroup > group) {
					group = lastGroup
				}
			}
			if down {
				group--
			} else {
				group++
			}
		}
	}
	return empty, errors.New("Whirlpool quote iteration budget exhausted")
}

// Reference implementation above preserves the pre-optimization algorithm;
// it is test-only, not a historical release binary.
func BenchmarkWhirlpoolReview(b *testing.B) {
	price, _ := WhirlpoolSqrtPriceAtTick(0)
	limit, _ := WhirlpoolSqrtPriceAtTick(-60)
	pool := ClmmPool{price, big.NewInt(1000000), 0, 1, 3000}
	ticks := []ClmmTick{}
	for tick := int32(-176); tick < 176; tick += 2 {
		ticks = append(ticks, ClmmTick{Tick: tick, LiquidityNet: big.NewInt(0)})
	}
	for _, mode := range []string{"static", "adaptive"} {
		var adaptive *WhirlpoolAdaptiveFee
		if mode == "adaptive" {
			adaptive = &WhirlpoolAdaptiveFee{FilterPeriod: 10, DecayPeriod: 120, ReductionFactor: 5000, ControlFactor: 100, MaximumVolatility: 100000, TickGroupSize: 4, LastReferenceTimestamp: 100, LastMajorSwapTimestamp: 100, VolatilityReference: 20000, Volatility: 25000}
		}
		for _, fn := range []struct {
			name string
			swap func(ClmmPool, []ClmmTick, []int32, uint64, uint64, bool, *WhirlpoolAdaptiveFee, *big.Int) (WhirlpoolSwapResult, error)
		}{{"current", WhirlpoolSwapExactIn}, {"previous_reference", previousWhirlpoolSwapExactIn}} {
			b.Run(mode+"/"+fn.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := fn.swap(pool, ticks, []int32{-176, -88, 0, 88}, 3000, 105, true, adaptive, limit); e != nil {
						b.Fatal(e)
					}
				}
			})
		}
	}
}
