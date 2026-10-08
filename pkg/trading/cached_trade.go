package trading

// High-level native cache -> independent buy/sell -> V1; no implicit RPC.
import (
	"context"
	"errors"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"math"
)

type CachedTradeRequest struct {
	Candidates                []subscription.PoolTradeHint
	InputMint, OutputMint     solana.PublicKey
	DexType                   soltradesdk.DexType
	TradeType                 soltradesdk.TradeType
	Snapshot                  *subscription.AccountCacheSnapshot
	Hints                     []subscription.PoolTradeHint
	Context                   subscription.CacheReadContext
	UnixTimestamp             uint64
	Payer                     solana.PublicKey
	Amount                    uint64
	FixedOutputAmount         *uint64
	RecentBlockhash           solana.Hash
	SlippageBps               *uint16
	MaximumArrays             int
	V1Config                  *serialization.V1Config
	NativeInput, NativeOutput bool
	TemporaryWsolSeed         string
	RentLamports              uint64
	TipAccount                solana.PublicKey
	TipLamports               uint64
}
type PreparedCachedTrade struct {
	EstimatedNativeResidualLamports uint64
	Route                           subscription.PreparedCachedRoute
	Instructions                    []solana.Instruction
	Compiled                        *serialization.CompiledV1Message
	RequiredNativeLamports          uint64
}

func cachedProgram(d soltradesdk.DexType) (solana.PublicKey, bool) {
	switch d {
	case soltradesdk.DexTypePumpFun:
		return instruction.PUMPFUN_PROGRAM, true
	case soltradesdk.DexTypeMeteoraDammV2:
		return instruction.METEORA_DAMM_V2_PROGRAM, true
	case soltradesdk.DexTypePumpSwap:
		return instruction.PUMPSWAP_PROGRAM, true
	case soltradesdk.DexTypeStonkFun, soltradesdk.DexTypeLaunchLab, soltradesdk.DexTypeBonk:
		return solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj"), true
	case soltradesdk.DexTypeRaydiumAmmV4:
		return subscription.AmmV4Program, true
	case soltradesdk.DexTypeRaydiumCpmm:
		return subscription.CpmmProgram, true
	case soltradesdk.DexTypeRaydiumClmm:
		return subscription.ClmmProgram, true
	case soltradesdk.DexTypeOrcaWhirlpool:
		return subscription.WhirlpoolProgram, true
	case soltradesdk.DexTypeMeteoraDlmm:
		return subscription.DlmmProgram, true
	}
	return solana.PublicKey{}, false
}
func PrepareCachedTrade(r CachedTradeRequest) (PreparedCachedTrade, error) {
	empty := PreparedCachedTrade{}
	if r.SlippageBps != nil && *r.SlippageBps >= 10000 {
		return empty, errors.New("invalid cached trade slippage: expected basis points in [0, 10000)")
	}
	if r.TradeType != soltradesdk.TradeTypeBuy && r.TradeType != soltradesdk.TradeTypeSell {
		return empty, errors.New("cached trade requires independent Buy or Sell")
	}
	if r.Snapshot == nil {
		return empty, errors.New("provide frozen cache snapshot")
	}
	if err := r.Snapshot.AssertUsable(); err != nil {
		return empty, err
	}
	if len(r.Hints) == 0 {

		failures := []error{errors.New("no candidate route passed current state validation and quoting")}
		var selected PreparedCachedTrade
		found := false
		err := subscription.VisitCandidateRoutes(r.Candidates, r.InputMint, r.OutputMint, 5, func(hints []subscription.PoolTradeHint) (bool, error) {
			if err := r.Snapshot.AssertUsable(); err != nil {
				return false, err
			}
			candidate := r
			candidate.Hints = hints
			candidate.Candidates = nil
			result, err := PrepareCachedTrade(candidate)
			if err != nil {
				var unavailable *subscription.CacheNotReadyError
				if errors.As(err, &unavailable) {
					return false, err
				}
				failures = append(failures, err)
				return false, nil
			}
			selected = result
			found = true
			return true, nil
		})
		if err != nil {
			return empty, err
		}
		if found {
			return selected, nil
		}
		return empty, errors.Join(failures...)

	}
	program, ok := cachedProgram(r.DexType)
	if !ok || r.Snapshot == nil || len(r.Hints) == 0 {
		return empty, errors.New("unsupported cached trade protocol or empty path")
	}
	i := 0
	if r.TradeType == soltradesdk.TradeTypeBuy {
		i = len(r.Hints) - 1
	}
	anchor := r.Hints[i]
	p, e := r.Snapshot.Get(anchor.Pool, r.Context, &program)
	if e != nil {
		return empty, e
	}
	_ = p
	stateNative := false
	if r.DexType == soltradesdk.DexTypePumpFun {
		state, err := r.Snapshot.PumpFun(anchor, r.Context)
		if err != nil {
			return empty, err
		}
		mint := anchor.InputMint
		if r.TradeType == soltradesdk.TradeTypeBuy {
			mint = anchor.OutputMint
		}
		if state.Mint != mint {
			return empty, errors.New("PumpFun anchor direction does not match independent trade type")
		}
		stateNative = state.Quote == solana.WrappedSol
	}
	if r.DexType == soltradesdk.DexTypeStonkFun || r.DexType == soltradesdk.DexTypeLaunchLab || r.DexType == soltradesdk.DexTypeBonk {
		var accounts instruction.StonkFunCurveAccounts
		if r.DexType == soltradesdk.DexTypeStonkFun {
			accounts, _, e = r.Snapshot.StonkFunCurve(anchor, r.Context)
		} else {
			accounts, _, e = r.Snapshot.LaunchLabCurve(anchor, r.Context)
		}
		if e != nil {
			return empty, e
		}
		mint := anchor.InputMint
		if r.TradeType == soltradesdk.TradeTypeBuy {
			mint = anchor.OutputMint
		}
		if accounts.BaseMint != mint {
			return empty, errors.New("LaunchLab anchor direction does not match independent trade type")
		}
	}
	slippage := uint16(100)
	if r.SlippageBps != nil {
		slippage = *r.SlippageBps
	}
	arrays := r.MaximumArrays
	if arrays == 0 {
		arrays = 8
	}
	var route subscription.PreparedCachedRoute
	if r.DexType == soltradesdk.DexTypeMeteoraDammV2 {
		route, e = r.Snapshot.PrepareExplicitDammV2Route(r.Hints, r.Context, r.UnixTimestamp, r.Payer, r.Amount, r.FixedOutputAmount, r.TradeType == soltradesdk.TradeTypeBuy)
	} else {
		if r.FixedOutputAmount != nil {
			return empty, errors.New("fixed output amount is only supported by explicit DAMM v2 preparation")
		}
		route, e = r.Snapshot.PrepareRoute(r.Hints, r.Context, r.UnixTimestamp, r.Payer, r.Amount, slippage, arrays, stateNative)
	}
	if e != nil {
		return empty, e
	}
	instructions := append(append([]solana.Instruction{}, route.SetupInstructions...), route.SwapInstructions...)
	required := uint64(0)
	residual := uint64(0)
	if stateNative {
		n, err := subscription.SettlePumpFunNativeQuote(route, r.Payer, r.NativeInput, r.NativeOutput, r.TemporaryWsolSeed, r.RentLamports)
		if err != nil {
			return empty, err
		}
		instructions = n.Instructions
		required = n.RequiredLamports
		residual = n.EstimatedNativeResidualLamports
		if r.TradeType == soltradesdk.TradeTypeSell && len(route.Legs) > 1 {
			route.EstimatedIntermediateResiduals = append([]subscription.RouteResidual{}, route.EstimatedIntermediateResiduals...)
			route.EstimatedIntermediateResiduals[0].Amount = route.Legs[0].MinimumNetAmountOut - route.Legs[1].AmountIn
		}
	} else if r.NativeInput || r.NativeOutput {
		n, e := subscription.SettleCachedRouteWithNativeSol(route, r.Payer, r.TemporaryWsolSeed, r.RentLamports, r.NativeInput, r.NativeOutput)
		if e != nil {
			return empty, e
		}
		instructions = n.Instructions
		required = n.RequiredLamports
	}

	if r.TipLamports != 0 || !r.TipAccount.IsZero() {
		if r.TipLamports == 0 || r.TipAccount.IsZero() || r.TipAccount == r.Payer {
			return empty, errors.New("provide a positive tip and a distinct non-default recipient")
		}
		if r.TipLamports > math.MaxUint64-required {
			return empty, errors.New("native funding plus tip exceeds u64")
		}
		required += r.TipLamports
		tip := system.NewTransferInstruction(r.TipLamports, r.Payer, r.TipAccount).Build()
		instructions = append([]solana.Instruction{tip}, instructions...)
	}
	compute, loaded := uint32(300000), uint32(64*1024*1024)
	config := serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded}
	if r.V1Config != nil {
		config = *r.V1Config
		if config.ComputeUnitLimit == nil {
			config.ComputeUnitLimit = &compute
		}
		if config.LoadedAccountsDataSizeLimit == nil {
			config.LoadedAccountsDataSizeLimit = &loaded
		}
	}
	compiled, e := serialization.CompileV1Message(r.Payer, instructions, r.RecentBlockhash, config)
	if e != nil {
		return empty, e
	}
	if e = r.Snapshot.AssertUsable(); e != nil {
		return empty, e
	}
	return PreparedCachedTrade{Route: route, Instructions: instructions, Compiled: compiled, RequiredNativeLamports: required, EstimatedNativeResidualLamports: residual}, nil
}

type CachedWireSubmit func(context.Context, []byte, soltradesdk.TradeType) (solana.Signature, error)
type CachedTradeReceipt struct {
	Signature            solana.Signature
	Submitted, Confirmed bool
	MinimumNetAmountOut  uint64
}
type CachedTradeExecutor struct{ DexType soltradesdk.DexType }

func (e *CachedTradeExecutor) Prepare(r CachedTradeRequest) (PreparedCachedTrade, error) {
	if e == nil || e.DexType != r.DexType {
		return PreparedCachedTrade{}, errors.New("factory/request protocol mismatch")
	}
	return PrepareCachedTrade(r)
}

// Submission uses a caller-owned SWQOS/raw-wire transport; never polls RPC.
func (e *CachedTradeExecutor) Execute(ctx context.Context, r CachedTradeRequest, signers []solana.PrivateKey, submit CachedWireSubmit) (CachedTradeReceipt, error) {
	empty := CachedTradeReceipt{}
	if submit == nil {
		return empty, errors.New("provide a raw-wire submission transport")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	prepared, err := e.Prepare(r)
	if err != nil {
		return empty, err
	}
	wire, err := serialization.SignV1Transaction(prepared.Compiled, signers)
	if err != nil {
		return empty, err
	}
	var signature solana.Signature
	copy(signature[:], wire[len(prepared.Compiled.Message):len(prepared.Compiled.Message)+64])
	if err := r.Snapshot.AssertUsable(); err != nil {
		return empty, err
	}
	returned, err := submit(ctx, append([]byte{}, wire...), r.TradeType)
	if err != nil {
		return empty, err
	}
	if returned != signature {
		return empty, errors.New("submission signature does not match signed V1 transaction")
	}
	return CachedTradeReceipt{signature, true, false, prepared.Route.MinimumNetAmountOut}, nil
}
func (f *TradeExecutorFactory) CreateCachedExecutor(d soltradesdk.DexType) (*CachedTradeExecutor, error) {
	if _, ok := cachedProgram(d); !ok {
		return nil, errors.New("protocol has no native cached trade executor")
	}
	return &CachedTradeExecutor{d}, nil
}

func (c *TradingClient) PrepareCachedTrade(r CachedTradeRequest) (PreparedCachedTrade, error) {
	return PrepareCachedTrade(r)
}
func (c *TradingClient) ExecuteCachedTrade(ctx context.Context, r CachedTradeRequest, signers []solana.PrivateKey, submit CachedWireSubmit) (CachedTradeReceipt, error) {
	e, err := c.factory.CreateCachedExecutor(r.DexType)
	if err != nil {
		return CachedTradeReceipt{}, err
	}
	return e.Execute(ctx, r, signers, submit)
}
