package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// ===== Token Program Constants =====

const (
	// TokenProgramID is the SPL Token program ID
	TokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	// Token2022ProgramID is the Token-2022 program ID
	Token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"

	// AssociatedTokenProgramID is the Associated Token Account program ID
	AssociatedTokenProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"

	// WSOLMint is the wrapped SOL mint address
	WSOLMint = "So11111111111111111111111111111111111111112"

	// USDCMint is the USDC mint address
	USDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	// USDTMint is the USDT mint address
	USDTMint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
)

// TokenAccountState represents the state of a token account
type TokenAccountState uint8

const (
	// TokenAccountUninitialized is an uninitialized account
	TokenAccountUninitialized TokenAccountState = 0
	// TokenAccountInitialized is an initialized account
	TokenAccountInitialized TokenAccountState = 1
	// TokenAccountFrozen is a frozen account
	TokenAccountFrozen TokenAccountState = 2
)

// ===== TokenAccount =====

// TokenAccount represents an SPL token account
type TokenAccount struct {
	Mint            solana.PublicKey
	Owner           solana.PublicKey
	Amount          uint64
	Delegate        *solana.PublicKey
	State           TokenAccountState
	IsNative        *uint64
	DelegatedAmount uint64
	CloseAuthority  *solana.PublicKey
}

// TokenAccountSize is the size of a token account in bytes
const TokenAccountSize = 165

// DecodeTokenAccount decodes a token account from account data
func DecodeTokenAccount(data []byte) (*TokenAccount, error) {
	if len(data) < TokenAccountSize {
		return nil, errors.New("invalid token account data length")
	}

	acc := &TokenAccount{}
	buf := bytes.NewReader(data)

	// Mint (32 bytes)
	var mint [32]byte
	if err := binary.Read(buf, binary.LittleEndian, &mint); err != nil {
		return nil, fmt.Errorf("failed to read mint: %w", err)
	}
	acc.Mint = mint

	// Owner (32 bytes)
	var owner [32]byte
	if err := binary.Read(buf, binary.LittleEndian, &owner); err != nil {
		return nil, fmt.Errorf("failed to read owner: %w", err)
	}
	acc.Owner = owner

	// Amount (8 bytes)
	if err := binary.Read(buf, binary.LittleEndian, &acc.Amount); err != nil {
		return nil, fmt.Errorf("failed to read amount: %w", err)
	}

	// Delegate (COption<Pubkey>)
	var hasDelegate uint32
	if err := binary.Read(buf, binary.LittleEndian, &hasDelegate); err != nil {
		return nil, fmt.Errorf("failed to read delegate option: %w", err)
	}
	if hasDelegate == 1 {
		var delegate [32]byte
		if err := binary.Read(buf, binary.LittleEndian, &delegate); err != nil {
			return nil, fmt.Errorf("failed to read delegate: %w", err)
		}
		pubkey := solana.PublicKeyFromBytes(delegate[:])
		acc.Delegate = &pubkey
	}

	// State (1 byte)
	var state uint8
	if err := binary.Read(buf, binary.LittleEndian, &state); err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}
	acc.State = TokenAccountState(state)

	// IsNative (COption<u64>)
	var isNative uint32
	if err := binary.Read(buf, binary.LittleEndian, &isNative); err != nil {
		return nil, fmt.Errorf("failed to read is_native option: %w", err)
	}
	if isNative == 1 {
		var nativeAmount uint64
		if err := binary.Read(buf, binary.LittleEndian, &nativeAmount); err != nil {
			return nil, fmt.Errorf("failed to read native amount: %w", err)
		}
		acc.IsNative = &nativeAmount
	}

	// DelegatedAmount (8 bytes)
	if err := binary.Read(buf, binary.LittleEndian, &acc.DelegatedAmount); err != nil {
		return nil, fmt.Errorf("failed to read delegated amount: %w", err)
	}

	// CloseAuthority (COption<Pubkey>)
	var hasCloseAuthority uint32
	if err := binary.Read(buf, binary.LittleEndian, &hasCloseAuthority); err != nil {
		return nil, fmt.Errorf("failed to read close authority option: %w", err)
	}
	if hasCloseAuthority == 1 {
		var closeAuthority [32]byte
		if err := binary.Read(buf, binary.LittleEndian, &closeAuthority); err != nil {
			return nil, fmt.Errorf("failed to read close authority: %w", err)
		}
		pubkey := solana.PublicKeyFromBytes(closeAuthority[:])
		acc.CloseAuthority = &pubkey
	}

	return acc, nil
}

// IsInitialized returns true if the account is initialized
func (ta *TokenAccount) IsInitialized() bool {
	return ta.State == TokenAccountInitialized || ta.State == TokenAccountFrozen
}

// IsFrozen returns true if the account is frozen
func (ta *TokenAccount) IsFrozen() bool {
	return ta.State == TokenAccountFrozen
}

// IsNativeWSOL returns true if this is a native WSOL account
func (ta *TokenAccount) IsNativeWSOL() bool {
	return ta.IsNative != nil
}

// UIAmount returns the amount as a UI amount with decimals
func (ta *TokenAccount) UIAmount(decimals uint8) float64 {
	return float64(ta.Amount) / float64(pow10(decimals))
}

// ===== Mint =====

// Mint represents an SPL token mint
type Mint struct {
	MintAuthority   *solana.PublicKey
	Supply          uint64
	Decimals        uint8
	IsInitialized   bool
	FreezeAuthority *solana.PublicKey
}

// MintSize is the size of a mint account in bytes
const MintSize = 82

// DecodeMint decodes a mint from account data
func DecodeMint(data []byte) (*Mint, error) {
	if len(data) < MintSize {
		return nil, errors.New("invalid mint data length")
	}

	mint := &Mint{}
	buf := bytes.NewReader(data)

	// MintAuthority (COption<Pubkey>)
	var hasAuthority uint32
	if err := binary.Read(buf, binary.LittleEndian, &hasAuthority); err != nil {
		return nil, fmt.Errorf("failed to read mint authority option: %w", err)
	}
	if hasAuthority == 1 {
		var authority [32]byte
		if err := binary.Read(buf, binary.LittleEndian, &authority); err != nil {
			return nil, fmt.Errorf("failed to read mint authority: %w", err)
		}
		pubkey := solana.PublicKeyFromBytes(authority[:])
		mint.MintAuthority = &pubkey
	}

	// Supply (8 bytes)
	if err := binary.Read(buf, binary.LittleEndian, &mint.Supply); err != nil {
		return nil, fmt.Errorf("failed to read supply: %w", err)
	}

	// Decimals (1 byte)
	if err := binary.Read(buf, binary.LittleEndian, &mint.Decimals); err != nil {
		return nil, fmt.Errorf("failed to read decimals: %w", err)
	}

	// IsInitialized (1 byte)
	var isInitialized uint8
	if err := binary.Read(buf, binary.LittleEndian, &isInitialized); err != nil {
		return nil, fmt.Errorf("failed to read is_initialized: %w", err)
	}
	mint.IsInitialized = isInitialized == 1

	// FreezeAuthority (COption<Pubkey>)
	var hasFreezeAuthority uint32
	if err := binary.Read(buf, binary.LittleEndian, &hasFreezeAuthority); err != nil {
		return nil, fmt.Errorf("failed to read freeze authority option: %w", err)
	}
	if hasFreezeAuthority == 1 {
		var freezeAuthority [32]byte
		if err := binary.Read(buf, binary.LittleEndian, &freezeAuthority); err != nil {
			return nil, fmt.Errorf("failed to read freeze authority: %w", err)
		}
		pubkey := solana.PublicKeyFromBytes(freezeAuthority[:])
		mint.FreezeAuthority = &pubkey
	}

	return mint, nil
}

// HasMintAuthority returns true if the mint has a mint authority
func (m *Mint) HasMintAuthority() bool {
	return m.MintAuthority != nil
}

// HasFreezeAuthority returns true if the mint has a freeze authority
func (m *Mint) HasFreezeAuthority() bool {
	return m.FreezeAuthority != nil
}

// UISupply returns the supply as a UI amount
func (m *Mint) UISupply() float64 {
	return float64(m.Supply) / float64(pow10(m.Decimals))
}

// ===== TokenInstructionBuilder =====

// TokenInstructionBuilder builds SPL token instructions
type TokenInstructionBuilder struct {
	programID solana.PublicKey
}

func newAccountMetaSlice(accounts ...*solana.AccountMeta) solana.AccountMetaSlice {
	return solana.AccountMetaSlice(accounts)
}

func newGenericInstruction(
	programID solana.PublicKey,
	accounts solana.AccountMetaSlice,
	data []byte,
) *solana.GenericInstruction {
	return solana.NewInstruction(programID, accounts, data)
}

// TokenInstructionType represents the type of token instruction
type TokenInstructionType uint8

const (
	// TokenInstructionInitializeMint initializes a new mint
	TokenInstructionInitializeMint TokenInstructionType = 0
	// TokenInstructionInitializeAccount initializes a new account
	TokenInstructionInitializeAccount TokenInstructionType = 1
	// TokenInstructionInitializeMultisig initializes a multisig
	TokenInstructionInitializeMultisig TokenInstructionType = 2
	// TokenInstructionTransfer transfers tokens
	TokenInstructionTransfer TokenInstructionType = 3
	// TokenInstructionApprove approves a delegate
	TokenInstructionApprove TokenInstructionType = 4
	// TokenInstructionRevoke revokes a delegate
	TokenInstructionRevoke TokenInstructionType = 5
	// TokenInstructionSetAuthority sets a new authority
	TokenInstructionSetAuthority TokenInstructionType = 6
	// TokenInstructionMintTo mints new tokens
	TokenInstructionMintTo TokenInstructionType = 7
	// TokenInstructionBurn burns tokens
	TokenInstructionBurn TokenInstructionType = 8
	// TokenInstructionCloseAccount closes an account
	TokenInstructionCloseAccount TokenInstructionType = 9
	// TokenInstructionFreezeAccount freezes an account
	TokenInstructionFreezeAccount TokenInstructionType = 10
	// TokenInstructionThawAccount thaws an account
	TokenInstructionThawAccount TokenInstructionType = 11
	// TokenInstructionTransferChecked transfers tokens with validation
	TokenInstructionTransferChecked TokenInstructionType = 12
	// TokenInstructionApproveChecked approves with validation
	TokenInstructionApproveChecked TokenInstructionType = 13
	// TokenInstructionMintToChecked mints with validation
	TokenInstructionMintToChecked TokenInstructionType = 14
	// TokenInstructionBurnChecked burns with validation
	TokenInstructionBurnChecked TokenInstructionType = 15
	// TokenInstructionSyncNative syncs native account balance
	TokenInstructionSyncNative TokenInstructionType = 17
)

// NewTokenInstructionBuilder creates a new instruction builder
func NewTokenInstructionBuilder() *TokenInstructionBuilder {
	return &TokenInstructionBuilder{
		programID: solana.MustPublicKeyFromBase58(TokenProgramID),
	}
}

// NewToken2022InstructionBuilder creates a builder for Token-2022
func NewToken2022InstructionBuilder() *TokenInstructionBuilder {
	return &TokenInstructionBuilder{
		programID: solana.MustPublicKeyFromBase58(Token2022ProgramID),
	}
}

// WithProgramID sets a custom program ID
func (b *TokenInstructionBuilder) WithProgramID(programID solana.PublicKey) *TokenInstructionBuilder {
	b.programID = programID
	return b
}

// BuildTransfer creates a transfer instruction
func (b *TokenInstructionBuilder) BuildTransfer(
	source, destination, owner solana.PublicKey,
	amount uint64,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := make([]byte, 9)
	data[0] = byte(TokenInstructionTransfer)
	binary.LittleEndian.PutUint64(data[1:], amount)

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(source, false, true),
		solana.NewAccountMeta(destination, false, true),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildTransferChecked creates a checked transfer instruction
func (b *TokenInstructionBuilder) BuildTransferChecked(
	source, mint, destination, owner solana.PublicKey,
	amount uint64,
	decimals uint8,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := make([]byte, 10)
	data[0] = byte(TokenInstructionTransferChecked)
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = decimals

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(source, false, true),
		solana.NewAccountMeta(mint, false, false),
		solana.NewAccountMeta(destination, false, true),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildInitializeAccount creates an initialize account instruction
func (b *TokenInstructionBuilder) BuildInitializeAccount(
	account, mint, owner solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{byte(TokenInstructionInitializeAccount)}

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(account, false, true),
		solana.NewAccountMeta(mint, false, false),
		solana.NewAccountMeta(owner, false, false),
		solana.NewAccountMeta(solana.SysVarRentPubkey, false, false),
	)

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildCloseAccount creates a close account instruction
func (b *TokenInstructionBuilder) BuildCloseAccount(
	account, destination, owner solana.PublicKey,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{byte(TokenInstructionCloseAccount)}

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(account, false, true),
		solana.NewAccountMeta(destination, false, true),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildSyncNative creates a sync native instruction (for WSOL)
func (b *TokenInstructionBuilder) BuildSyncNative(
	account solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{byte(TokenInstructionSyncNative)}

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(account, false, true),
	)

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildApprove creates an approve instruction
func (b *TokenInstructionBuilder) BuildApprove(
	source, delegate, owner solana.PublicKey,
	amount uint64,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := make([]byte, 9)
	data[0] = byte(TokenInstructionApprove)
	binary.LittleEndian.PutUint64(data[1:], amount)

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(source, false, true),
		solana.NewAccountMeta(delegate, false, false),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildRevoke creates a revoke instruction
func (b *TokenInstructionBuilder) BuildRevoke(
	source, owner solana.PublicKey,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{byte(TokenInstructionRevoke)}

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(source, false, true),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildMintTo creates a mint to instruction
func (b *TokenInstructionBuilder) BuildMintTo(
	mint, account, mintAuthority solana.PublicKey,
	amount uint64,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := make([]byte, 9)
	data[0] = byte(TokenInstructionMintTo)
	binary.LittleEndian.PutUint64(data[1:], amount)

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(mint, false, true),
		solana.NewAccountMeta(account, false, true),
		solana.NewAccountMeta(mintAuthority, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// BuildBurn creates a burn instruction
func (b *TokenInstructionBuilder) BuildBurn(
	account, mint, owner solana.PublicKey,
	amount uint64,
	signers ...solana.PublicKey,
) *solana.GenericInstruction {
	data := make([]byte, 9)
	data[0] = byte(TokenInstructionBurn)
	binary.LittleEndian.PutUint64(data[1:], amount)

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(account, false, true),
		solana.NewAccountMeta(mint, false, true),
		solana.NewAccountMeta(owner, len(signers) == 0, false),
	)

	for _, signer := range signers {
		accounts.Append(solana.NewAccountMeta(signer, true, false))
	}

	return newGenericInstruction(b.programID, accounts, data)
}

// ===== Associated Token Account =====

// GetAssociatedTokenAddress returns the associated token account address
func GetAssociatedTokenAddress(
	wallet, mint solana.PublicKey,
	allowOwnerOffCurve bool,
	programID ...solana.PublicKey,
) (solana.PublicKey, uint8, error) {
	tokenProgram := solana.MustPublicKeyFromBase58(TokenProgramID)
	if len(programID) > 0 {
		tokenProgram = programID[0]
	}

	seeds := [][]byte{
		wallet.Bytes(),
		tokenProgram.Bytes(),
		mint.Bytes(),
	}

	ataProgram := solana.MustPublicKeyFromBase58(AssociatedTokenProgramID)
	return solana.FindProgramAddress(seeds, ataProgram)
}

// BuildCreateAssociatedTokenAccount creates an ATA creation instruction
func BuildCreateAssociatedTokenAccount(
	payer, wallet, mint solana.PublicKey,
	programID ...solana.PublicKey,
) (*solana.GenericInstruction, solana.PublicKey, error) {
	tokenProgram := solana.MustPublicKeyFromBase58(TokenProgramID)
	if len(programID) > 0 {
		tokenProgram = programID[0]
	}

	ata, _, err := GetAssociatedTokenAddress(wallet, mint, false, tokenProgram)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(payer, true, true),
		solana.NewAccountMeta(ata, false, true),
		solana.NewAccountMeta(wallet, false, false),
		solana.NewAccountMeta(mint, false, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
		solana.NewAccountMeta(tokenProgram, false, false),
	)

	ataProgram := solana.MustPublicKeyFromBase58(AssociatedTokenProgramID)
	ix := newGenericInstruction(ataProgram, accounts, []byte{})

	return ix, ata, nil
}

// BuildCreateIdempotentATA creates an ATA with idempotent instruction
func BuildCreateIdempotentATA(
	payer, wallet, mint solana.PublicKey,
	programID ...solana.PublicKey,
) (*solana.GenericInstruction, solana.PublicKey, error) {
	tokenProgram := solana.MustPublicKeyFromBase58(TokenProgramID)
	if len(programID) > 0 {
		tokenProgram = programID[0]
	}

	ata, _, err := GetAssociatedTokenAddress(wallet, mint, false, tokenProgram)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	// Idempotent instruction uses data byte 0x01
	accounts := newAccountMetaSlice(
		solana.NewAccountMeta(payer, true, true),
		solana.NewAccountMeta(ata, false, true),
		solana.NewAccountMeta(wallet, false, false),
		solana.NewAccountMeta(mint, false, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
		solana.NewAccountMeta(tokenProgram, false, false),
	)

	ataProgram := solana.MustPublicKeyFromBase58(AssociatedTokenProgramID)
	ix := newGenericInstruction(ataProgram, accounts, []byte{0x01})

	return ix, ata, nil
}

// ===== Utility Functions =====

// pow10 returns 10^n
func pow10(n uint8) uint64 {
	result := uint64(1)
	for i := uint8(0); i < n; i++ {
		result *= 10
	}
	return result
}

// TokensToUIAmount converts raw token amount to UI amount
func TokensToUIAmount(amount uint64, decimals uint8) float64 {
	return float64(amount) / float64(pow10(decimals))
}

// UIAmountToTokens converts UI amount to raw token amount
func UIAmountToTokens(uiAmount float64, decimals uint8) uint64 {
	return uint64(uiAmount * float64(pow10(decimals)))
}

// LamportsToSOL converts lamports to SOL
func LamportsToSOL(lamports uint64) float64 {
	return float64(lamports) / 1_000_000_000.0
}

// SOLToLamports converts SOL to lamports
func SOLToLamports(sol float64) uint64 {
	return uint64(sol * 1_000_000_000)
}

// ===== Errors =====

var (
	ErrInvalidTokenAccount = errors.New("invalid token account")
	ErrInvalidMint         = errors.New("invalid mint")
	ErrInsufficientBalance = errors.New("insufficient token balance")
	ErrAccountFrozen       = errors.New("token account is frozen")
	ErrInvalidDecimals     = errors.New("invalid decimals")
)
