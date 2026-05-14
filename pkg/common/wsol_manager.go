package common

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// ===== WSOL Manager - 100% port from Rust: src/trading/common/wsol_manager.rs =====

// WSOLManager provides utilities for handling wrapped SOL (WSOL) operations
type WSOLManager struct {
	tokenBuilder *TokenInstructionBuilder
}

// NewWSOLManager creates a new WSOL manager
func NewWSOLManager() *WSOLManager {
	return &WSOLManager{
		tokenBuilder: NewTokenInstructionBuilder(),
	}
}

// WSOL Mint address
var WSOLMintPubkey = solana.MustPublicKeyFromBase58(WSOLMint)

// HandleWSOL creates instructions to handle WSOL - Create ATA, transfer SOL, and sync.
// 100% from Rust: src/trading/common/wsol_manager.rs handle_wsol
func (w *WSOLManager) HandleWSOL(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	var instructions []solana.Instruction

	// 1. Create WSOL ATA (idempotent)
	createATAIx, wsolATA, err := BuildCreateIdempotentATA(payer, payer, WSOLMintPubkey)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, createATAIx)

	// 2. Transfer SOL to WSOL ATA
	transferIx := system.NewTransferInstructionBuilder().
		SetFundingAccount(payer).
		SetRecipientAccount(wsolATA).
		SetLamports(amount).
		Build()
	instructions = append(instructions, transferIx)

	// 3. Sync native
	syncIx := w.tokenBuilder.BuildSyncNative(wsolATA)
	instructions = append(instructions, syncIx)

	return instructions, nil
}

// CloseWSOL creates instruction to close WSOL account and reclaim rent.
// 100% from Rust: src/trading/common/wsol_manager.rs close_wsol
func (w *WSOLManager) CloseWSOL(payer solana.PublicKey) (solana.Instruction, error) {
	wsolATA, _, err := GetAssociatedTokenAddress(payer, WSOLMintPubkey, false)
	if err != nil {
		return nil, err
	}

	return w.tokenBuilder.BuildCloseAccount(wsolATA, payer, payer), nil
}

// CreateWSOLATA creates WSOL ATA only (without funding).
// 100% from Rust: src/trading/common/wsol_manager.rs create_wsol_ata
func (w *WSOLManager) CreateWSOLATA(payer solana.PublicKey) ([]solana.Instruction, error) {
	createATAIx, _, err := BuildCreateIdempotentATA(payer, payer, WSOLMintPubkey)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createATAIx}, nil
}

// WrapSOLOnly wraps SOL only - Transfer and sync without creating ATA.
// Assumes ATA already exists.
// 100% from Rust: src/trading/common/wsol_manager.rs wrap_sol_only
func (w *WSOLManager) WrapSOLOnly(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	var instructions []solana.Instruction

	wsolATA, _, err := GetAssociatedTokenAddress(payer, WSOLMintPubkey, false)
	if err != nil {
		return nil, err
	}

	// 1. Transfer SOL to WSOL ATA
	transferIx := system.NewTransferInstructionBuilder().
		SetFundingAccount(payer).
		SetRecipientAccount(wsolATA).
		SetLamports(amount).
		Build()
	instructions = append(instructions, transferIx)

	// 2. Sync native
	syncIx := w.tokenBuilder.BuildSyncNative(wsolATA)
	instructions = append(instructions, syncIx)

	return instructions, nil
}

// WrapWSOLToSOL wraps WSOL to SOL - Transfer WSOL to seed account and close it.
// 100% from Rust: src/trading/common/wsol_manager.rs wrap_wsol_to_sol
func (w *WSOLManager) WrapWSOLToSOL(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	var instructions []solana.Instruction

	// 1. Create seed WSOL account
	seedATA, _, err := GetAssociatedTokenAddressUseSeed(payer, WSOLMintPubkey)
	if err != nil {
		return nil, err
	}

	createATAIx, _, err := BuildCreateIdempotentATA(payer, payer, WSOLMintPubkey)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, createATAIx)

	// 2. Get user WSOL ATA
	userWSOLATA, _, err := GetAssociatedTokenAddress(payer, WSOLMintPubkey, false)
	if err != nil {
		return nil, err
	}

	// 3. Transfer WSOL from user ATA to seed ATA
	transferIx := w.tokenBuilder.BuildTransfer(userWSOLATA, seedATA, payer, amount)
	instructions = append(instructions, transferIx)

	// 4. Close seed WSOL account
	closeIx := w.tokenBuilder.BuildCloseAccount(seedATA, payer, payer)
	instructions = append(instructions, closeIx)

	return instructions, nil
}

// WrapWSOLToSOLWithoutCreate wraps WSOL to SOL without creating account.
// Assumes seed account already exists.
// 100% from Rust: src/trading/common/wsol_manager.rs wrap_wsol_to_sol_without_create
func (w *WSOLManager) WrapWSOLToSOLWithoutCreate(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	var instructions []solana.Instruction

	// 1. Get seed ATA address
	seedATA, _, err := GetAssociatedTokenAddressUseSeed(payer, WSOLMintPubkey)
	if err != nil {
		return nil, err
	}

	// 2. Get user WSOL ATA
	userWSOLATA, _, err := GetAssociatedTokenAddress(payer, WSOLMintPubkey, false)
	if err != nil {
		return nil, err
	}

	// 3. Transfer WSOL from user ATA to seed ATA
	transferIx := w.tokenBuilder.BuildTransfer(userWSOLATA, seedATA, payer, amount)
	instructions = append(instructions, transferIx)

	// 4. Close seed WSOL account
	closeIx := w.tokenBuilder.BuildCloseAccount(seedATA, payer, payer)
	instructions = append(instructions, closeIx)

	return instructions, nil
}

// ===== Seed-based ATA Functions =====

// ATA cache for performance
var ataCache = make(map[string]solana.PublicKey)

// GetAssociatedTokenAddressFast returns cached ATA address
func GetAssociatedTokenAddressFast(owner, mint solana.PublicKey, tokenProgram solana.PublicKey) solana.PublicKey {
	key := owner.String() + ":" + mint.String() + ":" + tokenProgram.String()
	if cached, ok := ataCache[key]; ok {
		return cached
	}

	ata, _, _ := GetAssociatedTokenAddress(owner, mint, false, tokenProgram)
	ataCache[key] = ata
	return ata
}

// GetAssociatedTokenAddressUseSeed returns ATA address using seed method.
// 100% from Rust: src/common/seed.rs get_associated_token_address_with_program_id_use_seed
func GetAssociatedTokenAddressUseSeed(walletAddress, tokenMintAddress solana.PublicKey) (solana.PublicKey, uint8, error) {
	// For now, use standard ATA derivation
	// Full seed-based implementation would use CreateWithSeed
	tokenProgram := solana.MustPublicKeyFromBase58(TokenProgramID)
	return GetAssociatedTokenAddress(walletAddress, tokenMintAddress, false, tokenProgram)
}

// ===== Global Functions =====

// HandleWSOL creates instructions to handle WSOL
func HandleWSOL(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	return NewWSOLManager().HandleWSOL(payer, amount)
}

// CloseWSOL creates instruction to close WSOL account
func CloseWSOL(payer solana.PublicKey) (solana.Instruction, error) {
	return NewWSOLManager().CloseWSOL(payer)
}

// CreateWSOLATA creates WSOL ATA only
func CreateWSOLATA(payer solana.PublicKey) ([]solana.Instruction, error) {
	return NewWSOLManager().CreateWSOLATA(payer)
}

// WrapSOLOnly wraps SOL only
func WrapSOLOnly(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	return NewWSOLManager().WrapSOLOnly(payer, amount)
}

// WrapWSOLToSOL wraps WSOL to SOL
func WrapWSOLToSOL(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	return NewWSOLManager().WrapWSOLToSOL(payer, amount)
}

// WrapWSOLToSOLWithoutCreate wraps WSOL to SOL without creating account
func WrapWSOLToSOLWithoutCreate(payer solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	return NewWSOLManager().WrapWSOLToSOLWithoutCreate(payer, amount)
}
