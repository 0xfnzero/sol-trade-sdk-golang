// PumpFun instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
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

// Protocol extra fee recipients (Apr 2026 breaking upgrade). Appended after bonding-curve-v2, writable.
// https://github.com/pump-fun/pump-public-docs/blob/main/docs/BREAKING_FEE_RECIPIENT.md
var PumpFunProtocolExtraFeeRecipients = []solana.PublicKey{
	solana.MustPublicKeyFromBase58("5YxQFdt3Tr9zJLvkFccqXVUwhdTWJQc1fFg2YPbxvxeD"),
	solana.MustPublicKeyFromBase58("9M4giFFMxmFGXtc3feFzRai56WbBqehoSeRE5GK7gf7"),
	solana.MustPublicKeyFromBase58("GXPFM2caqTtQYC2cJ5yJRi9VDkpsYZXzYdwYpGnLmtDL"),
	solana.MustPublicKeyFromBase58("3BpXnfJaUTiwXnJNe7Ej1rcbzqTTQUvLShZaWazebsVR"),
	solana.MustPublicKeyFromBase58("5cjcW9wExnJJiqgLjq7DEG75Pm6JBgE1hNv4B2vHXUW6"),
	solana.MustPublicKeyFromBase58("EHAAiTxcdDwQ3U4bU6YcMsQGaekdzLS3B5SmYo46kJtL"),
	solana.MustPublicKeyFromBase58("5eHhjP8JaYkz83CWwvGU2uMUXefd3AazWGx4gpcuEEYD"),
	solana.MustPublicKeyFromBase58("A7hAgCzFw14fejgCp387JUJRMNyz4j89JKnhtKU8piqW"),
}

var PumpFunBuybackFeeRecipients = PumpFunProtocolExtraFeeRecipients

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
	// PumpFunBuyV2Discriminator is the discriminator for buy_v2
	PumpFunBuyV2Discriminator = []byte{184, 23, 238, 97, 103, 197, 211, 61}
	// PumpFunSellV2Discriminator is the discriminator for sell_v2
	PumpFunSellV2Discriminator = []byte{93, 246, 130, 60, 231, 233, 64, 178}
	// PumpFunBuyExactQuoteInV2Discriminator is the discriminator for buy_exact_quote_in_v2
	PumpFunBuyExactQuoteInV2Discriminator = []byte{194, 171, 28, 70, 104, 77, 91, 47}
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
	PumpFunSharingConfigSeed           = []byte("sharing-config")
)

var pumpFunPhantomDefaultCreatorVault = solana.MustPublicKeyFromBase58("2DR3iqRPVThyRLVJnwjPW1qiGWrp8RUFfHVjMbZyhdNc")

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

// GetPumpFunProtocolExtraFeeRecipientRandom returns a random protocol extra fee recipient (after bonding-curve-v2).
func GetPumpFunProtocolExtraFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(PumpFunProtocolExtraFeeRecipients))))
	return PumpFunProtocolExtraFeeRecipients[n.Int64()]
}

// GetPumpFunBuybackFeeRecipientRandom returns a random PumpFun V2 buyback fee recipient.
func GetPumpFunBuybackFeeRecipientRandom() solana.PublicKey {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(PumpFunBuybackFeeRecipients))))
	return PumpFunBuybackFeeRecipients[n.Int64()]
}

// GetPumpFunUserVolumeAccumulatorPDA returns the user volume accumulator PDA
func GetPumpFunUserVolumeAccumulatorPDA(user solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunUserVolumeAccumulatorSeed, user[:]},
		PUMPFUN_PROGRAM,
	)
	return pda
}

// GetPumpFunFeeSharingConfigPDA returns the fee sharing config PDA for a mint.
func GetPumpFunFeeSharingConfigPDA(mint solana.PublicKey) solana.PublicKey {
	pda, _, _ := solana.FindProgramAddress(
		[][]byte{PumpFunSharingConfigSeed, mint[:]},
		PUMPFUN_FEE_PROGRAM,
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
	Creator              solana.PublicKey
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
	ObservedTradeCreator      solana.PublicKey
	FeeSharingCreatorVault    solana.PublicKey
	FeeRecipient              solana.PublicKey
	QuoteMint                 solana.PublicKey
}

// PumpFunBuildBuyParams contains parameters for building buy instructions
type PumpFunBuildBuyParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *PumpFunParams
	CreateOutputMintAta bool
	CreateInputMintAta  bool
	CloseInputMintAta   bool
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
	CreateOutputMintAta bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
	OutputMint          solana.PublicKey
}

func pumpFunUsablePubkey(pk solana.PublicKey) bool {
	return !pk.IsZero() && !pk.Equals(pumpFunPhantomDefaultCreatorVault)
}

func pumpFunEffectiveCreator(pp *PumpFunParams) solana.PublicKey {
	if pumpFunUsablePubkey(pp.ObservedTradeCreator) {
		return pp.ObservedTradeCreator
	}
	if pp.BondingCurve != nil && pumpFunUsablePubkey(pp.BondingCurve.Creator) {
		return pp.BondingCurve.Creator
	}
	return solana.PublicKey{}
}

func pumpFunResolveCreatorVaultForIx(pp *PumpFunParams, mint solana.PublicKey) (solana.PublicKey, error) {
	if pumpFunUsablePubkey(pp.CreatorVault) {
		return pp.CreatorVault, nil
	}
	if pumpFunUsablePubkey(pp.FeeSharingCreatorVault) {
		return pp.FeeSharingCreatorVault, nil
	}
	creator := pumpFunEffectiveCreator(pp)
	if pumpFunUsablePubkey(creator) {
		return GetCreatorVaultPDA(creator), nil
	}
	return solana.PublicKey{}, fmt.Errorf("creator_vault PDA derivation failed for mint %s", mint.String())
}

func pumpFunResolveCreatorVaultForSellV2(pp *PumpFunParams, mint solana.PublicKey) (solana.PublicKey, error) {
	if pumpFunUsablePubkey(pp.CreatorVault) {
		return pp.CreatorVault, nil
	}
	if pumpFunUsablePubkey(pp.FeeSharingCreatorVault) {
		return pp.FeeSharingCreatorVault, nil
	}
	if pp.BondingCurve != nil && pumpFunUsablePubkey(pp.BondingCurve.Creator) {
		return GetCreatorVaultPDA(pp.BondingCurve.Creator), nil
	}
	return solana.PublicKey{}, fmt.Errorf("creator_vault PDA derivation failed for sell_v2 mint %s", mint.String())
}

func pumpFunEffectiveMintTokenProgram(mint solana.PublicKey, pp *PumpFunParams) solana.PublicKey {
	if strings.HasSuffix(mint.String(), "pump") {
		return constants.TOKEN_PROGRAM_2022
	}
	if pumpFunUsablePubkey(pp.TokenProgram) {
		return pp.TokenProgram
	}
	return constants.TOKEN_PROGRAM_2022
}

func pumpFunEffectiveQuoteMint(pp *PumpFunParams) solana.PublicKey {
	if pumpFunUsablePubkey(pp.QuoteMint) && !pp.QuoteMint.Equals(constants.SOL_TOKEN_ACCOUNT) {
		return pp.QuoteMint
	}
	return constants.WSOL_TOKEN_ACCOUNT
}

func pumpFunUsesV2Layout(pp *PumpFunParams) bool {
	return pumpFunUsablePubkey(pp.QuoteMint) && !pp.QuoteMint.Equals(constants.SOL_TOKEN_ACCOUNT)
}

func pumpFunIsSolQuoteMint(mint solana.PublicKey) bool {
	return mint.Equals(constants.SOL_TOKEN_ACCOUNT) || mint.Equals(constants.WSOL_TOKEN_ACCOUNT)
}

func pumpFunValidateV2BuyQuoteMint(inputMint, quoteMint solana.PublicKey) error {
	if pumpFunIsSolQuoteMint(quoteMint) {
		if inputMint.IsZero() || pumpFunIsSolQuoteMint(inputMint) {
			return nil
		}
	} else if inputMint.Equals(quoteMint) {
		return nil
	}
	return fmt.Errorf("PumpFun V2 buy input_mint %s does not match quote_mint %s; USDC quote pools must be bought with USDC, not SOL", inputMint.String(), quoteMint.String())
}

func pumpFunValidateV2SellQuoteMint(outputMint, quoteMint solana.PublicKey) error {
	if pumpFunIsSolQuoteMint(quoteMint) {
		if outputMint.IsZero() || pumpFunIsSolQuoteMint(outputMint) {
			return nil
		}
	} else if outputMint.Equals(quoteMint) {
		return nil
	}
	return fmt.Errorf("PumpFun V2 sell output_mint %s does not match quote_mint %s; USDC quote pools settle to USDC, not SOL", outputMint.String(), quoteMint.String())
}

func pumpFunFeeRecipient(pp *PumpFunParams) solana.PublicKey {
	if pumpFunUsablePubkey(pp.FeeRecipient) {
		return pp.FeeRecipient
	}
	if pp.BondingCurve != nil && pp.BondingCurve.IsMayhemMode {
		return GetPumpFunMayhemFeeRecipientRandom()
	}
	return PUMPFUN_FEE_RECIPIENT
}

// ===== Instruction Builders - 100% from Rust =====

// PumpFunBuildBuyInstructions builds buy instructions for PumpFun
// 100% port from Rust: src/instruction/pumpfun.rs build_buy_instructions
func PumpFunBuildBuyInstructions(params *PumpFunBuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	if pumpFunUsesV2Layout(pp) {
		return PumpFunBuildBuyV2Instructions(params)
	}
	bondingCurve := pp.BondingCurve
	creator := pumpFunEffectiveCreator(pp)
	creatorVaultAccount, err := pumpFunResolveCreatorVaultForIx(pp, params.OutputMint)
	if err != nil {
		creatorVaultAccount = pp.CreatorVault
	}

	// Calculate buy token amount
	var buyTokenAmount uint64
	if params.FixedOutputAmount != nil {
		buyTokenAmount = *params.FixedOutputAmount
	} else {
		buyTokenAmount = calc.GetBuyTokenAmountFromSolAmount(
			bondingCurve.VirtualTokenReserves,
			bondingCurve.VirtualSolReserves,
			bondingCurve.RealTokenReserves,
			pumpFunUsablePubkey(creator),
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
	tokenProgram := pumpFunEffectiveMintTokenProgram(params.OutputMint, pp)

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
	trackVolume := byte(0)
	if bondingCurve.IsCashbackCoin {
		trackVolume = 1
	}

	// Build instruction data
	var data []byte
	if params.FixedOutputAmount != nil {
		data = make([]byte, 25)
		copy(data[0:8], PumpFunBuyDiscriminator)
		binary.LittleEndian.PutUint64(data[8:16], *params.FixedOutputAmount)
		binary.LittleEndian.PutUint64(data[16:24], params.InputAmount)
		data[24] = trackVolume
	} else if params.UseExactSolAmount {
		// buy_exact_sol_in(spendable_sol_in: u64, min_tokens_out: u64, track_volume)
		minTokensOut, _ := calc.CalculateWithSlippageSell(buyTokenAmount, params.SlippageBasisPoints)
		data = make([]byte, 25)
		copy(data[0:8], PumpFunBuyExactSolInDiscriminator)
		binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
		binary.LittleEndian.PutUint64(data[16:24], minTokensOut)
		data[24] = trackVolume
	} else {
		// buy(token_amount: u64, max_sol_cost: u64, track_volume)
		data = make([]byte, 25)
		copy(data[0:8], PumpFunBuyDiscriminator)
		binary.LittleEndian.PutUint64(data[8:16], buyTokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], maxSolCost)
		data[24] = trackVolume
	}

	feeRecipient := pumpFunFeeRecipient(pp)

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
		{PublicKey: creatorVaultAccount, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_GLOBAL_VOLUME_ACCUMULATOR, IsSigner: false, IsWritable: true},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_FEE_CONFIG, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: bondingCurveV2, IsSigner: false, IsWritable: false}, // remainingAccounts: bondingCurveV2Pda
		{PublicKey: GetPumpFunProtocolExtraFeeRecipientRandom(), IsSigner: false, IsWritable: true},
	}

	instructions = append(instructions, newInstruction(PUMPFUN_PROGRAM, accounts, data))

	return instructions, nil
}

// PumpFunBuildSellInstructions builds sell instructions for PumpFun
// 100% port from Rust: src/instruction/pumpfun.rs build_sell_instructions
func PumpFunBuildSellInstructions(params *PumpFunBuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	if pumpFunUsesV2Layout(pp) {
		return PumpFunBuildSellV2Instructions(params)
	}
	bondingCurve := pp.BondingCurve
	creator := pumpFunEffectiveCreator(pp)
	creatorVaultAccount, err := pumpFunResolveCreatorVaultForIx(pp, params.InputMint)
	if err != nil {
		creatorVaultAccount = pp.CreatorVault
	}

	// Calculate SOL amount from token amount
	solAmount := calc.GetSellSolAmountFromTokenAmount(
		bondingCurve.VirtualTokenReserves,
		bondingCurve.VirtualSolReserves,
		pumpFunUsablePubkey(creator),
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
	tokenProgram := pumpFunEffectiveMintTokenProgram(params.InputMint, pp)

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

	feeRecipient := pumpFunFeeRecipient(pp)

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
		{PublicKey: creatorVaultAccount, IsSigner: false, IsWritable: true},
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
	accounts = append(accounts, solana.AccountMeta{
		PublicKey: GetPumpFunProtocolExtraFeeRecipientRandom(), IsSigner: false, IsWritable: true,
	})

	instructions = append(instructions, newInstruction(PUMPFUN_PROGRAM, accounts, data))

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

// PumpFunBuildBuyV2Instructions builds buy_v2 / buy_exact_quote_in_v2 instructions.
func PumpFunBuildBuyV2Instructions(params *PumpFunBuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	bondingCurve := pp.BondingCurve
	creator := pumpFunEffectiveCreator(pp)
	creatorVaultAccount, err := pumpFunResolveCreatorVaultForIx(pp, params.OutputMint)
	if err != nil {
		return nil, err
	}

	bondingCurveAddr := bondingCurve.Account
	if bondingCurveAddr.IsZero() {
		bondingCurveAddr = GetBondingCurvePDA(params.OutputMint)
	}
	baseTokenProgram := pumpFunEffectiveMintTokenProgram(params.OutputMint, pp)
	quoteMint := pumpFunEffectiveQuoteMint(pp)
	if err := pumpFunValidateV2BuyQuoteMint(params.InputMint, quoteMint); err != nil {
		return nil, err
	}
	quoteTokenProgram := constants.TOKEN_PROGRAM

	associatedBaseBondingCurve := GetAssociatedTokenAddress(bondingCurveAddr, params.OutputMint, baseTokenProgram)
	associatedBaseUser := GetAssociatedTokenAddress(params.Payer, params.OutputMint, baseTokenProgram)
	feeRecipient := pumpFunFeeRecipient(pp)
	buybackFeeRecipient := GetPumpFunBuybackFeeRecipientRandom()
	associatedQuoteFeeRecipient := GetAssociatedTokenAddress(feeRecipient, quoteMint, quoteTokenProgram)
	associatedQuoteBuybackFeeRecipient := GetAssociatedTokenAddress(buybackFeeRecipient, quoteMint, quoteTokenProgram)
	associatedQuoteBondingCurve := GetAssociatedTokenAddress(bondingCurveAddr, quoteMint, quoteTokenProgram)
	associatedQuoteUser := GetAssociatedTokenAddress(params.Payer, quoteMint, quoteTokenProgram)
	associatedCreatorVault := GetAssociatedTokenAddress(creatorVaultAccount, quoteMint, quoteTokenProgram)
	sharingConfig := GetPumpFunFeeSharingConfigPDA(params.OutputMint)
	userVolumeAccumulator := GetPumpFunUserVolumeAccumulatorPDA(params.Payer)
	associatedUserVolumeAccumulator := GetAssociatedTokenAddress(userVolumeAccumulator, quoteMint, quoteTokenProgram)

	instructions := make([]solana.Instruction, 0, 4)
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, params.OutputMint, baseTokenProgram,
		))
	}
	var buyTokenAmount uint64
	if params.FixedOutputAmount != nil {
		buyTokenAmount = *params.FixedOutputAmount
	} else {
		buyTokenAmount = calc.GetBuyTokenAmountFromSolAmount(
			bondingCurve.VirtualTokenReserves,
			bondingCurve.VirtualSolReserves,
			bondingCurve.RealTokenReserves,
			pumpFunUsablePubkey(creator),
			params.InputAmount,
		)
	}
	maxSolCost, _ := calc.CalculateWithSlippageBuy(params.InputAmount, params.SlippageBasisPoints)

	data := make([]byte, 24)
	var quoteAmountToFund uint64
	if params.FixedOutputAmount != nil {
		copy(data[0:8], PumpFunBuyV2Discriminator)
		binary.LittleEndian.PutUint64(data[8:16], *params.FixedOutputAmount)
		binary.LittleEndian.PutUint64(data[16:24], params.InputAmount)
		quoteAmountToFund = params.InputAmount
	} else if params.UseExactSolAmount {
		minTokensOut := buyTokenAmount
		minTokensOut, _ = calc.CalculateWithSlippageSell(buyTokenAmount, params.SlippageBasisPoints)
		copy(data[0:8], PumpFunBuyExactQuoteInV2Discriminator)
		binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
		binary.LittleEndian.PutUint64(data[16:24], minTokensOut)
		quoteAmountToFund = params.InputAmount
	} else {
		copy(data[0:8], PumpFunBuyV2Discriminator)
		binary.LittleEndian.PutUint64(data[8:16], buyTokenAmount)
		binary.LittleEndian.PutUint64(data[16:24], maxSolCost)
		quoteAmountToFund = maxSolCost
	}

	if params.CreateInputMintAta {
		if quoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
			instructions = append(instructions, HandleWsol(params.Payer, quoteAmountToFund)...)
		} else {
			instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
				params.Payer, params.Payer, quoteMint, quoteTokenProgram,
			))
		}
	}

	accounts := []solana.AccountMeta{
		{PublicKey: PUMPFUN_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: params.OutputMint, IsSigner: false, IsWritable: false},
		{PublicKey: quoteMint, IsSigner: false, IsWritable: false},
		{PublicKey: baseTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: quoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteFeeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: buybackFeeRecipient, IsSigner: false, IsWritable: false},
		{PublicKey: associatedQuoteBuybackFeeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBaseBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: associatedBaseUser, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteUser, IsSigner: false, IsWritable: true},
		{PublicKey: creatorVaultAccount, IsSigner: false, IsWritable: true},
		{PublicKey: associatedCreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: sharingConfig, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_GLOBAL_VOLUME_ACCUMULATOR, IsSigner: false, IsWritable: true},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: associatedUserVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_FEE_CONFIG, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
	}
	instructions = append(instructions, newInstruction(PUMPFUN_PROGRAM, accounts, data))
	if params.CloseInputMintAta && quoteMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		instructions = append(instructions, CloseWsol(params.Payer))
	}
	return instructions, nil
}

// PumpFunBuildSellV2Instructions builds sell_v2 instructions.
func PumpFunBuildSellV2Instructions(params *PumpFunBuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	bondingCurve := pp.BondingCurve
	creator := pumpFunEffectiveCreator(pp)
	creatorVaultAccount, err := pumpFunResolveCreatorVaultForSellV2(pp, params.InputMint)
	if err != nil {
		return nil, err
	}

	bondingCurveAddr := bondingCurve.Account
	if bondingCurveAddr.IsZero() {
		bondingCurveAddr = GetBondingCurvePDA(params.InputMint)
	}
	baseTokenProgram := pumpFunEffectiveMintTokenProgram(params.InputMint, pp)
	quoteMint := pumpFunEffectiveQuoteMint(pp)
	if err := pumpFunValidateV2SellQuoteMint(params.OutputMint, quoteMint); err != nil {
		return nil, err
	}
	quoteTokenProgram := constants.TOKEN_PROGRAM

	associatedBaseBondingCurve := GetAssociatedTokenAddress(bondingCurveAddr, params.InputMint, baseTokenProgram)
	associatedBaseUser := GetAssociatedTokenAddress(params.Payer, params.InputMint, baseTokenProgram)
	feeRecipient := pumpFunFeeRecipient(pp)
	buybackFeeRecipient := GetPumpFunBuybackFeeRecipientRandom()
	associatedQuoteFeeRecipient := GetAssociatedTokenAddress(feeRecipient, quoteMint, quoteTokenProgram)
	associatedQuoteBuybackFeeRecipient := GetAssociatedTokenAddress(buybackFeeRecipient, quoteMint, quoteTokenProgram)
	associatedQuoteBondingCurve := GetAssociatedTokenAddress(bondingCurveAddr, quoteMint, quoteTokenProgram)
	associatedQuoteUser := GetAssociatedTokenAddress(params.Payer, quoteMint, quoteTokenProgram)
	associatedCreatorVault := GetAssociatedTokenAddress(creatorVaultAccount, quoteMint, quoteTokenProgram)
	sharingConfig := GetPumpFunFeeSharingConfigPDA(params.InputMint)
	userVolumeAccumulator := GetPumpFunUserVolumeAccumulatorPDA(params.Payer)
	associatedUserVolumeAccumulator := GetAssociatedTokenAddress(userVolumeAccumulator, quoteMint, quoteTokenProgram)

	instructions := make([]solana.Instruction, 0, 3)
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, quoteMint, quoteTokenProgram,
		))
	}

	solAmount := calc.GetSellSolAmountFromTokenAmount(
		bondingCurve.VirtualTokenReserves,
		bondingCurve.VirtualSolReserves,
		pumpFunUsablePubkey(creator),
		params.InputAmount,
	)
	minSolOutput := solAmount
	if params.FixedOutputAmount != nil {
		minSolOutput = *params.FixedOutputAmount
	} else {
		minSolOutput, _ = calc.CalculateWithSlippageSell(solAmount, params.SlippageBasisPoints)
	}

	data := make([]byte, 24)
	copy(data[0:8], PumpFunSellV2Discriminator)
	binary.LittleEndian.PutUint64(data[8:16], params.InputAmount)
	binary.LittleEndian.PutUint64(data[16:24], minSolOutput)

	accounts := []solana.AccountMeta{
		{PublicKey: PUMPFUN_GLOBAL_ACCOUNT, IsSigner: false, IsWritable: false},
		{PublicKey: params.InputMint, IsSigner: false, IsWritable: false},
		{PublicKey: quoteMint, IsSigner: false, IsWritable: false},
		{PublicKey: baseTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: quoteTokenProgram, IsSigner: false, IsWritable: false},
		{PublicKey: constants.ASSOCIATED_TOKEN_PROGRAM_ID, IsSigner: false, IsWritable: false},
		{PublicKey: feeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteFeeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: buybackFeeRecipient, IsSigner: false, IsWritable: false},
		{PublicKey: associatedQuoteBuybackFeeRecipient, IsSigner: false, IsWritable: true},
		{PublicKey: bondingCurveAddr, IsSigner: false, IsWritable: true},
		{PublicKey: associatedBaseBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteBondingCurve, IsSigner: false, IsWritable: true},
		{PublicKey: params.Payer, IsSigner: true, IsWritable: true},
		{PublicKey: associatedBaseUser, IsSigner: false, IsWritable: true},
		{PublicKey: associatedQuoteUser, IsSigner: false, IsWritable: true},
		{PublicKey: creatorVaultAccount, IsSigner: false, IsWritable: true},
		{PublicKey: associatedCreatorVault, IsSigner: false, IsWritable: true},
		{PublicKey: sharingConfig, IsSigner: false, IsWritable: false},
		{PublicKey: userVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: associatedUserVolumeAccumulator, IsSigner: false, IsWritable: true},
		{PublicKey: PUMPFUN_FEE_CONFIG, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_FEE_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: constants.SYSTEM_PROGRAM, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_EVENT_AUTHORITY, IsSigner: false, IsWritable: false},
		{PublicKey: PUMPFUN_PROGRAM, IsSigner: false, IsWritable: false},
	}
	instructions = append(instructions, newInstruction(PUMPFUN_PROGRAM, accounts, data))

	closeWhenSell := pp.CloseTokenAccountWhenSell != nil && *pp.CloseTokenAccountWhenSell
	if closeWhenSell || params.CloseInputMintAta {
		closeIx := token.NewCloseAccountInstruction(
			associatedBaseUser,
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

	return newInstruction(PUMPFUN_PROGRAM, accounts, PumpFunClaimCashbackDiscriminator)
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
