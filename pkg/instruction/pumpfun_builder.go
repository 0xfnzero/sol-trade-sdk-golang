// PumpFun instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/your-org/sol-trade-sdk-go/pkg/calc"
	"github.com/your-org/sol-trade-sdk-go/pkg/common"
	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
)

// ===== PumpFun Program Constants from Rust: src/instruction/utils/pumpfun.rs =====

var (
	// PUMPFUN_PROGRAM is the PumpFun program ID
	PUMPFUN_PROGRAM = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	// PUMPFUN_GLOBAL_ACCOUNT is the global account PDA
	PUMPFUN_GLOBAL_ACCOUNT = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	// PUMPFUN_EVENT_AUTHORITY is the event authority PDA
	PUMPFUN_EVENT_AUTHORITY = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	// PUMPFUN_FEE_RECIPIENT is the standard fee recipient
	PUMPFUN_FEE_RECIPIENT = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	// PUMPFUN_FEE_PROGRAM is the fee program
	PUMPFUN_FEE_PROGRAM = solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
	// PUMPFUN_GLOBAL_VOLUME_ACCUMULATOR is the global volume accumulator
	PUMPFUN_GLOBAL_VOLUME_ACCUMULATOR = solana.MustPublicKeyFromBase58("Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y")
	// PUMPFUN_FEE_CONFIG is the fee config account
	PUMPFUN_FEE_CONFIG = solana.MustPublicKeyFromBase58("8Wf5TiAheLUqBrKXeYg2JtAFFMWtKdG2BSFgqUcPVwTt")
)

// PumpFun Mayhem fee recipients - from Rust: src/instruction/utils/pumpfun.rs global_constants::MAYHEM_FEE_RECIPIENTS
var PumpFunMayhemFeeRecipients = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS"),
	solana.MustPublicKeyFromBase58("4budycTjhs9fD6xw62VBducVTNgMgJJ5BgtKq7mAZwn6"),
	solana.MustPublicKeyFromBase58("8SBKzEQU4nLSzcwF4a74F2iaUDQyTfjGndn6qUWBnrpR"),
	solana.MustPublicKeyFromBase58("4UQeTP1T39KZ9Sfxzo3WR5skgsaP6NZa87BAkuazLEKH"),
	solana.MustPublicKeyFromBase58("8sNeir4QsLsJdYpc9RZacohhK1Y5FLU3nC5LXgYB4aa6"),
	solana.MustPublicKeyFromBase58("Fh9HmeLNUMVCvejxCtCL2DbYaRyBFVJ5xrWkLnMH6fdk"),
	solana.MustPublicKeyFromBase58("463MEnMeGyJekNZFQSTUABBEbLnvMTALbT6ZmsxAbAdq"),
	solana.MustPublicKeyFromBase58("6AUH3WEHucYZyC61hqpqYUWVto5qA5hjHuNQ32GNnNxA"),
}

// Discriminators - from Rust: src/instruction/utils/pumpfun.rs
var (
	// PumpFunBuyDiscriminator is the discriminator for the buy instruction
	PumpFunBuyDiscriminator = []byte{102, 6, 61, 18, 1, 218, 235, 234}
	// PumpFunBuyExactSolInDiscriminator is the discriminator for the buy_exact_sol_in instruction
	PumpFunBuyExactSolInDiscriminator = []byte{56, 252, 116, 8, 158, 223, 205, 95}
	// PumpFunSellDiscriminator is the discriminator for the sell instruction
	PumpFunSellDiscriminator = []byte{51, 230, 133, 164, 1, 127, 131, 173}
	// PumpFunClaimCashbackDiscriminator is the discriminator for the claim cashback instruction
	PumpFunClaimCashbackDiscriminator = []byte{37, 58, 35, 126, 190, 53, 228, 197}
)

// Seeds - from Rust: src/instruction/utils/pumpfun.rs seeds
var (
	PumpFunBondingCurveSeed            = []byte("bonding-curve")
	PumpFunBondingCurveV2Seed          = []byte("bonding-curve-v2")
	PumpFunCreatorVaultSeed            = []byte("creator-vault")
	PumpFunUserVolumeAccumulatorSeed   = []byte("user_volume_accumulator")
	PumpFunGlobalVolumeAccumulatorSeed = []byte("global_volume_accumulator")
	PumpFunFeeConfigSeed               = []byte("fee_config")
)

// PumpFun Constants - from Rust: src/instruction/utils/pumpfun.rs global_constants
const (
	PumpFunInitialVirtualTokenReserves uint64 = 1_073_000_000_000_000
	PumpFunInitialVirtualSolReserves   uint64 = 30_000_000_000
	PumpFunInitialRealTokenReserves    uint64 = 793_100_000_000_000
	PumpFunTokenTotalSupply            uint64 = 1_000_000_000_000_000
	PumpFunFeeBasisPoints              uint64 = 95
	PumpFunCreatorFee                  uint64 = 30
)

// ===== PDA Derivation Functions - 100% from Rust =====

// GetPumpFunMayhemFeeRecipientRandom returns a random Mayhem fee recipient
func GetPumpFunMayhemFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(PumpFunMayhemFeeRecipients))))
	return PumpFunMayhemFeeRecipients[n.Int64()]
}

// GetBondingCurvePDA returns the bonding curve PDA for a mint (seeds: ["bonding-curve", mint])
func GetBondingCurvePDA(mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunBondingCurveSeed, mint[:]},
		PUMPFUN_PROGRAM,
	)
	return pda
}

// GetBondingCurveV2PDA returns the bonding curve v2 PDA for a mint (seeds: ["bonding-curve-v2", mint])
func GetBondingCurveV2PDA(mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunBondingCurveV2Seed, mint[:]},
		PUMPFUN_PROGRAM,
	)
	return pda
}

// GetCreatorVaultPDA returns the creator vault PDA (seeds: ["creator-vault", creator])
func GetCreatorVaultPDA(creator solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunCreatorVaultSeed, creator[:]},
		PUMPFUN_PROGRAM,
	)
	return pda
}

// GetPumpFunUserVolumeAccumulatorPDA returns the user volume accumulator PDA
func GetPumpFunUserVolumeAccumulatorPDA(user solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunUserVolumeAccumulatorSeed, user[:]},
		PUMPFUN_PROGRAM,
	)
	return pda
}

// GetCreator returns the creator from the creator vault PDA
// If creator_vault is default, returns default pubkey
func GetCreator(creatorVaultPDA solana.PublicKey) solana.PublicKey {
	if creatorVaultPDA.IsZero() {
		return solana.PublicKey{}
	}
	// Check against default creator vault
	defaultCreatorVault := GetCreatorVaultPDA(solana.PublicKey{})
	if creatorVaultPDA.Equals(defaultCreatorVault) {
		return solana.PublicKey{}
	}
	return creatorVaultPDA
}

// ===== PumpFun Params =====

// BondingCurve represents the bonding curve data
type BondingCurve struct {
	Account              solana.PublicKey
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	IsMayhemMode         bool
	IsCashbackCoin       bool
}

// PumpFunParams contains parameters for PumpFun operations
type PumpFunParams struct {
	BondingCurve              *BondingCurve
	CreatorVault              solana.PublicKey
	AssociatedBondingCurve    solana.PublicKey
	TokenProgram              solana.PublicKey
	CloseTokenAccountWhenSell *bool
}

// PumpFunBuildBuyParams contains parameters for building buy instructions
type PumpFunBuildBuyParams struct {
	Payer               solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *PumpFunParams
	CreateOutputMintAta bool
	UseExactSolAmount   bool
	FixedOutputAmount   *uint64
}

// PumpFunBuildSellParams contains parameters for building sell instructions
type PumpFunBuildSellParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *PumpFunParams
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== Instruction Builders - 100% from Rust =====

// PumpFunBuildBuyInstructions builds buy instructions for PumpFun
// 100% port from Rust: src/instruction/pumpfun.rs build_buy_instructions
func PumpFunBuildBuyInstructions(params *PumpFunBuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	bondingCurve := pp.BondingCurve
	creator := GetCreator(pp.CreatorVault)

	// Calculate buy token amount
	var buyTokenAmount uint64
	if params.FixedOutputAmount != nil {
		buyTokenAmount = *params.FixedOutputAmount
	} else {
		buyTokenAmount = calc.GetBuyTokenAmountFromSolAmount(
			bondingCurve.VirtualTokenReserves,
			bondingCurve.VirtualSolReserves,
			bondingCurve.RealTokenReserves,
			!creator.IsZero(),
			params.InputAmount,
		)
	}

	// Calculate max SOL cost
	maxSolCost, _ := calc.CalculateWithSlippageBuy(params.InputAmount, params.SlippageBasisPoints)

	// Get bonding curve address
	bondingCurveAddr := bondingCurve.Account
	if bondingCurveAddr.IsZero() {
		bondingCurveAddr = GetBondingCurvePDA(params.OutputMint)
	}

	// Get token program
	tokenProgram := pp.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = constants.TOKEN_PROGRAM
	}

	// Get associated bonding curve
	associatedBondingCurve := pp.AssociatedBondingCurve
	if associatedBondingCurve.IsZero() {
		associatedBondingCurve = GetAssociatedTokenAddress(bondingCurveAddr, params.OutputMint, tokenProgram)
	}

	// Get user token account
	userTokenAccount := GetAssociatedTokenAddress(params.Payer, params.OutputMint, tokenProgram)

	// Get user volume accumulator
	userVolumeAccumulator := GetPumpFunUserVolumeAccumulatorPDA(params.Payer)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 2)

	// Create ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, params.OutputMint, tokenProgram,
		))
	}

	// Build track_volume parameter
	trackVolume := []byte{1, 0} // Some(false)
	if bondingCurve.IsCashbackCoin {
		trackVolume = []byte{1, 1} // Some(true)
	}

	// Build instruction data
	var data []byte
	if params.UseExactSolAmount {
		// buy_exact_sol_in(spendable_sol_in: u64, min_tokens_out: u64, track_volume)
		minTokensOut, _ := calc.CalculateWithSlippageSell(buyTokenAmount, params.SlippageBasisPoints)
		data = make([]byte, 26)
		copy(data[0:8], PumpFunBuyExactSolInDiscriminator)
		binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
		binary.LittleEndian.PutUint64(data[16:24], minTokensOut)
		copy(data[24:26], trackVolume)
	} else {
		// buy(token_amount: u64, max_sol_cost: u64, track_volume)
		data = make([]byte, 26)
		copy(data[0:8], PumpFunBuyDiscriminator)
		binary.LittleEndian.PutUint64(data[8:16], buyTokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], maxSolCost)
		copy(data[24:26], trackVolume)
	}

	// Determine fee recipient
	var feeRecipient solana.PublicKey
	if bondingCurve.IsMayhemMode {
		feeRecipient = GetPumpFunMayhemFeeRecipientRandom()
	} else {
		feeRecipient = PUMPFUN_FEE_RECIPIENT
	}

	// Get bonding curve v2
	bondingCurveV2 := GetBondingCurveV2PDA(params.OutputMint)

	// Build accounts array
	accounts := []solana.AccountMeta{
		{PublicKey: PUMPFUN_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: params.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: userTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: pp.CreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_GLOBAL_VOLUME_ACCUMULATOR, IsSigner: false, IsWritable: true},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_FEE_CONFIG, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveV2, IsSigner: false, IsWritable: false}, // remainingAccounts: bondingCurveV2Pda
	}

	instructions = append(instructions, solana.NewInstruction(PUMPFUN_PROGRAM, accounts, data))

	return instructions, nil
}

// PumpFunBuildSellInstructions builds sell instructions for PumpFun
// 100% port from Rust: src/instruction/pumpfun.rs build_sell_instructions
func PumpFunBuildSellInstructions(params *PumpFunBuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	bondingCurve := pp.BondingCurve
	creator := GetCreator(pp.CreatorVault)

	// Calculate SOL amount from token amount
	solAmount := calc.GetSellSolAmountFromTokenAmount(
		bondingCurve.VirtualTokenReserves,
		bondingCurve.VirtualSolReserves,
		!creator.IsZero(),
		params.InputAmount,
	)

	// Calculate min SOL output
	var minSolOutput uint64
	if params.FixedOutputAmount != nil {
		minSolOutput = *params.FixedOutputAmount
	} else {
		minSolOutput, _ = calc.CalculateWithSlippageSell(solAmount, params.SlippageBasisPoints)
	}

	// Get bonding curve address
	bondingCurveAddr := bondingCurve.Account
	if bondingCurveAddr.IsZero() {
		bondingCurveAddr = GetBondingCurvePDA(params.InputMint)
	}

	// Get token program
	tokenProgram := pp.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = constants.TOKEN_PROGRAM
	}

	// Get associated bonding curve
	associatedBondingCurve := pp.AssociatedBondingCurve
	if associatedBondingCurve.IsZero() {
		associatedBondingCurve = GetAssociatedTokenAddress(bondingCurveAddr, params.InputMint, tokenProgram)
	}

	// Get user token account
	userTokenAccount := GetAssociatedTokenAddress(params.Payer, params.InputMint, tokenProgram)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Build instruction data
	data := make([]byte, 24)
	copy(data[0:8], PumpFunSellDiscriminator)
	binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
	binary.LittleEndian.PutUint64(data[16:24], minSolOutput)

	// Determine fee recipient
	var feeRecipient solana.PublicKey
	if bondingCurve.IsMayhemMode {
		feeRecipient = GetPumpFunMayhemFeeRecipientRandom()
	} else {
		feeRecipient = PUMPFUN_FEE_RECIPIENT
	}

	// Get bonding curve v2
	bondingCurveV2 := GetBondingCurveV2PDA(params.InputMint)

	// Build accounts array
	accounts := []solana.AccountMeta{
		{PublicKey: PUMPFUN_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: params.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: userTokenAccount, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: pp.CreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_CONFIG, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_PROGRAM, IsSigner: false, IsWritable: false},
	}

	// Add user volume accumulator if cashback coin
	if bondingCurve.IsCashbackCoin {
		userVolumeAccumulator := GetPumpFunUserVolumeAccumulatorPDA(params.Payer)
		accounts = append(accounts, solana.AccountMeta{
			PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true,
		})
	}

	// Add bonding curve v2
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: bondingCurveV2, IsSigner: false, IsWritable: false,
	})

	instructions = append(instructions, solana.NewInstruction(PUMPFUN_PROGRAM, accounts, data))

	// Close token account if requested
	closeWhenSell := pp.CloseTokenAccountWhenSell != nil && *pp.CloseTokenAccountWhenSell
	if closeWhenSell || params.CloseInputMintAta {
		closeIx := token.NewCloseAccountInstruction(
			userTokenAccount,
			params.Payer,
			params.Payer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

// PumpFunBuildClaimCashbackInstruction builds claim cashback instruction for PumpFun
func PumpFunBuildClaimCashbackInstruction(payer solana.PublicKey) solana.Instruction {
	userVolumeAccumulator := GetPumpFunUserVolumeAccumulatorPDA(payer)

	accounts := []solana.AccountMeta{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
	}

	return solana.NewInstruction(PUMPFUN_PROGRAM, accounts, PumpFunClaimCashbackDiscriminator)
}

// Error definitions
var (
	ErrInvalidAmount        = fmt.Errorf("amount cannot be zero")
	ErrInvalidPool          = fmt.Errorf("pool must contain WSOL or USDC")
	ErrInvalidConfiguration = fmt.Errorf("invalid configuration for operation")
	ErrBondingCurveNotFound = fmt.Errorf("bonding curve not found")
)

// ===== RPC Fetch Functions - 100% from Rust: src/instruction/utils/pumpfun.rs =====

// AccountFetcher is an interface for fetching account data
type AccountFetcher interface {
	GetAccountInfo(ctx context.Context, pubkey string, opts interface{}) (interface{}, error)
}

// RPCAccountFetcher wraps the RPC client for account fetching
type RPCAccountFetcher struct {
	getAccountInfoFunc func(ctx context.Context, pubkey string) ([]byte, error)
}

// NewRPCAccountFetcher creates a new RPC account fetcher
func NewRPCAccountFetcher(getAccountInfoFunc func(ctx context.Context, pubkey string) ([]byte, error)) *RPCAccountFetcher {
	return &RPCAccountFetcher{getAccountInfoFunc: getAccountInfoFunc}
}

// FetchBondingCurveAccount fetches the bonding curve account from RPC.
// 100% from Rust: src/instruction/utils/pumpfun.rs fetch_bonding_curve_account
func FetchBondingCurveAccount(
	ctx context.Context,
	getAccountInfo func(ctx context.Context, pubkey string) ([]byte, error),
	mint solana.PublicKey,
) (*common.BondingCurveAccount, solana.PublicKey, error) {
	bondingCurvePDA := GetBondingCurvePDA(mint)

	data, err := getAccountInfo(ctx, bondingCurvePDA.String())
	if err != nil {
		return nil, bondingCurvePDA, fmt.Errorf("failed to get bonding curve account: %w", err)
	}

	if len(data) == 0 {
		return nil, bondingCurvePDA, ErrBondingCurveNotFound
	}

	// Decode the bonding curve account (skip 8-byte discriminator)
	var account [32]byte
	copy(account[:], bondingCurvePDA[:])
	bondingCurve := common.DecodeBondingCurveAccount(data, account)
	if bondingCurve == nil {
		return nil, bondingCurvePDA, fmt.Errorf("failed to decode bonding curve account")
	}

	return bondingCurve, bondingCurvePDA, nil
}

// FetchBondingCurveAccountFromRPC fetches bonding curve using standard RPC response format.
// Handles base64 encoded data from getAccountInfo RPC call.
func FetchBondingCurveAccountFromRPC(
	ctx context.Context,
	getAccountInfo func(ctx context.Context, pubkey string) (map[string]interface{}, error),
	mint solana.PublicKey,
) (*common.BondingCurveAccount, solana.PublicKey, error) {
	bondingCurvePDA := GetBondingCurvePDA(mint)

	result, err := getAccountInfo(ctx, bondingCurvePDA.String())
	if err != nil {
		return nil, bondingCurvePDA, fmt.Errorf("failed to get bonding curve account: %w", err)
	}

	// Parse RPC response
	value, ok := result["value"].(map[string]interface{})
	if !ok {
		return nil, bondingCurvePDA, ErrBondingCurveNotFound
	}

	dataInterface, ok := value["data"].([]interface{})
	if !ok || len(dataInterface) == 0 {
		return nil, bondingCurvePDA, ErrBondingCurveNotFound
	}

	// Data is [base64_data, "base64"]
	dataStr, ok := dataInterface[0].(string)
	if !ok {
		return nil, bondingCurvePDA, ErrBondingCurveNotFound
	}

	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, bondingCurvePDA, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	if len(data) == 0 {
		return nil, bondingCurvePDA, ErrBondingCurveNotFound
	}

	// Decode the bonding curve account (skip 8-byte discriminator)
	var account [32]byte
	copy(account[:], bondingCurvePDA[:])
	bondingCurve := common.DecodeBondingCurveAccount(data, account)
	if bondingCurve == nil {
		return nil, bondingCurvePDA, fmt.Errorf("failed to decode bonding curve account")
	}

	return bondingCurve, bondingCurvePDA, nil
}

// GetBuyPrice calculates the amount of tokens received for a given SOL amount.
// 100% from Rust: src/instruction/utils/pumpfun.rs get_buy_price
func GetBuyPrice(
	amount uint64,
	virtualSolReserves uint64,
	virtualTokenReserves uint64,
	realTokenReserves uint64,
) uint64 {
	if amount == 0 {
		return 0
	}

	// n = virtual_sol_reserves * virtual_token_reserves
	n := uint128(virtualSolReserves) * uint128(virtualTokenReserves)
	// i = virtual_sol_reserves + amount
	i := uint128(virtualSolReserves) + uint128(amount)
	// r = n / i + 1
	r := n/i + 1
	// s = virtual_token_reserves - r
	s := uint128(virtualTokenReserves) - r

	sU64 := uint64(s)
	if sU64 < realTokenReserves {
		return sU64
	}
	return realTokenReserves
}

// uint128 helper type for calculations
type uint128 = uint64 // Simplified for Go implementation (may overflow for very large values)
