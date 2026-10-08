// Meteora DAMM V2 instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
)

// ===== Meteora DAMM V2 Program Constants from Rust: src/instruction/utils/meteora_damm_v2.rs =====

var (
	// METEORA_DAMM_V2_PROGRAM is the Meteora DAMM V2 program ID
	METEORA_DAMM_V2_PROGRAM = solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")
	// METEORA_DAMM_V2_AUTHORITY is the program authority
	METEORA_DAMM_V2_AUTHORITY = solana.MustPublicKeyFromBase58("HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC")
)

// Discriminators - from Rust: src/instruction/utils/meteora_damm_v2.rs
var (
	// MeteoraDammV2SwapDiscriminator is the discriminator for swap instruction
	MeteoraDammV2SwapDiscriminator = []byte{248, 198, 158, 145, 225, 117, 135, 200}
	// MeteoraDammV2Swap2Discriminator is the discriminator for swap2 instruction
	MeteoraDammV2Swap2Discriminator = []byte{65, 75, 63, 76, 235, 91, 91, 136}
)

const (
	MeteoraDammV2SwapModeExactIn     uint8 = 0
	MeteoraDammV2SwapModePartialFill uint8 = 1
	MeteoraDammV2SwapModeExactOut    uint8 = 2
)

var (
	METEORA_DAMM_V2_SYSVAR_INSTRUCTIONS = solana.MustPublicKeyFromBase58("Sysvar1nstructions1111111111111111111111111")
)

// Seeds - from Rust: src/instruction/utils/meteora_damm_v2.rs seeds
var (
	MeteoraDammV2EventAuthoritySeed = []byte("__event_authority")
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetMeteoraDammV2EventAuthorityPDA returns the event authority PDA
func GetMeteoraDammV2EventAuthorityPDA() solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{MeteoraDammV2EventAuthoritySeed},
		METEORA_DAMM_V2_PROGRAM,
	)
	return pda
}

// ===== Meteora DAMM V2 Params =====

// MeteoraDammV2Params contains parameters for Meteora DAMM V2 operations
type MeteoraDammV2Params struct {
	Pool                     solana.PublicKey
	TokenAMint               solana.PublicKey
	TokenBMint               solana.PublicKey
	TokenAVault              solana.PublicKey
	TokenBVault              solana.PublicKey
	TokenAProgram            solana.PublicKey
	TokenBProgram            solana.PublicKey
	TokenAReserve            uint64
	TokenBReserve            uint64
	ReferralTokenAccount     *solana.PublicKey
	SwapMode                 *uint8
	IncludeRateLimiterSysvar bool
}

func (p *MeteoraDammV2Params) resolvedSwapMode() uint8 {
	if p == nil || p.SwapMode == nil {
		return MeteoraDammV2SwapModePartialFill
	}
	mode := *p.SwapMode
	return mode
}

func meteoraDammV2ResolveAmounts(swapMode uint8, amountIn uint64, fixedOutput uint64) (uint64, uint64, error) {
	switch swapMode {
	case MeteoraDammV2SwapModeExactOut:
		return fixedOutput, amountIn, nil
	case MeteoraDammV2SwapModeExactIn, MeteoraDammV2SwapModePartialFill:
		return amountIn, fixedOutput, nil
	default:
		return 0, 0, fmt.Errorf("unsupported MeteoraDammV2 swap_mode %d", swapMode)
	}
}

func meteoraDammV2BuildAccountMetas(
	pp *MeteoraDammV2Params,
	payer solana.PublicKey,
	inputTokenAccount solana.PublicKey,
	outputTokenAccount solana.PublicKey,
	eventAuthority solana.PublicKey,
) []solana.AccountMeta {
	accounts := []solana.AccountMeta{
		{PublicKey: METEORA_DAMM_V2_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: pp.Pool, IsSigner: false, IsWritable: true},
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: pp.TokenAVault, IsSigner: false, IsWritable: true},
		{PublicKey: pp.TokenBVault, IsSigner: false, IsWritable: true},
		{PublicKey: pp.TokenAMint, IsSigner: false, IsWritable: false},
		{PublicKey: pp.TokenBMint, IsSigner: false, IsWritable: false},
		{PublicKey: payer, IsSigner: true, IsWritable: false},
		{PublicKey: pp.TokenAProgram, IsSigner: false, IsWritable: false},
		{PublicKey: pp.TokenBProgram, IsSigner: false, IsWritable: false},
	}
	if pp.ReferralTokenAccount != nil {
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: *pp.ReferralTokenAccount, IsSigner: false, IsWritable: true,
		})
	} else {
		accounts = append(accounts, solana.AccountMeta{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false})
	}
	accounts = append(accounts,
		solana.AccountMeta{PublicKey: eventAuthority, IsSigner: false, IsWritable: false},
		solana.AccountMeta{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false},
	)
	if pp.IncludeRateLimiterSysvar {
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: METEORA_DAMM_V2_SYSVAR_INSTRUCTIONS, IsSigner: false, IsWritable: false,
		})
	}
	return accounts
}

// MeteoraDammV2BuildBuyParams contains parameters for building buy instructions
type MeteoraDammV2BuildBuyParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *MeteoraDammV2Params
	CreateInputMintAta  bool
	CreateOutputMintAta bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// MeteoraDammV2BuildSellParams contains parameters for building sell instructions
type MeteoraDammV2BuildSellParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *MeteoraDammV2Params
	CreateOutputMintAta bool
	CloseOutputMintAta  bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== Instruction Builders - 100% from Rust =====

func meteoraDammV2MintMatches(requested, expected solana.PublicKey) bool {
	return requested.Equals(expected) ||
		(expected.Equals(constants.WSOL_TOKEN_ACCOUNT) && requested.Equals(constants.SOL_TOKEN_ACCOUNT))
}

func ensureMeteoraDammV2ExpectedMint(label string, requested, expected solana.PublicKey) error {
	if !requested.IsZero() && !meteoraDammV2MintMatches(requested, expected) {
		return fmt.Errorf("%s must match the Meteora DAMM v2 pool side (%s), got %s", label, expected.String(), requested.String())
	}
	return nil
}

// MeteoraDammV2BuildBuyInstructions builds buy instructions for Meteora DAMM V2
// 100% port from Rust: src/instruction/meteora_damm_v2.rs build_buy_instructions
func MeteoraDammV2BuildBuyInstructions(params *MeteoraDammV2BuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if pool contains WSOL or USDC
	isWsol := pp.TokenAMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.TokenBMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.TokenAMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.TokenBMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Determine if token A is input (WSOL/USDC)
	isAIn := pp.TokenAMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.TokenAMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !params.OutputMint.IsZero() {
		isAIn = meteoraDammV2MintMatches(params.OutputMint, pp.TokenBMint)
	}

	// Calculate swap2 amounts
	amountIn := params.InputAmount
	var fixedOutput uint64
	if params.FixedOutputAmount != nil {
		fixedOutput = *params.FixedOutputAmount
	} else {
		return nil, fmt.Errorf("fixed_output_amount must be set for MeteoraDammV2 swap")
	}
	swapMode := pp.resolvedSwapMode()
	amount0, amount1, err := meteoraDammV2ResolveAmounts(swapMode, amountIn, fixedOutput)
	if err != nil {
		return nil, err
	}

	inputMint := pp.TokenBMint
	outputMint := pp.TokenAMint
	inputTokenProgram := pp.TokenBProgram
	outputTokenProgram := pp.TokenAProgram
	if isAIn {
		inputMint = pp.TokenAMint
		outputMint = pp.TokenBMint
		inputTokenProgram = pp.TokenAProgram
		outputTokenProgram = pp.TokenBProgram
	}
	if err := ensureMeteoraDammV2ExpectedMint("InputMint", params.InputMint, inputMint); err != nil {
		return nil, err
	}
	if err := ensureMeteoraDammV2ExpectedMint("OutputMint", params.OutputMint, outputMint); err != nil {
		return nil, err
	}

	// Get user token accounts
	inputTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, inputTokenProgram)
	outputTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, outputTokenProgram)

	// Get event authority
	eventAuthority := GetMeteoraDammV2EventAuthorityPDA()

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle input account creation/wrapping
	if params.CreateInputMintAta {
		if inputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
			instructions = append(instructions, HandleWsol(params.Payer, amountIn)...)
		} else {
			instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
				params.Payer, params.Payer, inputMint, inputTokenProgram,
			))
		}
	}

	// Create output ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, outputTokenProgram,
		))
	}

	// Build swap2 instruction data
	data := make([]byte, 25)
	copy(data[0:8], MeteoraDammV2Swap2Discriminator)
	binary.LittleEndian.PutUint64(data[8:16], amount0)
	binary.LittleEndian.PutUint64(data[16:24], amount1)
	data[24] = swapMode

	accounts := meteoraDammV2BuildAccountMetas(pp, params.Payer, inputTokenAccount, outputTokenAccount, eventAuthority)

	instructions = append(instructions, newInstruction(METEORA_DAMM_V2_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta && inputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	return instructions, nil
}

// MeteoraDammV2BuildSellInstructions builds sell instructions for Meteora DAMM V2
// 100% port from Rust: src/instruction/meteora_damm_v2.rs build_sell_instructions
func MeteoraDammV2BuildSellInstructions(params *MeteoraDammV2BuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if pool contains WSOL or USDC
	isWsol := pp.TokenAMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.TokenBMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.TokenAMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.TokenBMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Determine if token A is input (token being sold)
	isAIn := pp.TokenBMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.TokenBMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !params.InputMint.IsZero() {
		isAIn = meteoraDammV2MintMatches(params.InputMint, pp.TokenAMint)
	}

	// Calculate swap2 amounts
	var fixedOutput uint64
	if params.FixedOutputAmount != nil {
		fixedOutput = *params.FixedOutputAmount
	} else {
		return nil, fmt.Errorf("fixed_output_amount must be set for MeteoraDammV2 swap")
	}
	swapMode := pp.resolvedSwapMode()
	amount0, amount1, err := meteoraDammV2ResolveAmounts(swapMode, params.InputAmount, fixedOutput)
	if err != nil {
		return nil, err
	}

	inputMint := pp.TokenBMint
	outputMint := pp.TokenAMint
	inputTokenProgram := pp.TokenBProgram
	outputTokenProgram := pp.TokenAProgram
	if isAIn {
		inputMint = pp.TokenAMint
		outputMint = pp.TokenBMint
		inputTokenProgram = pp.TokenAProgram
		outputTokenProgram = pp.TokenBProgram
	}
	if err := ensureMeteoraDammV2ExpectedMint("InputMint", params.InputMint, inputMint); err != nil {
		return nil, err
	}
	if err := ensureMeteoraDammV2ExpectedMint("OutputMint", params.OutputMint, outputMint); err != nil {
		return nil, err
	}

	// Get user token accounts
	inputTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, inputTokenProgram)
	outputTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, outputTokenProgram)

	// Get event authority
	eventAuthority := GetMeteoraDammV2EventAuthorityPDA()

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL ATA for receiving if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, outputTokenProgram,
		))
	}

	// Build swap2 instruction data
	data := make([]byte, 25)
	copy(data[0:8], MeteoraDammV2Swap2Discriminator)
	binary.LittleEndian.PutUint64(data[8:16], amount0)
	binary.LittleEndian.PutUint64(data[16:24], amount1)
	data[24] = swapMode

	accounts := meteoraDammV2BuildAccountMetas(pp, params.Payer, inputTokenAccount, outputTokenAccount, eventAuthority)

	instructions = append(instructions, newInstruction(METEORA_DAMM_V2_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta && outputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	// Close input token account if requested
	if params.CloseInputMintAta {
		closeIx := token.NewCloseAccountInstruction(
			inputTokenAccount,
			params.Payer,
			params.Payer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

// Meteora DAMM V2 error definitions
var (
	ErrMeteoraDammV2InvalidPool = fmt.Errorf("meteora damm v2: invalid pool configuration")
)

// ===== Pool Types and Decoder - from Rust: src/instruction/utils/meteora_damm_v2_types.rs =====

// MeteoraPoolSize is the size of a Meteora DAMM V2 pool
const MeteoraPoolSize = 1104

// MeteoraDammV2Pool represents a simplified Meteora DAMM V2 pool
type MeteoraBaseFeeStruct struct {
	CliffFeeNumerator uint64
	FeeSchedulerMode  uint8
	Padding0          [5]byte
	NumberOfPeriod    uint16
	PeriodFrequency   uint64
	ReductionFactor   uint64
	Padding1          uint64
}

type MeteoraDynamicFeeStruct struct {
	Initialized              uint8
	Padding                  [7]byte
	MaxVolatilityAccumulator uint32
	VariableFeeControl       uint32
	BinStep                  uint16
	FilterPeriod             uint16
	DecayPeriod              uint16
	ReductionFactor          uint16
	LastUpdateTimestamp      uint64
	BinStepU128              [16]byte
	SqrtPriceReference       [16]byte
	VolatilityAccumulator    [16]byte
	VolatilityReference      [16]byte
}

type MeteoraPoolFeesStruct struct {
	// Current meanings; old padding fields remain raw compatibility overlays.
	CompoundingFeeBps  uint16
	InitSqrtPrice      [16]byte
	BaseFee            MeteoraBaseFeeStruct
	ProtocolFeePercent uint8
	PartnerFeePercent  uint8
	ReferralFeePercent uint8
	Padding0           [5]byte
	DynamicFee         MeteoraDynamicFeeStruct
	Padding1           [2]uint64
}

type MeteoraPoolMetrics struct {
	TotalLpAFee       [16]byte
	TotalLpBFee       [16]byte
	TotalProtocolAFee uint64
	TotalProtocolBFee uint64
	TotalPartnerAFee  uint64
	TotalPartnerBFee  uint64
	TotalPosition     uint64
	Padding           uint64
}

type MeteoraRewardInfo struct {
	Initialized                               uint8
	RewardTokenFlag                           uint8
	Padding0                                  [6]byte
	Padding1                                  [8]byte
	Mint                                      solana.PublicKey
	Vault                                     solana.PublicKey
	Funder                                    solana.PublicKey
	RewardDuration                            uint64
	RewardDurationEnd                         uint64
	RewardRate                                [16]byte
	RewardPerTokenStored                      [32]byte
	LastUpdateTime                            uint64
	CumulativeSecondsWithEmptyLiquidityReward uint64
}

type MeteoraDammV2Pool struct {
	DeadLiquidityFeeCheckpoint uint64
	FeeVersion                 uint8
	Creator                    solana.PublicKey
	TokenAAmount               uint64
	TokenBAmount               uint64
	LayoutVersion              uint8
	PoolFees                   MeteoraPoolFeesStruct
	TokenAMint                 solana.PublicKey
	TokenBMint                 solana.PublicKey
	TokenAVault                solana.PublicKey
	TokenBVault                solana.PublicKey
	WhitelistedVault           solana.PublicKey
	Partner                    solana.PublicKey
	Liquidity                  [16]byte
	Padding                    [16]byte
	ProtocolAFee               uint64
	ProtocolBFee               uint64
	PartnerAFee                uint64
	PartnerBFee                uint64
	SqrtMinPrice               [16]byte
	SqrtMaxPrice               [16]byte
	SqrtPrice                  [16]byte
	ActivationPoint            uint64
	ActivationType             uint8
	PoolStatus                 uint8
	TokenAFlag                 uint8
	TokenBFlag                 uint8
	CollectFeeMode             uint8
	PoolType                   uint8
	Padding0                   [2]byte
	FeeAPerLiquidity           [32]byte
	FeeBPerLiquidity           [32]byte
	PermanentLockLiquidity     [16]byte
	Metrics                    MeteoraPoolMetrics
	Padding1                   [10]uint64
	RewardInfos                [2]MeteoraRewardInfo
}

func DecodeMeteoraPool(data []byte) *MeteoraDammV2Pool {
	if len(data) < MeteoraPoolSize {
		return nil
	}
	offset := 0
	take := func(size int) []byte { v := data[offset : offset+size]; offset += size; return v }
	readBaseFeeStruct := func() MeteoraBaseFeeStruct {
		var value MeteoraBaseFeeStruct
		value.CliffFeeNumerator = binary.LittleEndian.Uint64(take(8))
		value.FeeSchedulerMode = take(1)[0]
		copy(value.Padding0[:], take(5))
		value.NumberOfPeriod = binary.LittleEndian.Uint16(take(2))
		value.PeriodFrequency = binary.LittleEndian.Uint64(take(8))
		value.ReductionFactor = binary.LittleEndian.Uint64(take(8))
		value.Padding1 = binary.LittleEndian.Uint64(take(8))
		return value
	}
	readDynamicFeeStruct := func() MeteoraDynamicFeeStruct {
		var value MeteoraDynamicFeeStruct
		value.Initialized = take(1)[0]
		copy(value.Padding[:], take(7))
		value.MaxVolatilityAccumulator = binary.LittleEndian.Uint32(take(4))
		value.VariableFeeControl = binary.LittleEndian.Uint32(take(4))
		value.BinStep = binary.LittleEndian.Uint16(take(2))
		value.FilterPeriod = binary.LittleEndian.Uint16(take(2))
		value.DecayPeriod = binary.LittleEndian.Uint16(take(2))
		value.ReductionFactor = binary.LittleEndian.Uint16(take(2))
		value.LastUpdateTimestamp = binary.LittleEndian.Uint64(take(8))
		copy(value.BinStepU128[:], take(16))
		copy(value.SqrtPriceReference[:], take(16))
		copy(value.VolatilityAccumulator[:], take(16))
		copy(value.VolatilityReference[:], take(16))
		return value
	}
	readPoolFeesStruct := func() MeteoraPoolFeesStruct {
		var value MeteoraPoolFeesStruct
		value.BaseFee = readBaseFeeStruct()
		value.ProtocolFeePercent = take(1)[0]
		value.PartnerFeePercent = take(1)[0]
		value.ReferralFeePercent = take(1)[0]
		copy(value.Padding0[:], take(5))
		value.DynamicFee = readDynamicFeeStruct()
		for i := range value.Padding1 {
			value.Padding1[i] = binary.LittleEndian.Uint64(take(8))
		}
		return value
	}
	readPoolMetrics := func() MeteoraPoolMetrics {
		var value MeteoraPoolMetrics
		copy(value.TotalLpAFee[:], take(16))
		copy(value.TotalLpBFee[:], take(16))
		value.TotalProtocolAFee = binary.LittleEndian.Uint64(take(8))
		value.TotalProtocolBFee = binary.LittleEndian.Uint64(take(8))
		value.TotalPartnerAFee = binary.LittleEndian.Uint64(take(8))
		value.TotalPartnerBFee = binary.LittleEndian.Uint64(take(8))
		value.TotalPosition = binary.LittleEndian.Uint64(take(8))
		value.Padding = binary.LittleEndian.Uint64(take(8))
		return value
	}
	readRewardInfo := func() MeteoraRewardInfo {
		var value MeteoraRewardInfo
		value.Initialized = take(1)[0]
		value.RewardTokenFlag = take(1)[0]
		copy(value.Padding0[:], take(6))
		copy(value.Padding1[:], take(8))
		copy(value.Mint[:], take(32))
		copy(value.Vault[:], take(32))
		copy(value.Funder[:], take(32))
		value.RewardDuration = binary.LittleEndian.Uint64(take(8))
		value.RewardDurationEnd = binary.LittleEndian.Uint64(take(8))
		copy(value.RewardRate[:], take(16))
		copy(value.RewardPerTokenStored[:], take(32))
		value.LastUpdateTime = binary.LittleEndian.Uint64(take(8))
		value.CumulativeSecondsWithEmptyLiquidityReward = binary.LittleEndian.Uint64(take(8))
		return value
	}
	readPool := func() MeteoraDammV2Pool {
		var value MeteoraDammV2Pool
		value.PoolFees = readPoolFeesStruct()
		copy(value.TokenAMint[:], take(32))
		copy(value.TokenBMint[:], take(32))
		copy(value.TokenAVault[:], take(32))
		copy(value.TokenBVault[:], take(32))
		copy(value.WhitelistedVault[:], take(32))
		copy(value.Partner[:], take(32))
		copy(value.Liquidity[:], take(16))
		copy(value.Padding[:], take(16))
		value.ProtocolAFee = binary.LittleEndian.Uint64(take(8))
		value.ProtocolBFee = binary.LittleEndian.Uint64(take(8))
		value.PartnerAFee = binary.LittleEndian.Uint64(take(8))
		value.PartnerBFee = binary.LittleEndian.Uint64(take(8))
		copy(value.SqrtMinPrice[:], take(16))
		copy(value.SqrtMaxPrice[:], take(16))
		copy(value.SqrtPrice[:], take(16))
		value.ActivationPoint = binary.LittleEndian.Uint64(take(8))
		value.ActivationType = take(1)[0]
		value.PoolStatus = take(1)[0]
		value.TokenAFlag = take(1)[0]
		value.TokenBFlag = take(1)[0]
		value.CollectFeeMode = take(1)[0]
		value.PoolType = take(1)[0]
		copy(value.Padding0[:], take(2))
		copy(value.FeeAPerLiquidity[:], take(32))
		copy(value.FeeBPerLiquidity[:], take(32))
		copy(value.PermanentLockLiquidity[:], take(16))
		value.Metrics = readPoolMetrics()
		for i := range value.Padding1 {
			value.Padding1[i] = binary.LittleEndian.Uint64(take(8))
		}
		for i := range value.RewardInfos {
			value.RewardInfos[i] = readRewardInfo()
		}
		return value
	}
	value := readPool()
	value.PoolFees.CompoundingFeeBps = binary.LittleEndian.Uint16(data[46:48])
	copy(value.PoolFees.InitSqrtPrice[:], data[144:160])
	value.DeadLiquidityFeeCheckpoint = binary.LittleEndian.Uint64(data[400:408])
	value.FeeVersion = data[478]
	copy(value.Creator[:], data[640:672])
	value.TokenAAmount = binary.LittleEndian.Uint64(data[672:680])
	value.TokenBAmount = binary.LittleEndian.Uint64(data[680:688])
	value.LayoutVersion = data[688]
	return &value
}

// ===== Async Fetch Functions - from Rust: src/instruction/utils/meteora_damm_v2.rs =====

// MeteoraPoolFetcher defines interface for fetching pool data from RPC
type MeteoraPoolFetcher interface {
	GetAccountInfo(pubkey solana.PublicKey) ([]byte, error)
}

// FetchMeteoraPool fetches a Meteora DAMM V2 pool from RPC.
// 100% from Rust: src/instruction/utils/meteora_damm_v2.rs fetch_pool
func FetchMeteoraPool(fetcher MeteoraPoolFetcher, poolAddress solana.PublicKey) (*MeteoraDammV2Pool, error) {
	data, err := fetcher.GetAccountInfo(poolAddress)
	if err != nil {
		return nil, err
	}
	if len(data) < 8+MeteoraPoolSize {
		return nil, fmt.Errorf("account data too short")
	}

	if !bytes.Equal(data[:8], []byte{241, 154, 109, 4, 17, 177, 109, 188}) {
		return nil, fmt.Errorf("meteora pool discriminator mismatch")
	}
	pool := DecodeMeteoraPool(data[8:])
	if pool == nil {
		return nil, fmt.Errorf("failed to decode meteora pool")
	}
	return pool, nil
}
