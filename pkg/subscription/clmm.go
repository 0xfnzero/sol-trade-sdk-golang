package subscription

// Cache-only native CLMM preparation. No discovery, RPC, signing or sending.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

var ClmmProgram = solana.MustPublicKeyFromBase58("CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK")

type CachedClmmQuote struct {
	AmountIn, EstimatedNetAmountOut, MinimumNetAmountOut, MinimumAmountOut, StateSlot, Epoch uint64
	SqrtPriceLimit                                                                           *big.Int
}

func clmmWide(d []byte, o int) *big.Int {
	b := append([]byte(nil), d[o:o+16]...)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return new(big.Int).SetBytes(b)
}
func (s *AccountCacheSnapshot) PrepareClmm(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int) (instruction.RaydiumClmmSwapV2Accounts, CachedClmmQuote, solana.Instruction, error) {
	fail := func(e error) (instruction.RaydiumClmmSwapV2Accounts, CachedClmmQuote, solana.Instruction, error) {
		return instruction.RaydiumClmmSwapV2Accounts{}, CachedClmmQuote{}, nil, e
	}
	if amount == 0 || slippageBps >= 10000 || maximumArrays < 1 || maximumArrays > 32 {
		return fail(errors.New("invalid cached CLMM request"))
	}
	p, e := s.Get(h.Pool, ctx, &ClmmProgram)
	if e != nil {
		return fail(e)
	}
	d := p.Data
	if len(d) < 1544 || !bytes.Equal(d[:8], []byte{247, 237, 227, 245, 215, 195, 222, 70}) || d[389]&16 != 0 {
		return fail(errors.New("invalid or disabled CLMM pool"))
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	if e = h.matches(key(73), key(105)); e != nil {
		return fail(e)
	}
	if unixTimestamp <= binary.LittleEndian.Uint64(d[1080:]) {
		return fail(errors.New("CLMM pool is not open"))
	}
	config, e := s.Get(key(9), ctx, &ClmmProgram)
	if e != nil {
		return fail(e)
	}
	c := config.Data
	spacing := binary.LittleEndian.Uint16(d[235:])
	if len(c) < 55 || !bytes.Equal(c[:8], []byte{218, 244, 33, 104, 203, 203, 43, 111}) || spacing == 0 || binary.LittleEndian.Uint16(c[51:]) != spacing {
		return fail(errors.New("invalid CLMM config"))
	}
	mint0, e := s.Get(key(73), ctx, nil)
	if e != nil {
		return fail(e)
	}
	mint1, e := s.Get(key(105), ctx, nil)
	if e != nil {
		return fail(e)
	}
	fee0, e := instruction.TokenTransferFeeForEpoch(mint0.Data, mint0.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	fee1, e := instruction.TokenTransferFeeForEpoch(mint1.Data, mint1.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	down := h.InputMint == key(73)
	fi, fo, ip, op := fee0, fee1, mint0.Owner, mint1.Owner
	if !down {
		fi, fo, ip, op = fee1, fee0, mint1.Owner, mint0.Owner
	}
	fee, e := fi.Calculate(amount, false)
	if e != nil {
		return fail(e)
	}
	netInput := amount - fee
	if netInput == 0 {
		return fail(errors.New("CLMM input is zero after transfer fee"))
	}
	pool := calc.ClmmPool{SqrtPrice: clmmWide(d, 253), Liquidity: clmmWide(d, 237), TickCurrent: int32(binary.LittleEndian.Uint32(d[269:])), TickSpacing: spacing, FeeRate: binary.LittleEndian.Uint32(c[47:])}
	step := int32(spacing) * 60
	q := pool.TickCurrent / step
	if pool.TickCurrent%step < 0 {
		q--
	}
	start := q * step
	ticks := []calc.ClmmTick{}
	arrays := []solana.PublicKey{}
	var bitmap *solana.PublicKey
	var extension []byte
	bit := func(b []byte, o int, i int32) bool { return b[o+int(i/8)]&(1<<uint(i%8)) != 0 }
	for offset := int32(0); offset <= (calc.ClmmMaxTick-calc.ClmmMinTick)/step+1; offset++ {
		next := start + offset*step
		if down {
			next = start - offset*step
		}
		if next > calc.ClmmMaxTick || next+step <= calc.ClmmMinTick {
			break
		}
		index := next / step
		initialized := false
		if index >= -512 && index < 512 {
			initialized = bit(d, 904, index+512)
		} else {
			if extension == nil {
				k, _, e := solana.FindProgramAddress([][]byte{[]byte("pool_tick_array_bitmap_extension"), h.Pool[:]}, ClmmProgram)
				if e != nil {
					return fail(e)
				}
				bitmap = &k
				ea, e := s.Get(k, ctx, &ClmmProgram)
				if e != nil {
					return fail(e)
				}
				extension = ea.Data
				if len(extension) < 1832 || !bytes.Equal(extension[:8], []byte{60, 150, 36, 219, 97, 128, 139, 153}) || !bytes.Equal(extension[8:40], h.Pool[:]) {
					return fail(errors.New("invalid CLMM bitmap extension"))
				}
			}
			ed := extension
			pos := index - 512
			o := 40
			if index < 512 {
				distance := -index - 513
				pos = distance/512*512 + 511 - distance%512
				o = 936
			}
			if pos < 0 || pos >= 7168 {
				return fail(errors.New("CLMM bitmap index outside range"))
			}
			initialized = bit(ed, o, pos)
		}
		if !initialized {
			continue
		}
		seed := make([]byte, 4)
		binary.BigEndian.PutUint32(seed, uint32(next))
		address, _, e := solana.FindProgramAddress([][]byte{[]byte("tick_array"), h.Pool[:], seed}, ClmmProgram)
		if e != nil {
			return fail(e)
		}
		ta, e := s.Get(address, ctx, &ClmmProgram)
		if e != nil {
			return fail(e)
		}
		td := ta.Data
		if len(td) < 10240 || !bytes.Equal(td[:8], []byte{192, 155, 85, 205, 49, 249, 129, 42}) || !bytes.Equal(td[8:40], h.Pool[:]) || int32(binary.LittleEndian.Uint32(td[40:])) != next {
			return fail(errors.New("invalid CLMM tick array"))
		}
		for i := 0; i < 60; i++ {
			o := 44 + i*168
			gross := clmmWide(td, o+20)
			orders, partial := binary.LittleEndian.Uint64(td[o+124:]), binary.LittleEndian.Uint64(td[o+132:])
			if gross.Sign() != 0 || orders != 0 || partial != 0 {
				tick := int32(binary.LittleEndian.Uint32(td[o:]))
				if tick != next+int32(i)*int32(spacing) {
					return fail(errors.New("CLMM tick index mismatch"))
				}
				net := clmmWide(td, o+4)
				if net.Bit(127) != 0 {
					net.Sub(net, new(big.Int).Lsh(big.NewInt(1), 128))
				}
				ticks = append(ticks, calc.ClmmTick{Tick: tick, LiquidityNet: net, LiquidityGross: gross, Orders: orders, PartialOrders: partial})
			}
		}
		arrays = append(arrays, address)
		if len(arrays) > maximumArrays {
			return fail(errors.New("CLMM quote exceeds array budget"))
		}
		boundary := next + step - 1
		if boundary > calc.ClmmMaxTick {
			boundary = calc.ClmmMaxTick
		}
		if down {
			boundary = next
			if boundary < calc.ClmmMinTick {
				boundary = calc.ClmmMinTick
			}
		}
		limit, e := calc.ClmmSqrtPriceAtTick(boundary)
		if e != nil {
			return fail(e)
		}
		if (down && limit.Cmp(pool.SqrtPrice) >= 0) || (!down && limit.Cmp(pool.SqrtPrice) <= 0) {
			continue
		}
		result, e := calc.ClmmSwapExactIn(pool, ticks, netInput, limit, d[390], d[1096:1176], unixTimestamp, down)
		if e != nil {
			return fail(e)
		}
		if result.Consumed == netInput {
			fee, e := fo.Calculate(result.AmountOut, false)
			if e != nil {
				return fail(e)
			}
			net := result.AmountOut - fee
			minimum := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).SetUint64(net), new(big.Int).SetUint64(uint64(10000-slippageBps))), big.NewInt(10000)).Uint64()
			if minimum == 0 {
				return fail(errors.New("CLMM quote has zero protected output"))
			}
			ia, _, e := solana.FindProgramAddress([][]byte{payer[:], ip[:], h.InputMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
			if e != nil {
				return fail(e)
			}
			oa, _, e := solana.FindProgramAddress([][]byte{payer[:], op[:], h.OutputMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
			if e != nil {
				return fail(e)
			}
			iv, ov := key(137), key(169)
			if !down {
				iv, ov = ov, iv
			}
			a := instruction.RaydiumClmmSwapV2Accounts{Payer: payer, AmmConfig: key(9), PoolState: h.Pool, InputTokenAccount: ia, OutputTokenAccount: oa, InputVault: iv, OutputVault: ov, ObservationState: key(201), TokenProgram: solana.TokenProgramID, TokenProgram2022: solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"), InputVaultMint: h.InputMint, OutputVaultMint: h.OutputMint, TickArrays: arrays, TickArrayBitmapExtension: bitmap}
			quote := CachedClmmQuote{AmountIn: amount, EstimatedNetAmountOut: net, MinimumNetAmountOut: minimum, MinimumAmountOut: minimum, StateSlot: ctx.Slot, Epoch: ctx.Epoch, SqrtPriceLimit: limit}
			ix, e := instruction.BuildRaydiumClmmSwapV2(a, instruction.SwapV2Args{Amount: amount, OtherAmountThreshold: minimum, SqrtPriceLimit: limit, AmountSpecifiedIsInput: true})
			if e != nil {
				return fail(e)
			}
			return a, quote, ix, nil
		}
		if len(arrays) >= maximumArrays {
			return fail(errors.New("CLMM quote exceeds array budget"))
		}
	}
	return fail(errors.New("insufficient CLMM liquidity in supplied snapshot"))
}
