package subscription

// Cache-only native Orca quote/preparation from fixed/dynamic tick arrays.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
	"strconv"
	"strings"
)

var WhirlpoolProgram = solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")

type CachedWhirlpoolQuote struct {
	AmountIn, EstimatedNetAmountOut, MinimumNetAmountOut, MinimumAmountOut, StateSlot, Epoch uint64
	MinimumFeeRate, MaximumFeeRate                                                           uint32
}

func DecodeWhirlpoolTicks(d []byte, pool solana.PublicKey, start int32, spacing uint16) ([]calc.ClmmTick, error) {
	if spacing == 0 || start%(int32(spacing)*88) != 0 {
		return nil, errors.New("invalid Whirlpool tick array start/spacing")
	}
	fixed := len(d) >= 8 && bytes.Equal(d[:8], []byte{69, 97, 189, 190, 110, 7, 66, 187})
	dynamic := len(d) >= 8 && bytes.Equal(d[:8], []byte{17, 216, 246, 142, 225, 199, 218, 56})
	if !fixed && !dynamic {
		return nil, errors.New("unknown Whirlpool array discriminator")
	}
	offset := 12
	if fixed {
		offset = 9956
	}
	if len(d) < offset+32 || len(d) < 12 || int32(binary.LittleEndian.Uint32(d[8:])) != start || !bytes.Equal(d[offset:offset+32], pool[:]) {
		return nil, errors.New("Whirlpool array identity mismatch")
	}
	ticks := []calc.ClmmTick{}
	o := 60
	if fixed {
		o = 12
	}
	for i := 0; i < 88; i++ {
		if o >= len(d) || d[o] > 1 {
			return nil, errors.New("invalid or truncated Whirlpool tick tag")
		}
		initialized := d[o] == 1
		o++
		if dynamic && (d[44+i/8]&(1<<uint(i%8)) != 0) != initialized {
			return nil, errors.New("Whirlpool tick bitmap mismatch")
		}
		if fixed || initialized {
			if o+112 > len(d) {
				return nil, errors.New("truncated Whirlpool tick")
			}
			if initialized {
				tick := start + int32(i)*int32(spacing)
				if tick < calc.ClmmMinTick || tick > calc.ClmmMaxTick {
					return nil, errors.New("Whirlpool initialized tick outside range")
				}
				net := clmmWide(d, o)
				if net.Bit(127) != 0 {
					net.Sub(net, new(big.Int).Lsh(big.NewInt(1), 128))
				}
				ticks = append(ticks, calc.ClmmTick{Tick: tick, LiquidityNet: net, LiquidityGross: clmmWide(d, o+16)})
			}
			o += 112
		}
	}
	return ticks, nil
}
func (s *AccountCacheSnapshot) PrepareWhirlpool(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int) (instruction.WhirlpoolSwapV2Accounts, CachedWhirlpoolQuote, solana.Instruction, error) {
	fail := func(e error) (instruction.WhirlpoolSwapV2Accounts, CachedWhirlpoolQuote, solana.Instruction, error) {
		return instruction.WhirlpoolSwapV2Accounts{}, CachedWhirlpoolQuote{}, nil, e
	}
	if amount == 0 || slippageBps >= 10000 || maximumArrays < 1 || maximumArrays > 32 {
		return fail(errors.New("invalid cached Whirlpool request"))
	}
	p, e := s.Get(h.Pool, ctx, &WhirlpoolProgram)
	if e != nil {
		return fail(e)
	}
	d := p.Data
	if len(d) < 653 || !bytes.Equal(d[:8], []byte{63, 149, 209, 12, 225, 128, 99, 9}) {
		return fail(errors.New("invalid Whirlpool pool"))
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	if e = h.matches(key(101), key(181)); e != nil {
		return fail(e)
	}
	spacing := binary.LittleEndian.Uint16(d[41:])
	if spacing == 0 {
		return fail(errors.New("Whirlpool spacing is zero"))
	}
	m0, e := s.Get(key(101), ctx, nil)
	if e != nil {
		return fail(e)
	}
	m1, e := s.Get(key(181), ctx, nil)
	if e != nil {
		return fail(e)
	}
	f0, e := instruction.TokenTransferFeeForEpoch(m0.Data, m0.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	f1, e := instruction.TokenTransferFeeForEpoch(m1.Data, m1.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	down := h.InputMint == key(101)
	fi, fo := f0, f1
	if !down {
		fi, fo = f1, f0
	}
	fee, e := fi.Calculate(amount, false)
	if e != nil {
		return fail(e)
	}
	netInput := amount - fee
	if netInput == 0 {
		return fail(errors.New("Whirlpool input is zero after transfer fee"))
	}
	pool := calc.ClmmPool{SqrtPrice: clmmWide(d, 65), Liquidity: clmmWide(d, 49), TickCurrent: int32(binary.LittleEndian.Uint32(d[81:])), TickSpacing: spacing, FeeRate: uint32(binary.LittleEndian.Uint16(d[45:]))}
	var adaptive *calc.WhirlpoolAdaptiveFee
	if binary.LittleEndian.Uint16(d[43:]) != spacing {
		oracle, _, e := solana.FindProgramAddress([][]byte{[]byte("oracle"), h.Pool[:]}, WhirlpoolProgram)
		if e != nil {
			return fail(e)
		}
		oa, e := s.Get(oracle, ctx, &WhirlpoolProgram)
		if e != nil {
			return fail(e)
		}
		od := oa.Data
		if len(od) < 110 || !bytes.Equal(od[:8], []byte{139, 194, 131, 179, 140, 179, 229, 244}) || !bytes.Equal(od[8:40], h.Pool[:]) {
			return fail(errors.New("invalid Whirlpool oracle"))
		}
		if unixTimestamp < binary.LittleEndian.Uint64(od[40:]) {
			return fail(errors.New("Whirlpool trade is not enabled"))
		}
		u16 := func(o int) uint16 { return binary.LittleEndian.Uint16(od[o:]) }
		u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(od[o:]) }
		u64 := func(o int) uint64 { return binary.LittleEndian.Uint64(od[o:]) }
		adaptive = &calc.WhirlpoolAdaptiveFee{FilterPeriod: u16(48), DecayPeriod: u16(50), ReductionFactor: u16(52), ControlFactor: u32(54), MaximumVolatility: u32(58), TickGroupSize: u16(62), LastReferenceTimestamp: u64(82), LastMajorSwapTimestamp: u64(90), VolatilityReference: u32(98), ReferenceGroup: int32(u32(102)), Volatility: u32(106)}
	}
	step := int32(spacing) * 88
	shifted := pool.TickCurrent
	if !down {
		shifted += int32(spacing)
	}
	q := shifted / step
	if shifted%step < 0 {
		q--
	}
	start := q * step
	arrays := []solana.PublicKey{}
	starts := []int32{}
	ticks := []calc.ClmmTick{}
	budget := maximumArrays
	if budget > 6 {
		budget = 6
	}
	for i := 0; i < budget; i++ {
		st := start + int32(i)*step
		if down {
			st = start - int32(i)*step
		}
		if st > calc.ClmmMaxTick || st+step <= calc.ClmmMinTick {
			break
		}
		address, _, e := solana.FindProgramAddress([][]byte{[]byte("tick_array"), h.Pool[:], []byte(strconv.FormatInt(int64(st), 10))}, WhirlpoolProgram)
		if e != nil {
			return fail(e)
		}
		td, e := s.Get(address, ctx, &WhirlpoolProgram)
		if e != nil {
			return fail(e)
		}
		decoded, e := DecodeWhirlpoolTicks(td.Data, h.Pool, st, spacing)
		if e != nil {
			return fail(e)
		}
		ticks = append(ticks, decoded...)
		starts = append(starts, st)
		arrays = append(arrays, address)
		result, e := calc.WhirlpoolSwapExactIn(pool, ticks, starts, netInput, unixTimestamp, down, adaptive, nil)
		if e != nil {
			if strings.Contains(e.Error(), "requires more tick arrays") {
				continue
			}
			return fail(e)
		}
		if result.Consumed != netInput {
			continue
		}
		fee, e := fo.Calculate(result.AmountOut, false)
		if e != nil {
			return fail(e)
		}
		net := result.AmountOut - fee
		minimum := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).SetUint64(net), new(big.Int).SetUint64(uint64(10000-slippageBps))), big.NewInt(10000)).Uint64()
		if minimum == 0 {
			return fail(errors.New("Whirlpool quote has zero protected output"))
		}
		quote := CachedWhirlpoolQuote{amount, net, minimum, minimum, ctx.Slot, ctx.Epoch, result.MinimumFeeRate, result.MaximumFeeRate}
		for len(arrays) < 3 {
			arrays = append(arrays, arrays[len(arrays)-1])
		}
		ma, mb := key(101), key(181)
		ownerA, _, e := solana.FindProgramAddress([][]byte{payer[:], m0.Owner[:], ma[:]}, solana.SPLAssociatedTokenAccountProgramID)
		if e != nil {
			return fail(e)
		}
		ownerB, _, e := solana.FindProgramAddress([][]byte{payer[:], m1.Owner[:], mb[:]}, solana.SPLAssociatedTokenAccountProgramID)
		if e != nil {
			return fail(e)
		}
		a := instruction.WhirlpoolSwapV2Accounts{TokenProgramA: m0.Owner, TokenProgramB: m1.Owner, TokenAuthority: payer, Whirlpool: h.Pool, MintA: ma, MintB: mb, OwnerA: ownerA, VaultA: key(133), OwnerB: ownerB, VaultB: key(213), TickArrays: arrays}
		ix, e := instruction.BuildWhirlpoolSwapV2(a, instruction.SwapV2Args{Amount: amount, OtherAmountThreshold: minimum, AmountSpecifiedIsInput: true}, down)
		if e != nil {
			return fail(e)
		}
		return a, quote, ix, nil
	}
	return fail(errors.New("Whirlpool quote exceeds loaded array budget"))
}
