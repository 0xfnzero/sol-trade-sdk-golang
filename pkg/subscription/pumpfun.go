package subscription

// Current fee-program schedules and exact-in quotes. All state is frozen; no RPC.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

type PumpFunCurrentFees struct{ ProtocolFeeBps, CreatorFeeBps uint64 }
type pumpFeeTier struct {
	threshold *big.Int
	fees      [3]uint64
}

func DecodePumpFunCurrentFees(data []byte, quote solana.PublicKey, marketCap *big.Int, creatorOverride uint64) (PumpFunCurrentFees, error) {
	empty := PumpFunCurrentFees{}
	if len(data) < 69 || !bytes.Equal(data[:8], []byte{143, 52, 146, 187, 219, 123, 76, 155}) || marketCap == nil || marketCap.Sign() < 0 || marketCap.BitLen() > 128 {
		return empty, errors.New("invalid PumpFun FeeConfig or market cap")
	}
	offset := 41
	readFees := func() ([3]uint64, error) {
		var fees [3]uint64
		if offset+24 > len(data) {
			return fees, errors.New("truncated PumpFun fees")
		}
		for i := range fees {
			fees[i] = binary.LittleEndian.Uint64(data[offset+i*8:])
			if fees[i] > 10000 {
				return fees, errors.New("invalid PumpFun fee rate")
			}
		}
		offset += 24
		return fees, nil
	}
	flat, err := readFees()
	if err != nil {
		return empty, err
	}
	readTiers := func() ([]pumpFeeTier, error) {
		if offset+4 > len(data) {
			return nil, errors.New("truncated PumpFun tier count")
		}
		n := uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		if n > uint64((len(data)-offset)/40) {
			return nil, errors.New("truncated PumpFun fee tiers")
		}
		tiers := make([]pumpFeeTier, 0, n)
		for i := uint64(0); i < n; i++ {
			lo := new(big.Int).SetUint64(binary.LittleEndian.Uint64(data[offset:]))
			hi := new(big.Int).SetUint64(binary.LittleEndian.Uint64(data[offset+8:]))
			threshold := new(big.Int).Add(lo, hi.Lsh(hi, 64))
			offset += 16
			if i > 0 && threshold.Cmp(tiers[i-1].threshold) <= 0 {
				return nil, errors.New("PumpFun fee tiers are not strictly ordered")
			}
			fees, e := readFees()
			if e != nil {
				return nil, e
			}
			tiers = append(tiers, pumpFeeTier{threshold, fees})
		}
		return tiers, nil
	}
	tiers, err := readTiers()
	if err != nil {
		return empty, err
	}
	var stable []pumpFeeTier
	if offset < len(data) {
		stable, err = readTiers()
		if err != nil {
			return empty, err
		}
	}
	var exotic [3]uint64
	if offset < len(data) {
		exotic, err = readFees()
		if err != nil {
			return empty, err
		}
	}
	native := quote.IsZero() || quote == pumpWSOL || quote == solana.MustPublicKeyFromBase58("9pan9bMn5HatX4EJdBwg9VgCa7Uz5HL8N1m5D3NdXejP")
	var selected [3]uint64
	if native || quote == pumpUSDC {
		schedule := tiers
		if !native && len(stable) > 0 {
			schedule = stable
		}
		if len(schedule) == 0 {
			return empty, errors.New("PumpFun fee tiers cannot be empty")
		}
		selected = schedule[0].fees
		for _, tier := range schedule {
			if marketCap.Cmp(tier.threshold) >= 0 {
				selected = tier.fees
			} else {
				break
			}
		}
	} else {
		selected = flat
		if exotic != [3]uint64{} {
			selected = exotic
		}
	}
	creator := selected[2]
	if creatorOverride != 0 {
		creator = creatorOverride
	}
	if creator > 10000 || selected[1] > 10000-creator {
		return empty, errors.New("PumpFun combined fees exceed 100%")
	}
	return PumpFunCurrentFees{selected[1], creator}, nil
}

var pumpWSOL = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
var pumpUSDC = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

type CachedPumpFunState struct {
	Curve                                        *common.BondingCurveAccount
	Mint, Quote, TokenProgram, QuoteTokenProgram solana.PublicKey
	BuyFees, SellFees                            PumpFunCurrentFees
}

func (s *AccountCacheSnapshot) PumpFun(h PoolTradeHint, ctx CacheReadContext) (CachedPumpFunState, error) {
	empty := CachedPumpFunState{}
	a, err := s.Get(h.Pool, ctx, &instruction.PUMPFUN_PROGRAM)
	if err != nil {
		return empty, err
	}
	c := common.DecodeBondingCurveAccount(a.Data, [32]byte(h.Pool))
	if c == nil {
		return empty, errors.New("invalid PumpFun curve")
	}
	quote := solana.PublicKey(c.QuoteMint)
	if quote.IsZero() {
		quote = pumpWSOL
	}
	mint := h.InputMint
	if mint == quote {
		mint = h.OutputMint
	}
	if err = h.matches(mint, quote); err != nil {
		return empty, err
	}
	if instruction.GetBondingCurvePDA(mint) != h.Pool || c.Complete || c.VirtualTokenReserves == 0 || c.VirtualSolReserves == 0 {
		return empty, errors.New("invalid, completed or mismatched PumpFun curve")
	}
	g, err := s.Get(instruction.PUMPFUN_GLOBAL_ACCOUNT, ctx, &instruction.PUMPFUN_PROGRAM)
	if err != nil {
		return empty, err
	}
	if len(g.Data) < 1045 || !bytes.Equal(g.Data[:8], []byte{167, 232, 232, 177, 200, 108, 114, 127}) || g.Data[8] > 1 {
		return empty, errors.New("invalid PumpFun Global")
	}
	if len(g.Data) > 1045 && len(g.Data) < 1054 || len(a.Data) > 115 && len(a.Data) < 123 {
		return empty, errors.New("truncated PumpFun configurable fee fields")
	}
	if len(g.Data) >= 1054 && g.Data[1045] > 1 {
		return empty, errors.New("invalid PumpFun creator fee gate")
	}
	var override uint64
	if len(g.Data) >= 1054 && g.Data[1045] == 1 && len(a.Data) >= 123 {
		override = binary.LittleEndian.Uint64(a.Data[115:])
	}
	if override != 0 && override > binary.LittleEndian.Uint64(g.Data[1046:]) {
		return empty, errors.New("PumpFun creator fee exceeds Global maximum")
	}
	f, err := s.Get(instruction.PUMPFUN_FEE_CONFIG, ctx, &instruction.PUMPFUN_FEE_PROGRAM)
	if err != nil {
		return empty, err
	}
	address, bump, err := solana.FindProgramAddress([][]byte{[]byte("fee_config"), instruction.PUMPFUN_PROGRAM[:]}, instruction.PUMPFUN_FEE_PROGRAM)
	if err != nil || address != instruction.PUMPFUN_FEE_CONFIG || len(f.Data) < 9 || f.Data[8] != bump {
		return empty, errors.New("PumpFun FeeConfig PDA bump mismatch")
	}
	accounts := make([]CachedAccount, 2)
	for i, k := range []solana.PublicKey{mint, quote} {
		accounts[i], err = s.Get(k, ctx, nil)
		if err != nil {
			return empty, err
		}
		fee, e := instruction.TokenTransferFeeForEpoch(accounts[i].Data, accounts[i].Owner, ctx.Epoch)
		if e != nil {
			return empty, e
		}
		if fee.BasisPoints != 0 && fee.MaximumFee != 0 {
			return empty, errors.New("PumpFun nonzero transfer-fee quotes are not yet verified")
		}
	}
	supply := binary.LittleEndian.Uint64(accounts[0].Data[36:])
	if supply == 0 {
		return empty, errors.New("PumpFun mint supply is zero")
	}
	cap := func(v uint64) *big.Int {
		n := new(big.Int).Mul(new(big.Int).SetUint64(v), new(big.Int).SetUint64(c.VirtualSolReserves))
		return n.Div(n, new(big.Int).SetUint64(c.VirtualTokenReserves))
	}
	buy, err := DecodePumpFunCurrentFees(f.Data, quote, cap(supply), override)
	if err != nil {
		return empty, err
	}
	sellSupply := supply
	if !c.IsMayhemMode {
		sellSupply = 1_000_000_000_000_000
	}
	sell, err := DecodePumpFunCurrentFees(f.Data, quote, cap(sellSupply), override)
	if err != nil {
		return empty, err
	}
	if err = s.AssertUsable(); err != nil {
		return empty, err
	}
	return CachedPumpFunState{c, mint, quote, accounts[0].Owner, accounts[1].Owner, buy, sell}, nil
}

type CachedPumpFunQuote struct {
	AmountIn, EstimatedNetAmountOut, MinimumNetAmountOut uint64
	Fees                                                 PumpFunCurrentFees
}

func QuoteCachedPumpFunExactIn(s CachedPumpFunState, amount uint64, buy bool, slippage uint16) (CachedPumpFunQuote, error) {
	empty := CachedPumpFunQuote{}
	if amount == 0 || slippage > 9999 || s.Curve == nil || s.Curve.Complete || s.Curve.VirtualTokenReserves == 0 || s.Curve.VirtualSolReserves == 0 {
		return empty, errors.New("invalid PumpFun exact-in request")
	}
	f := s.SellFees
	if buy {
		f = s.BuyFees
	}
	creator := f.CreatorFeeBps
	if s.Curve.Creator == [32]byte{} {
		creator = 0
	}
	if f.ProtocolFeeBps > 10000 || f.CreatorFeeBps > 10000-f.ProtocolFeeBps {
		return empty, errors.New("invalid PumpFun fee rates")
	}
	vt := new(big.Int).SetUint64(s.Curve.VirtualTokenReserves)
	vq := new(big.Int).SetUint64(s.Curve.VirtualSolReserves)
	var estimate *big.Int
	if buy {
		net := new(big.Int).Mul(new(big.Int).SetUint64(amount-1), big.NewInt(10000))
		net.Div(net, new(big.Int).SetUint64(10000+f.ProtocolFeeBps+creator))
		estimate = new(big.Int).Mul(net, vt)
		estimate.Div(estimate, new(big.Int).Add(vq, net))
		real := new(big.Int).SetUint64(s.Curve.RealTokenReserves)
		if estimate.Cmp(real) > 0 {
			estimate = real
		}
	} else {
		gross := new(big.Int).Mul(new(big.Int).SetUint64(amount), vq)
		gross.Div(gross, new(big.Int).Add(vt, new(big.Int).SetUint64(amount)))
		fee := func(bps uint64) *big.Int {
			v := new(big.Int).Mul(gross, new(big.Int).SetUint64(bps))
			v.Add(v, big.NewInt(9999))
			return v.Div(v, big.NewInt(10000))
		}
		estimate = new(big.Int).Sub(gross, fee(f.ProtocolFeeBps))
		estimate.Sub(estimate, fee(creator))
	}
	minimum := new(big.Int).Mul(estimate, new(big.Int).SetUint64(uint64(10000-slippage)))
	minimum.Div(minimum, big.NewInt(10000))
	if estimate.Sign() <= 0 || minimum.Sign() <= 0 || !estimate.IsUint64() {
		return empty, errors.New("PumpFun quote has zero or invalid protected output")
	}
	return CachedPumpFunQuote{amount, estimate.Uint64(), minimum.Uint64(), f}, nil
}

type PreparedPumpFun struct {
	State       CachedPumpFunState
	Quote       CachedPumpFunQuote
	Instruction solana.Instruction
}

func (s *AccountCacheSnapshot) PreparePumpFun(h PoolTradeHint, ctx CacheReadContext, payer solana.PublicKey, amount uint64, slippage uint16) (PreparedPumpFun, error) {
	return s.preparePumpFunRouteLeg(h, ctx, payer, amount, slippage, true)
}

func (s *AccountCacheSnapshot) preparePumpFunRouteLeg(h PoolTradeHint, ctx CacheReadContext, payer solana.PublicKey, amount uint64, slippage uint16, allowNative bool) (PreparedPumpFun, error) {
	empty := PreparedPumpFun{}
	if payer.IsZero() {
		return empty, errors.New("missing PumpFun payer")
	}
	state, err := s.PumpFun(h, ctx)
	if err != nil {
		return empty, err
	}
	if state.Quote == pumpWSOL && !allowNative {
		return empty, errors.New("PumpFun native quote requires cached trade settlement")
	}
	buy := h.InputMint == state.Quote
	quote, err := QuoteCachedPumpFunExactIn(state, amount, buy, slippage)
	if err != nil {
		return empty, err
	}
	token := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	token2022 := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	if state.QuoteTokenProgram != token {
		return empty, errors.New("PumpFun V2 quote token program is unsupported")
	}
	mintString := state.Mint.String()
	if len(mintString) >= 4 && mintString[len(mintString)-4:] == "pump" && state.TokenProgram != token2022 {
		return empty, errors.New("PumpFun mint suffix and token program mismatch")
	}
	raw, err := s.Get(h.Pool, ctx, &instruction.PUMPFUN_PROGRAM)
	if err != nil {
		return empty, err
	}
	if len(raw.Data) >= 125 && raw.Data[124] != 0 {
		return empty, errors.New("PumpFun holder-reward account layout is not yet verified")
	}
	config := instruction.GetPumpFunFeeSharingConfigPDA(state.Mint)
	sharing, err := s.GetOptional(config, ctx, &instruction.PUMPFUN_FEE_PROGRAM)
	if err != nil {
		return empty, err
	}
	var active *solana.PublicKey
	if sharing != nil {
		active, err = DecodePumpFunSharingCreatorVault(sharing.Data, state.Mint)
		if err != nil {
			return empty, err
		}
	}
	creator := solana.PublicKey(state.Curve.Creator)
	vault := instruction.GetCreatorVaultPDA(creator)
	if active != nil {
		vault = *active
	}
	global, err := s.Get(instruction.PUMPFUN_GLOBAL_ACCOUNT, ctx, &instruction.PUMPFUN_PROGRAM)
	if err != nil {
		return empty, err
	}
	first := func(start, n int) (solana.PublicKey, error) {
		for i := 0; i < n; i++ {
			key := solana.PublicKeyFromBytes(global.Data[start+i*32 : start+(i+1)*32])
			if !key.IsZero() {
				return key, nil
			}
		}
		return solana.PublicKey{}, errors.New("missing PumpFun current fee recipient")
	}
	start := 41
	if state.Curve.IsMayhemMode {
		start = 483
	}
	recipient, err := first(start, 1)
	if err != nil {
		return empty, err
	}
	buyback, err := first(741, 8)
	if err != nil {
		return empty, err
	}
	pp := &instruction.PumpFunParams{BondingCurve: &instruction.BondingCurve{Account: h.Pool, VirtualTokenReserves: state.Curve.VirtualTokenReserves, VirtualSolReserves: state.Curve.VirtualSolReserves, RealTokenReserves: state.Curve.RealTokenReserves, Creator: creator, IsMayhemMode: state.Curve.IsMayhemMode, IsCashbackCoin: state.Curve.IsCashbackCoin}, CreatorVault: vault, TokenProgram: state.TokenProgram, FeeRecipient: recipient, QuoteMint: state.Quote}
	if active != nil {
		pp.FeeSharingCreatorVault = *active
	}
	var swaps []solana.Instruction
	if buy {
		swaps, err = instruction.PumpFunBuildBuyV2Instructions(&instruction.PumpFunBuildBuyParams{Payer: payer, InputMint: state.Quote, OutputMint: state.Mint, InputAmount: amount, ProtocolParams: pp, UseExactSolAmount: true})
	} else {
		swaps, err = instruction.PumpFunBuildSellV2Instructions(&instruction.PumpFunBuildSellParams{Payer: payer, InputMint: state.Mint, OutputMint: state.Quote, InputAmount: amount, ProtocolParams: pp, FixedOutputAmount: &quote.MinimumNetAmountOut})
	}
	if err != nil {
		return empty, err
	}
	expected := 26
	if buy {
		expected = 27
	}
	if len(swaps) != 1 || len(swaps[0].Accounts()) != expected {
		return empty, errors.New("unexpected PumpFun V2 layout")
	}
	data, err := swaps[0].Data()
	if err != nil {
		return empty, err
	}
	data = bytes.Clone(data)
	binary.LittleEndian.PutUint64(data[8:], amount)
	binary.LittleEndian.PutUint64(data[16:], quote.MinimumNetAmountOut)
	keys := swaps[0].Accounts()
	for i, k := range keys {
		copy := *k
		keys[i] = &copy
	}
	ata := func(owner solana.PublicKey) solana.PublicKey {
		address, _, _ := solana.FindProgramAddress([][]byte{owner[:], token[:], state.Quote[:]}, solana.SPLAssociatedTokenAccountProgramID)
		return address
	}
	keys[6].PublicKey = recipient
	keys[7].PublicKey = ata(recipient)
	keys[8].PublicKey = buyback
	keys[8].IsWritable = true
	keys[9].PublicKey = ata(buyback)
	if err = s.AssertUsable(); err != nil {
		return empty, err
	}
	return PreparedPumpFun{state, quote, solana.NewInstruction(instruction.PUMPFUN_PROGRAM, keys, data)}, nil
}
