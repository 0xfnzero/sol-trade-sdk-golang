package subscription

// Current-state AMM v4 V2 exact-in preparation; no RPC or orderbook dependencies.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

var AmmV4Program = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
var AmmV4Authority = solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1")

type CachedAmmV4State struct {
	Pool, CoinMint, PcMint, CoinVault, PcVault                   solana.PublicKey
	CoinReserve, PcReserve, SwapFeeNumerator, SwapFeeDenominator uint64
}
type AmmV4Quote struct{ AmountIn, AmountOut, MinimumAmountOut, SwapFee uint64 }

func (s *AccountCacheSnapshot) AmmV4(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64) (CachedAmmV4State, error) {
	fail := func(err error) (CachedAmmV4State, error) { return CachedAmmV4State{}, err }
	a, err := s.Get(h.Pool, ctx, &AmmV4Program)
	if err != nil {
		return fail(err)
	}
	d := a.Data
	if len(d) != 752 {
		return fail(errors.New("invalid AMM v4 pool length"))
	}
	num := func(o int) uint64 { return binary.LittleEndian.Uint64(d[o : o+8]) }
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	status := num(0)
	if (status != 1 && status != 6 && status != 7) || (status == 7 && unixTimestamp < num(224)) {
		return fail(errors.New("AMM v4 swap is disabled or not open"))
	}
	if num(8) > 255 {
		return fail(errors.New("AMM v4 authority nonce mismatch"))
	}
	authority, err := solana.CreateProgramAddress([][]byte{[]byte("amm authority"), {byte(num(8))}}, AmmV4Program)
	if err != nil || authority != AmmV4Authority {
		return fail(errors.New("AMM v4 authority nonce mismatch"))
	}
	if err = h.matches(key(400), key(432)); err != nil {
		return fail(err)
	}
	if key(336) == key(368) {
		return fail(errors.New("AMM v4 vaults collide"))
	}
	numerator, denominator := num(176), num(184)
	if denominator == 0 || numerator >= denominator {
		return fail(errors.New("invalid AMM v4 swap fee"))
	}
	reserves := []uint64{}
	for _, offsets := range [][4]int{{336, 400, 192, 32}, {368, 432, 200, 40}} {
		vault, mint, pnl, decs := offsets[0], offsets[1], offsets[2], offsets[3]
		m, e := s.Get(key(mint), ctx, &solana.TokenProgramID)
		if e != nil {
			return fail(e)
		}
		if _, e = instruction.TokenTransferFeeForEpoch(m.Data, m.Owner, ctx.Epoch); e != nil {
			return fail(e)
		}
		if uint64(m.Data[44]) != num(decs) {
			return fail(errors.New("AMM v4 mint decimals mismatch"))
		}
		v, e := s.Get(key(vault), ctx, &solana.TokenProgramID)
		if e != nil {
			return fail(e)
		}
		mintKey := key(mint)
		if len(v.Data) != 165 || !bytes.Equal(v.Data[:32], mintKey[:]) || !bytes.Equal(v.Data[32:64], AmmV4Authority[:]) || v.Data[108] != 1 {
			return fail(errors.New("invalid AMM v4 vault identity or state"))
		}
		amount := binary.LittleEndian.Uint64(v.Data[64:72])
		if num(pnl) >= amount {
			return fail(errors.New("AMM v4 PnL exhausts vault balance"))
		}
		reserves = append(reserves, amount-num(pnl))
	}
	return CachedAmmV4State{h.Pool, key(400), key(432), key(336), key(368), reserves[0], reserves[1], numerator, denominator}, nil
}
func QuoteCachedAmmV4ExactIn(p CachedAmmV4State, amount uint64, coinIn bool, slippageBps uint16) (AmmV4Quote, error) {
	if amount == 0 || slippageBps >= 10000 || p.CoinReserve == 0 || p.PcReserve == 0 || p.SwapFeeDenominator == 0 || p.SwapFeeNumerator >= p.SwapFeeDenominator {
		return AmmV4Quote{}, errors.New("invalid AMM v4 quote request")
	}
	i, o := p.CoinReserve, p.PcReserve
	if !coinIn {
		i, o = o, i
	}
	n := new(big.Int).Mul(cpN(amount), cpN(p.SwapFeeNumerator))
	fee := new(big.Int).Quo(new(big.Int).Add(n, cpN(p.SwapFeeDenominator-1)), cpN(p.SwapFeeDenominator)).Uint64()
	net := amount - fee
	out := new(big.Int).Quo(new(big.Int).Mul(cpN(o), cpN(net)), new(big.Int).Add(cpN(i), cpN(net))).Uint64()
	minimum := new(big.Int).Quo(new(big.Int).Mul(cpN(out), cpN(uint64(10000-slippageBps))), cpN(10000)).Uint64()
	return AmmV4Quote{amount, out, minimum, fee}, nil
}

type PreparedAmmV4 struct {
	State       CachedAmmV4State
	Quote       AmmV4Quote
	Instruction solana.Instruction
}

func (s *AccountCacheSnapshot) PrepareAmmV4(h PoolTradeHint, ctx CacheReadContext, unixTimestamp uint64, payer solana.PublicKey, amount uint64, slippageBps uint16) (PreparedAmmV4, error) {
	p, err := s.AmmV4(h, ctx, unixTimestamp)
	if err != nil {
		return PreparedAmmV4{}, err
	}
	q, err := QuoteCachedAmmV4ExactIn(p, amount, h.InputMint == p.CoinMint, slippageBps)
	if err != nil {
		return PreparedAmmV4{}, err
	}
	if q.MinimumAmountOut == 0 {
		return PreparedAmmV4{}, errors.New("AMM v4 quote has zero protected output")
	}
	ata := func(mint solana.PublicKey) (solana.PublicKey, error) {
		k, _, e := solana.FindProgramAddress([][]byte{payer[:], solana.TokenProgramID[:], mint[:]}, solana.SPLAssociatedTokenAccountProgramID)
		return k, e
	}
	ia, err := ata(h.InputMint)
	if err != nil {
		return PreparedAmmV4{}, err
	}
	oa, err := ata(h.OutputMint)
	if err != nil {
		return PreparedAmmV4{}, err
	}
	keys := []solana.PublicKey{solana.TokenProgramID, p.Pool, AmmV4Authority, p.CoinVault, p.PcVault, ia, oa, payer}
	metas := solana.AccountMetaSlice{}
	for i, k := range keys {
		metas = append(metas, &solana.AccountMeta{PublicKey: k, IsSigner: i == 7, IsWritable: i == 1 || i >= 3 && i <= 6})
	}
	data := make([]byte, 17)
	data[0] = 16
	binary.LittleEndian.PutUint64(data[1:], q.AmountIn)
	binary.LittleEndian.PutUint64(data[9:], q.MinimumAmountOut)
	return PreparedAmmV4{p, q, solana.NewInstruction(AmmV4Program, metas, data)}, nil
}
