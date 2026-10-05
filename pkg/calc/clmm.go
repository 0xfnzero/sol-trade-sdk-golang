package calc

// Native Raydium CLMM exact integer math; no RPC or Rust runtime.
import (
	"encoding/binary"
	"errors"
	"math/big"
)

const ClmmMinTick int32 = -443636
const ClmmMaxTick int32 = 443636

var clmmQ64 = new(big.Int).Lsh(big.NewInt(1), 64)
var clmmU128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
var clmmMinPrice = big.NewInt(4295048016)
var clmmMaxPrice = clmmHexDecimal("79226673521066979257578248091")
var clmmFactors = func() []*big.Int {
	values := []string{"fffcb933bd6fb800", "fff97272373d4000", "fff2e50f5f657000", "ffe5caca7e10f000", "ffcb9843d60f7000", "ff973b41fa98e800", "ff2ea16466c9b000", "fe5dee046a9a3800", "fcbe86c7900bb000", "f987a7253ac65800", "f3392b0822bb6000", "e7159475a2caf000", "d097f3bdfd2f2000", "a9f746462d9f8000", "70d869a156f31c00", "31be135f97ed3200", "9aa508b5b85a500", "5d6af8dedc582c", "2216e584f5fa"}
	out := make([]*big.Int, len(values))
	for i, value := range values {
		out[i], _ = new(big.Int).SetString(value, 16)
	}
	return out
}()

func clmmHexDecimal(s string) *big.Int { n, _ := new(big.Int).SetString(s, 10); return n }
func cAdd(a, b *big.Int) *big.Int      { return new(big.Int).Add(a, b) }
func cSub(a, b *big.Int) *big.Int      { return new(big.Int).Sub(a, b) }
func cMul(a, b *big.Int) *big.Int      { return new(big.Int).Mul(a, b) }
func cDiv(a, b *big.Int) *big.Int      { return new(big.Int).Quo(a, b) }
func cCopy(a *big.Int) *big.Int        { return new(big.Int).Set(a) }
func cMin(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return cCopy(a)
	}
	return cCopy(b)
}
func cMax(a, b *big.Int) *big.Int {
	if a.Cmp(b) > 0 {
		return cCopy(a)
	}
	return cCopy(b)
}
func cValid(n *big.Int, bits int) bool { return n != nil && n.Sign() >= 0 && n.BitLen() <= bits }
func cFloor(a, b int32) int32 {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}
func ClmmSqrtPriceAtTick(tick int32) (*big.Int, error) {
	if tick < ClmmMinTick || tick > ClmmMaxTick {
		return nil, errors.New("CLMM tick outside range")
	}
	abs := tick
	if abs < 0 {
		abs = -abs
	}
	ratio := cCopy(clmmQ64)
	for b, f := range clmmFactors {
		if abs&(1<<b) != 0 {
			ratio.Rsh(cMul(ratio, f), 64)
		}
	}
	if tick > 0 {
		return cDiv(clmmU128, ratio), nil
	}
	return ratio, nil
}
func ClmmTickAtSqrtPrice(price *big.Int) (int32, error) {
	if price == nil || price.Cmp(clmmMinPrice) < 0 || price.Cmp(clmmMaxPrice) >= 0 {
		return 0, errors.New("CLMM price outside range")
	}
	lo, hi := ClmmMinTick, ClmmMaxTick
	for lo < hi {
		m := lo + (hi-lo+1)/2
		p, _ := ClmmSqrtPriceAtTick(m)
		if p.Cmp(price) <= 0 {
			lo = m
		} else {
			hi = m - 1
		}
	}
	return lo, nil
}
func cDelta(a, b, l *big.Int, token0, up bool) *big.Int {
	low, high := cMin(a, b), cMax(a, b)
	n := cMul(l, cSub(high, low))
	d := clmmQ64
	if token0 {
		n = cMul(n, clmmQ64)
		d = cMul(high, low)
	}
	if up {
		return llCeil(n, d)
	}
	return cDiv(n, d)
}

type ClmmSwapStepResult struct {
	SqrtPrice                *big.Int
	AmountIn, AmountOut, Fee uint64
}

func ClmmSwapStep(current, target, liquidity *big.Int, remaining uint64, feeRate uint32, down bool) (ClmmSwapStepResult, error) {
	var result ClmmSwapStepResult
	if !cValid(current, 128) || !cValid(target, 128) || !cValid(liquidity, 128) || current.Sign() == 0 || target.Sign() == 0 || feeRate >= 1000000 || (down && target.Cmp(current) > 0) || (!down && target.Cmp(current) < 0) {
		return result, errors.New("invalid CLMM step")
	}
	rate := llN(uint64(feeRate))
	den := llN(1000000 - uint64(feeRate))
	net := cDiv(cMul(llN(remaining), den), llN(1000000))
	needed := cDelta(current, target, liquidity, down, true)
	var next *big.Int
	if needed.Cmp(net) <= 0 {
		next = cCopy(target)
	} else if liquidity.Sign() == 0 {
		return result, errors.New("CLMM zero liquidity")
	} else if down {
		wide := new(big.Int).Lsh(cCopy(liquidity), 64)
		next = llCeil(cMul(wide, current), cAdd(wide, cMul(net, current)))
	} else {
		next = cAdd(current, cDiv(new(big.Int).Lsh(net, 64), liquidity))
	}
	input := needed
	if next.Cmp(target) != 0 {
		input = cDelta(current, next, liquidity, down, true)
	}
	output := cDelta(current, next, liquidity, !down, false)
	fee := llCeil(cMul(input, rate), den)
	if next.Cmp(target) != 0 {
		fee = cSub(llN(remaining), input)
	}
	if !cValid(next, 128) || !cValid(input, 64) || !cValid(output, 64) || !cValid(fee, 64) {
		return result, errors.New("CLMM step overflow")
	}
	return ClmmSwapStepResult{next, input.Uint64(), output.Uint64(), fee.Uint64()}, nil
}

type ClmmTick struct {
	Tick                         int32
	LiquidityNet, LiquidityGross *big.Int
	Orders, PartialOrders        uint64
}
type ClmmPool struct {
	SqrtPrice, Liquidity *big.Int
	TickCurrent          int32
	TickSpacing          uint16
	FeeRate              uint32
}
type ClmmSwapResult struct {
	Consumed, AmountOut uint64
	SqrtPrice           *big.Int
	TickCurrent         int32
	Liquidity           *big.Int
}

func ClmmSwapExactIn(pool ClmmPool, ticks []ClmmTick, amount uint64, limit *big.Int, feeOn uint8, dynamic []byte, timestamp uint64, down bool) (ClmmSwapResult, error) {
	var empty ClmmSwapResult
	if !cValid(pool.SqrtPrice, 128) || !cValid(pool.Liquidity, 128) || pool.SqrtPrice.Cmp(clmmMinPrice) < 0 || pool.SqrtPrice.Cmp(clmmMaxPrice) > 0 || pool.TickCurrent < ClmmMinTick || pool.TickCurrent > ClmmMaxTick || pool.TickSpacing == 0 || pool.FeeRate >= 1000000 || feeOn > 2 {
		return empty, errors.New("invalid CLMM pool")
	}
	if !cValid(limit, 128) || limit.Cmp(clmmMinPrice) < 0 || limit.Cmp(clmmMaxPrice) > 0 || (down && limit.Cmp(pool.SqrtPrice) >= 0) || (!down && limit.Cmp(pool.SqrtPrice) <= 0) {
		return empty, errors.New("invalid CLMM limit")
	}
	if len(dynamic) != 80 {
		return empty, errors.New("invalid CLMM dynamic bytes")
	}
	u16 := func(o int) uint16 { return binary.LittleEndian.Uint16(dynamic[o:]) }
	u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(dynamic[o:]) }
	inputFee := feeOn == 0 || (feeOn == 1 && down) || (feeOn == 2 && !down)
	enabled := false
	for _, v := range dynamic {
		enabled = enabled || v != 0
	}
	group := cFloor(pool.TickCurrent, int32(pool.TickSpacing))
	reference := int32(u32(14))
	volatilityReference, volatility := u32(18), u32(22)
	maximum, control := u32(10), u32(6)
	if enabled {
		if u16(0) == 0 || u16(2) <= u16(0) || u16(4) == 0 || u16(4) >= 10000 || control == 0 || control >= 100000 || uint64(maximum)*uint64(pool.TickSpacing) > 0xffffffff {
			return empty, errors.New("invalid CLMM dynamic fee params")
		}
		last := binary.LittleEndian.Uint64(dynamic[26:])
		elapsed := uint64(0)
		if timestamp > last {
			elapsed = timestamp - last
		}
		if elapsed >= uint64(u16(0)) {
			reference = group
			volatilityReference = 0
			if elapsed < uint64(u16(2)) {
				volatilityReference = uint32(uint64(volatility) * uint64(u16(4)) / 10000)
			}
		}
	}
	orders := make([]uint64, len(ticks))
	seen := map[int32]bool{}
	signedBound := new(big.Int).Lsh(big.NewInt(1), 127)
	negativeSignedBound := new(big.Int).Neg(signedBound)
	for i, t := range ticks {
		if seen[t.Tick] || t.Tick < ClmmMinTick || t.Tick > ClmmMaxTick || t.Tick%int32(pool.TickSpacing) != 0 || t.LiquidityNet == nil || t.LiquidityNet.Cmp(negativeSignedBound) < 0 || t.LiquidityNet.Cmp(signedBound) >= 0 || !cValid(t.LiquidityGross, 128) {
			return empty, errors.New("invalid or duplicate CLMM tick")
		}
		seen[t.Tick] = true
		v := cAdd(llN(t.Orders), llN(t.PartialOrders))
		if !v.IsUint64() {
			return empty, errors.New("CLMM order overflow")
		}
		orders[i] = v.Uint64()
	}
	current, liquidity := cCopy(pool.SqrtPrice), cCopy(pool.Liquidity)
	tick := pool.TickCurrent
	remaining, output := amount, uint64(0)
	result := func() ClmmSwapResult {
		return ClmmSwapResult{amount - remaining, output, cCopy(current), tick, cCopy(liquidity)}
	}
	addOutput := func(v *big.Int) error {
		n := cAdd(llN(output), v)
		if !cValid(n, 64) {
			return errors.New("CLMM output overflow")
		}
		output = n.Uint64()
		return nil
	}
	consume := func(v *big.Int) error {
		n := cSub(llN(remaining), v)
		if !cValid(n, 64) {
			return errors.New("CLMM input overconsumption")
		}
		remaining = n.Uint64()
		return nil
	}
	for iter := 0; iter < 8192; iter++ {
		if remaining == 0 || current.Cmp(limit) == 0 {
			return result(), nil
		}
		index := -1
		for i, t := range ticks {
			if (t.LiquidityGross.Sign() > 0 || orders[i] > 0) && ((down && t.Tick <= tick) || (!down && t.Tick > tick)) && (index < 0 || (down && t.Tick > ticks[index].Tick) || (!down && t.Tick < ticks[index].Tick)) {
				index = i
			}
		}
		nextTick := ClmmMaxTick
		if down {
			nextTick = ClmmMinTick
		}
		if index >= 0 {
			nextTick = ticks[index].Tick
		}
		nextPrice, _ := ClmmSqrtPriceAtTick(nextTick)
		target := cMin(nextPrice, limit)
		if down {
			target = cMax(nextPrice, limit)
		}
		fee := pool.FeeRate
		skipped := true
		bound := cCopy(target)
		if enabled {
			distance := int64(reference) - int64(group)
			if distance < 0 {
				distance = -distance
			}
			v := uint64(volatilityReference) + uint64(distance)*10000
			if v > uint64(maximum) {
				v = uint64(maximum)
			}
			volatility = uint32(v)
			crossed := llN(v * uint64(pool.TickSpacing))
			extra := llCeil(cMul(llN(uint64(control)), cMul(crossed, crossed)), llN(10000000000000))
			f := cAdd(llN(uint64(fee)), extra)
			if f.Cmp(llN(100000)) > 0 {
				fee = 100000
			} else {
				fee = uint32(f.Uint64())
			}
			skipped = liquidity.Sign() == 0 || volatility == maximum
			if !skipped {
				g := int64(group)
				if !down {
					g++
				}
				boundary := g * int64(pool.TickSpacing)
				if boundary < int64(ClmmMinTick) {
					boundary = int64(ClmmMinTick)
				}
				if boundary > int64(ClmmMaxTick) {
					boundary = int64(ClmmMaxTick)
				}
				p, _ := ClmmSqrtPriceAtTick(int32(boundary))
				bound = cMin(target, p)
				if down {
					bound = cMax(target, p)
				}
			}
		}
		old := cCopy(current)
		if current.Cmp(bound) != 0 {
			rate := uint32(0)
			if inputFee {
				rate = fee
			}
			step, e := ClmmSwapStep(current, bound, liquidity, remaining, rate, down)
			if e != nil {
				return empty, e
			}
			used := llN(step.AmountIn)
			if inputFee {
				used = cAdd(used, llN(step.Fee))
			}
			if e = consume(used); e != nil {
				return empty, e
			}
			net := llN(step.AmountOut)
			if !inputFee {
				net = cSub(net, llCeil(cMul(net, llN(uint64(fee))), llN(1000000)))
			}
			if e = addOutput(net); e != nil {
				return empty, e
			}
			current = step.SqrtPrice
		}
		if current.Cmp(nextPrice) == 0 {
			if index < 0 {
				break
			}
			t := ticks[index]
			if orders[index] > 0 && remaining > 0 {
				square := cMul(current, current)
				price := new(big.Int).Rsh(cCopy(square), 64)
				if !down && new(big.Int).And(square, cSub(clmmQ64, big.NewInt(1))).Sign() != 0 {
					price = cAdd(price, big.NewInt(1))
				}
				if price.Sign() == 0 {
					return empty, errors.New("CLMM zero order price")
				}
				feeAmount := big.NewInt(0)
				if inputFee {
					feeAmount = llCeil(cMul(llN(remaining), llN(uint64(fee))), llN(1000000))
				}
				available := cSub(llN(remaining), feeAmount)
				matched := cDiv(cMul(available, clmmQ64), price)
				if down {
					matched = cDiv(cMul(available, price), clmmQ64)
				}
				gross := cMin(matched, llN(orders[index]))
				consumed := available
				if matched.Cmp(llN(orders[index])) > 0 {
					if down {
						consumed = llCeil(cMul(gross, clmmQ64), price)
					} else {
						consumed = llCeil(cMul(gross, price), clmmQ64)
					}
					if !cValid(consumed, 64) {
						return empty, errors.New("CLMM order input overflow")
					}
					feeAmount = big.NewInt(0)
					if inputFee {
						feeAmount = llCeil(cMul(consumed, llN(uint64(fee))), llN(1000000-uint64(fee)))
					}
				}
				if e := consume(cAdd(consumed, feeAmount)); e != nil {
					return empty, e
				}
				orders[index] -= gross.Uint64()
				net := cCopy(gross)
				if !inputFee {
					net = cSub(net, llCeil(cMul(gross, llN(uint64(fee))), llN(1000000)))
				}
				if e := addOutput(net); e != nil {
					return empty, e
				}
			}
			if t.LiquidityGross.Sign() > 0 && orders[index] == 0 {
				change := cCopy(t.LiquidityNet)
				if down {
					change.Neg(change)
				}
				liquidity = cAdd(liquidity, change)
				if !cValid(liquidity, 128) {
					return empty, errors.New("CLMM liquidity crossing overflow")
				}
			}
			tick = nextTick
			if (down && orders[index] == 0) || (!down && orders[index] > 0) {
				tick--
			}
		} else if current.Cmp(old) != 0 {
			var e error
			tick, e = ClmmTickAtSqrtPrice(current)
			if e != nil {
				return empty, e
			}
		}
		if enabled {
			if skipped {
				boundaryTick := tick
				if current.Cmp(nextPrice) == 0 {
					boundaryTick = nextTick
				}
				group = cFloor(boundaryTick, int32(pool.TickSpacing))
				if !down && boundaryTick%int32(pool.TickSpacing) == 0 {
					group--
				}
			}
			if down {
				group--
			} else {
				group++
			}
		}
	}
	if remaining > 0 && current.Cmp(limit) != 0 {
		return empty, errors.New("CLMM quote iteration budget exhausted")
	}
	return result(), nil
}
