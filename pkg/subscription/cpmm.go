package subscription

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

var CpmmProgram = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")

type CachedCpmmState struct {
	Pool, Config, BaseMint, QuoteMint, BaseVault, QuoteVault, BaseTokenProgram, QuoteTokenProgram, Observation solana.PublicKey
	BaseReserve, QuoteReserve, TradeFeeRate, ProtocolFeeRate, FundFeeRate, CreatorFeeRate                      uint64
	CreatorFeeOn                                                                                               uint8
	EnableCreatorFee                                                                                           bool
	BaseTransferFee, QuoteTransferFee                                                                          calc.TokenTransferFee
	OpenTime                                                                                                   uint64
}
type CpmmQuote struct{ AmountIn, AmountOut, MinimumAmountOut, TradeFee, CreatorFee uint64 }

func (p CachedCpmmState) validFees() bool {
	c := uint64(0)
	if p.EnableCreatorFee {
		c = p.CreatorFeeRate
	}
	return p.TradeFeeRate < 1000000 && c < 1000000 && p.TradeFeeRate+c < 1000000 && p.ProtocolFeeRate <= 1000000 && p.FundFeeRate <= 1000000 && p.ProtocolFeeRate+p.FundFeeRate <= 1000000 && p.CreatorFeeOn <= 2
}
func (s *AccountCacheSnapshot) Cpmm(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64) (CachedCpmmState, error) {
	fail := func(e error) (CachedCpmmState, error) { return CachedCpmmState{}, e }
	pool, e := s.Get(h.Pool, ctx, &CpmmProgram)
	if e != nil {
		return fail(e)
	}
	d := pool.Data
	if len(d) < 637 || !bytes.Equal(d[:8], []byte{247, 237, 227, 245, 215, 195, 222, 70}) || d[329]&4 != 0 || d[390] > 1 {
		return fail(errors.New("invalid or disabled cached CPMM pool"))
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	num := func(b []byte, o int) uint64 { return binary.LittleEndian.Uint64(b[o : o+8]) }
	if e = h.matches(key(168), key(200)); e != nil {
		return fail(e)
	}
	opened := num(d, 373)
	if unixTimestamp < opened {
		return fail(errors.New("CPMM pool is not open"))
	}
	config, e := s.Get(key(8), ctx, &CpmmProgram)
	if e != nil {
		return fail(e)
	}
	f := config.Data
	if len(f) < 236 || !bytes.Equal(f[:8], []byte{218, 244, 33, 104, 203, 203, 43, 111}) {
		return fail(errors.New("invalid cached CPMM config"))
	}
	p := CachedCpmmState{Pool: h.Pool, Config: key(8), BaseMint: key(168), QuoteMint: key(200), BaseVault: key(72), QuoteVault: key(104), BaseTokenProgram: key(232), QuoteTokenProgram: key(264), Observation: key(296), TradeFeeRate: num(f, 12), ProtocolFeeRate: num(f, 20), FundFeeRate: num(f, 28), CreatorFeeRate: num(f, 108), CreatorFeeOn: d[389], EnableCreatorFee: d[390] == 1, OpenTime: opened}
	if !p.validFees() {
		return fail(errors.New("invalid CPMM fee configuration"))
	}
	reserve := func(vault, mint, program solana.PublicKey, offsets []int) (uint64, error) {
		a, e := s.Get(vault, ctx, &program)
		if e != nil {
			return 0, e
		}
		v := a.Data
		if len(v) < 165 || !bytes.Equal(v[:32], mint[:]) || !bytes.Equal(v[32:64], instruction.RAYDIUM_CPMM_AUTHORITY[:]) || v[108] != 1 {
			return 0, errors.New("invalid cached CPMM vault")
		}
		amount := num(v, 64)
		for _, o := range offsets {
			fee := num(d, o)
			if fee > amount {
				return 0, errors.New("CPMM fees exceed cached vault balance")
			}
			amount -= fee
		}
		return amount, nil
	}
	if p.BaseReserve, e = reserve(p.BaseVault, p.BaseMint, p.BaseTokenProgram, []int{341, 357, 397}); e != nil {
		return fail(e)
	}
	if p.QuoteReserve, e = reserve(p.QuoteVault, p.QuoteMint, p.QuoteTokenProgram, []int{349, 365, 405}); e != nil {
		return fail(e)
	}
	base, e := s.Get(p.BaseMint, ctx, &p.BaseTokenProgram)
	if e != nil {
		return fail(e)
	}
	quote, e := s.Get(p.QuoteMint, ctx, &p.QuoteTokenProgram)
	if e != nil {
		return fail(e)
	}
	if p.BaseTransferFee, e = instruction.TokenTransferFeeForEpoch(base.Data, base.Owner, ctx.Epoch); e != nil {
		return fail(e)
	}
	if p.QuoteTransferFee, e = instruction.TokenTransferFeeForEpoch(quote.Data, quote.Owner, ctx.Epoch); e != nil {
		return fail(e)
	}
	return p, nil
}
func cpN(n uint64) *big.Int { return new(big.Int).SetUint64(n) }
func cpCeil(n *big.Int, d uint64) uint64 {
	return new(big.Int).Quo(new(big.Int).Add(n, cpN(d-1)), cpN(d)).Uint64()
}
func QuoteCachedCpmmExactIn(p CachedCpmmState, amount uint64, baseIn bool, slippageBps uint16) (CpmmQuote, error) {
	fail := func(s string) (CpmmQuote, error) { return CpmmQuote{}, errors.New(s) }
	if amount == 0 {
		return fail("amount cannot be zero")
	}
	if slippageBps > 9999 {
		return fail("slippage must be 0..9999")
	}
	if !p.validFees() {
		return fail("invalid CPMM fee configuration")
	}
	i, o, inf, outf := p.BaseReserve, p.QuoteReserve, p.BaseTransferFee, p.QuoteTransferFee
	if !baseIn {
		i, o, inf, outf = p.QuoteReserve, p.BaseReserve, p.QuoteTransferFee, p.BaseTransferFee
	}
	if i == 0 || o == 0 {
		return fail("empty CPMM reserves")
	}
	transfer, e := inf.Calculate(amount, false)
	if e != nil {
		return CpmmQuote{}, e
	}
	net := amount - transfer
	onInput := p.CreatorFeeOn == 0 || p.CreatorFeeOn == 1 && baseIn || p.CreatorFeeOn == 2 && !baseIn
	creatorRate := uint64(0)
	if p.EnableCreatorFee {
		creatorRate = p.CreatorFeeRate
	}
	totalRate := p.TradeFeeRate
	if onInput {
		totalRate += creatorRate
	}
	fee := cpCeil(new(big.Int).Mul(cpN(net), cpN(totalRate)), 1000000)
	creator := uint64(0)
	if onInput && totalRate != 0 {
		creator = new(big.Int).Quo(new(big.Int).Mul(cpN(fee), cpN(creatorRate)), cpN(totalRate)).Uint64()
	}
	trade := fee - creator
	swapped := new(big.Int).Quo(new(big.Int).Mul(cpN(o), cpN(net-fee)), new(big.Int).Add(cpN(i), cpN(net-fee))).Uint64()
	gross := swapped
	if !onInput {
		creator = cpCeil(new(big.Int).Mul(cpN(swapped), cpN(creatorRate)), 1000000)
		gross -= creator
	}
	transfer, e = outf.Calculate(gross, false)
	if e != nil {
		return CpmmQuote{}, e
	}
	received := gross - transfer
	min := new(big.Int).Quo(new(big.Int).Mul(cpN(received), cpN(uint64(10000-slippageBps))), cpN(10000)).Uint64()
	return CpmmQuote{amount, received, min, trade, creator}, nil
}
func BuildCachedCpmmExactIn(p CachedCpmmState, payer solana.PublicKey, amount, minimumAmountOut uint64, baseIn bool) (solana.Instruction, error) {
	if amount == 0 {
		return nil, errors.New("amount cannot be zero")
	}
	im, om, iv, ov, ip, op := p.BaseMint, p.QuoteMint, p.BaseVault, p.QuoteVault, p.BaseTokenProgram, p.QuoteTokenProgram
	if !baseIn {
		im, om, iv, ov, ip, op = p.QuoteMint, p.BaseMint, p.QuoteVault, p.BaseVault, p.QuoteTokenProgram, p.BaseTokenProgram
	}
	ata := func(mint, program solana.PublicKey) (solana.PublicKey, error) {
		k, _, e := solana.FindProgramAddress([][]byte{payer[:], program[:], mint[:]}, solana.SPLAssociatedTokenAccountProgramID)
		return k, e
	}
	ia, e := ata(im, ip)
	if e != nil {
		return nil, e
	}
	oa, e := ata(om, op)
	if e != nil {
		return nil, e
	}
	keys := []solana.PublicKey{payer, instruction.RAYDIUM_CPMM_AUTHORITY, p.Config, p.Pool, ia, oa, iv, ov, ip, op, im, om, p.Observation}
	metas := solana.AccountMetaSlice{}
	for i, k := range keys {
		metas = append(metas, &solana.AccountMeta{PublicKey: k, IsSigner: i == 0, IsWritable: i == 0 || i >= 3 && i <= 7 || i == 12})
	}
	data := make([]byte, 24)
	copy(data, []byte{143, 190, 90, 218, 196, 30, 51, 222})
	binary.LittleEndian.PutUint64(data[8:], amount)
	binary.LittleEndian.PutUint64(data[16:], minimumAmountOut)
	return solana.NewInstruction(CpmmProgram, metas, data), nil
}

type PreparedCpmm struct {
	State       CachedCpmmState
	Quote       CpmmQuote
	Instruction solana.Instruction
}

func (s *AccountCacheSnapshot) PrepareCpmm(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16) (PreparedCpmm, error) {
	p, e := s.Cpmm(h, ctx, unixTimestamp)
	if e != nil {
		return PreparedCpmm{}, e
	}
	baseIn := h.InputMint == p.BaseMint
	q, e := QuoteCachedCpmmExactIn(p, amount, baseIn, slippageBps)
	if e != nil {
		return PreparedCpmm{}, e
	}
	if q.MinimumAmountOut == 0 {
		return PreparedCpmm{}, errors.New("CPMM quote has zero protected output")
	}
	ix, e := BuildCachedCpmmExactIn(p, payer, q.AmountIn, q.MinimumAmountOut, baseIn)
	if e != nil {
		return PreparedCpmm{}, e
	}
	return PreparedCpmm{p, q, ix}, nil
}
