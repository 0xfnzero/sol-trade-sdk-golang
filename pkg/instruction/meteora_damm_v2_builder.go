// Meteora DAMM V2 instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
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
	Pool          solana.PublicKey
	TokenAMint    solana.PublicKey
	TokenBMint    solana.PublicKey
	TokenAVault   solana.PublicKey
	TokenBVault   solana.PublicKey
	TokenAProgram solana.PublicKey
	TokenBProgram solana.PublicKey
	TokenAReserve uint64
	TokenBReserve uint64
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

	// Calculate minimum amount out
	amountIn := params.InputAmount
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		return nil, fmt.Errorf("fixed_output_amount must be set for MeteoraDammV2 swap")
	}

	// Get user token accounts
	var inputTokenAccount, outputTokenAccount solana.PublicKey
	var inputTokenProgram, outputTokenProgram solana.PublicKey

	if isAIn {
		inputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.InputMint, pp.TokenAProgram)
		outputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.OutputMint, pp.TokenBProgram)
		inputTokenProgram = pp.TokenAProgram
		outputTokenProgram = pp.TokenBProgram
	} else {
		inputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.InputMint, pp.TokenBProgram)
		outputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.OutputMint, pp.TokenAProgram)
		inputTokenProgram = pp.TokenBProgram
		outputTokenProgram = pp.TokenAProgram
	}

	// Get event authority
	eventAuthority := GetMeteoraDammV2EventAuthorityPDA()

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	if params.CreateInputMintAta {
		instructions = append(instructions, HandleWsol(params.Payer, amountIn)...)
	}

	// Create output ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, params.OutputMint, outputTokenProgram,
		))
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], MeteoraDammV2SwapDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], amountIn)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)

	// Build accounts array (14 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: METEORA_DAMM_V2_AUTHORITY, IsSigner: false, IsWritable: false}, // 0: Pool Authority (readonly)
		{PublicKey: pp.Pool, IsSigner: false, IsWritable: true},                    // 1: Pool
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},          // 2: Input Token Account
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},         // 3: Output Token Account
		{PublicKey: pp.TokenAVault, IsSigner: false, IsWritable: true},             // 4: Token A Vault
		{PublicKey: pp.TokenBVault, IsSigner: false, IsWritable: true},             // 5: Token B Vault
		{PublicKey: pp.TokenAMint, IsSigner: false, IsWritable: false},             // 6: Token A Mint (readonly)
		{PublicKey: pp.TokenBMint, IsSigner: false, IsWritable: false},             // 7: Token B Mint (readonly)
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},                // 8: User Transfer Authority
		{PublicKey: pp.TokenAProgram, IsSigner: false, IsWritable: false},          // 9: Token Program (readonly)
		{PublicKey: pp.TokenBProgram, IsSigner: false, IsWritable: false},          // 10: Token Program (readonly)
		{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false},   // 11: Referral Token Account (readonly)
		{PublicKey: eventAuthority, IsSigner: false, IsWritable: false},            // 12: Event Authority (readonly)
		{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false},   // 13: Program (readonly)
	}

	instructions = append(instructions, solana.NewInstruction(METEORA_DAMM_V2_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta {
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

	// Calculate minimum amount out
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		return nil, fmt.Errorf("fixed_output_amount must be set for MeteoraDammV2 swap")
	}

	// Get user token accounts
	var inputTokenAccount, outputTokenAccount solana.PublicKey
	var inputTokenProgram solana.PublicKey

	if isAIn {
		inputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.InputMint, pp.TokenAProgram)
		outputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.OutputMint, pp.TokenBProgram)
		inputTokenProgram = pp.TokenAProgram
	} else {
		inputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.InputMint, pp.TokenBProgram)
		outputTokenAccount = GetAssociatedTokenAddress(params.Payer, params.OutputMint, pp.TokenAProgram)
		inputTokenProgram = pp.TokenBProgram
	}

	// Get event authority
	eventAuthority := GetMeteoraDammV2EventAuthorityPDA()

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL ATA for receiving if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, params.OutputMint, constants.TOKEN_PROGRAM,
		))
	}

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], MeteoraDammV2SwapDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
	binary.LittleEndian.PutUint64(data[16:24], minimumAmountOut)

	// Build accounts array (14 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: METEORA_DAMM_V2_AUTHORITY, IsSigner: false, IsWritable: false}, // 0: Pool Authority (readonly)
		{PublicKey: pp.Pool, IsSigner: false, IsWritable: true},                    // 1: Pool
		{PublicKey: inputTokenAccount, IsSigner: false, IsWritable: true},          // 2: Input Token Account
		{PublicKey: outputTokenAccount, IsSigner: false, IsWritable: true},         // 3: Output Token Account
		{PublicKey: pp.TokenAVault, IsSigner: false, IsWritable: true},             // 4: Token A Vault
		{PublicKey: pp.TokenBVault, IsSigner: false, IsWritable: true},             // 5: Token B Vault
		{PublicKey: pp.TokenAMint, IsSigner: false, IsWritable: false},             // 6: Token A Mint (readonly)
		{PublicKey: pp.TokenBMint, IsSigner: false, IsWritable: false},             // 7: Token B Mint (readonly)
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},                // 8: User Transfer Authority
		{PublicKey: pp.TokenAProgram, IsSigner: false, IsWritable: false},          // 9: Token Program (readonly)
		{PublicKey: pp.TokenBProgram, IsSigner: false, IsWritable: false},          // 10: Token Program (readonly)
		{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false},   // 11: Referral Token Account (readonly)
		{PublicKey: eventAuthority, IsSigner: false, IsWritable: false},            // 12: Event Authority (readonly)
		{PublicKey: METEORA_DAMM_V2_PROGRAM, IsSigner: false, IsWritable: false},   // 13: Program (readonly)
	}

	instructions = append(instructions, solana.NewInstruction(METEORA_DAMM_V2_PROGRAM, accounts, data))

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

// Meteora DAMM V2 error definitions
var (
	ErrMeteoraDammV2InvalidPool = fmt.Errorf("meteora damm v2: invalid pool configuration")
)

// ===== Pool Types and Decoder - from Rust: src/instruction/utils/meteora_damm_v2_types.rs =====

// MeteoraPoolSize is the size of a Meteora DAMM V2 pool
const MeteoraPoolSize = 1104

// MeteoraDammV2Pool represents a simplified Meteora DAMM V2 pool
type MeteoraDammV2Pool struct {
	TokenAMint  solana.PublicKey
	TokenBMint  solana.PublicKey
	TokenAVault solana.PublicKey
	TokenBVault solana.PublicKey
	Liquidity   [16]byte // u128
	SqrtPrice   [16]byte // u128
	PoolStatus  uint8
	TokenAFlag  uint8
	TokenBFlag  uint8
}

// DecodeMeteoraPool decodes a Meteora DAMM V2 pool from account data.
// 100% from Rust: src/instruction/utils/meteora_damm_v2_types.rs pool_decode
func DecodeMeteoraPool(data []byte) *MeteoraDammV2Pool {
	if len(data) < MeteoraPoolSize {
		return nil
	}

	pool := &MeteoraDammV2Pool{}
	offset := 0

	// Skip pool_fees structure (first 248 bytes)
	offset = 248

	// token_a_mint: Pubkey (32 bytes)
	copy(pool.TokenAMint[:], data[offset:offset+32])
	offset += 32

	// token_b_mint: Pubkey
	copy(pool.TokenBMint[:], data[offset:offset+32])
	offset += 32

	// token_a_vault: Pubkey
	copy(pool.TokenAVault[:], data[offset:offset+32])
	offset += 32

	// token_b_vault: Pubkey
	copy(pool.TokenBVault[:], data[offset:offset+32])
	offset += 32

	// Skip whitelisted_vault, partner (64 bytes)
	offset += 64

	// liquidity: u128 (16 bytes)
	copy(pool.Liquidity[:], data[offset:offset+16])
	offset += 16

	// Skip padding (16 bytes)
	offset += 16

	// Skip protocol_a_fee, protocol_b_fee, partner_a_fee, partner_b_fee (32 bytes)
	offset += 32

	// Skip sqrt_min_price, sqrt_max_price (32 bytes)
	offset += 32

	// sqrt_price: u128
	copy(pool.SqrtPrice[:], data[offset:offset+16])
	offset += 16

	// Skip activation_point (8 bytes)
	offset += 8

	// activation_type: u8, pool_status: u8, token_a_flag: u8, token_b_flag: u8
	pool.PoolStatus = data[offset+1]
	pool.TokenAFlag = data[offset+2]
	pool.TokenBFlag = data[offset+3]

	return pool
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

	// Skip 8-byte discriminator
	pool := DecodeMeteoraPool(data[8:])
	if pool == nil {
		return nil, fmt.Errorf("failed to decode meteora pool")
	}
	return pool, nil
}
