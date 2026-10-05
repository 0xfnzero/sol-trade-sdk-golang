package subscription

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
)

// CachedDammV2State is validated state, not an automatic quotation.
// Rust swap2 requires the caller to provide an explicit output threshold.
type CachedDammV2State struct {
	PoolAddress                  solana.PublicKey
	Pool                         instruction.MeteoraDammV2Pool
	TokenAProgram, TokenBProgram solana.PublicKey
	TransferFees                 [2]calc.TokenTransferFee
	Reserves                     [2]uint64
}

func dammCompareU128(a, b [16]byte) int {
	for i := 15; i >= 0; i-- {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func (s *AccountCacheSnapshot) DammV2(h PoolTradeHint, c CacheReadContext, unixTimestamp uint64) (CachedDammV2State, error) {
	empty := CachedDammV2State{}
	fail := func(message string) (CachedDammV2State, error) { return empty, errors.New(message) }
	a, e := s.Get(h.Pool, c, &instruction.METEORA_DAMM_V2_PROGRAM)
	if e != nil {
		return empty, e
	}
	if len(a.Data) < 1112 || !bytes.Equal(a.Data[:8], []byte{241, 154, 109, 4, 17, 177, 109, 188}) {
		return fail("invalid DAMM v2 discriminator or size")
	}
	p := instruction.DecodeMeteoraPool(a.Data[8:])
	if e = h.matches(p.TokenAMint, p.TokenBMint); e != nil {
		return empty, e
	}
	if p.PoolStatus != 0 || p.Liquidity == [16]byte{} || p.ActivationType > 1 || p.SqrtMinPrice == [16]byte{} || dammCompareU128(p.SqrtPrice, p.SqrtMinPrice) < 0 || dammCompareU128(p.SqrtPrice, p.SqrtMaxPrice) > 0 {
		return fail("DAMM v2 pool is inactive or invalid")
	}
	point := c.Slot
	if p.ActivationType == 1 {
		point = unixTimestamp
	}
	if point < p.ActivationPoint {
		return fail("DAMM v2 pool is not activated")
	}
	if p.TokenAVault == p.TokenBVault {
		return fail("DAMM v2 vaults collide")
	}
	state := CachedDammV2State{PoolAddress: h.Pool, Pool: *p}
	mints := [2]solana.PublicKey{p.TokenAMint, p.TokenBMint}
	vaults := [2]solana.PublicKey{p.TokenAVault, p.TokenBVault}
	flags := [2]uint8{p.TokenAFlag, p.TokenBFlag}
	programs := [2]solana.PublicKey{}
	for i, k := range mints {
		mint, e := s.Get(k, c, nil)
		if e != nil {
			return empty, e
		}
		program := constants.TOKEN_PROGRAM
		if flags[i] == 1 {
			program = constants.TOKEN_PROGRAM_2022
		}
		if flags[i] > 1 || mint.Owner != program {
			return fail("DAMM v2 mint program flag mismatch")
		}
		programs[i] = program
		state.TransferFees[i], e = instruction.TokenTransferFeeForEpoch(mint.Data, mint.Owner, c.Epoch)
		if e != nil {
			return empty, e
		}
		v, e := s.Get(vaults[i], c, &program)
		if e != nil {
			return empty, e
		}
		if len(v.Data) < 165 || !bytes.Equal(v.Data[:32], k[:]) || !bytes.Equal(v.Data[32:64], instruction.METEORA_DAMM_V2_AUTHORITY[:]) || v.Data[108] != 1 {
			return fail("invalid DAMM v2 vault identity or state")
		}
		state.Reserves[i] = binary.LittleEndian.Uint64(v.Data[64:72])
	}
	state.TokenAProgram, state.TokenBProgram = programs[0], programs[1]
	if e = s.AssertUsable(); e != nil {
		return empty, e
	}
	return state, nil
}

// PrepareExplicitDammV2Route builds a direct exact-in swap with a caller threshold, not a quote.
func (s *AccountCacheSnapshot) PrepareExplicitDammV2Route(hints []PoolTradeHint, c CacheReadContext, timestamp uint64, payer solana.PublicKey, amount uint64, minimum *uint64, buy bool) (PreparedCachedRoute, error) {
	empty := PreparedCachedRoute{}
	if minimum == nil {
		return empty, errors.New("DAMM v2 requires explicit fixed output amount; it is not a quote")
	}
	if amount == 0 || *minimum == 0 || payer.IsZero() || len(hints) != 1 {
		return empty, errors.New("explicit DAMM v2 requires one independent positive exact-in swap")
	}
	h := hints[0]
	state, e := s.DammV2(h, c, timestamp)
	if e != nil {
		return empty, e
	}
	for _, f := range state.TransferFees {
		if f.BasisPoints != 0 && f.MaximumFee != 0 {
			return empty, errors.New("DAMM v2 net transfer-fee thresholds are not yet verified")
		}
	}
	p := state.Pool
	mode := uint8(0)
	params := &instruction.MeteoraDammV2Params{Pool: h.Pool, TokenAMint: p.TokenAMint, TokenBMint: p.TokenBMint, TokenAVault: p.TokenAVault, TokenBVault: p.TokenBVault, TokenAProgram: state.TokenAProgram, TokenBProgram: state.TokenBProgram, SwapMode: &mode, IncludeRateLimiterSysvar: p.PoolFees.BaseFee.FeeSchedulerMode == 2}
	var swaps []solana.Instruction
	if buy {
		swaps, e = instruction.MeteoraDammV2BuildBuyInstructions(&instruction.MeteoraDammV2BuildBuyParams{Payer: payer, InputMint: h.InputMint, OutputMint: h.OutputMint, InputAmount: amount, FixedOutputAmount: minimum, ProtocolParams: params})
	} else {
		swaps, e = instruction.MeteoraDammV2BuildSellInstructions(&instruction.MeteoraDammV2BuildSellParams{Payer: payer, InputMint: h.InputMint, OutputMint: h.OutputMint, InputAmount: amount, FixedOutputAmount: minimum, ProtocolParams: params})
	}
	if e != nil {
		return empty, e
	}
	if len(swaps) != 1 {
		return empty, errors.New("unexpected DAMM v2 exact-in instruction count")
	}
	setup := []solana.Instruction{}
	for _, mint := range []solana.PublicKey{h.InputMint, h.OutputMint} {
		program := state.TokenBProgram
		if mint == p.TokenAMint {
			program = state.TokenAProgram
		}
		ix, _, e := common.BuildCreateIdempotentATA(payer, payer, mint, program)
		if e != nil {
			return empty, e
		}
		setup = append(setup, ix)
	}
	if e = s.AssertUsable(); e != nil {
		return empty, e
	}
	return PreparedCachedRoute{Legs: []CachedRouteLeg{{Hint: h, AmountIn: amount, EstimatedNetAmountOut: nil, MinimumNetAmountOut: *minimum, Instruction: swaps[0]}}, SetupInstructions: setup, SwapInstructions: swaps, MinimumNetAmountOut: *minimum}, nil
}
