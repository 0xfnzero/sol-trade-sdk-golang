// Raydium CPMM instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/your-org/sol-trade-sdk-go/pkg/calc"
	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
)

// ===== Raydium CPMM Program Constants from Rust: src/instruction/utils/raydium_cpmm.rs =====

var (
	// RAYDIUM_CPMM_PROGRAM is the Raydium CPMM program ID
	RAYDIUM_CPMM_PROGRAM = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")
	// RAYDIUM_CPMM_AUTHORITY is the program authority
	RAYDIUM_CPMM_AUTHORITY = solana.MustPublicKeyFromBase58("GpMZbSM2GgvTKHJirzeGfMFoaZ8UR2X7F4v8vHTvxFbL")
)

// Discriminators - from Rust: src/instruction/utils/raydium_cpmm.rs
var (
	// RaydiumCPMMSwapBaseInDiscriminator is the discriminator for swap_base_in instruction
	RaydiumCPMMSwapBaseInDiscriminator = []byte{143, 190, 90, 218, 196, 30, 51, 222}
	// RaydiumCPMMSwapBaseOutDiscriminator is the discriminator for swap_base_out instruction
	RaydiumCPMMSwapBaseOutDiscriminator = []byte{55, 217, 98, 86, 163, 74, 180, 173}
)

// Seeds - from Rust: src/instruction/utils/raydium_cpmm.rs seeds
var (
	RaydiumCPMMPoolSeed             = []byte("pool")
	RaydiumCPMMPoolVaultSeed        = []byte("pool_vault")
	RaydiumCPMMObservationStateSeed = []byte("observation")
)

// Raydium CPMM Constants - from Rust: src/instruction/utils/raydium_cpmm.rs accounts
const (
	RaydiumCPMMFeeRateDenominator uint64 = 1_000_000
	RaydiumCPMMTradeFeeRate       uint64 = 2500
	RaydiumCPMMCreatorFeeRate     uint64 = 0
	RaydiumCPMMProtocolFeeRate    uint64 = 120000
	RaydiumCPMMFundFeeRate        uint64 = 40000
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetRaydiumCPMMPoolPDA returns the pool PDA (seeds: ["pool", amm_config, mint1, mint2])
func GetRaydiumCPMMPoolPDA(ammConfig, mint1, mint2 solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{RaydiumCPMMPoolSeed, ammConfig[:], mint1[:], mint2[:]},
		RAYDIUM_CPMM_PROGRAM,
	)
	return pda
}

// GetRaydiumCPMMVaultPDA returns the vault PDA (seeds: ["pool_vault", pool_state, mint])
func GetRaydiumCPMMVaultPDA(poolState, mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{RaydiumCPMMPoolVaultSeed, poolState[:], mint[:]},
		RAYDIUM_CPMM_PROGRAM,
	)
	return pda
}

// GetRaydiumCPMMObservationStatePDA returns the observation state PDA
func GetRaydiumCPMMObservationStatePDA(poolState solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{RaydiumCPMMObservationStateSeed, poolState[:]},
		RAYDIUM_CPMM_PROGRAM,
	)
	return pda
}

// GetRaydiumCPMMVaultAccount returns the vault account based on params
func GetRaydiumCPMMVaultAccount(poolState, tokenMint solana.PublicKey, params *RaydiumCPMMParams) solana.PublicKey {
	if params.BaseMint.Equals(tokenMint) && !params.BaseVault.IsZero() {
		return params.BaseVault
	}
	if params.QuoteMint.Equals(tokenMint) && !params.QuoteVault.IsZero() {
		return params.QuoteVault
	}
	return GetRaydiumCPMMVaultPDA(poolState, tokenMint)
}

// ===== Raydium CPMM Params =====

// RaydiumCPMMParams contains parameters for Raydium CPMM operations
type RaydiumCPMMParams struct {
	PoolState         solana.PublicKey
	AmmConfig         solana.PublicKey
	BaseMint          solana.PublicKey
	QuoteMint         solana.PublicKey
	BaseVault         solana.PublicKey
	QuoteVault        solana.PublicKey
	BaseReserve       uint64
	QuoteReserve      uint64
	BaseTokenProgram  solana.PublicKey
	QuoteTokenProgram solana.PublicKey
	ObservationState  solana.PublicKey
}

// RaydiumCPMMBuildBuyParams contains parameters for building buy instructions
type RaydiumCPMMBuildBuyParams struct {
	Payer               solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *RaydiumCPMMParams
	CreateInputMintAta  bool
	CreateOutputMintAta bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// RaydiumCPMMBuildSellParams contains parameters for building sell instructions
type RaydiumCPMMBuildSellParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *RaydiumCPMMParams
	CreateOutputMintAta bool
	CloseOutputMintAta  bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== Instruction Builders - 100% from Rust =====

// RaydiumCPMMBuildBuyInstructions builds buy instructions for Raydium CPMM
// 100% port from Rust: src/instruction/raydium_cpmm.rs build_buy_instructions
func RaydiumCPMMBuildBuyInstructions(params *RaydiumCPMMBuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Get pool state
	poolState := pp.PoolState
	if poolState.IsZero() {
		poolState = GetRaydiumCPMMPoolPDA(pp.AmmConfig, pp.BaseMint, pp.QuoteMint)
	}

	// Check if pool contains WSOL or USDC
	isWsol := pp.BaseMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.BaseMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Determine if base is input (WSOL/USDC)
	isBaseIn := pp.BaseMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.BaseMint.Equals(constants.USDC_TOKEN_ACCOUNT)

	// Get mint token program for output
	mintTokenProgram := pp.QuoteTokenProgram
	if isBaseIn {
		mintTokenProgram = pp.QuoteTokenProgram
	} else {
		mintTokenProgram = pp.BaseTokenProgram
	}

	// Calculate swap amounts
	amountIn := params.InputAmount
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		result := calc.RaydiumCPMMGetAmountOut(amountIn, pp.BaseReserve, pp.QuoteReserve, isBaseIn)
		minAmountOut, _ := calc.CalculateWithSlippageSell(result, params.SlippageBasisPoints)
		minimumAmountOut = minAmountOut
	}

	// Determine input/output mints
	inputMint := constants.WSOL_TOKEN_ACCOUNT
	if isUsdc {
		inputMint = constants.USDC_TOKEN_ACCOUNT
	}
	outputMint := params.OutputMint

	// Get user token accounts
	inputTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, constants.TOKEN_PROGRAM)
	outputTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, mintTokenProgram)

	// Get vault accounts
	inputVaultAccount := GetRaydiumCPMMVaultAccount(poolState, inputMint, pp)
	outputVaultAccount := GetRaydiumCPMMVaultAccount(poolState, outputMint, pp)

	// Get observation state
	observationStateAccount := pp.ObservationState
	if observationStateAccount.IsZero() {
		observationStateAccount = GetRaydiumCPMMObservationStatePDA(poolState)
	}

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	if params.CreateInputMintAta {
		instructions = append(instructions, HandleWsol(params.Payer, amountIn)...)
	}

	// Create output ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, mintTokenProgram,
		))
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RaydiumCPMMSwapBaseInDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], amountIn)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)

	// Build accounts array (13 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},              // 0: Payer (signer)
		{PublicKey: RAYDIUM_CPMM_AUTHORITY, IsSigner: false, IsWritable: false},  // 1: Authority (readonly)
		{PublicKey: pp.AmmConfig, IsSigner: false, IsWritable: false},            // 2: Amm Config (readonly)
		{PublicKey: poolState, IsSigner: false, IsWritable: true},                // 3: Pool State
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},        // 4: Input Token Account
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},       // 5: Output Token Account
		{PublicKey: inputVaultAccount, IsSigner: false, IsWritable: true},        // 6: Input Vault Account
		{PublicKey: outputVaultAccount, IsSigner: false, IsWritable: true},       // 7: Output Vault Account
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false}, // 8: Input Token Program (readonly)
		{PublicKey: mintTokenProgram, IsSigner: false, IsWritable: false},        // 9: Output Token Program (readonly)
		{PublicKey: inputMint, IsSigner: false, IsWritable: false},               // 10: Input token mint (readonly)
		{PublicKey: outputMint, IsSigner: false, IsWritable: false},              // 11: Output token mint (readonly)
		{PublicKey: observationStateAccount, IsSigner: false, IsWritable: true},  // 12: Observation State Account
	}

	instructions = append(instructions, solana.NewInstruction(RAYDIUM_CPMM_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	return instructions, nil
}

// RaydiumCPMMBuildSellInstructions builds sell instructions for Raydium CPMM
// 100% port from Rust: src/instruction/raydium_cpmm.rs build_sell_instructions
func RaydiumCPMMBuildSellInstructions(params *RaydiumCPMMBuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Get pool state
	poolState := pp.PoolState
	if poolState.IsZero() {
		poolState = GetRaydiumCPMMPoolPDA(pp.AmmConfig, pp.BaseMint, pp.QuoteMint)
	}

	// Check if pool contains WSOL or USDC
	isWsol := pp.BaseMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.BaseMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Determine if quote is output (WSOL/USDC)
	isQuoteOut := pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT)

	// Get mint token program for input
	mintTokenProgram := pp.BaseTokenProgram
	if isQuoteOut {
		mintTokenProgram = pp.BaseTokenProgram
	} else {
		mintTokenProgram = pp.QuoteTokenProgram
	}

	// Calculate minimum amount out
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		result := calc.RaydiumCPMMGetAmountOut(params.InputAmount, pp.BaseReserve, pp.QuoteReserve, isQuoteOut)
		minAmountOut, _ := calc.CalculateWithSlippageSell(result, params.SlippageBasisPoints)
		minimumAmountOut = minAmountOut
	}

	// Determine output mint
	outputMint := constants.WSOL_TOKEN_ACCOUNT
	if isUsdc {
		outputMint = constants.USDC_TOKEN_ACCOUNT
	}
	inputMint := params.InputMint

	// Get user token accounts
	inputTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, mintTokenProgram)
	outputTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, constants.TOKEN_PROGRAM)

	// Get vault accounts
	inputVaultAccount := GetRaydiumCPMMVaultAccount(poolState, inputMint, pp)
	outputVaultAccount := GetRaydiumCPMMVaultAccount(poolState, outputMint, pp)

	// Get observation state
	observationStateAccount := pp.ObservationState
	if observationStateAccount.IsZero() {
		observationStateAccount = GetRaydiumCPMMObservationStatePDA(poolState)
	}

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL ATA for receiving if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, constants.TOKEN_PROGRAM,
		))
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], RaydiumCPMMSwapBaseInDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)

	// Build accounts array (13 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},              // 0: Payer (signer)
		{PublicKey: RAYDIUM_CPMM_AUTHORITY, IsSigner: false, IsWritable: false},  // 1: Authority (readonly)
		{PublicKey: pp.AmmConfig, IsSigner: false, IsWritable: false},            // 2: Amm Config (readonly)
		{PublicKey: poolState, IsSigner: false, IsWritable: true},                // 3: Pool State
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},        // 4: Input Token Account
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},       // 5: Output Token Account
		{PublicKey: inputVaultAccount, IsSigner: false, IsWritable: true},        // 6: Input Vault Account
		{PublicKey: outputVaultAccount, IsSigner: false, IsWritable: true},       // 7: Output Vault Account
		{PublicKey: mintTokenProgram, IsSigner: false, IsWritable: false},        // 8: Input Token Program (readonly)
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false}, // 9: Output Token Program (readonly)
		{PublicKey: inputMint, IsSigner: false, IsWritable: false},               // 10: Input token mint (readonly)
		{PublicKey: outputMint, IsSigner: false, IsWritable: false},              // 11: Output token mint (readonly)
		{PublicKey: observationStateAccount, IsSigner: false, IsWritable: true},  // 12: Observation State Account
	}

	instructions = append(instructions, solana.NewInstruction(RAYDIUM_CPMM_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta {
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

// Raydium CPMM error definitions
var (
	ErrRaydiumCPMMInvalidPool = fmt.Errorf("raydium cpmm: invalid pool configuration")
)

// ===== Pool State Decoder - from Rust: src/instruction/utils/raydium_cpmm_types.rs =====

const RaydiumCPMMpoolStateSize = 629

// RaydiumCPMMpoolState represents a decoded Raydium CPMM pool state
type RaydiumCPMMpoolState struct {
	AmmConfig           solana.PublicKey
	PoolCreator         solana.PublicKey
	Token0Vault         solana.PublicKey
	Token1Vault         solana.PublicKey
	LpMint              solana.PublicKey
	Token0Mint          solana.PublicKey
	Token1Mint          solana.PublicKey
	Token0Program       solana.PublicKey
	Token1Program       solana.PublicKey
	ObservationKey      solana.PublicKey
	AuthBump            uint8
	Status              uint8
	LpMintDecimals      uint8
	Mint0Decimals       uint8
	Mint1Decimals       uint8
	LpSupply            uint64
	ProtocolFeesToken0  uint64
	ProtocolFeesToken1  uint64
	FundFeesToken0      uint64
	FundFeesToken1      uint64
	OpenTime            uint64
	RecentEpoch         uint64
}

// DecodeRaydiumCPMMpoolState decodes a Raydium CPMM pool state from account data
// 100% from Rust: src/instruction/utils/raydium_cpmm_types.rs pool_state_decode
func DecodeRaydiumCPMMpoolState(data []byte) *RaydiumCPMMpoolState {
	if len(data) < RaydiumCPMMpoolStateSize {
		return nil
	}

	pool := &RaydiumCPMMpoolState{}
	offset := 0

	// amm_config: Pubkey
	copy(pool.AmmConfig[:], data[offset:offset+32])
	offset += 32

	// pool_creator: Pubkey
	copy(pool.PoolCreator[:], data[offset:offset+32])
	offset += 32

	// token0_vault: Pubkey
	copy(pool.Token0Vault[:], data[offset:offset+32])
	offset += 32

	// token1_vault: Pubkey
	copy(pool.Token1Vault[:], data[offset:offset+32])
	offset += 32

	// lp_mint: Pubkey
	copy(pool.LpMint[:], data[offset:offset+32])
	offset += 32

	// token0_mint: Pubkey
	copy(pool.Token0Mint[:], data[offset:offset+32])
	offset += 32

	// token1_mint: Pubkey
	copy(pool.Token1Mint[:], data[offset:offset+32])
	offset += 32

	// token0_program: Pubkey
	copy(pool.Token0Program[:], data[offset:offset+32])
	offset += 32

	// token1_program: Pubkey
	copy(pool.Token1Program[:], data[offset:offset+32])
	offset += 32

	// observation_key: Pubkey
	copy(pool.ObservationKey[:], data[offset:offset+32])
	offset += 32

	// auth_bump: u8
	pool.AuthBump = data[offset]
	offset += 1

	// status: u8
	pool.Status = data[offset]
	offset += 1

	// lp_mint_decimals: u8
	pool.LpMintDecimals = data[offset]
	offset += 1

	// mint0_decimals: u8
	pool.Mint0Decimals = data[offset]
	offset += 1

	// mint1_decimals: u8
	pool.Mint1Decimals = data[offset]
	offset += 1

	// lp_supply: u64
	pool.LpSupply = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// protocol_fees_token0: u64
	pool.ProtocolFeesToken0 = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// protocol_fees_token1: u64
	pool.ProtocolFeesToken1 = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// fund_fees_token0: u64
	pool.FundFeesToken0 = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// fund_fees_token1: u64
	pool.FundFeesToken1 = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// open_time: u64
	pool.OpenTime = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// recent_epoch: u64
	pool.RecentEpoch = binary.LittleEndian.Uint64(data[offset:])

	return pool
}

// ===== Async Fetch Functions - from Rust: src/instruction/utils/raydium_cpmm.rs =====

// RaydiumCPMMpoolFetcher defines interface for fetching Raydium CPMM pool data from RPC
type RaydiumCPMMpoolFetcher interface {
	GetAccountInfo(pubkey solana.PublicKey) ([]byte, error)
}

// FetchRaydiumCPMMpoolState fetches a Raydium CPMM pool state from RPC.
// 100% from Rust: src/instruction/utils/raydium_cpmm.rs fetch_pool_state
func FetchRaydiumCPMMpoolState(fetcher RaydiumCPMMpoolFetcher, poolAddress solana.PublicKey) (*RaydiumCPMMpoolState, error) {
	data, err := fetcher.GetAccountInfo(poolAddress)
	if err != nil {
		return nil, err
	}
	if len(data) < 8+RaydiumCPMMpoolStateSize {
		return nil, fmt.Errorf("account data too short")
	}

	// Skip 8-byte discriminator
	pool := DecodeRaydiumCPMMpoolState(data[8:])
	if pool == nil {
		return nil, fmt.Errorf("failed to decode raydium cpmm pool state")
	}
	return pool, nil
}
