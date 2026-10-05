package calc

// Native Orca integer exact-in traversal with adaptive fee reference/spacing rules.
import (
	"errors"
	"math/big"
	"sort"
)

var whirlpoolMinPrice = big.NewInt(4295048016)
var whirlpoolMaxPrice = clmmHexDecimal("79226673515401279992447579055")

func wpFactors(v []string) []*big.Int {
	out := make([]*big.Int, len(v))
	for i, s := range v {
		out[i] = clmmHexDecimal(s)
	}
	return out
}

var wpPositive = wpFactors([]string{"79232123823359799118286999567", "79236085330515764027303304731", "79244008939048815603706035061", "79259858533276714757314932305", "79291567232598584799939703904", "79355022692464371645785046466", "79482085999252804386437311141", "79736823300114093921829183326", "80248749790819932309965073892", "81282483887344747381513967011", "83390072131320151908154831281", "87770609709833776024991924138", "97234110755111693312479820773", "119332217159966728226237229890", "179736315981702064433883588727", "407748233172238350107850275304", "2098478828474011932436660412517", "55581415166113811149459800483533", "38992368544603139932233054999993551"})
var wpNegative = wpFactors([]string{"18445821805675392311", "18444899583751176498", "18443055278223354162", "18439367220385604838", "18431993317065449817", "18417254355718160513", "18387811781193591352", "18329067761203520168", "18212142134806087854", "17980523815641551639", "17526086738831147013", "16651378430235024244", "15030750278693429944", "12247334978882834399", "8131365268884726200", "3584323654723342297", "696457651847595233", "26294789957452057", "37481735321082"})

func WhirlpoolSqrtPriceAtTick(tick int32) (*big.Int, error) {
	if tick < ClmmMinTick || tick > ClmmMaxTick {
		return nil, errors.New("Whirlpool tick outside range")
	}
	positive := tick >= 0
	abs := tick
	if abs < 0 {
		abs = -abs
	}
	shift := uint(64)
	factors := wpNegative
	if positive {
		shift = 96
		factors = wpPositive
	}
	ratio := new(big.Int).Lsh(big.NewInt(1), shift)
	for b, f := range factors {
		if abs&(1<<b) != 0 {
			ratio.Rsh(cMul(ratio, f), shift)
		}
	}
	if positive {
		ratio.Rsh(ratio, 32)
	}
	return ratio, nil
}
func WhirlpoolTickAtSqrtPrice(price *big.Int) (int32, error) {
	if price == nil || price.Cmp(whirlpoolMinPrice) < 0 || price.Cmp(whirlpoolMaxPrice) > 0 {
		return 0, errors.New("Whirlpool price outside range")
	}
	lo, hi := ClmmMinTick, ClmmMaxTick
	for lo < hi {
		m := lo + (hi-lo+1)/2
		p, _ := WhirlpoolSqrtPriceAtTick(m)
		if p.Cmp(price) <= 0 {
			lo = m
		} else {
			hi = m - 1
		}
	}
	return lo, nil
}

type WhirlpoolAdaptiveFee struct {
	FilterPeriod, DecayPeriod, ReductionFactor, TickGroupSize         uint16
	ControlFactor, MaximumVolatility, VolatilityReference, Volatility uint32
	LastReferenceTimestamp, LastMajorSwapTimestamp                    uint64
	ReferenceGroup                                                    int32
}
type WhirlpoolSwapResult struct {
	Consumed, AmountOut, Fee       uint64
	MinimumFeeRate, MaximumFeeRate uint32
	SqrtPrice                      *big.Int
	TickCurrent                    int32
	Liquidity                      *big.Int
}

func WhirlpoolSwapExactIn(pool ClmmPool, ticks []ClmmTick, arrayStarts []int32, amount, timestamp uint64, down bool, adaptive *WhirlpoolAdaptiveFee, limit *big.Int) (WhirlpoolSwapResult, error) {
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
		if s%step != 0 || s > ClmmMaxTick || int64(s)+int64(step) <= int64(ClmmMinTick) || i > 0 && int64(s)-int64(starts[i-1]) != int64(step) {
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
	signedBound := new(big.Int).Lsh(big.NewInt(1), 127)
	negativeSignedBound := new(big.Int).Neg(signedBound)
	for _, t := range ticks {
		if t.Tick < lower || t.Tick > upper || t.Tick%int32(pool.TickSpacing) != 0 || t.LiquidityNet == nil || t.LiquidityNet.Cmp(negativeSignedBound) < 0 || t.LiquidityNet.Cmp(signedBound) >= 0 {
			return empty, errors.New("invalid or duplicate Whirlpool tick")
		}
	}
	orderedTicks := ticks
	if !sort.SliceIsSorted(ticks, func(i, j int) bool { return ticks[i].Tick < ticks[j].Tick }) {
		orderedTicks = append([]ClmmTick(nil), ticks...)
		sort.Slice(orderedTicks, func(i, j int) bool { return orderedTicks[i].Tick < orderedTicks[j].Tick })
	}
	for i := 1; i < len(orderedTicks); i++ {
		if orderedTicks[i-1].Tick == orderedTicks[i].Tick {
			return empty, errors.New("invalid or duplicate Whirlpool tick")
		}
	}
	var cachedIndex int32
	var cachedPrice *big.Int
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
		position := sort.Search(len(orderedTicks), func(i int) bool { return orderedTicks[i].Tick > tick })
		if down {
			position--
		}
		if position >= 0 && position < len(orderedTicks) {
			next = &orderedTicks[position]
		}
		nextIndex := upper
		if down {
			nextIndex = lower
		}
		if next != nil {
			nextIndex = next.Tick
		}
		if cachedPrice == nil || nextIndex != cachedIndex {
			var e error
			cachedPrice, e = WhirlpoolSqrtPriceAtTick(nextIndex)
			if e != nil {
				return empty, e
			}
			cachedIndex = nextIndex
		}
		nextPrice := cachedPrice
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
