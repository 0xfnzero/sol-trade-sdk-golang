package instruction

import (
	"fmt"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
	"github.com/gagliardetto/solana-go"
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
		putUint64LE(buyData[8:16], bp.InputAmount)
		// Min tokens out (with slippage)
		minTokensOut := calculateMinOutput(0, bp.SlippageBasisPoints) // Simplified
		putUint64LE(buyData[16:24], minTokensOut)
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
		putUint64LE(buyData[8:16], 0) // Simplified
		// Max SOL cost
		maxSolCost := calculateMaxCost(bp.InputAmount, bp.SlippageBasisPoints)
		putUint64LE(buyData[16:24], maxSolCost)
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

	buyIx := newInstruction(
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
	putUint64LE(sellData[8:16], bp.InputAmount)
	minSolOutput := calculateMinOutput(0, bp.SlippageBasisPoints) // Simplified
	putUint64LE(sellData[16:24], minSolOutput)

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

	sellIx := newInstruction(
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

	return newInstruction(constants.PUMPSWAP_PROGRAM_ID, accounts, data)
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

func putUint64LE(dst []byte, v uint64) {
	leBytes := uint64ToLEBytes(v)
	copy(dst, leBytes[:])
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
			constants.TOKEN_PROGRAM,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], BONK_BUY_DISCRIMINATOR)
	putUint64LE(data[8:16], bp.InputAmount)
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	putUint64LE(data[16:24], minAmountOut)

	userBaseTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, protocolParams.MintTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, constants.TOKEN_PROGRAM)

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
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	buyIx := newInstruction(constants.BONK_PROGRAM_ID, accounts, data)
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
	putUint64LE(data[8:16], bp.InputAmount)
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	putUint64LE(data[16:24], minAmountOut)

	userBaseTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, protocolParams.MintTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, constants.TOKEN_PROGRAM)

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
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	sellIx := newInstruction(constants.BONK_PROGRAM_ID, accounts, data)
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
	protocolParams, ok := bp.ProtocolParams.(*params.RaydiumCpmmParams)
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
			protocolParams.QuoteTokenProgram,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RAYDIUM_CPMM_SWAP_DISCRIMINATOR)
	putUint64LE(data[8:16], bp.InputAmount)
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	putUint64LE(data[16:24], minAmountOut)

	inputTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, protocolParams.BaseTokenProgram)
	outputTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, protocolParams.QuoteTokenProgram)

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: protocolParams.AmmConfig, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.PoolState, IsSigner: false, IsWritable: true},
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.BaseVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.QuoteVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.BaseTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.QuoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.ObservationState, IsSigner: false, IsWritable: true},
	}

	swapIx := newInstruction(constants.RAYDIUM_CPMM_PROGRAM_ID, accounts, data)
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
			constants.TOKEN_PROGRAM,
		)
		instructions = append(instructions, createATAIx)
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RAYDIUM_AMM_V4_SWAP_DISCRIMINATOR)
	putUint64LE(data[8:16], bp.InputAmount)
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	putUint64LE(data[16:24], minAmountOut)

	userSourceTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, constants.TOKEN_PROGRAM)
	userDestinationTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, constants.TOKEN_PROGRAM)

	// Build accounts for Raydium AMM V4 swap.
	accounts := []solana.AccountMeta{
		{PublicKey: protocolParams.Amm, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenCoin, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenPc, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.CoinMint, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.PcMint, IsSigner: false, IsWritable: false},
		{PublicKey: userSourceTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userDestinationTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: false},
	}

	swapIx := newInstruction(constants.RAYDIUM_AMM_V4_PROGRAM_ID, accounts, data)
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
	putUint64LE(data[8:16], bp.InputAmount)
	minAmountOut := calculateMinOutput(0, bp.SlippageBasisPoints)
	putUint64LE(data[16:24], minAmountOut)

	userSourceTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.InputMint, protocolParams.TokenAProgram)
	userDestinationTokenAccount := GetAssociatedTokenAddress(bp.Payer, bp.OutputMint, protocolParams.TokenBProgram)

	// Build accounts
	accounts := []solana.AccountMeta{
		{PublicKey: bp.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: protocolParams.Pool, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenAVault, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenBVault, IsSigner: false, IsWritable: true},
		{PublicKey: bp.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bp.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: userSourceTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userDestinationTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: protocolParams.TokenAProgram, IsSigner: false, IsWritable: false},
		{PublicKey: protocolParams.TokenBProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
	}

	swapIx := newInstruction(constants.METEORA_DAMM_V2_PROGRAM_ID, accounts, data)
	instructions = append(instructions, swapIx)

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for Meteora DAMM V2
func (b *MeteoraDammV2InstructionBuilder) BuildSellInstructions(bp *BuildParams) ([]solana.Instruction, error) {
	// Same as buy but with swapped input/output
	return b.BuildBuyInstructions(bp)
}
