package subscription

// Cache-only native DLMM preparation, including bitmap-certified empty intervals.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math"
)

var DlmmProgram = solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")

type CachedDlmmQuote struct {
	AmountIn, EstimatedNetAmountOut, MinimumNetAmountOut, MinimumAmountOut, StateSlot, Epoch uint64
	BinsCrossed                                                                              uint32
}

func DecodeDlmmBins(d []byte, pool solana.PublicKey, index int64) ([]calc.DlmmBin, error) {
	if index < -6338 || index > 6337 || len(d) < 10136 || !bytes.Equal(d[:8], []byte{92, 142, 92, 220, 5, 148, 70, 181}) || int64(binary.LittleEndian.Uint64(d[8:])) != index || !bytes.Equal(d[24:56], pool[:]) {
		return nil, errors.New("invalid DLMM bin array identity")
	}
	bins := []calc.DlmmBin{}
	for i := 0; i < 70; i++ {
		o := 56 + i*144
		ax, ay := binary.LittleEndian.Uint64(d[o:]), binary.LittleEndian.Uint64(d[o+8:])
		price := clmmWide(d, o+16)
		oo, po := binary.LittleEndian.Uint64(d[o+112:]), binary.LittleEndian.Uint64(d[o+128:])
		if price.Sign() == 0 {
			if ax != 0 || ay != 0 || oo != 0 || po != 0 {
				return nil, errors.New("zero DLMM bin price with liquidity")
			}
			continue
		}
		bid := index*70 + int64(i)
		if index < -6338 || index > 6337 || bid < -443636 || bid > 443636 {
			return nil, errors.New("DLMM bin outside range")
		}
		bins = append(bins, calc.DlmmBin{BinID: int32(bid), AmountX: ax, AmountY: ay, Price: price, OpenOrder: oo, ProcessedOrder: po, AskSide: d[o+140]})
	}
	return bins, nil
}
func (s *AccountCacheSnapshot) PrepareDlmm(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16, maximumArrays int) (instruction.MeteoraDlmmSwap2Accounts, CachedDlmmQuote, solana.Instruction, error) {
	fail := func(e error) (instruction.MeteoraDlmmSwap2Accounts, CachedDlmmQuote, solana.Instruction, error) {
		return instruction.MeteoraDlmmSwap2Accounts{}, CachedDlmmQuote{}, nil, e
	}
	if amount == 0 || unixTimestamp > math.MaxInt64 || slippageBps >= 10000 || maximumArrays < 1 || maximumArrays > 32 {
		return fail(errors.New("invalid cached DLMM request"))
	}
	p, e := s.Get(h.Pool, ctx, &DlmmProgram)
	if e != nil {
		return fail(e)
	}
	d := p.Data
	if len(d) < 904 || !bytes.Equal(d[:8], []byte{33, 11, 49, 98, 181, 101, 177, 13}) || d[82] != 0 {
		return fail(errors.New("disabled or invalid DLMM pool"))
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	u16 := func(o int) uint16 { return binary.LittleEndian.Uint16(d[o:]) }
	u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(d[o:]) }
	u64 := func(o int) uint64 { return binary.LittleEndian.Uint64(d[o:]) }
	if e = h.matches(key(88), key(120)); e != nil {
		return fail(e)
	}
	clock := unixTimestamp
	if d[86] == 0 {
		clock = ctx.Slot
	}
	if d[86] > 1 || clock < u64(816) {
		return fail(errors.New("DLMM not activated"))
	}
	if d[35] > 2 {
		return fail(errors.New("unknown DLMM function type"))
	}
	orders := d[35] == 2 || (d[35] == 0 && key(264) == (solana.PublicKey{}) && key(408) == (solana.PublicKey{}))
	state := calc.DlmmPool{ActiveID: int32(u32(76)), BinStep: u16(80), FeeMode: d[36], Static: calc.DlmmStaticFee{BaseFactor: u16(8), Power: d[34], Control: u32(16), MaximumVolatility: u32(20), FilterPeriod: u16(10), DecayPeriod: u16(12), ReductionFactor: u16(14)}, Variable: calc.DlmmVariableFee{Volatility: u32(40), Reference: u32(44), IndexReference: int32(u32(48)), LastTimestamp: int64(u64(56))}}
	mx, e := s.Get(key(88), ctx, nil)
	if e != nil {
		return fail(e)
	}
	my, e := s.Get(key(120), ctx, nil)
	if e != nil {
		return fail(e)
	}
	fx, e := instruction.TokenTransferFeeForEpoch(mx.Data, mx.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	fy, e := instruction.TokenTransferFeeForEpoch(my.Data, my.Owner, ctx.Epoch)
	if e != nil {
		return fail(e)
	}
	down := h.InputMint == key(88)
	fi, fo := fx, fy
	pi, po := mx.Owner, my.Owner
	if !down {
		fi, fo = fy, fx
		pi, po = my.Owner, mx.Owner
	}
	fee, e := fi.Calculate(amount, false)
	if e != nil {
		return fail(e)
	}
	netInput := amount - fee
	if netInput == 0 {
		return fail(errors.New("zero DLMM input after transfer fee"))
	}
	arrays := []solana.PublicKey{}
	loaded := []int64{}
	bins := []calc.DlmmBin{}
	var bitmap *solana.PublicKey
	var extension []byte
	base := int64(state.ActiveID) / 70
	if state.ActiveID < 0 && state.ActiveID%70 != 0 {
		base--
	}
	for offset := int64(0); offset < 12676; offset++ {
		index := base + offset
		if down {
			index = base - offset
		}
		if index*70 > 443636 || (index+1)*70 <= -443636 {
			break
		}
		initialized := false
		if index >= -512 && index < 512 {
			pos := index + 512
			initialized = d[584+pos/8]&(1<<uint(pos%8)) != 0
		} else {
			if extension == nil {
				k, _, err := solana.FindProgramAddress([][]byte{[]byte("bitmap"), h.Pool[:]}, DlmmProgram)
				if err != nil {
					return fail(err)
				}
				bitmap = &k
				a, err := s.Get(k, ctx, &DlmmProgram)
				if err != nil {
					return fail(err)
				}
				extension = a.Data
				if len(extension) < 1576 || !bytes.Equal(extension[:8], []byte{80, 111, 124, 113, 55, 237, 18, 5}) || !bytes.Equal(extension[8:40], h.Pool[:]) {
					return fail(errors.New("DLMM bitmap identity mismatch"))
				}
			}
			pos, off := index-512, int64(40)
			if index < 0 {
				pos = -index - 513
				off = 808
			}
			if pos < 0 || pos >= 6144 {
				return fail(errors.New("DLMM bitmap index outside range"))
			}
			initialized = extension[off+pos/8]&(1<<uint(pos%8)) != 0
		}
		loaded = append(loaded, index)
		if initialized {
			seed := make([]byte, 8)
			binary.LittleEndian.PutUint64(seed, uint64(index))
			address, _, err := solana.FindProgramAddress([][]byte{[]byte("bin_array"), h.Pool[:], seed}, DlmmProgram)
			if err != nil {
				return fail(err)
			}
			a, err := s.Get(address, ctx, &DlmmProgram)
			if err != nil {
				return fail(err)
			}
			bs, err := DecodeDlmmBins(a.Data, h.Pool, index)
			if err != nil {
				return fail(err)
			}
			bins = append(bins, bs...)
			arrays = append(arrays, address)
		}
		if !initialized {
			continue
		}
		r, err := calc.DlmmSwapExactIn(state, bins, loaded, netInput, int64(unixTimestamp), down, orders, true, false)
		var partial *calc.InsufficientDlmmArrays
		if err != nil && !errors.As(err, &partial) {
			return fail(err)
		}
		if err == nil && r.Complete && r.RemainingIn == 0 && len(arrays) > 0 {
			fee, err := fo.Calculate(r.AmountOut, false)
			if err != nil {
				return fail(err)
			}
			net := r.AmountOut - fee
			minimum := net/10000*uint64(10000-slippageBps) + net%10000*uint64(10000-slippageBps)/10000
			if minimum == 0 {
				return fail(errors.New("zero protected DLMM output"))
			}
			in, _, err := solana.FindProgramAddress([][]byte{payer[:], pi[:], h.InputMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
			if err != nil {
				return fail(err)
			}
			out, _, err := solana.FindProgramAddress([][]byte{payer[:], po[:], h.OutputMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
			if err != nil {
				return fail(err)
			}
			a := instruction.MeteoraDlmmSwap2Accounts{LbPair: h.Pool, ReserveX: key(152), ReserveY: key(184), UserTokenIn: in, UserTokenOut: out, TokenXMint: key(88), TokenYMint: key(120), Oracle: key(552), User: payer, TokenXProgram: mx.Owner, TokenYProgram: my.Owner, BitmapExtension: bitmap, BinArrays: arrays}
			q := CachedDlmmQuote{amount, net, minimum, minimum, ctx.Slot, ctx.Epoch, r.BinsCrossed}
			ix, err := instruction.BuildMeteoraDlmmSwap2(a, amount, minimum)
			return a, q, ix, err
		}
		if len(arrays) >= maximumArrays {
			return fail(errors.New("DLMM quote exceeds array budget"))
		}
	}
	return fail(errors.New("insufficient DLMM liquidity in supplied snapshot"))
}
