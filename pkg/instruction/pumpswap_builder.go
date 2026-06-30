// PumpSwap instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	solanarpc "github.com/gagliardetto/solana-go/rpc"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
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

// Protocol extra fee recipients (Apr 2026). After pool-v2: readonly pubkey, then quote ATA (writable).
var PROTOCOL_EXTRA_FEE_RECIPIENTS = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("5YxQFdt3Tr9zJLvkFccqXVUwhdTWJQc1fFg2YPbxvxeD"),
	solana.MustPublicKeyFromBase58("9M4giFFMxmFGXtc3feFzRai56WbBqehoSeRE5GK7gf7"),
	solana.MustPublicKeyFromBase58("GXPFM2caqTtQYC2cJ5yJRi9VDkpsYZXzYdwYpGnLmtDL"),
	solana.MustPublicKeyFromBase58("3BpXnfJaUTiwXnJNe7Ej1rcbzqTTQUvLShZaWazebsVR"),
	solana.MustPublicKeyFromBase58("5cjcW9wExnJJiqgLjq7DEG75Pm6JBgE1hNv4B2vHXUW6"),
	solana.MustPublicKeyFromBase58("EHAAiTxcdDwQ3U4bU6YcMsQGaekdzLS3B5SmYo46kJtL"),
	solana.MustPublicKeyFromBase58("5eHhjP8JaYkz83CWwvGU2uMUXefd3AazWGx4gpcuEEYD"),
	solana.MustPublicKeyFromBase58("A7hAgCzFw14fejgCp387JUJRMNyz4j89JKnhtKU8piqW"),
}

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
	POOL_V2_SEED                   = []byte("pool-v2")
	POOL_SEED                      = []byte("pool")
	POOL_AUTHORITY_SEED            = []byte("pool-authority")
	USER_VOLUME_ACCUMULATOR_SEED   = []byte("user_volume_accumulator")
	CREATOR_VAULT_SEED             = []byte("creator_vault")
	FEE_CONFIG_SEED                = []byte("fee_config")
	GLOBAL_VOLUME_ACCUMULATOR_SEED = []byte("global_volume_accumulator")
)

// Fee basis points - from Rust: src/instruction/utils/pumpswap.rs
const (
	PUMPSWAP_LP_FEE_BASIS_POINTS           uint64 = 25
	PUMPSWAP_PROTOCOL_FEE_BASIS_POINTS     uint64 = 5
	PUMPSWAP_COIN_CREATOR_FEE_BASIS_POINTS uint64 = 5
	pumpSwapSPLMintSupplyOffset                   = 36
	pumpSwapSPLMintSupplyLen                      = 8
	pumpSwapFeeTierLen                            = 16 + 8*3
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetMayhemFeeRecipientRandom returns a random Mayhem fee recipient
func GetMayhemFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(MAYHEM_FEE_RECIPIENTS))))
	return MAYHEM_FEE_RECIPIENTS[n.Int64()]
}

// GetProtocolFeeRecipientRandom returns the static fallback recipient. Rust may use cached GlobalConfig when warmed.
func GetProtocolFeeRecipientRandom() solana.PublicKey {
	return FEE_RECIPIENT
}

// GetProtocolExtraFeeRecipientRandom returns a random protocol extra fee recipient (PumpSwap Apr 2026).
func GetProtocolExtraFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(PROTOCOL_EXTRA_FEE_RECIPIENTS))))
	return PROTOCOL_EXTRA_FEE_RECIPIENTS[n.Int64()]
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

// GetCoinCreatorVaultAta returns the coin creator vault ATA for the quote mint.
func GetCoinCreatorVaultAta(coinCreator, quoteMint solana.PublicKey) solana.PublicKey {
	authority := GetCoinCreatorVaultAuthority(coinCreator)
	return GetAssociatedTokenAddress(authority, quoteMint, constants.TOKEN_PROGRAM)
}

// GetPumpSwapUserVolumeAccumulatorPDA returns the user volume accumulator PDA for PumpSwap.
func GetPumpSwapUserVolumeAccumulatorPDA(user solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{USER_VOLUME_ACCUMULATOR_SEED, user[:]},
		PUMPSWAP_PROGRAM,
	)
	return pda
}

// GetUserVolumeAccumulatorWsolAta returns the WSOL ATA of UserVolumeAccumulator
func GetUserVolumeAccumulatorWsolAta(user solana.PublicKey) solana.PublicKey {
	accumulator := GetPumpSwapUserVolumeAccumulatorPDA(user)
	return GetAssociatedTokenAddress(accumulator, constants.WSOL_TOKEN_ACCOUNT, constants.TOKEN_PROGRAM)
}

// GetUserVolumeAccumulatorQuoteAta returns the quote-mint ATA of UserVolumeAccumulator
func GetUserVolumeAccumulatorQuoteAta(user, quoteMint, quoteTokenProgram solana.PublicKey) solana.PublicKey {
	accumulator := GetPumpSwapUserVolumeAccumulatorPDA(user)
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
	PoolCreator               solana.PublicKey
	CoinCreator               solana.PublicKey
	CoinCreatorKnown          bool
	CashbackFeeBasisPoints    uint64
	FeeBasisPoints            *calc.PumpSwapFeeBasisPoints
	BaseMintSupply            *uint64
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

	// Transfer native SOL lamports to the WSOL ATA.
	instructions = append(instructions, system.NewTransferInstruction(
		amount,
		owner,
		wsolAta,
	).Build())

	// Sync native
	instructions = append(instructions, token.NewSyncNativeInstruction(
		wsolAta,
	).Build())

	return instructions
}

// HandleWsolForMint mirrors Rust push_create_or_wrap_user_token_account.
func HandleWsolForMint(owner, mint, tokenProgram solana.PublicKey, amount uint64) []solana.Instruction {
	if mint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		return HandleWsol(owner, amount)
	}
	return []solana.Instruction{CreateAssociatedTokenAccountIdempotent(owner, owner, mint, tokenProgram)}
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

// CloseWsolForMint mirrors Rust push_close_wsol_if_needed.
func CloseWsolForMint(owner, mint, tokenProgram solana.PublicKey) solana.Instruction {
	if !mint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		return nil
	}
	ata := GetAssociatedTokenAddress(owner, mint, tokenProgram)
	return token.NewCloseAccountInstruction(
		ata,
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
	}

	// Idempotent discriminator = 1
	data := []byte{1}

	return newInstruction(constants.ASSOCIATED_TOKEN_PROGRAM_ID, accounts, data)
}

// ===== Instruction Builders - 100% from Rust =====

func effectivePumpSwapFeeBasisPoints(pp *PumpSwapParams) calc.PumpSwapFeeBasisPoints {
	hasCoinCreator := false
	if !pp.CoinCreatorKnown {
		hasCoinCreator = !pp.CoinCreatorVaultAuthority.Equals(DEFAULT_COIN_CREATOR_VAULT_AUTH)
	} else {
		hasCoinCreator = !pp.CoinCreator.IsZero()
	}

	fees := calc.LegacyPumpSwapFeeBasisPoints(hasCoinCreator)
	if pp.FeeBasisPoints != nil {
		fees = *pp.FeeBasisPoints
		if !hasCoinCreator {
			fees.CoinCreatorFeeBasisPoints = 0
		}
	}
	fees.CoinCreatorFeeBasisPoints += pp.CashbackFeeBasisPoints
	return fees
}

func shouldAddPoolV2(pp *PumpSwapParams) bool {
	if !pp.CoinCreatorKnown {
		return true
	}
	return !pp.CoinCreator.IsZero()
}

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
	inputStableMint := pp.QuoteMint
	inputStableTokenProgram := pp.QuoteTokenProgram
	outputTradeMint := pp.BaseMint
	outputTradeTokenProgram := pp.BaseTokenProgram
	if !quoteIsWsolOrUsdc {
		inputStableMint = pp.BaseMint
		inputStableTokenProgram = pp.BaseTokenProgram
		outputTradeMint = pp.QuoteMint
		outputTradeTokenProgram = pp.QuoteTokenProgram
	}

	feeBasisPoints := effectivePumpSwapFeeBasisPoints(pp)

	// Calculate trade amounts
	var tokenAmount uint64
	var solAmount uint64

	if quoteIsWsolOrUsdc {
		result, err := calc.BuyQuoteInputInternalWithFees(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			feeBasisPoints,
		)
		if err != nil {
			return nil, err
		}
		tokenAmount = result.Base
		solAmount = result.MaxQuote
	} else {
		result, err := calc.SellBaseInputInternalWithFees(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			feeBasisPoints,
		)
		if err != nil {
			return nil, err
		}
		tokenAmount = result.MinQuote
		solAmount = params.InputAmount
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
		feeRecipient = GetProtocolFeeRecipientRandom()
	}
	feeRecipientAta := GetAssociatedTokenAddress(feeRecipient, pp.QuoteMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	// CRITICAL FIX: Use input_amount when useExactQuoteAmount=true (buy_exact_quote_in mode)
	// to avoid "insufficient funds" when buying MAX
	if params.CreateInputMintAta {
		wrapAmount := params.InputAmount
		if !params.UseExactQuoteAmount {
			wrapAmount = solAmount
		}
		instructions = append(instructions, HandleWsolForMint(params.Payer, inputStableMint, inputStableTokenProgram, wrapAmount)...)
	}

	// Create output token ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputTradeMint, outputTradeTokenProgram,
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
		userVolumeAccumulator := GetPumpSwapUserVolumeAccumulatorPDA(params.Payer)
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

	if shouldAddPoolV2(pp) {
		poolV2 := GetPoolV2PDA(pp.BaseMint)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: poolV2, IsSigner: false, IsWritable: false,
		})
	}
	protocolExtra := GetProtocolExtraFeeRecipientRandom()
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: protocolExtra, IsSigner: false, IsWritable: false,
	})
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: GetAssociatedTokenAddress(protocolExtra, pp.QuoteMint, constants.TOKEN_PROGRAM), IsSigner: false, IsWritable: true,
	})

	// Build instruction data
	var data []byte
	trackVolume := byte(0)
	if pp.IsCashbackCoin {
		trackVolume = 1
	}
	if params.FixedOutputAmount != nil {
		data = make([]byte, 25)
		copy(data[0:8], PUMPSWAP_BUY_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], tokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], solAmount)
		data[24] = trackVolume
	} else if quoteIsWsolOrUsdc && params.UseExactQuoteAmount {
		// buy_exact_quote_in(spendable_quote_in, min_base_amount_out, track_volume)
		minBaseAmountOut, _ := calc.CalculateWithSlippageSell(tokenAmount, params.SlippageBasisPoints)
		data = make([]byte, 25)
		copy(data[0:8], PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
		binary.LittleEndian.PutUint64(data[16:24], minBaseAmountOut)
		data[24] = trackVolume
	} else if quoteIsWsolOrUsdc {
		// buy(token_amount, max_quote, track_volume)
		data = make([]byte, 25)
		copy(data[0:8], PUMPSWAP_BUY_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], tokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], solAmount)
		data[24] = trackVolume
	} else {
		data = make([]byte, 24)
		copy(data[0:8], PUMPSWAP_SELL_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], solAmount)
		binary.LittleEndian.PutUint64(data[16:24], tokenAmount)
	}

	instructions = append(instructions, newInstruction(PUMPSWAP_PROGRAM, accounts, data))

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
	outputStableMint := pp.QuoteMint
	outputStableTokenProgram := pp.QuoteTokenProgram
	if !quoteIsWsolOrUsdc {
		outputStableMint = pp.BaseMint
		outputStableTokenProgram = pp.BaseTokenProgram
	}

	feeBasisPoints := effectivePumpSwapFeeBasisPoints(pp)

	// Calculate trade amounts
	tokenAmount := params.InputAmount
	var solAmount uint64

	if quoteIsWsolOrUsdc {
		result, err := calc.SellBaseInputInternalWithFees(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			feeBasisPoints,
		)
		if err != nil {
			return nil, err
		}
		solAmount = result.MinQuote
	} else {
		result, err := calc.BuyQuoteInputInternalWithFees(
			params.InputAmount,
			params.SlippageBasisPoints,
			pp.PoolBaseTokenReserves,
			pp.PoolQuoteTokenReserves,
			feeBasisPoints,
		)
		if err != nil {
			return nil, err
		}
		tokenAmount = result.MaxQuote
		solAmount = result.Base
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
		feeRecipient = GetProtocolFeeRecipientRandom()
	}
	feeRecipientAta := GetAssociatedTokenAddress(feeRecipient, pp.QuoteMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL/USDC ATA if needed for receiving
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputStableMint, outputStableTokenProgram,
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
		userVolumeAccumulator := GetPumpSwapUserVolumeAccumulatorPDA(params.Payer)
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
		userVolumeAccumulator := GetPumpSwapUserVolumeAccumulatorPDA(params.Payer)
		accounts = append(accounts,
			solana.AccountMeta{PublicKey: quoteAta, IsSigner: false, IsWritable: true},
			solana.AccountMeta{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		)
	}

	if shouldAddPoolV2(pp) {
		poolV2 := GetPoolV2PDA(pp.BaseMint)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: poolV2, IsSigner: false, IsWritable: false,
		})
	}
	protocolExtra := GetProtocolExtraFeeRecipientRandom()
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: protocolExtra, IsSigner: false, IsWritable: false,
	})
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: GetAssociatedTokenAddress(protocolExtra, pp.QuoteMint, constants.TOKEN_PROGRAM), IsSigner: false, IsWritable: true,
	})

	// Build instruction data
	data := make([]byte, 24)
	if quoteIsWsolOrUsdc {
		copy(data[0:8], PUMPSWAP_SELL_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], tokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], solAmount)
	} else {
		copy(data[0:8], PUMPSWAP_BUY_DISCRIMINATOR)
		binary.LittleEndian.PutUint64(data[8:16], solAmount)
		binary.LittleEndian.PutUint64(data[16:24], tokenAmount)
	}

	instructions = append(instructions, newInstruction(PUMPSWAP_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta {
		if closeIx := CloseWsolForMint(params.Payer, outputStableMint, outputStableTokenProgram); closeIx != nil {
			instructions = append(instructions, closeIx)
		}
	}

	// Close base token account if requested
	if params.CloseInputMintAta {
		inputAccount := userBaseTokenAccount
		inputProgram := pp.BaseTokenProgram
		if !quoteIsWsolOrUsdc {
			inputAccount = userQuoteTokenAccount
			inputProgram = pp.QuoteTokenProgram
		}
		instructions = append(instructions, newInstruction(
			inputProgram,
			[]solana.AccountMeta{
				{PublicKey: inputAccount, IsSigner: false, IsWritable: true},
				{PublicKey: params.Payer, IsSigner: false, IsWritable: true},
				{PublicKey: params.Payer, IsSigner: true, IsWritable: false},
			},
			[]byte{9},
		))
	}

	return instructions, nil
}

// BuildClaimCashbackInstruction builds claim cashback instruction for PumpSwap
func BuildClaimCashbackInstruction(payer, quoteMint, quoteTokenProgram solana.PublicKey) solana.Instruction {
	userVolumeAccumulator := GetPumpSwapUserVolumeAccumulatorPDA(payer)
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

	return newInstruction(PUMPSWAP_PROGRAM, accounts, PUMPSWAP_CLAIM_CASHBACK_DISCRIMINATOR)
}

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

type PumpSwapFeeTier struct {
	MarketCapLamportsThreshold *big.Int
	Fees                       calc.PumpSwapFeeBasisPoints
}

type PumpSwapFeeConfig struct {
	FlatFees       calc.PumpSwapFeeBasisPoints
	FeeTiers       []PumpSwapFeeTier
	StableFeeTiers []PumpSwapFeeTier
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

func DecodeMintSupply(data []byte) (uint64, bool) {
	end := pumpSwapSPLMintSupplyOffset + pumpSwapSPLMintSupplyLen
	if len(data) < end {
		return 0, false
	}
	return binary.LittleEndian.Uint64(data[pumpSwapSPLMintSupplyOffset:end]), true
}

func decodePumpSwapFees(data []byte, offset int) (calc.PumpSwapFeeBasisPoints, bool) {
	if len(data) < offset+24 {
		return calc.PumpSwapFeeBasisPoints{}, false
	}
	return calc.PumpSwapFeeBasisPoints{
		LPFeeBasisPoints:          binary.LittleEndian.Uint64(data[offset : offset+8]),
		ProtocolFeeBasisPoints:    binary.LittleEndian.Uint64(data[offset+8 : offset+16]),
		CoinCreatorFeeBasisPoints: binary.LittleEndian.Uint64(data[offset+16 : offset+24]),
	}, true
}

func decodePumpSwapFeeTiers(data []byte, offset int) ([]PumpSwapFeeTier, int, bool) {
	if len(data) < offset+4 {
		return nil, offset, false
	}
	count := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	byteLen := count * pumpSwapFeeTierLen
	if len(data) < offset+byteLen {
		return nil, offset, false
	}
	tiers := make([]PumpSwapFeeTier, 0, count)
	for i := 0; i < count; i++ {
		lo := binary.LittleEndian.Uint64(data[offset : offset+8])
		hi := binary.LittleEndian.Uint64(data[offset+8 : offset+16])
		threshold := new(big.Int).SetUint64(hi)
		threshold.Lsh(threshold, 64)
		threshold.Add(threshold, new(big.Int).SetUint64(lo))
		offset += 16
		fees, ok := decodePumpSwapFees(data, offset)
		if !ok {
			return nil, offset, false
		}
		offset += 24
		tiers = append(tiers, PumpSwapFeeTier{
			MarketCapLamportsThreshold: threshold,
			Fees:                       fees,
		})
	}
	return tiers, offset, true
}

func DecodeFeeConfig(data []byte) *PumpSwapFeeConfig {
	offset := 8  // discriminator
	offset++     // bump
	offset += 32 // admin
	flatFees, ok := decodePumpSwapFees(data, offset)
	if !ok {
		return nil
	}
	offset += 24

	feeTiers, next, ok := decodePumpSwapFeeTiers(data, offset)
	if !ok {
		return nil
	}
	offset = next
	stableFeeTiers, _, ok := decodePumpSwapFeeTiers(data, offset)
	if !ok {
		return nil
	}
	return &PumpSwapFeeConfig{
		FlatFees:       flatFees,
		FeeTiers:       feeTiers,
		StableFeeTiers: stableFeeTiers,
	}
}

func FetchFeeConfig(fetcher PoolFetcher) (*PumpSwapFeeConfig, error) {
	data, err := fetcher.GetAccountInfo(FEE_CONFIG)
	if err != nil {
		return nil, err
	}
	config := DecodeFeeConfig(data)
	if config == nil {
		return nil, fmt.Errorf("failed to decode PumpSwap fee config")
	}
	return config, nil
}

func IsCanonicalPumpPool(baseMint, poolCreator solana.PublicKey) bool {
	return GetPumpPoolAuthorityPDA(baseMint).Equals(poolCreator)
}

func PoolMarketCapLamports(baseMintSupply, baseReserve, quoteReserve uint64) (*big.Int, bool) {
	if baseReserve == 0 {
		return nil, false
	}
	value := new(big.Int).SetUint64(quoteReserve)
	value.Mul(value, new(big.Int).SetUint64(baseMintSupply))
	value.Div(value, new(big.Int).SetUint64(baseReserve))
	return value, true
}

func CalculateFeeTier(feeTiers []PumpSwapFeeTier, marketCapLamports *big.Int) (*calc.PumpSwapFeeBasisPoints, bool) {
	if len(feeTiers) == 0 || marketCapLamports == nil {
		return nil, false
	}
	first := feeTiers[0]
	if marketCapLamports.Cmp(first.MarketCapLamportsThreshold) < 0 {
		fees := first.Fees
		return &fees, true
	}
	for i := len(feeTiers) - 1; i >= 0; i-- {
		tier := feeTiers[i]
		if marketCapLamports.Cmp(tier.MarketCapLamportsThreshold) >= 0 {
			fees := tier.Fees
			return &fees, true
		}
	}
	fees := first.Fees
	return &fees, true
}

func ComputePumpSwapFeeBasisPoints(
	feeConfig *PumpSwapFeeConfig,
	poolCreator, baseMint solana.PublicKey,
	baseMintSupply *uint64,
	baseReserve, quoteReserve uint64,
) calc.PumpSwapFeeBasisPoints {
	if feeConfig == nil {
		return calc.LegacyPumpSwapFeeBasisPoints(true)
	}
	if !IsCanonicalPumpPool(baseMint, poolCreator) {
		return feeConfig.FlatFees
	}
	if baseMintSupply == nil {
		return calc.LegacyPumpSwapFeeBasisPoints(true)
	}
	marketCap, ok := PoolMarketCapLamports(*baseMintSupply, baseReserve, quoteReserve)
	if !ok {
		return calc.LegacyPumpSwapFeeBasisPoints(true)
	}
	if fees, ok := CalculateFeeTier(feeConfig.FeeTiers, marketCap); ok {
		return *fees
	}
	return feeConfig.FlatFees
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

type RPCPoolFetcher struct {
	ctx    context.Context
	client *solanarpc.Client
}

func NewRPCPoolFetcher(ctx context.Context, client *solanarpc.Client) *RPCPoolFetcher {
	return &RPCPoolFetcher{ctx: ctx, client: client}
}

func (f *RPCPoolFetcher) GetAccountInfo(pubkey solana.PublicKey) ([]byte, error) {
	account, err := f.client.GetAccountInfo(f.ctx, pubkey)
	if err != nil {
		return nil, err
	}
	data := account.GetBinary()
	if data == nil {
		return nil, fmt.Errorf("account %s not found or empty", pubkey)
	}
	return data, nil
}

func (f *RPCPoolFetcher) GetTokenAccountBalance(pubkey solana.PublicKey) (uint64, error) {
	balance, err := f.client.GetTokenAccountBalance(f.ctx, pubkey, solanarpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Value == nil {
		return 0, fmt.Errorf("token account %s balance not found", pubkey)
	}
	return strconv.ParseUint(balance.Value.Amount, 10, 64)
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

func NewPumpSwapParamsFromPoolData(
	fetcher PoolFetcher,
	poolAddress solana.PublicKey,
	pool *PumpSwapPool,
	feeBasisPoints *calc.PumpSwapFeeBasisPoints,
) (*PumpSwapParams, error) {
	baseBalance, quoteBalance, err := GetTokenBalances(fetcher, pool)
	if err != nil {
		return nil, err
	}

	baseTokenProgramAta := GetAssociatedTokenAddress(poolAddress, pool.BaseMint, constants.TOKEN_PROGRAM)
	quoteTokenProgramAta := GetAssociatedTokenAddress(poolAddress, pool.QuoteMint, constants.TOKEN_PROGRAM)
	baseTokenProgram := constants.TOKEN_PROGRAM
	if !pool.PoolBaseTokenAccount.Equals(baseTokenProgramAta) {
		baseTokenProgram = constants.TOKEN_PROGRAM_2022
	}
	quoteTokenProgram := constants.TOKEN_PROGRAM
	if !pool.PoolQuoteTokenAccount.Equals(quoteTokenProgramAta) {
		quoteTokenProgram = constants.TOKEN_PROGRAM_2022
	}

	var baseMintSupply *uint64
	if mintData, err := fetcher.GetAccountInfo(pool.BaseMint); err == nil {
		if supply, ok := DecodeMintSupply(mintData); ok {
			baseMintSupply = &supply
		}
	}

	effectiveFees := calc.LegacyPumpSwapFeeBasisPoints(true)
	if feeBasisPoints != nil {
		effectiveFees = *feeBasisPoints
	} else {
		feeConfig, _ := FetchFeeConfig(fetcher)
		effectiveFees = ComputePumpSwapFeeBasisPoints(
			feeConfig,
			pool.Creator,
			pool.BaseMint,
			baseMintSupply,
			baseBalance,
			quoteBalance,
		)
	}
	if pool.CoinCreator.IsZero() {
		effectiveFees.CoinCreatorFeeBasisPoints = 0
	}

	return &PumpSwapParams{
		Pool:                      poolAddress,
		BaseMint:                  pool.BaseMint,
		QuoteMint:                 pool.QuoteMint,
		PoolBaseTokenAccount:      pool.PoolBaseTokenAccount,
		PoolQuoteTokenAccount:     pool.PoolQuoteTokenAccount,
		PoolBaseTokenReserves:     baseBalance,
		PoolQuoteTokenReserves:    quoteBalance,
		CoinCreatorVaultAta:       GetCoinCreatorVaultAta(pool.CoinCreator, pool.QuoteMint),
		CoinCreatorVaultAuthority: GetCoinCreatorVaultAuthority(pool.CoinCreator),
		BaseTokenProgram:          baseTokenProgram,
		QuoteTokenProgram:         quoteTokenProgram,
		IsMayhemMode:              pool.IsMayhemMode,
		IsCashbackCoin:            pool.IsCashbackCoin,
		PoolCreator:               pool.Creator,
		CoinCreator:               pool.CoinCreator,
		CoinCreatorKnown:          true,
		FeeBasisPoints:            &effectiveFees,
		BaseMintSupply:            baseMintSupply,
	}, nil
}

func NewPumpSwapParamsFromPoolAddress(
	fetcher PoolFetcher,
	poolAddress solana.PublicKey,
	feeBasisPoints *calc.PumpSwapFeeBasisPoints,
) (*PumpSwapParams, error) {
	pool, err := FetchPool(fetcher, poolAddress)
	if err != nil {
		return nil, err
	}
	return NewPumpSwapParamsFromPoolData(fetcher, poolAddress, pool, feeBasisPoints)
}

func NewPumpSwapParamsFromPoolAddressByRPC(
	ctx context.Context,
	client *solanarpc.Client,
	poolAddress solana.PublicKey,
	feeBasisPoints *calc.PumpSwapFeeBasisPoints,
) (*PumpSwapParams, error) {
	return NewPumpSwapParamsFromPoolAddress(
		NewRPCPoolFetcher(ctx, client),
		poolAddress,
		feeBasisPoints,
	)
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

func NewPumpSwapParamsFromMint(
	fetcher PoolFetcher,
	mint solana.PublicKey,
	feeBasisPoints *calc.PumpSwapFeeBasisPoints,
) (*PumpSwapParams, error) {
	pool, poolAddress, err := FindByMint(fetcher, mint)
	if err != nil {
		return nil, err
	}
	return NewPumpSwapParamsFromPoolData(fetcher, poolAddress, pool, feeBasisPoints)
}

func NewPumpSwapParamsFromMintByRPC(
	ctx context.Context,
	client *solanarpc.Client,
	mint solana.PublicKey,
	feeBasisPoints *calc.PumpSwapFeeBasisPoints,
) (*PumpSwapParams, error) {
	return NewPumpSwapParamsFromMint(
		NewRPCPoolFetcher(ctx, client),
		mint,
		feeBasisPoints,
	)
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
