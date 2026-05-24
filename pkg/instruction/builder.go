package instruction

import (
	"fmt"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
	"github.com/gagliardetto/solana-go"
)

// InstructionBuilder defines the common protocol instruction builder surface.
type InstructionBuilder interface {
	BuildBuyInstructions(builderParams *BuildParams) ([]solana.Instruction, error)
	BuildSellInstructions(builderParams *BuildParams) ([]solana.Instruction, error)
}

// BuildParams contains protocol-independent fields used by the adapter builders.
type BuildParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      interface{}
	CreateInputATA      bool
	CreateOutputATA     bool
	CloseOutputATA      bool
	CloseInputATA       bool
	UseSeedOptimize     bool
	FixedOutputAmount   *uint64
	UseExactSolAmount   *bool
}

// CreateInstructionBuilder creates an instruction builder for the given DEX type.
func CreateInstructionBuilder(dexType soltradesdk.DexType) (InstructionBuilder, error) {
	switch dexType {
	case soltradesdk.DexTypePumpFun:
		return NewPumpFunInstructionBuilder(), nil
	case soltradesdk.DexTypePumpSwap:
		return NewPumpSwapInstructionBuilder(), nil
	case soltradesdk.DexTypeBonk:
		return NewBonkInstructionBuilder(), nil
	case soltradesdk.DexTypeRaydiumCpmm:
		return NewRaydiumCPMMInstructionBuilder(), nil
	case soltradesdk.DexTypeRaydiumAmmV4:
		return NewRaydiumAmmV4InstructionBuilder(), nil
	case soltradesdk.DexTypeMeteoraDammV2:
		return NewMeteoraDammV2InstructionBuilder(), nil
	default:
		return nil, fmt.Errorf("unsupported DEX type: %v", dexType)
	}
}

type PumpFunInstructionBuilder struct{}

func NewPumpFunInstructionBuilder() *PumpFunInstructionBuilder {
	return &PumpFunInstructionBuilder{}
}

func (b *PumpFunInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpFunParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	useExact := true
	if bp.UseExactSolAmount != nil {
		useExact = *bp.UseExactSolAmount
	}
	return PumpFunBuildBuyInstructions(&PumpFunBuildBuyParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toPumpFunParams(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CreateInputMintAta:  bp.CreateInputATA,
		UseExactSolAmount:   useExact,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *PumpFunInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpFunParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return PumpFunBuildSellInstructions(&PumpFunBuildSellParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toPumpFunParams(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

type PumpSwapInstructionBuilder struct{}

func NewPumpSwapInstructionBuilder() *PumpSwapInstructionBuilder {
	return &PumpSwapInstructionBuilder{}
}

func (b *PumpSwapInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpSwapParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	useExactQuote := true
	if bp.UseExactSolAmount != nil {
		useExactQuote = *bp.UseExactSolAmount
	}
	return BuildBuyInstructions(&BuildBuyParams{
		Payer:               bp.Payer,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toPumpSwapParams(protocolParams),
		CreateInputMintAta:  bp.CreateInputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		CreateOutputMintAta: bp.CreateOutputATA,
		UseExactQuoteAmount: useExactQuote,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *PumpSwapInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpSwapParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return BuildSellInstructions(&BuildSellParams{
		Payer:               bp.Payer,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toPumpSwapParams(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseOutputMintAta:  bp.CloseOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

type BonkInstructionBuilder struct{}

func NewBonkInstructionBuilder() *BonkInstructionBuilder {
	return &BonkInstructionBuilder{}
}

func (b *BonkInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.BonkParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return BonkBuildBuyInstructions(&BonkBuildBuyParams{
		Payer:               bp.Payer,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toBonkParams(protocolParams),
		CreateInputMintAta:  bp.CreateInputATA,
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *BonkInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.BonkParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return BonkBuildSellInstructions(&BonkBuildSellParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toBonkParams(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseOutputMintAta:  bp.CloseOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

type RaydiumCPMMInstructionBuilder struct{}

func NewRaydiumCPMMInstructionBuilder() *RaydiumCPMMInstructionBuilder {
	return &RaydiumCPMMInstructionBuilder{}
}

func (b *RaydiumCPMMInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumCpmmParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return RaydiumCPMMBuildBuyInstructions(&RaydiumCPMMBuildBuyParams{
		Payer:               bp.Payer,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toRaydiumCPMMParams(protocolParams),
		CreateInputMintAta:  bp.CreateInputATA,
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *RaydiumCPMMInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumCpmmParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return RaydiumCPMMBuildSellInstructions(&RaydiumCPMMBuildSellParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toRaydiumCPMMParams(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseOutputMintAta:  bp.CloseOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

type RaydiumAmmV4InstructionBuilder struct{}

func NewRaydiumAmmV4InstructionBuilder() *RaydiumAmmV4InstructionBuilder {
	return &RaydiumAmmV4InstructionBuilder{}
}

func (b *RaydiumAmmV4InstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumAmmV4Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return RaydiumAmmV4BuildBuyInstructions(&RaydiumAmmV4BuildBuyParams{
		Payer:               bp.Payer,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toRaydiumAmmV4Params(protocolParams),
		CreateInputMintAta:  bp.CreateInputATA,
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *RaydiumAmmV4InstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumAmmV4Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return RaydiumAmmV4BuildSellInstructions(&RaydiumAmmV4BuildSellParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toRaydiumAmmV4Params(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseOutputMintAta:  bp.CloseOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

type MeteoraDammV2InstructionBuilder struct{}

func NewMeteoraDammV2InstructionBuilder() *MeteoraDammV2InstructionBuilder {
	return &MeteoraDammV2InstructionBuilder{}
}

func (b *MeteoraDammV2InstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.MeteoraDammV2Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return MeteoraDammV2BuildBuyInstructions(&MeteoraDammV2BuildBuyParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toMeteoraDammV2Params(protocolParams),
		CreateInputMintAta:  bp.CreateInputATA,
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func (b *MeteoraDammV2InstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.MeteoraDammV2Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}
	return MeteoraDammV2BuildSellInstructions(&MeteoraDammV2BuildSellParams{
		Payer:               bp.Payer,
		InputMint:           bp.InputMint,
		OutputMint:          bp.OutputMint,
		InputAmount:         bp.InputAmount,
		SlippageBasisPoints: bp.SlippageBasisPoints,
		ProtocolParams:      toMeteoraDammV2Params(protocolParams),
		CreateOutputMintAta: bp.CreateOutputATA,
		CloseOutputMintAta:  bp.CloseOutputATA,
		CloseInputMintAta:   bp.CloseInputATA,
		FixedOutputAmount:   bp.FixedOutputAmount,
	})
}

func toPumpFunParams(p *params.PumpFunParams) *PumpFunParams {
	var bondingCurve *BondingCurve
	if p.BondingCurve != nil {
		bondingCurve = &BondingCurve{
			Account:              p.BondingCurve.Account,
			VirtualTokenReserves: p.BondingCurve.VirtualTokenReserves,
			VirtualSolReserves:   p.BondingCurve.VirtualSolReserves,
			RealTokenReserves:    p.BondingCurve.RealTokenReserves,
			Creator:              p.BondingCurve.Creator,
			IsMayhemMode:         p.BondingCurve.IsMayhemMode,
			IsCashbackCoin:       p.BondingCurve.IsCashbackCoin,
		}
	}
	return &PumpFunParams{
		BondingCurve:              bondingCurve,
		CreatorVault:              p.CreatorVault,
		AssociatedBondingCurve:    p.AssociatedBondingCurve,
		TokenProgram:              p.TokenProgram,
		CloseTokenAccountWhenSell: p.CloseTokenAccountWhenSell,
		ObservedTradeCreator:      p.ObservedTradeCreator,
		FeeSharingCreatorVault:    p.FeeSharingCreatorVault,
		FeeRecipient:              p.FeeRecipient,
		QuoteMint:                 p.QuoteMint,
	}
}

func toPumpSwapParams(p *params.PumpSwapParams) *PumpSwapParams {
	return &PumpSwapParams{
		Pool:                      p.Pool,
		BaseMint:                  p.BaseMint,
		QuoteMint:                 p.QuoteMint,
		PoolBaseTokenAccount:      p.PoolBaseTokenAccount,
		PoolQuoteTokenAccount:     p.PoolQuoteTokenAccount,
		PoolBaseTokenReserves:     p.PoolBaseTokenReserves,
		PoolQuoteTokenReserves:    p.PoolQuoteTokenReserves,
		CoinCreatorVaultAta:       p.CoinCreatorVaultATA,
		CoinCreatorVaultAuthority: p.CoinCreatorVaultAuth,
		BaseTokenProgram:          p.BaseTokenProgram,
		QuoteTokenProgram:         p.QuoteTokenProgram,
		IsMayhemMode:              p.IsMayhemMode,
		IsCashbackCoin:            p.IsCashbackCoin,
	}
}

func toBonkParams(p *params.BonkParams) *BonkParams {
	return &BonkParams{
		PoolState:                 p.PoolState,
		BaseVault:                 p.BaseVault,
		QuoteVault:                p.QuoteVault,
		PlatformConfig:            p.PlatformConfig,
		PlatformAssociatedAccount: p.PlatformAssociatedAccount,
		CreatorAssociatedAccount:  p.CreatorAssociatedAccount,
		GlobalConfig:              p.GlobalConfig,
		MintTokenProgram:          p.MintTokenProgram,
		VirtualBase:               p.VirtualBase.Lo,
		VirtualQuote:              p.VirtualQuote.Lo,
		RealBase:                  p.RealBase.Lo,
		RealQuote:                 p.RealQuote.Lo,
	}
}

func toRaydiumCPMMParams(p *params.RaydiumCpmmParams) *RaydiumCPMMParams {
	return &RaydiumCPMMParams{
		PoolState:         p.PoolState,
		AmmConfig:         p.AmmConfig,
		BaseMint:          p.BaseMint,
		QuoteMint:         p.QuoteMint,
		BaseVault:         p.BaseVault,
		QuoteVault:        p.QuoteVault,
		BaseReserve:       p.BaseReserve,
		QuoteReserve:      p.QuoteReserve,
		BaseTokenProgram:  p.BaseTokenProgram,
		QuoteTokenProgram: p.QuoteTokenProgram,
		ObservationState:  p.ObservationState,
	}
}

func toRaydiumAmmV4Params(p *params.RaydiumAmmV4Params) *RaydiumAmmV4Params {
	return &RaydiumAmmV4Params{
		Amm:                   p.Amm,
		AmmOpenOrders:         p.AmmOpenOrders,
		AmmTargetOrders:       p.AmmTargetOrders,
		TokenCoin:             p.TokenCoin,
		TokenPc:               p.TokenPc,
		SerumProgram:          p.SerumProgram,
		SerumMarket:           p.SerumMarket,
		SerumBids:             p.SerumBids,
		SerumAsks:             p.SerumAsks,
		SerumEventQueue:       p.SerumEventQueue,
		SerumCoinVaultAccount: p.SerumCoinVaultAccount,
		SerumPcVaultAccount:   p.SerumPcVaultAccount,
		SerumVaultSigner:      p.SerumVaultSigner,
		CoinMint:              p.CoinMint,
		PcMint:                p.PcMint,
		CoinReserve:           p.CoinReserve,
		PcReserve:             p.PcReserve,
	}
}

func toMeteoraDammV2Params(p *params.MeteoraDammV2Params) *MeteoraDammV2Params {
	return &MeteoraDammV2Params{
		Pool:          p.Pool,
		TokenAMint:    p.TokenAMint,
		TokenBMint:    p.TokenBMint,
		TokenAVault:   p.TokenAVault,
		TokenBVault:   p.TokenBVault,
		TokenAProgram: p.TokenAProgram,
		TokenBProgram: p.TokenBProgram,
		TokenAReserve: p.TokenAReserve,
		TokenBReserve: p.TokenBReserve,
	}
}
