package instruction

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
	"github.com/your-org/sol-trade-sdk-go/pkg/params"
)

// InstructionBuilder defines the interface for building trade instructions
type InstructionBuilder interface {
	BuildBuyInstructions(builderParams *BuildParams) ([]solana.Instruction, error)
	BuildSellInstructions(builderParams *BuildParams) ([]solana.Instruction, error)
}

// BuildParams contains parameters for building instructions
type BuildParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      interface{}
	CreateOutputATA     bool
	CloseInputATA       bool
	UseSeedOptimize     bool
	FixedOutputAmount   *uint64
	UseExactSolAmount   *bool
}

// PumpFunInstructionBuilder builds instructions for PumpFun protocol
type PumpFunInstructionBuilder struct{}

// NewPumpFunInstructionBuilder creates a new PumpFun instruction builder
func NewPumpFunInstructionBuilder() *PumpFunInstructionBuilder {
	return &PumpFunInstructionBuilder{}
}

// BuildBuyInstructions builds buy instructions for PumpFun
func (b *PumpFunInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpFunParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Get bonding curve address
	bondingCurveAddr := protocolParams.BondingCurve.Account
	if bondingCurveAddr.IsZero() {
		// Derive PDA
		bondingCurveAddr = GetBondingCurvePDA(bp.OutputMint)
	}

	// Get associated bonding curve token account
	associatedBondingCurve := protocolParams.AssociatedBondingCurve
	if associatedBondingCurve.IsZero() {
		associatedBondingCurve = GetAssociatedTokenAddress(bondingCurveAddr, bp.OutputMint, protocolParams.TokenProgram)
	}

	// Get user token account
	userTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, protocolParams.TokenProgram)

	// Create ATA instruction if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.TokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build buy instruction data
	buyData := make([]byte, 26)
	if bp.UseExactSolAmount == nil || *bp.UseExactSolAmount {
		// buy_exact_sol_in
		copy(buyData[0:8], constants.BUY_EXACT_SOL_IN_DISCRIMINATOR[:])
		// Amount in
		leBytes := uint64ToLEBytes(bp.InputAmount)
		copy(buyData[8:16], leBytes[:])
		// Min tokens out (with slippage)
		minTokensOut := calculateMinOutput(0, bp.SlippageBasisPoints) // Simplified
		copy(buyData[16:24], uint64ToLEBytes(minTokensOut)[:])
		// Track volume
		trackVolume := [2]byte{1, 0}
		if protocolParams.BondingCurve.IsCashbackCoin {
			trackVolume = [2]byte{1, 1}
		}
		copy(buyData[24:26], trackVolume[:])
	} else {
		// Regular buy
		copy(buyData[0:8], constants.BUY_DISCRIMINATOR[:])
		// Token amount
		copy(buyData[8:16], uint64ToLEBytes(0)[:]) // Simplified
		// Max SOL cost
		maxSolCost := calculateMaxCost(bp.InputAmount, bp.SlippageBasisPoints)
		copy(buyData[16:24], uint64ToLEBytes(maxSolCost)[:])
	}

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: GetGlobalAccount(), IsSigner: false, IsWritable: false},
		{PublicKey: GetFeeRecipient(protocolParams.BondingCurve.IsMayhemMode), IsSigner: false, IsWritable: true},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: userTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.TokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.CreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: GetEventAuthority(), IsSigner: false, IsWritable: false},
		{PublicKey: constants.PUMPFUN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		// Additional accounts...
	}

	buyIx := solana.NewInstruction(
		constants.PUMPFUN_PROGRAM_ID,
		accounts,
		buyData,
	)
	instructions = append(instructions, buyIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for PumpFun
func (b *PumpFunInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpFunParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Get bonding curve address
	bondingCurveAddr := protocolParams.BondingCurve.Account
	if bondingCurveAddr.IsZero() {
		bondingCurveAddr = GetBondingCurvePDA(bp.InputMint)
	}

	// Get associated bonding curve token account
	associatedBondingCurve := protocolParams.AssociatedBondingCurve
	if associatedBondingCurve.IsZero() {
		associatedBondingCurve = GetAssociatedTokenAddress(bondingCurveAddr, bp.InputMint, protocolParams.TokenProgram)
	}

	// Get user token account
	userTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, protocolParams.TokenProgram)

	// Build sell instruction data
	sellData := make([]byte, 24)
	copy(sellData[0:8], constants.SELL_DISCRIMINATOR[:])
	copy(sellData[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minSolOutput := calculateMinOutput(0, bp.SlippageBasisPoints) // Simplified
	copy(sellData[16:24], uint64ToLEBytes(minSolOutput)[:])

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: GetGlobalAccount(), IsSigner: false, IsWritable: false},
		{PublicKey: GetFeeRecipient(protocolParams.BondingCurve.IsMayhemMode), IsSigner: false, IsWritable: true},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: userTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.CreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: GetEventAuthority(), IsSigner: false, IsWritable: false},
		{PublicKey: constants.PUMPFUN_PROGRAM_ID, IsSigner: false, IsWritable: false},
	}

	// Add cashback account if needed
	if protocolParams.BondingCurve.IsCashbackCoin {
		userVolumeAccumulator := GetUserVolumeAccumulatorPDA(bp.Payer)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey:  userVolumeAccumulator,
			IsSigner:   false,
			IsWritable: true,
		})
	}

	sellIx := solana.NewInstruction(
		constants.PUMPFUN_PROGRAM_ID,
		accounts,
		sellData,
	)
	instructions = append(instructions, sellIx)

	// Close token account if requested
	if bp.CloseInputATA || (protocolParams.CloseTokenAccountWhenSell != nil && *protocolParams.CloseTokenAccountWhenSell) {
		closeIx := BuildCloseAccountInstruction(
			protocolParams.TokenProgram,
			userTokenAccount,
			bp.Payer,
			bp.Payer,
		)
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

// PumpSwapInstructionBuilder builds instructions for PumpSwap protocol
type PumpSwapInstructionBuilder struct{}

// NewPumpSwapInstructionBuilder creates a new PumpSwap instruction builder
func NewPumpSwapInstructionBuilder() *PumpSwapInstructionBuilder {
	return &PumpSwapInstructionBuilder{}
}

// BuildBuyInstructions builds buy instructions for PumpSwap
func (b *PumpSwapInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpSwapParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Create ATA if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.BaseTokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build swap instruction
	// Note: This is a simplified version - full implementation requires
	// proper account derivation and amount calculations
	swapIx := b.buildSwapInstruction(bp, protocolParams, true)
	instructions = append(instructions, swapIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for PumpSwap
func (b *PumpSwapInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.PumpSwapParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Build swap instruction
	swapIx := b.buildSwapInstruction(bp, protocolParams, false)
	instructions = append(instructions, swapIx)

	// Close ATA if needed
	if bp.CloseInputATA {
		userTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, protocolParams.BaseTokenProgram)
		closeIx := BuildCloseAccountInstruction(
			protocolParams.BaseTokenProgram,
			userTokenAccount,
			bp.Payer,
			bp.Payer,
		)
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

func (b *PumpSwapInstructionBuilder) buildSwapInstruction(bp *BuildParams, params *params.PumpSwapParams, isBuy bool) solana.Instruction {
	// Simplified swap instruction building
	// Full implementation requires proper discriminator and account setup
	data := make([]byte, 24)
	// Add instruction discriminator and amounts

	accounts := []solana.AccountMeta{
		{PublicKey: params.Pool, IsSigner: false, IsWritable: true},
		// Add more accounts...
	}

	return solana.NewInstruction(constants.PUMPSWAP_PROGRAM_ID, accounts, data)
}

// Helper functions

func uint64ToLEBytes(v uint64) [8]byte {
	return [8]byte{
		byte(v),
		byte(v >> 8),
		byte(v >> 16),
		byte(v >> 24),
		byte(v >> 32),
		byte(v >> 40),
		byte(v >> 48),
		byte(v >> 56),
	}
}

func calculateMinOutput(amount, slippage uint64) uint64 {
	if slippage == 0 {
		return amount
	}
	return amount * (10000 - slippage) / 10000
}

func calculateMaxCost(amount, slippage uint64) uint64 {
	if slippage == 0 {
		return amount
	}
	return amount * (10000 + slippage) / 10000
}

// CreateInstructionBuilder creates an instruction builder for the given DEX type
func CreateInstructionBuilder(dexType soltradesdk.DexType) (InstructionBuilder, error) {
	switch dexType {
	case soltradesdk.DexTypePumpFun:
		return NewPumpFunInstructionBuilder(), nil
	case soltradesdk.DexTypePumpSwap:
		return NewPumpSwapInstructionBuilder(), nil
	case soltradesdk.DexTypeBonk:
		return NewBonkInstructionBuilder(), nil
	case soltradesdk.DexTypeRaydiumCPMM:
		return NewRaydiumCPMMInstructionBuilder(), nil
	case soltradesdk.DexTypeRaydiumAmmV4:
		return NewRaydiumAmmV4InstructionBuilder(), nil
	case soltradesdk.DexTypeMeteoraDammV2:
		return NewMeteoraDammV2InstructionBuilder(), nil
	default:
		return nil, fmt.Errorf("unsupported DEX type: %v", dexType)
	}
}

// ===== Bonk Instruction Builder =====

// BonkInstructionBuilder builds instructions for Bonk protocol
type BonkInstructionBuilder struct{}

// NewBonkInstructionBuilder creates a new Bonk instruction builder
func NewBonkInstructionBuilder() *BonkInstructionBuilder {
	return &BonkInstructionBuilder{}
}

// Bonk discriminators
var (
	BONK_BUY_DISCRIMINATOR  = []byte{102, 6, 61, 18, 1, 218, 235, 234}
	BONK_SELL_DISCRIMINATOR = []byte{51, 230, 133, 164, 1, 127, 131, 173}
)

// BuildBuyInstructions builds buy instructions for Bonk
func (b *BonkInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.BonkParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Create ATA if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.TokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], BONK_BUY_DISCRIMINATOR)
	copy(data[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	copy(data[16:24], uint64ToLEBytes(minAmountOut)[:])

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: protocolParams.PoolState, IsSigner: false, IsWritable: true},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false}, // base mint
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},  // quote mint
		{PublicKey: protocolParams.BaseVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.QuoteVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.PlatformConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.PlatformAssociatedAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.CreatorAssociatedAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.GlobalConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.UserBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.UserQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	buyIx := solana.NewInstruction(constants.BONK_PROGRAM_ID, accounts, data)
	instructions = append(instructions, buyIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for Bonk
func (b *BonkInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.BonkParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], BONK_SELL_DISCRIMINATOR)
	copy(data[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	copy(data[16:24], uint64ToLEBytes(minAmountOut)[:])

	// Build accounts (swap mints for sell)
	accounts := []solana.AccountMeta{
		{PublicKey: protocolParams.PoolState, IsSigner: false, IsWritable: true},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},  // base mint (selling)
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false}, // quote mint
		{PublicKey: protocolParams.BaseVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.QuoteVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.PlatformConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.PlatformAssociatedAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.CreatorAssociatedAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.GlobalConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.UserBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.UserQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	sellIx := solana.NewInstruction(constants.BONK_PROGRAM_ID, accounts, data)
	instructions = append(instructions, sellIx)

	return instructions, nil
}

// ===== Raydium CPMM Instruction Builder =====

// RaydiumCPMMInstructionBuilder builds instructions for Raydium CPMM protocol
type RaydiumCPMMInstructionBuilder struct{}

// NewRaydiumCPMMInstructionBuilder creates a new Raydium CPMM instruction builder
func NewRaydiumCPMMInstructionBuilder() *RaydiumCPMMInstructionBuilder {
	return &RaydiumCPMMInstructionBuilder{}
}

// Raydium CPMM discriminators
var (
	RAYDIUM_CPMM_SWAP_DISCRIMINATOR = []byte{248, 198, 158, 145, 225, 117, 135, 200}
)

// BuildBuyInstructions builds buy instructions for Raydium CPMM
func (b *RaydiumCPMMInstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumCPMMParams)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Create ATA if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.TokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RAYDIUM_CPMM_SWAP_DISCRIMINATOR)
	copy(data[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	copy(data[16:24], uint64ToLEBytes(minAmountOut)[:])

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: protocolParams.Authority, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.AmmConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.PoolState, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.InputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.OutputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.InputVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.OutputVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.InputTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.OutputTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.ObservationState, IsSigner: false, IsWritable: true},
	}

	swapIx := solana.NewInstruction(constants.RAYDIUM_CPMM_PROGRAM_ID, accounts, data)
	instructions = append(instructions, swapIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for Raydium CPMM
func (b *RaydiumCPMMInstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	// Same as buy but with swapped input/output
	return b.BuildBuyInstructions(bp)
}

// ===== Raydium AMM V4 Instruction Builder =====

// RaydiumAmmV4InstructionBuilder builds instructions for Raydium AMM V4 protocol
type RaydiumAmmV4InstructionBuilder struct{}

// NewRaydiumAmmV4InstructionBuilder creates a new Raydium AMM V4 instruction builder
func NewRaydiumAmmV4InstructionBuilder() *RaydiumAmmV4InstructionBuilder {
	return &RaydiumAmmV4InstructionBuilder{}
}

// Raydium AMM V4 discriminators
var (
	RAYDIUM_AMM_V4_SWAP_DISCRIMINATOR = []byte{248, 198, 158, 145, 225, 117, 135, 200}
)

// BuildBuyInstructions builds buy instructions for Raydium AMM V4
func (b *RaydiumAmmV4InstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumAmmV4Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Create ATA if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.TokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RAYDIUM_AMM_V4_SWAP_DISCRIMINATOR)
	copy(data[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	copy(data[16:24], uint64ToLEBytes(minAmountOut)[:])

	// Build accounts (17 accounts for Raydium AMM V4 swap)
	accounts := []solana.AccountMeta{
		{PublicKey: protocolParams.Amm, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.AmmAuthority, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.AmmOpenOrders, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.AmmTargetOrders, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.PoolCoinTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.PoolPcTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.SerumMarket, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumBids, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumAsks, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumEventQueue, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumCoinVaultAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumPcVaultAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.SerumVaultSigner, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.UserSourceTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.UserDestinationTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: false},
	}

	swapIx := solana.NewInstruction(constants.RAYDIUM_AMM_V4_PROGRAM_ID, accounts, data)
	instructions = append(instructions, swapIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for Raydium AMM V4
func (b *RaydiumAmmV4InstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	// Same as buy but with swapped input/output
	return b.BuildBuyInstructions(bp)
}

// ===== Meteora DAMM V2 Instruction Builder =====

// MeteoraDammV2InstructionBuilder builds instructions for Meteora DAMM V2 protocol
type MeteoraDammV2InstructionBuilder struct{}

// NewMeteoraDammV2InstructionBuilder creates a new Meteora DAMM V2 instruction builder
func NewMeteoraDammV2InstructionBuilder() *MeteoraDammV2InstructionBuilder {
	return &MeteoraDammV2InstructionBuilder{}
}

// Meteora DAMM V2 discriminators
var (
	METEORA_DAMM_V2_SWAP_DISCRIMINATOR = []byte{248, 198, 158, 145, 225, 117, 135, 200}
)

// BuildBuyInstructions builds buy instructions for Meteora DAMM V2
func (b *MeteoraDammV2InstructionBuilder) BuildBuyInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	protocolParams, ok := bp.ProtocolParams.(*params.MeteoraDammV2Params)
	if !ok {
		return nil, soltradesdk.ErrInvalidProtocolParams
	}

	if bp.InputAmount == 0 {
		return nil, soltradesdk.ErrInvalidAmount
	}

	var instructions []solana.Instruction

	// Create ATA if needed
	if bp.CreateOutputATA {
		createATAIx := CreateAssociatedTokenAccountInstruction(
			bp.Payer,
			bp.Payer,
			bp.OutputMint,
			protocolParams.TokenAProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], METEORA_DAMM_V2_SWAP_DISCRIMINATOR)
	copy(data[8:16], uint64ToLEBytes(bp.InputAmount)[:])
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	copy(data[16:24], uint64ToLEBytes(minAmountOut)[:])

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: protocolParams.Pool, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenAVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenBVault, IsSigner: false, IsWritable: true},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.UserSourceTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.UserDestinationTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenAProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.TokenBProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	swapIx := solana.NewInstruction(constants.METEORA_DAMM_V2_PROGRAM_ID, accounts, data)
	instructions = append(instructions, swapIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for Meteora DAMM V2
func (b *MeteoraDammV2InstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	// Same as buy but with swapped input/output
	return b.BuildBuyInstructions(bp)
}
