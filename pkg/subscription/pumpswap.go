package subscription

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
)

// CachedPumpSwapState contains only validated snapshot data; no RPC or fee fallback.
type CachedPumpSwapState struct {
	PoolAddress                                                        solana.PublicKey
	Pool                                                               instruction.PumpSwapPool
	BaseReserve, QuoteReserve, BaseMintSupply                          uint64
	BaseTokenProgram, QuoteTokenProgram                                solana.PublicKey
	BaseTransferFee, QuoteTransferFee                                  calc.TokenTransferFee
	FeeBasisPoints                                                     calc.PumpSwapFeeBasisPoints
	DisableFlags                                                       uint8
	ProtocolFeeRecipients, ReservedFeeRecipients, BuybackFeeRecipients []solana.PublicKey
	MayhemEnabled, CashbackEnabled                                     bool
}

func (s *AccountCacheSnapshot) PumpSwap(h PoolTradeHint, c CacheReadContext) (CachedPumpSwapState, error) {
	fail := func(message string) (CachedPumpSwapState, error) { return CachedPumpSwapState{}, errors.New(message) }
	a, e := s.Get(h.Pool, c, &instruction.PUMPSWAP_PROGRAM)
	if e != nil {
		return CachedPumpSwapState{}, e
	}
	p := instruction.DecodePoolAccount(a.Data)
	if p == nil || a.Data[243] > 1 || a.Data[244] > 1 {
		return fail("invalid PumpSwap pool layout")
	}
	if e = h.matches(p.BaseMint, p.QuoteMint); e != nil {
		return CachedPumpSwapState{}, e
	}
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, p.Index)
	address, bump, e := solana.FindProgramAddress([][]byte{[]byte("pool"), index, p.Creator[:], p.BaseMint[:], p.QuoteMint[:]}, instruction.PUMPSWAP_PROGRAM)
	if e != nil || address != h.Pool || bump != p.PoolBump {
		return fail("PumpSwap pool PDA mismatch")
	}
	if p.PoolBaseTokenAccount == p.PoolQuoteTokenAccount {
		return fail("PumpSwap vaults collide")
	}
	g, e := s.Get(instruction.PUMPSWAP_GLOBAL_ACCOUNT, c, &instruction.PUMPSWAP_PROGRAM)
	if e != nil {
		return CachedPumpSwapState{}, e
	}
	d := g.Data
	if len(d) < 899 || !bytes.Equal(d[:8], []byte{149, 8, 156, 202, 160, 252, 176, 217}) || d[417] > 1 || d[642] > 1 {
		return fail("invalid PumpSwap global config")
	}
	a, e = s.Get(instruction.FEE_CONFIG, c, &instruction.FEE_PROGRAM)
	if e != nil {
		return CachedPumpSwapState{}, e
	}
	feeAddress, feeBump, e := solana.FindProgramAddress([][]byte{[]byte("fee_config"), instruction.PUMPSWAP_PROGRAM[:]}, instruction.FEE_PROGRAM)
	if e != nil || feeAddress != instruction.FEE_CONFIG || len(a.Data) < 9 || a.Data[8] != feeBump {
		return fail("PumpSwap fee config PDA bump mismatch")
	}
	config := instruction.DecodeFeeConfig(a.Data)
	if config == nil {
		return fail("invalid PumpSwap fee config")
	}
	for _, tiers := range [][]instruction.PumpSwapFeeTier{config.FeeTiers, config.StableFeeTiers} {
		for i := 1; i < len(tiers); i++ {
			if tiers[i-1].MarketCapLamportsThreshold.Cmp(tiers[i].MarketCapLamportsThreshold) >= 0 {
				return fail("PumpSwap fee tiers are not strictly ordered")
			}
		}
	}
	state := CachedPumpSwapState{PoolAddress: h.Pool, Pool: *p, DisableFlags: d[56], MayhemEnabled: d[417] == 1, CashbackEnabled: d[642] == 1}
	mints := [2]solana.PublicKey{p.BaseMint, p.QuoteMint}
	vaults := [2]solana.PublicKey{p.PoolBaseTokenAccount, p.PoolQuoteTokenAccount}
	for i, mint := range mints {
		m, e := s.Get(mint, c, nil)
		if e != nil {
			return CachedPumpSwapState{}, e
		}
		fee, e := instruction.TokenTransferFeeForEpoch(m.Data, m.Owner, c.Epoch)
		if e != nil {
			return CachedPumpSwapState{}, e
		}
		v, e := s.Get(vaults[i], c, &m.Owner)
		if e != nil {
			return CachedPumpSwapState{}, e
		}
		if len(v.Data) < 165 || !bytes.Equal(v.Data[:32], mint[:]) || !bytes.Equal(v.Data[32:64], h.Pool[:]) || v.Data[108] != 1 {
			return fail("invalid PumpSwap vault identity or state")
		}
		reserve := binary.LittleEndian.Uint64(v.Data[64:72])
		if reserve == 0 {
			return fail("PumpSwap reserves are empty")
		}
		if i == 0 {
			state.BaseReserve = reserve
			state.BaseMintSupply = binary.LittleEndian.Uint64(m.Data[36:44])
			state.BaseTokenProgram = m.Owner
			state.BaseTransferFee = fee
		} else {
			state.QuoteReserve = reserve
			state.QuoteTokenProgram = m.Owner
			state.QuoteTransferFee = fee
		}
	}
	effectiveQuoteReserve, e := calc.EffectiveQuoteReserves(state.QuoteReserve, p.VirtualQuoteReserves)
	if e != nil {
		return CachedPumpSwapState{}, e
	}
	state.FeeBasisPoints = instruction.ComputePumpSwapFeeBasisPoints(config, p.Creator, p.BaseMint, &state.BaseMintSupply, state.BaseReserve, effectiveQuoteReserve, p.QuoteMint)
	if len(g.Data) > 940 && g.Data[940] == 1 && p.CreatorFeeBps > 0 {
		state.FeeBasisPoints.CoinCreatorFeeBasisPoints = p.CreatorFeeBps
	}
	keys := func(start, n int) []solana.PublicKey {
		out := make([]solana.PublicKey, n)
		for i := range out {
			copy(out[i][:], d[start+i*32:start+(i+1)*32])
		}
		return out
	}
	state.ProtocolFeeRecipients = keys(57, 8)
	state.ReservedFeeRecipients = append(keys(385, 1), keys(418, 7)...)
	state.BuybackFeeRecipients = keys(643, 8)
	if e = s.AssertUsable(); e != nil {
		return CachedPumpSwapState{}, e
	}
	return state, nil
}

type PumpSwapQuote struct{ AmountIn, AmountOut, MinimumAmountOut uint64 }
type PreparedPumpSwap struct {
	State       CachedPumpSwapState
	Quote       PumpSwapQuote
	Instruction solana.Instruction
}

// PreparePumpSwap builds one independent exact-in leg. Unsupported fee contexts fail explicitly.
func (s *AccountCacheSnapshot) PreparePumpSwap(h PoolTradeHint, c CacheReadContext, payer solana.PublicKey, amount uint64, slippageBps uint16) (PreparedPumpSwap, error) {
	fail := func(m string) (PreparedPumpSwap, error) { return PreparedPumpSwap{}, errors.New(m) }
	if amount == 0 || slippageBps >= 10000 || payer.IsZero() {
		return fail("invalid PumpSwap preparation request")
	}
	state, e := s.PumpSwap(h, c)
	if e != nil {
		return PreparedPumpSwap{}, e
	}
	p := state.Pool
	quoteIn := h.InputMint == p.QuoteMint
	flag := uint8(16)
	if quoteIn {
		flag = 8
	}
	if state.DisableFlags&flag != 0 {
		return fail("PumpSwap direction is disabled")
	}
	if p.IsCashbackCoin {
		return fail("PumpSwap cashback quote requires a verified current fee context")
	}
	for _, f := range []calc.TokenTransferFee{state.BaseTransferFee, state.QuoteTransferFee} {
		if f.BasisPoints != 0 && f.MaximumFee != 0 {
			return fail("PumpSwap transfer-fee quote semantics are not yet verified")
		}
	}
	fees := state.FeeBasisPoints
	if p.CoinCreator.IsZero() {
		fees.CoinCreatorFeeBasisPoints = 0
	}
	var out uint64
	if quoteIn {
		q, e := calc.BuyQuoteInputInternalWithFees(amount, 0, state.BaseReserve, state.QuoteReserve, p.VirtualQuoteReserves, fees)
		if e != nil {
			return PreparedPumpSwap{}, e
		}
		out = q.Base
	} else {
		q, e := calc.SellBaseInputInternalWithFees(amount, 0, state.BaseReserve, state.QuoteReserve, p.VirtualQuoteReserves, fees)
		if e != nil {
			return PreparedPumpSwap{}, e
		}
		out = q.UIQuote
	}
	minimum, e := calc.CalculateWithSlippageSell(out, uint64(slippageBps))
	if e != nil {
		return PreparedPumpSwap{}, e
	}
	if minimum == 0 {
		return fail("PumpSwap quote has zero protected output")
	}
	choose := func(keys []solana.PublicKey) (solana.PublicKey, bool) {
		for _, k := range keys {
			if !k.IsZero() {
				return k, true
			}
		}
		return solana.PublicKey{}, false
	}
	recipients := state.ProtocolFeeRecipients
	if p.IsMayhemMode {
		recipients = state.ReservedFeeRecipients
	}
	recipient, ok := choose(recipients)
	if !ok {
		return fail("missing current PumpSwap protocol recipient")
	}
	buyback, ok := choose(state.BuybackFeeRecipients)
	if !ok {
		return fail("missing current PumpSwap buyback recipient")
	}
	w := func(k solana.PublicKey) *solana.AccountMeta { return solana.Meta(k).WRITE() }
	r := func(k solana.PublicKey) *solana.AccountMeta { return solana.Meta(k) }
	ata := instruction.GetAssociatedTokenAddress
	accounts := solana.AccountMetaSlice{w(h.Pool), w(payer).SIGNER(), r(instruction.PUMPSWAP_GLOBAL_ACCOUNT), r(p.BaseMint), r(p.QuoteMint), w(ata(payer, p.BaseMint, state.BaseTokenProgram)), w(ata(payer, p.QuoteMint, state.QuoteTokenProgram)), w(p.PoolBaseTokenAccount), w(p.PoolQuoteTokenAccount), r(recipient), w(ata(recipient, p.QuoteMint, state.QuoteTokenProgram)), r(state.BaseTokenProgram), r(state.QuoteTokenProgram), r(solana.SystemProgramID), r(solana.SPLAssociatedTokenAccountProgramID), r(instruction.PUMPSWAP_EVENT_AUTHORITY), r(instruction.PUMPSWAP_PROGRAM), w(instruction.GetCoinCreatorVaultAtaWithTokenProgram(p.CoinCreator, p.QuoteMint, state.QuoteTokenProgram)), r(instruction.GetCoinCreatorVaultAuthority(p.CoinCreator))}
	if quoteIn {
		accounts = append(accounts, w(instruction.GLOBAL_VOLUME_ACCUMULATOR), w(instruction.GetPumpSwapUserVolumeAccumulatorPDA(payer)))
	}
	accounts = append(accounts, r(instruction.FEE_CONFIG), r(instruction.FEE_PROGRAM))
	if !p.CoinCreator.IsZero() {
		accounts = append(accounts, r(instruction.GetPoolV2PDA(p.BaseMint)))
	}
	accounts = append(accounts, r(buyback), w(ata(buyback, p.QuoteMint, state.QuoteTokenProgram)))
	length := 24
	disc := instruction.PUMPSWAP_SELL_DISCRIMINATOR
	if quoteIn {
		length = 25
		disc = instruction.PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR
	}
	data := make([]byte, length)
	copy(data, disc)
	binary.LittleEndian.PutUint64(data[8:16], amount)
	binary.LittleEndian.PutUint64(data[16:24], minimum)
	if e = s.AssertUsable(); e != nil {
		return PreparedPumpSwap{}, e
	}
	return PreparedPumpSwap{State: state, Quote: PumpSwapQuote{amount, out, minimum}, Instruction: solana.NewInstruction(instruction.PUMPSWAP_PROGRAM, accounts, data)}, nil
}
