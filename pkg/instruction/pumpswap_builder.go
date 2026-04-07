// PumpSwap instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/your-org/sol-trade-sdk-go/pkg/calc"
	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
)

// ===== PumpSwap Program Constants from Rust: src/instruction/utils/pumpswap.rs =====

var (
	PUMPSWAP_PROGRAM                = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
	PUMP_PROGRAM_ID                 = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	FEE_PROGRAM                     = solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
	FEE_RECIPIENT                   = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	PUMPSWAP_GLOBAL_ACCOUNT         = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw")
	PUMPSWAP_EVENT_AUTHORITY        = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
	GLOBAL_VOLUME_ACCUMULATOR       = solana.MustPublicKeyFromBase58("C2aFPdENg4A2HQsmrd5rTw5TaYBX5Ku887cWjbFKtZpw")
	FEE_CONFIG                      = solana.MustPublicKeyFromBase58("5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx")
	DEFAULT_COIN_CREATOR_VAULT_AUTH = solana.MustPublicKeyFromBase58("8N3GDaZ2iwN65oxVatKTLPNooAVUJTbfiVJ1ahyqwjSk")
)

// Mayhem fee recipients - from Rust: src/instruction/utils/pumpswap.rs MAYHEM_FEE_RECIPIENTS
var MAYHEM_FEE_RECIPIENTS = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS"),
	solana.MustPublicKeyFromBase58("4budycTjhs9fD6xw62VBducVTNgMgJJ5BgtKq7mAZwn6"),
	solana.MustPublicKeyFromBase58("8SBKzEQU4nLSzcwF4a74F2iaUDQyTfjGndn6qUWBnrpR"),
	solana.MustPublicKeyFromBase58("4UQeTP1T39KZ9Sfxzo3WR5skgsaP6NZa87BAkuazLEKH"),
	solana.MustPublicKeyFromBase58("8sNeir4QsLsJdYpc9RZacohhK1Y5FLU3nC5LXgYB4aa6"),
	solana.MustPublicKeyFromBase58("Fh9HmeLNUMVCvejxCtCL2DbYaRyBFVJ5xrWkLnMH6fdk"),
	solana.MustPublicKeyFromBase58("463MEnMeGyJekNZFQSTUABBEbLnvMTALbT6ZmsxAbAdq"),
	solana.MustPublicKeyFromBase58("6AUH3WEHucYZyC61hqpqYUWVto5qA5hjHuNQ32GNnNxA"),
}

// Discriminators - from Rust: src/instruction/utils/pumpswap.rs
var (
	PUMPSWAP_BUY_DISCRIMINATOR                = []byte{102, 6, 61, 18, 1, 218, 235, 234}
	PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR = []byte{198, 46, 21, 82, 180, 217, 232, 112}
	PUMPSWAP_SELL_DISCRIMINATOR               = []byte{51, 230, 133, 164, 1, 127, 131, 173}
	PUMPSWAP_CLAIM_CASHBACK_DISCRIMINATOR     = []byte{37, 58, 35, 126, 190, 53, 228, 197}
)

// Seeds - from Rust: src/instruction/utils/pumpswap.rs
var (
	POOL_V2_SEED                 = []byte("pool-v2")
	POOL_SEED                    = []byte("pool")
	POOL_AUTHORITY_SEED          = []byte("pool-authority")
	USER_VOLUME_ACCUMULATOR_SEED = []byte("user_volume_accumulator")
	CREATOR_VAULT_SEED           = []byte("creator_vault")
	FEE_CONFIG_SEED              = []byte("fee_config")
	GLOBAL_VOLUME_ACCUMULATOR_SEED = []byte("global_volume_accumulator")
)

// Fee basis points - from Rust: src/instruction/utils/pumpswap.rs
const (
	PUMPSWAP_LP_FEE_BASIS_POINTS           uint64 = 25
	PUMPSWAP_PROTOCOL_FEE_BASIS_POINTS     uint64 = 5
	PUMPSWAP_COIN_CREATOR_FEE_BASIS_POINTS uint64 = 5
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetMayhemFeeRecipientRandom returns a random Mayhem fee recipient
func GetMayhemFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(MAYHEM_FEE_RECIPIENTS))))
	return MAYHEM_FEE_RECIPIENTS[n.Int64()]
}

// GetPoolV2PDA returns the Pool v2 PDA (seeds: ["pool-v2", base_mint])
func GetPoolV2PDA(baseMint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{POOL_V2_SEED, baseMint[:]},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// GetPumpPoolAuthorityPDA returns the Pump program pool-authority PDA
func GetPumpPoolAuthorityPDA(mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{POOL_AUTHORITY_SEED, mint[:]},
		PUMP_PROGRAM_ID,
	)
	return pda
}

// GetCanonicalPoolPDA returns the canonical Pump pool PDA
func GetCanonicalPoolPDA(mint solana.PublicKey) solana.PublicKey {
	authority := GetPumpPoolAuthorityPDA(mint)
	index := make([]byte, 2)
	// index = 0 (little endian)
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{POOL_SEED, index, authority[:], mint[:], constants.WSOL_TOKEN_ACCOUNT[:]},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// GetCoinCreatorVaultAuthority returns the coin creator vault authority PDA
func GetCoinCreatorVaultAuthority(coinCreator solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{CREATOR_VAULT_SEED, coinCreator[:]},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// GetUserVolumeAccumulatorPDA returns the user volume accumulator PDA for PumpSwap
func GetUserVolumeAccumulatorPDA(user solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{USER_VOLUME_ACCUMULATOR_SEED, user[:]},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// GetUserVolumeAccumulatorWsolAta returns the WSOL ATA of UserVolumeAccumulator
func GetUserVolumeAccumulatorWsolAta(user solana.PublicKey) solana.PublicKey {
	accumulator := GetUserVolumeAccumulatorPDA(user)
	return GetAssociatedTokenAddress(accumulator, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM)
}

// GetUserVolumeAccumulatorQuoteAta returns the quote-mint ATA of UserVolumeAccumulator
func GetUserVolumeAccumulatorQuoteAta(user, quoteMint, quoteTokenProgram solana.PublicKey) solana.PublicKey {
	accumulator := GetUserVolumeAccumulatorPDA(user)
	return GetAssociatedTokenAddress(accumulator, quoteMint, quoteTokenProgram)
}

// ===== PumpSwap Params =====

// PumpSwapParams contains parameters for PumpSwap operations
type PumpSwapParams struct {
	Pool                      solana.PublicKey
	BaseMint                  solana.PublicKey
	QuoteMint                 solana.PublicKey
	PoolBaseTokenAccount      solana.PublicKey
	PoolQuoteTokenAccount     solana.PublicKey
	PoolBaseTokenReserves     uint64
	PoolQuoteTokenReserves    uint64
	CoinCreatorVaultAta       solana.PublicKey
	CoinCreatorVaultAuthority solana.PublicKey
	BaseTokenProgram          solana.PublicKey
	QuoteTokenProgram         solana.PublicKey
	IsMayhemMode              bool
	IsCashbackCoin            bool
}

// BuildBuyParams contains parameters for building buy instructions
type BuildBuyParams struct {
	Payer               solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *PumpSwapParams
	CreateInputMintAta  bool
	CloseInputMintAta   bool
	CreateOutputMintAta bool
	UseExactQuoteAmount bool
	FixedOutputAmount   *uint64
}

// BuildSellParams contains parameters for building sell instructions
type BuildSellParams struct {
	Payer               solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *PumpSwapParams
	CreateOutputMintAta bool
	CloseOutputMintAta  bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== WSOL Manager - 100% from Rust =====

// HandleWsol creates WSOL ATA and wraps SOL
func HandleWsol(owner solana.PublicKey, amount uint64) []solana.Instruction {
	wsolAta := GetAssociatedTokenAddress(owner, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM)
	instructions := make([]solana.Instruction, 0, 3)

	// Create ATA (idempotent)
	instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(owner, owner, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM))

	// Transfer SOL to WSOL ATA
	instructions = append(instructions, token.NewTransferInstruction(
		amount,
		owner,
		wsolAta,
		owner,
		[]solana.PublicKey{},
	).Build())

	// Sync native
	instructions = append(instructions, token.NewSyncNativeInstruction(
		wsolAta,
	).Build())

	return instructions
}

// CloseWsol closes WSOL ATA and reclaims rent
func CloseWsol(owner solana.PublicKey) solana.Instruction {
	wsolAta := GetAssociatedTokenAddress(owner, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM)
	return token.NewCloseAccountInstruction(
		wsolAta,
		owner,
		owner,
		[]solana.PublicKey{},
	).Build()
}

// CreateAssociatedTokenAccountIdempotent creates ATA if not exists
func CreateAssociatedTokenAccountIdempotent(payer, owner, mint, tokenProgram solana.PublicKey) solana.Instruction {
	ata := GetAssociatedTokenAddress(owner, mint, tokenProgram)

	accounts := []solana.AccountMeta{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
		{PublicKey: ata, IsSigner: false, IsWritable: true},
		{PublicKey: owner, IsSigner: false, IsWritable: false},
		{PublicKey: mint, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: constants.RENT, IsSigner: false, IsWritable: false},
	}

	// Idempotent discriminator = 1
	data := []byte{1}

	return solana.NewInstruction(constants.ASSOCIATED_TOKEN_PROGRAM_ID, accounts, data)
}

// ===== Instruction Builders - 100% from Rust =====

// BuildBuyInstructions builds buy instructions for PumpSwap
// 100% port from Rust: src/instruction/pumpswap.rs build_buy_instructions
func BuildBuyInstructions(params *BuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if pool contains WSOL or USDC
	isWsol := pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.BaseMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.BaseMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	quoteIsWsolOrUsdc := pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT)

	// Determine if has coin creator
	hasCoinCreator := !pp.CoinCreatorVaultAuthority.Equals(DEFAULT_COIN_CREATOR_VAULT_AUTHORITY)

	// Calculate trade amounts
	var tokenAmount uint64
	var solAmount uint64

	if quoteIsWsolOrUsdc {
		result, err := calc.BuyQuoteInputInternal(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			hasCoinCreator,
		)
		if err != nil {
			return nil, err
		}
		tokenAmount = result.Base
		solAmount = result.MaxQuote
	} else {
		return nil, ErrInvalidConfiguration
	}

	// Override token amount if fixed output is specified
	if params.FixedOutputAmount != nil {
		tokenAmount = *params.FixedOutputAmount
	}

	// Get user token accounts
	userBaseTokenAccount := GetAssociatedTokenAddress(params.Payer, pp.BaseMint, pp.BaseTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(params.Payer, pp.QuoteMint, pp.QuoteTokenProgram)

	// Determine fee recipient
	var feeRecipient solana.PublicKey
	if pp.IsMayhemMode {
		feeRecipient = GetMayhemFeeRecipientRandom()
	} else {
		feeRecipient = FEE_RECIPIENT
	}
	feeRecipientAta := GetAssociatedTokenAddress(feeRecipient, pp.QuoteMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	// CRITICAL FIX: Use input_amount when useExactQuoteAmount=true (buy_exact_quote_in mode)
	// to avoid "insufficient funds" when buying MAX
	if params.CreateInputMintAta && quoteIsWsolOrUsdc {
		wrapAmount := params.InputAmount
		if !params.UseExactQuoteAmount {
			wrapAmount = solAmount
		}
		instructions = append(instructions, HandleWsol(params.Payer, wrapAmount)...)
	}

	// Create output token ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, pp.BaseMint, pp.BaseTokenProgram,
		))
	}

	// Build accounts array
	accounts := []solana.AccountMeta{
		{PublicKey: pp.Pool, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: PUMPSWAP_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: pp.BaseMint, IsSigner: false, IsWritable: false},
		{PublicKey: pp.QuoteMint, IsSigner: false, IsWritable: false},
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: pp.PoolBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: pp.PoolQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipientAta, IsSigner: false, IsWritable: true},
		{PublicKey: pp.BaseTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: pp.QuoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: pp.CoinCreatorVaultAta, IsSigner: false, IsWritable: true},
		{PublicKey: pp.CoinCreatorVaultAuthority, IsSigner: false, IsWritable: false},
	}

	// Add volume accumulator accounts for quote buy
	if quoteIsWsolOrUsdc {
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: GLOBAL_VOLUME_ACCUMULATOR, IsSigner: false, IsWritable: true,
		})
		userVolumeAccumulator := GetUserVolumeAccumulatorPDA(params.Payer)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true,
		})
	}

	// Add fee config and program
	accounts = append(accounts,
		solana.AccountMeta{PublicKey: FEE_CONFIG, IsSigner: false, IsWritable: false},
		solana.AccountMeta{PublicKey: FEE_PROGRAM, IsSigner: false, IsWritable: false},
	)

	// Add cashback WSOL ATA if needed
	if pp.IsCashbackCoin {
		wsolAta := GetUserVolumeAccumulatorWsolAta(params.Payer)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: wsolAta, IsSigner: false, IsWritable: true,
		})
	}

	// Add pool v2 PDA
	poolV2 := GetPoolV2PDA(pp.BaseMint)
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: poolV2, IsSigner: false, IsWritable: false,
	})

	// Build instruction data
	var data []byte
	if params.UseExactQuoteAmount {
		// buy_exact_quote_in(spendable_quote_in, min_base_amount_out, track_volume)
		minBaseAmountOut, _ := calc.CalculateWithSlippageSell(tokenAmount, params.SlippageBasisPoints)
		data = make([]byte, 26)
		copy(data[0:8], PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
		binary.LittleEndian.PutUint64(data[16:24], minBaseAmountOut)
		// track_volume
		if pp.IsCashbackCoin {
			data[24] = 1
			data[25] = 1
		} else {
			data[24] = 1
			data[25] = 0
		}
	} else {
		// buy(token_amount, max_quote, track_volume)
		data = make([]byte, 26)
		copy(data[0:8], PUMPSWAP_BUY_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], tokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], solAmount)
		if pp.IsCashbackCoin {
			data[24] = 1
			data[25] = 1
		} else {
			data[24] = 1
			data[25] = 0
		}
	}

	instructions = append(instructions, solana.NewInstruction(PUMPSWAP_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	return instructions, nil
}

// BuildSellInstructions builds sell instructions for PumpSwap
// 100% port from Rust: src/instruction/pumpswap.rs build_sell_instructions
func BuildSellInstructions(params *BuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams

	// Check if pool contains WSOL or USDC
	isWsol := pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.BaseMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.BaseMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	quoteIsWsolOrUsdc := pp.QuoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.QuoteMint.Equals(constants.USDC_TOKEN_ACCOUNT)

	// Determine if has coin creator
	hasCoinCreator := !pp.CoinCreatorVaultAuthority.Equals(DEFAULT_COIN_CREATOR_VAULT_AUTHORITY)

	// Calculate trade amounts
	tokenAmount := params.InputAmount
	var solAmount uint64

	if quoteIsWsolOrUsdc {
		result, err := calc.SellBaseInputInternal(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			hasCoinCreator,
		)
		if err != nil {
			return nil, err
		}
		solAmount = result.MinQuote
	}

	// Override sol amount if fixed output is specified
	if params.FixedOutputAmount != nil {
		solAmount = *params.FixedOutputAmount
	}

	// Get user token accounts
	userBaseTokenAccount := GetAssociatedTokenAddress(params.Payer, pp.BaseMint, pp.BaseTokenProgram)
	userQuoteTokenAccount := GetAssociatedTokenAddress(params.Payer, pp.QuoteMint, pp.QuoteTokenProgram)

	// Determine fee recipient
	var feeRecipient solana.PublicKey
	if pp.IsMayhemMode {
		feeRecipient = GetMayhemFeeRecipientRandom()
	} else {
		feeRecipient = FEE_RECIPIENT
	}
	feeRecipientAta := GetAssociatedTokenAddress(feeRecipient, pp.QuoteMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL/USDC ATA if needed for receiving
	if params.CreateOutputMintAta && quoteIsWsolOrUsdc {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, pp.QuoteMint, pp.QuoteTokenProgram,
		))
	}

	// Build accounts array
	accounts := []solana.AccountMeta{
		{PublicKey: pp.Pool, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: PUMPSWAP_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: pp.BaseMint, IsSigner: false, IsWritable: false},
		{PublicKey: pp.QuoteMint, IsSigner: false, IsWritable: false},
		{PublicKey: userBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: userQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: pp.PoolBaseTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: pp.PoolQuoteTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipientAta, IsSigner: false, IsWritable: true},
		{PublicKey: pp.BaseTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: pp.QuoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: pp.CoinCreatorVaultAta, IsSigner: false, IsWritable: true},
		{PublicKey: pp.CoinCreatorVaultAuthority, IsSigner: false, IsWritable: false},
	}

	// Add volume accumulator accounts for non-quote sell
	if !quoteIsWsolOrUsdc {
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: GLOBAL_VOLUME_ACCUMULATOR, IsSigner: false, IsWritable: true,
		})
		userVolumeAccumulator := GetUserVolumeAccumulatorPDA(params.Payer)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true,
		})
	}

	// Add fee config and program
	accounts = append(accounts,
		solana.AccountMeta{PublicKey: FEE_CONFIG, IsSigner: false, IsWritable: false},
		solana.AccountMeta{PublicKey: FEE_PROGRAM, IsSigner: false, IsWritable: false},
	)

	// Add cashback accounts if needed
	if pp.IsCashbackCoin {
		quoteAta := GetUserVolumeAccumulatorQuoteAta(params.Payer, pp.QuoteMint, pp.QuoteTokenProgram)
		userVolumeAccumulator := GetUserVolumeAccumulatorPDA(params.Payer)
		accounts = append(accounts,
			solana.AccountMeta{PublicKey: quoteAta, IsSigner: false, IsWritable: true},
			solana.AccountMeta{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		)
	}

	// Add pool v2 PDA
	poolV2 := GetPoolV2PDA(pp.BaseMint)
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: poolV2, IsSigner: false, IsWritable: false,
	})

	// Build instruction data
	data := make([]byte, 24)
	if quoteIsWsolOrUsdc {
		copy(data[0:8], PUMPSWAP_SELL_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], tokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], solAmount)
	} else {
		copy(data[0:8], PUMPSWAP_SELL_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], solAmount)
		binary.LittleEndian.PutUint64(data[16:24], tokenAmount)
	}

	instructions = append(instructions, solana.NewInstruction(PUMPSWAP_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta && quoteIsWsolOrUsdc {
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

// BuildClaimCashbackInstruction builds claim cashback instruction for PumpSwap
func BuildClaimCashbackInstruction(payer, quoteMint, quoteTokenProgram solana.PublicKey) solana.Instruction {
	userVolumeAccumulator := GetUserVolumeAccumulatorPDA(payer)
	userVolumeAccumulatorWsolAta := GetUserVolumeAccumulatorWsolAta(payer)
	userWsolAta := GetAssociatedTokenAddress(payer, quoteMint, quoteTokenProgram)

	accounts := []solana.AccountMeta{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: quoteMint, IsSigner: false, IsWritable: false},
		{PublicKey: quoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: userVolumeAccumulatorWsolAta, IsSigner: false, IsWritable: true},
		{PublicKey: userWsolAta, IsSigner: false, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPSWAP_PROGRAM, IsSigner: false, IsWritable: false},
	}

	return solana.NewInstruction(PUMPSWAP_PROGRAM, accounts, PUMPSWAP_CLAIM_CASHBACK_DISCRIMINATOR)
}

// Error definitions
var (
	ErrInvalidAmount        = fmt.Errorf("amount cannot be zero")
	ErrInvalidPool          = fmt.Errorf("pool must contain WSOL or USDC")
	ErrInvalidConfiguration = fmt.Errorf("invalid configuration for operation")
)

// ===== Pool Types and Decoding - from Rust: src/instruction/utils/pumpswap_types.rs =====

// PoolSize is the size of a PumpSwap pool account in bytes
const PoolSize = 244

// PumpSwapPool represents a decoded PumpSwap pool
type PumpSwapPool struct {
	PoolBump              uint8
	Index                 uint16
	Creator               solana.PublicKey
	BaseMint              solana.PublicKey
	QuoteMint             solana.PublicKey
	LpMint                solana.PublicKey
	PoolBaseTokenAccount  solana.PublicKey
	PoolQuoteTokenAccount solana.PublicKey
	LpSupply              uint64
	CoinCreator           solana.PublicKey
	IsMayhemMode          bool
	IsCashbackCoin        bool
}

// DecodePool decodes a PumpSwap pool from account data
// Returns nil if data is invalid or too short
func DecodePool(data []byte) *PumpSwapPool {
	if len(data) < PoolSize {
		return nil
	}

	pool := &PumpSwapPool{}
	offset := 0

	// pool_bump: u8
	pool.PoolBump = data[offset]
	offset += 1

	// index: u16
	pool.Index = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// creator: Pubkey (32 bytes)
	copy(pool.Creator[:], data[offset:offset+32])
	offset += 32

	// base_mint: Pubkey
	copy(pool.BaseMint[:], data[offset:offset+32])
	offset += 32

	// quote_mint: Pubkey
	copy(pool.QuoteMint[:], data[offset:offset+32])
	offset += 32

	// lp_mint: Pubkey
	copy(pool.LpMint[:], data[offset:offset+32])
	offset += 32

	// pool_base_token_account: Pubkey
	copy(pool.PoolBaseTokenAccount[:], data[offset:offset+32])
	offset += 32

	// pool_quote_token_account: Pubkey
	copy(pool.PoolQuoteTokenAccount[:], data[offset:offset+32])
	offset += 32

	// lp_supply: u64
	pool.LpSupply = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// coin_creator: Pubkey
	copy(pool.CoinCreator[:], data[offset:offset+32])
	offset += 32

	// is_mayhem_mode: bool
	pool.IsMayhemMode = data[offset] == 1
	offset += 1

	// is_cashback_coin: bool
	pool.IsCashbackCoin = data[offset] == 1

	return pool
}

// GetFeeConfigPDA returns the fee config PDA
// Seeds: ["fee_config", PUMPSWAP_PROGRAM], owner: FEE_PROGRAM
func GetFeeConfigPDA() solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{FEE_CONFIG_SEED, PUMPSWAP_PROGRAM[:]},
		FEE_PROGRAM,
	)
	return pda
}

// FindPoolByMint finds a PumpSwap pool by mint using multiple methods
// Search order matches @pump-fun/pump-swap-sdk:
// 1. Pool v2 PDA ["pool-v2", base_mint]
// 2. Canonical pool PDA
// This is a simplified version - full implementation would require RPC client
func FindPoolByMint(mint solana.PublicKey) solana.PublicKey {
	// Try Pool v2 PDA first
	return GetPoolV2PDA(mint)
}

// GetGlobalVolumeAccumulatorPDA returns the global volume accumulator PDA
// Seeds: ["global_volume_accumulator"], owner: PUMPSWAP_PROGRAM
func GetGlobalVolumeAccumulatorPDA() solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{GLOBAL_VOLUME_ACCUMULATOR_SEED},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// ===== Async Fetch Functions (require RPC client - stubs for interface) =====

// PoolFetcher defines interface for fetching pool data from RPC
type PoolFetcher interface {
	GetAccountInfo(pubkey solana.PublicKey) ([]byte, error)
	GetTokenAccountBalance(pubkey solana.PublicKey) (uint64, error)
}

// FetchPool fetches a PumpSwap pool from RPC.
// 100% from Rust: src/instruction/utils/pumpswap.rs fetch_pool
func FetchPool(fetcher PoolFetcher, poolAddress solana.PublicKey) (*PumpSwapPool, error) {
	data, err := fetcher.GetAccountInfo(poolAddress)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("account data too short")
	}
	pool := DecodePool(data[8:])
	if pool == nil {
		return nil, fmt.Errorf("failed to decode pool")
	}
	return pool, nil
}

// GetTokenBalances returns token balances for a pool's token accounts.
// 100% from Rust: src/instruction/utils/pumpswap.rs get_token_balances
func GetTokenBalances(fetcher PoolFetcher, pool *PumpSwapPool) (baseBalance uint64, quoteBalance uint64, err error) {
	baseBalance, err = fetcher.GetTokenAccountBalance(pool.PoolBaseTokenAccount)
	if err != nil {
		return 0, 0, err
	}
	quoteBalance, err = fetcher.GetTokenAccountBalance(pool.PoolQuoteTokenAccount)
	if err != nil {
		return 0, 0, err
	}
	return baseBalance, quoteBalance, nil
}

// FindByMint finds a PumpSwap pool by mint with RPC lookup.
// 100% from Rust: src/instruction/utils/pumpswap.rs find_by_mint
func FindByMint(fetcher PoolFetcher, mint solana.PublicKey) (*PumpSwapPool, solana.PublicKey, error) {
	// 1. Try v2 PDA
	poolV2 := GetPoolV2PDA(mint)
	data, err := fetcher.GetAccountInfo(poolV2)
	if err == nil && len(data) >= 8 {
		pool := DecodePool(data[8:])
		if pool != nil && pool.BaseMint.Equals(mint) {
			return pool, poolV2, nil
		}
	}

	// 2. Try canonical pool PDA
	canonical := GetCanonicalPoolPDA(mint)
	data, err = fetcher.GetAccountInfo(canonical)
	if err == nil && len(data) >= 8 {
		pool := DecodePool(data[8:])
		if pool != nil && pool.BaseMint.Equals(mint) {
			return pool, canonical, nil
		}
	}

	return nil, solana.PublicKey{}, fmt.Errorf("no pool found for mint %s", mint)
}

// ===== Pool Size Constants - from Rust: src/instruction/utils/pumpswap.rs =====

const (
	// PoolDataLenSPL is the pool data size for SPL Token (8 discriminator + 244 data)
	PoolDataLenSPL = 8 + 244
	// PoolDataLenT22 is the pool data size for Token2022
	PoolDataLenT22 = 643
)

// ProgramAccountsFetcher defines interface for fetching program accounts from RPC
type ProgramAccountsFetcher interface {
	GetProgramAccounts(programID solana.PublicKey, filters []AccountFilter) ([]ProgramAccountResult, error)
}

// AccountFilter represents a filter for getProgramAccounts
type AccountFilter struct {
	Memcmp *MemcmpFilter
	Size   *uint64
}

// MemcmpFilter represents a memcmp filter
type MemcmpFilter struct {
	Offset uint64
	Bytes  solana.PublicKey
}

// ProgramAccountResult represents a result from getProgramAccounts
type ProgramAccountResult struct {
	Pubkey solana.PublicKey
	Data   []byte
}

// FindByBaseMint finds a PumpSwap pool by base mint using getProgramAccounts.
// 100% from Rust: src/instruction/utils/pumpswap.rs find_by_base_mint
// base_mint offset: 8(discriminator) + 1(bump) + 2(index) + 32(creator) = 43
func FindByBaseMint(fetcher ProgramAccountsFetcher, baseMint solana.PublicKey) (*PumpSwapPool, solana.PublicKey, error) {
	// base_mint offset: 8(discriminator) + 1(bump) + 2(index) + 32(creator) = 43
	memcmpOffset := uint64(43)

	filters := []AccountFilter{
		{Memcmp: &MemcmpFilter{Offset: memcmpOffset, Bytes: baseMint}},
	}

	results, err := fetcher.GetProgramAccounts(PUMPSWAP_PROGRAM, filters)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if len(results) == 0 {
		return nil, solana.PublicKey{}, fmt.Errorf("no pool found for base_mint %s", baseMint)
	}

	// Decode and sort by lp_supply (highest first)
	type poolResult struct {
		pubkey solana.PublicKey
		pool   *PumpSwapPool
	}
	var pools []poolResult

	for _, result := range results {
		if len(result.Data) > 8 {
			pool := DecodePool(result.Data[8:])
			if pool != nil {
				pools = append(pools, poolResult{pubkey: result.Pubkey, pool: pool})
			}
		}
	}

	if len(pools) == 0 {
		return nil, solana.PublicKey{}, fmt.Errorf("no valid pool decoded for base_mint %s", baseMint)
	}

	// Sort by lp_supply descending (simple bubble sort for small arrays)
	for i := 0; i < len(pools)-1; i++ {
		for j := i + 1; j < len(pools); j++ {
			if pools[j].pool.LpSupply > pools[i].pool.LpSupply {
				pools[i], pools[j] = pools[j], pools[i]
			}
		}
	}

	return pools[0].pool, pools[0].pubkey, nil
}

// FindByQuoteMint finds a PumpSwap pool by quote mint using getProgramAccounts.
// 100% from Rust: src/instruction/utils/pumpswap.rs find_by_quote_mint
// quote_mint offset: 8 + 1 + 2 + 32 + 32 = 75
func FindByQuoteMint(fetcher ProgramAccountsFetcher, quoteMint solana.PublicKey) (*PumpSwapPool, solana.PublicKey, error) {
	// quote_mint offset: 8 + 1 + 2 + 32 + 32 = 75
	memcmpOffset := uint64(75)

	filters := []AccountFilter{
		{Memcmp: &MemcmpFilter{Offset: memcmpOffset, Bytes: quoteMint}},
	}

	results, err := fetcher.GetProgramAccounts(PUMPSWAP_PROGRAM, filters)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if len(results) == 0 {
		return nil, solana.PublicKey{}, fmt.Errorf("no pool found for quote_mint %s", quoteMint)
	}

	// Decode and sort by lp_supply (highest first)
	type poolResult struct {
		pubkey solana.PublicKey
		pool   *PumpSwapPool
	}
	var pools []poolResult

	for _, result := range results {
		if len(result.Data) > 8 {
			pool := DecodePool(result.Data[8:])
			if pool != nil {
				pools = append(pools, poolResult{pubkey: result.Pubkey, pool: pool})
			}
		}
	}

	if len(pools) == 0 {
		return nil, solana.PublicKey{}, fmt.Errorf("no valid pool decoded for quote_mint %s", quoteMint)
	}

	// Sort by lp_supply descending
	for i := 0; i < len(pools)-1; i++ {
		for j := i + 1; j < len(pools); j++ {
			if pools[j].pool.LpSupply > pools[i].pool.LpSupply {
				pools[i], pools[j] = pools[j], pools[i]
			}
		}
	}

	return pools[0].pool, pools[0].pubkey, nil
}
