// Bonk instruction builder - Production-grade implementation
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

// ===== Bonk Program Constants from Rust: src/instruction/utils/bonk.rs =====

var (
	// BONK_PROGRAM is the Bonk program ID
	BONK_PROGRAM = solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
	// BONK_AUTHORITY is the program authority
	BONK_AUTHORITY = solana.MustPublicKeyFromBase58("WLHv2UAZm6z4KyaaELi5pjdbJh6RESMva1Rnn8pJVVh")
	// BONK_GLOBAL_CONFIG is the global config account
	BONK_GLOBAL_CONFIG = solana.MustPublicKeyFromBase58("6s1xP3hpbAfFoNtUNF8mfHsjr2Bd97JxFJRWLbL6aHuX")
	// BONK_USD1_GLOBAL_CONFIG is the USD1 global config account
	BONK_USD1_GLOBAL_CONFIG = solana.MustPublicKeyFromBase58("EPiZbnrThjyLnoQ6QQzkxeFqyL5uyg9RzNHHAudUPxBz")
	// BONK_EVENT_AUTHORITY is the event authority PDA
	BONK_EVENT_AUTHORITY = solana.MustPublicKeyFromBase58("2DPAtwB8L12vrMRExbLuyGnC7n2J5LNoZQSejeQGpwkr")
)

// Discriminators - from Rust: src/instruction/utils/bonk.rs
var (
	// BonkBuyExactInDiscriminator is the discriminator for the buy_exact_in instruction
	BonkBuyExactInDiscriminator = []byte{250, 234, 13, 123, 213, 156, 19, 236}
	// BonkSellExactInDiscriminator is the discriminator for the sell_exact_in instruction
	BonkSellExactInDiscriminator = []byte{149, 39, 222, 155, 211, 124, 152, 26}
)

// Seeds - from Rust: src/instruction/utils/bonk.rs seeds
var (
	BonkPoolSeed      = []byte("pool")
	BonkPoolVaultSeed = []byte("pool_vault")
)

// Bonk Constants - from Rust: src/instruction/utils/bonk.rs accounts
const (
	BonkProtocolFeeRate uint64 = 25  // 0.25%
	BonkPlatformFeeRate uint64 = 100 // 1%
	BonkShareFeeRate    uint64 = 0   // 0%
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetBonkPoolPDA returns the pool PDA (seeds: ["pool", base_mint, quote_mint])
func GetBonkPoolPDA(baseMint, quoteMint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{BonkPoolSeed, baseMint[:], quoteMint[:]},
		BONK_PROGRAM,
	)
	return pda
}

// GetBonkVaultPDA returns the vault PDA (seeds: ["pool_vault", pool_state, mint])
func GetBonkVaultPDA(poolState, mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{BonkPoolVaultSeed, poolState[:], mint[:]},
		BONK_PROGRAM,
	)
	return pda
}

// GetBonkPlatformAssociatedAccount returns the platform associated account PDA
func GetBonkPlatformAssociatedAccount(platformConfig solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{platformConfig[:], constants.WSOL_TOKEN_ACCOUNT[:]},
		BONK_PROGRAM,
	)
	return pda
}

// GetBonkCreatorAssociatedAccount returns the creator associated account PDA
func GetBonkCreatorAssociatedAccount(creator solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{creator[:], constants.WSOL_TOKEN_ACCOUNT[:]},
		BONK_PROGRAM,
	)
	return pda
}

// ===== Bonk Params =====

// BonkParams contains parameters for Bonk operations
type BonkParams struct {
	PoolState                 solana.PublicKey
	BaseVault                 solana.PublicKey
	QuoteVault                solana.PublicKey
	PlatformConfig            solana.PublicKey
	PlatformAssociatedAccount solana.PublicKey
	CreatorAssociatedAccount  solana.PublicKey
	GlobalConfig              solana.PublicKey
	MintTokenProgram          solana.PublicKey
	VirtualBase               uint64
	VirtualQuote              uint64
	RealBase                  uint64
	RealQuote                 uint64
}

// BonkBuildBuyParams contains parameters for building buy instructions
type BonkBuildBuyParams struct {
	Payer               solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *BonkParams
	CreateInputMintAta  bool
	CreateOutputMintAta bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// BonkBuildSellParams contains parameters for building sell instructions
type BonkBuildSellParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *BonkParams
	CreateOutputMintAta bool
	CloseOutputMintAta  bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== Instruction Builders - 100% from Rust =====

// BonkBuildBuyInstructions builds buy instructions for Bonk
// 100% port from Rust: src/instruction/bonk.rs build_buy_instructions
func BonkBuildBuyInstructions(params *BonkBuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if USD1 pool
	usd1Pool := pp.GlobalConfig.Equals(BONK_USD1_GLOBAL_CONFIG)

	// Get pool state
	poolState := pp.PoolState
	if poolState.IsZero() {
		if usd1Pool {
			poolState = GetBonkPoolPDA(params.OutputMint, constants.USD1_TOKEN_ACCOUNT)
		} else {
			poolState = GetBonkPoolPDA(params.OutputMint, constants.WSOL_TOKEN_ACCOUNT)
		}
	}

	// Get global config
	globalConfig := BONK_GLOBAL_CONFIG
	if usd1Pool {
		globalConfig = BONK_USD1_GLOBAL_CONFIG
	}

	// Get quote token mint
	quoteTokenMint := constants.WSOL_TOKEN_ACCOUNT
	if usd1Pool {
		quoteTokenMint = constants.USD1_TOKEN_ACCOUNT
	}

	// Calculate minimum amount out
	amountIn := params.InputAmount
	shareFeeRate := uint64(0)
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		minimumAmountOut = calc.GetBonkAmountOut(
			amountIn,
			BonkProtocolFeeRate,
			BonkPlatformFeeRate,
			shareFeeRate,
			pp.VirtualBase,
			pp.VirtualQuote,
			pp.RealBase,
			pp.RealQuote,
			params.SlippageBasisPoints,
		)
	}

	// Get user token accounts
	userBaseTokenAccount := GetAssociatedTokenAddress(params.Payer, params.OutputMint, pp.MintTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(params.Payer, quoteTokenMint, constants.TOKEN_PROGRAM)

	// Get vault accounts
	baseVaultAccount := pp.BaseVault
	if baseVaultAccount.IsZero() {
		baseVaultAccount = GetBonkVaultPDA(poolState, params.OutputMint)
	}
	quoteVaultAccount := pp.QuoteVault
	if quoteVaultAccount.IsZero() {
		quoteVaultAccount = GetBonkVaultPDA(poolState, quoteTokenMint)
	}

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	if params.CreateInputMintAta && !usd1Pool {
		instructions = append(instructions, HandleWsol(params.Payer, amountIn)...)
	}

	// Create output ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, params.OutputMint, pp.MintTokenProgram,
		))
	}

	// Build instruction data
	data := make([]byte, 32)
	copy(data[0:8], BonkBuyExactInDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], amountIn)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)
	binary.LittleEndian.PutUint64(data[24:32], shareFeeRate)

	// Build accounts array (18 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},                  // 0: Payer (signer)
		{PublicKey: BONK_AUTHORITY, IsSigner: false, IsWritable: false},              // 1: Authority (readonly)
		{PublicKey: globalConfig, IsSigner: false, IsWritable: false},                // 2: Global Config (readonly)
		{PublicKey: pp.PlatformConfig, IsSigner: false, IsWritable: false},           // 3: Platform Config (readonly)
		{PublicKey: poolState, IsSigner: false, IsWritable: true},                    // 4: Pool State
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},         // 5: User Base Token
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},        // 6: User Quote Token
		{PublicKey: baseVaultAccount, IsSigner: false, IsWritable: true},             // 7: Base Vault
		{PublicKey: quoteVaultAccount, IsSigner: false, IsWritable: true},            // 8: Quote Vault
		{PublicKey: params.OutputMint, IsSigner: false, IsWritable: false},           // 9: Base Token Mint (readonly)
		{PublicKey: quoteTokenMint, IsSigner: false, IsWritable: false},              // 10: Quote Token Mint (readonly)
		{PublicKey: pp.MintTokenProgram, IsSigner: false, IsWritable: false},         // 11: Base Token Program (readonly)
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},     // 12: Quote Token Program (readonly)
		{PublicKey: BONK_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},        // 13: Event Authority (readonly)
		{PublicKey: BONK_PROGRAM, IsSigner: false, IsWritable: false},                // 14: Program (readonly)
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},    // 15: System Program (readonly)
		{PublicKey: pp.PlatformAssociatedAccount, IsSigner: false, IsWritable: true}, // 16: Platform Associated Account
		{PublicKey: pp.CreatorAssociatedAccount, IsSigner: false, IsWritable: true},  // 17: Creator Associated Account
	}

	instructions = append(instructions, solana.NewInstruction(BONK_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta && !usd1Pool {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	return instructions, nil
}

// BonkBuildSellInstructions builds sell instructions for Bonk
// 100% port from Rust: src/instruction/bonk.rs build_sell_instructions
func BonkBuildSellInstructions(params *BonkBuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if USD1 pool
	usd1Pool := pp.GlobalConfig.Equals(BONK_USD1_GLOBAL_CONFIG)

	// Get pool state
	poolState := pp.PoolState
	if poolState.IsZero() {
		if usd1Pool {
			poolState = GetBonkPoolPDA(params.InputMint, constants.USD1_TOKEN_ACCOUNT)
		} else {
			poolState = GetBonkPoolPDA(params.InputMint, constants.WSOL_TOKEN_ACCOUNT)
		}
	}

	// Get global config
	globalConfig := BONK_GLOBAL_CONFIG
	if usd1Pool {
		globalConfig = BONK_USD1_GLOBAL_CONFIG
	}

	// Get quote token mint
	quoteTokenMint := constants.WSOL_TOKEN_ACCOUNT
	if usd1Pool {
		quoteTokenMint = constants.USD1_TOKEN_ACCOUNT
	}

	// Calculate minimum amount out
	shareFeeRate := uint64(0)
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		minimumAmountOut = calc.GetBonkAmountOut(
			params.InputAmount,
			BonkProtocolFeeRate,
			BonkPlatformFeeRate,
			shareFeeRate,
			pp.VirtualBase,
			pp.VirtualQuote,
			pp.RealBase,
			pp.RealQuote,
			params.SlippageBasisPoints,
		)
	}

	// Get user token accounts
	userBaseTokenAccount := GetAssociatedTokenAddress(params.Payer, params.InputMint, pp.MintTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(params.Payer, quoteTokenMint, constants.TOKEN_PROGRAM)

	// Get vault accounts
	baseVaultAccount := pp.BaseVault
	if baseVaultAccount.IsZero() {
		baseVaultAccount = GetBonkVaultPDA(poolState, params.InputMint)
	}
	quoteVaultAccount := pp.QuoteVault
	if quoteVaultAccount.IsZero() {
		quoteVaultAccount = GetBonkVaultPDA(poolState, quoteTokenMint)
	}

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL ATA for receiving if needed
	if params.CreateOutputMintAta && !usd1Pool {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, quoteTokenMint, constants.TOKEN_PROGRAM,
		))
	}

	// Build instruction data
	data := make([]byte, 32)
	copy(data[0:8], BonkSellExactInDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)
	binary.LittleEndian.PutUint64(data[24:32], shareFeeRate)

	// Build accounts array (18 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},                  // 0: Payer (signer)
		{PublicKey: BONK_AUTHORITY, IsSigner: false, IsWritable: false},              // 1: Authority (readonly)
		{PublicKey: globalConfig, IsSigner: false, IsWritable: false},                // 2: Global Config (readonly)
		{PublicKey: pp.PlatformConfig, IsSigner: false, IsWritable: false},           // 3: Platform Config (readonly)
		{PublicKey: poolState, IsSigner: false, IsWritable: true},                    // 4: Pool State
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},         // 5: User Base Token
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},        // 6: User Quote Token
		{PublicKey: baseVaultAccount, IsSigner: false, IsWritable: true},             // 7: Base Vault
		{PublicKey: quoteVaultAccount, IsSigner: false, IsWritable: true},            // 8: Quote Vault
		{PublicKey: params.InputMint, IsSigner: false, IsWritable: false},            // 9: Base Token Mint (readonly)
		{PublicKey: quoteTokenMint, IsSigner: false, IsWritable: false},              // 10: Quote Token Mint (readonly)
		{PublicKey: pp.MintTokenProgram, IsSigner: false, IsWritable: false},         // 11: Base Token Program (readonly)
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},     // 12: Quote Token Program (readonly)
		{PublicKey: BONK_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},        // 13: Event Authority (readonly)
		{PublicKey: BONK_PROGRAM, IsSigner: false, IsWritable: false},                // 14: Program (readonly)
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},    // 15: System Program (readonly)
		{PublicKey: pp.PlatformAssociatedAccount, IsSigner: false, IsWritable: true}, // 16: Platform Associated Account
		{PublicKey: pp.CreatorAssociatedAccount, IsSigner: false, IsWritable: true},  // 17: Creator Associated Account
	}

	instructions = append(instructions, solana.NewInstruction(BONK_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta && !usd1Pool {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	// Close base token account if requested
	if params.CloseInputMintAta {
		closeIx := token.NewCloseAccountInstruction(
			userBaseTokenAccount,
			params.Payer,
			params.Payer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

// Bonk error definitions
var (
	ErrBonkInvalidPool = fmt.Errorf("bonk: invalid pool configuration")
)

// ===== Pool State Decoder - from Rust: src/instruction/utils/bonk_types.rs =====

const BonkPoolStateSize = 421 // 8 + 1*5 + 8*10 + 32*7 + 8*8 + 8*5

// BonkVestingSchedule represents vesting schedule for a Bonk pool
type BonkVestingSchedule struct {
	TotalLockedAmount    uint64
	CliffPeriod          uint64
	UnlockPeriod         uint64
	StartTime            uint64
	AllocatedShareAmount uint64
}

// BonkPoolState represents a decoded Bonk pool state
type BonkPoolState struct {
	Epoch                 uint64
	AuthBump              uint8
	Status                uint8
	BaseDecimals          uint8
	QuoteDecimals         uint8
	MigrateType           uint8
	Supply                uint64
	TotalBaseSell         uint64
	VirtualBase           uint64
	VirtualQuote          uint64
	RealBase              uint64
	RealQuote             uint64
	TotalQuoteFundRaising uint64
	QuoteProtocolFee      uint64
	PlatformFee           uint64
	MigrateFee            uint64
	VestingSchedule       BonkVestingSchedule
	GlobalConfig          solana.PublicKey
	PlatformConfig        solana.PublicKey
	BaseMint              solana.PublicKey
	QuoteMint             solana.PublicKey
	BaseVault             solana.PublicKey
	QuoteVault            solana.PublicKey
	Creator               solana.PublicKey
}

// DecodeBonkPoolState decodes a Bonk pool state from account data
// 100% from Rust: src/instruction/utils/bonk_types.rs pool_state_decode
func DecodeBonkPoolState(data []byte) *BonkPoolState {
	if len(data) < BonkPoolStateSize {
		return nil
	}

	pool := &BonkPoolState{}
	offset := 0

	// epoch: u64
	pool.Epoch = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// auth_bump: u8
	pool.AuthBump = data[offset]
	offset += 1

	// status: u8
	pool.Status = data[offset]
	offset += 1

	// base_decimals: u8
	pool.BaseDecimals = data[offset]
	offset += 1

	// quote_decimals: u8
	pool.QuoteDecimals = data[offset]
	offset += 1

	// migrate_type: u8
	pool.MigrateType = data[offset]
	offset += 1

	// supply: u64
	pool.Supply = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// total_base_sell: u64
	pool.TotalBaseSell = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// virtual_base: u64
	pool.VirtualBase = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// virtual_quote: u64
	pool.VirtualQuote = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// real_base: u64
	pool.RealBase = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// real_quote: u64
	pool.RealQuote = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// total_quote_fund_raising: u64
	pool.TotalQuoteFundRaising = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// quote_protocol_fee: u64
	pool.QuoteProtocolFee = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// platform_fee: u64
	pool.PlatformFee = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// migrate_fee: u64
	pool.MigrateFee = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// vesting_schedule: VestingSchedule (5 * u64)
	pool.VestingSchedule = BonkVestingSchedule{
		TotalLockedAmount:    binary.LittleEndian.Uint64(data[offset:]),
		CliffPeriod:          binary.LittleEndian.Uint64(data[offset+8:]),
		UnlockPeriod:         binary.LittleEndian.Uint64(data[offset+16:]),
		StartTime:            binary.LittleEndian.Uint64(data[offset+24:]),
		AllocatedShareAmount: binary.LittleEndian.Uint64(data[offset+32:]),
	}
	offset += 40

	// global_config: Pubkey
	copy(pool.GlobalConfig[:], data[offset:offset+32])
	offset += 32

	// platform_config: Pubkey
	copy(pool.PlatformConfig[:], data[offset:offset+32])
	offset += 32

	// base_mint: Pubkey
	copy(pool.BaseMint[:], data[offset:offset+32])
	offset += 32

	// quote_mint: Pubkey
	copy(pool.QuoteMint[:], data[offset:offset+32])
	offset += 32

	// base_vault: Pubkey
	copy(pool.BaseVault[:], data[offset:offset+32])
	offset += 32

	// quote_vault: Pubkey
	copy(pool.QuoteVault[:], data[offset:offset+32])
	offset += 32

	// creator: Pubkey
	copy(pool.Creator[:], data[offset:offset+32])

	return pool
}

// ===== Async Fetch Functions - from Rust: src/instruction/utils/bonk.rs =====

// BonkPoolFetcher defines interface for fetching Bonk pool data from RPC
type BonkPoolFetcher interface {
	GetAccountInfo(pubkey solana.PublicKey) ([]byte, error)
}

// FetchBonkPoolState fetches a Bonk pool state from RPC.
// 100% from Rust: src/instruction/utils/bonk.rs fetch_pool_state
func FetchBonkPoolState(fetcher BonkPoolFetcher, poolAddress solana.PublicKey) (*BonkPoolState, error) {
	data, err := fetcher.GetAccountInfo(poolAddress)
	if err != nil {
		return nil, err
	}
	if len(data) < 8+BonkPoolStateSize {
		return nil, fmt.Errorf("account data too short")
	}

	// Skip 8-byte discriminator
	pool := DecodeBonkPoolState(data[8:])
	if pool == nil {
		return nil, fmt.Errorf("failed to decode bonk pool state")
	}
	return pool, nil
}
