package spl_token

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// ===== Token Program Constants =====

const (
	// Instruction discriminators
	InitializeMint           = 0
	InitializeAccount        = 1
	InitializeMultisig       = 2
	Transfer                 = 3
	Approve                  = 4
	Revoke                   = 5
	SetAuthority            = 6
	MintTo                   = 7
	Burn                     = 8
	CloseAccount            = 9
	FreezeAccount           = 10
	ThawAccount             = 11
	TransferChecked         = 12
	ApproveChecked          = 13
	MintToChecked           = 14
	BurnChecked             = 15
	InitializeAccount2      = 16
	SyncNative              = 17
	InitializeAccount3      = 20
	InitializeMultisig2     = 21
	InitializeMint2         = 22

	// Account states
	AccountUninitialized = 0
	AccountInitialized   = 1
	AccountFrozen        = 2

	// Authority types
	AuthorityMintTokens  = 0
	AuthorityFreezeAccount = 1
	AuthorityAccountOwner = 2
	AuthorityCloseAccount = 3
)

// ===== Token Account Layout =====

const (
	// Token Account size
	TokenAccountSize = 165

	// Mint size
	MintSize = 82

	// Multisig size
	MultisigSize = 355
)

// TokenAccount represents a token account
type TokenAccount struct {
	Mint         solana.PublicKey
	Owner        solana.PublicKey
	Amount       uint64
	Delegate     *solana.PublicKey
	State        uint8
	IsNative     *uint64
	DelegatedAmount uint64
	CloseAuthority *solana.PublicKey
}

// Mint represents a token mint
type Mint struct {
	MintAuthority         *solana.PublicKey
	Supply               uint64
	Decimals             uint8
	IsInitialized        bool
	FreezeAuthority      *solana.PublicKey
}

// DecodeTokenAccount decodes a token account from bytes
func DecodeTokenAccount(data []byte) (*TokenAccount, error) {
	if len(data) < TokenAccountSize {
		return nil, errors.New("invalid token account data length")
	}

	acc := &TokenAccount{}
	buf := bytes.NewReader(data)

	// Mint (32 bytes)
	var mint [32]byte
	binary.Read(buf, binary.LittleEndian, &mint)
	acc.Mint = mint

	// Owner (32 bytes)
	var owner [32]byte
	binary.Read(buf, binary.LittleEndian, &owner)
	acc.Owner = owner

	// Amount (8 bytes)
	binary.Read(buf, binary.LittleEndian, &acc.Amount)

	// Delegate (4 bytes option + 32 bytes)
	var hasDelegate uint32
	binary.Read(buf, binary.LittleEndian, &hasDelegate)
	if hasDelegate == 1 {
		var delegate [32]byte
		binary.Read(buf, binary.LittleEndian, &delegate)
		acc.Delegate = &delegate
	}

	// State (1 byte)
	binary.Read(buf, binary.LittleEndian, &acc.State)

	// IsNative (4 bytes option + 8 bytes)
	var isNative uint32
	binary.Read(buf, binary.LittleEndian, &isNative)
	if isNative == 1 {
		var nativeAmount uint64
		binary.Read(buf, binary.LittleEndian, &nativeAmount)
		acc.IsNative = &nativeAmount
	}

	// DelegatedAmount (8 bytes)
	binary.Read(buf, binary.LittleEndian, &acc.DelegatedAmount)

	// CloseAuthority (4 bytes option + 32 bytes)
	var hasCloseAuthority uint32
	binary.Read(buf, binary.LittleEndian, &hasCloseAuthority)
	if hasCloseAuthority == 1 {
		var closeAuthority [32]byte
		binary.Read(buf, binary.LittleEndian, &closeAuthority)
		acc.CloseAuthority = &closeAuthority
	}

	return acc, nil
}

// DecodeMint decodes a mint account from bytes
func DecodeMint(data []byte) (*Mint, error) {
	if len(data) < MintSize {
		return nil, errors.New("invalid mint data length")
	}

	mint := &Mint{}
	buf := bytes.NewReader(data)

	// MintAuthority (4 bytes option + 32 bytes)
	var hasAuthority uint32
	binary.Read(buf, binary.LittleEndian, &hasAuthority)
	if hasAuthority == 1 {
		var authority [32]byte
		binary.Read(buf, binary.LittleEndian, &authority)
		mint.MintAuthority = &authority
	}

	// Supply (8 bytes)
	binary.Read(buf, binary.LittleEndian, &mint.Supply)

	// Decimals (1 byte)
	binary.Read(buf, binary.LittleEndian, &mint.Decimals)

	// IsInitialized (1 byte)
	binary.Read(buf, binary.LittleEndian, &mint.IsInitialized)

	// FreezeAuthority (4 bytes option + 32 bytes)
	var hasFreezeAuthority uint32
	binary.Read(buf, binary.LittleEndian, &hasFreezeAuthority)
	if hasFreezeAuthority == 1 {
		var freezeAuthority [32]byte
		binary.Read(buf, binary.LittleEndian, &freezeAuthority)
		mint.FreezeAuthority = &freezeAuthority
	}

	return mint, nil
}

// ===== Instruction Building =====

// TransferInstruction creates a transfer instruction
func TransferInstruction(
	source, destination, owner solana.PublicKey,
	amount uint64,
	decimals uint8,
	checked bool,
) *solana.GenericInstruction {
	if checked {
		return TransferCheckedInstruction(source, destination, owner, amount, decimals)
	}
	return TransferUncheckedInstruction(source, destination, owner, amount)
}

// TransferUncheckedInstruction creates an unchecked transfer instruction
func TransferUncheckedInstruction(
	source, destination, owner solana.PublicKey,
	amount uint64,
) *solana.GenericInstruction {
	data := make([]byte, 9)
	data[0] = Transfer
	binary.LittleEndian.PutUint64(data[1:9], amount)

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		solana.NewAccountMetaSlice(
			solana.NewAccountMeta(source, false, true),
			solana.NewAccountMeta(destination, false, true),
			solana.NewAccountMeta(owner, true, false),
		),
		data,
	)
}

// TransferCheckedInstruction creates a checked transfer instruction
func TransferCheckedInstruction(
	source, mint, destination, owner solana.PublicKey,
	amount uint64,
	decimals uint8,
) *solana.GenericInstruction {
	data := make([]byte, 10)
	data[0] = TransferChecked
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = decimals

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		solana.NewAccountMetaSlice(
			solana.NewAccountMeta(source, false, true),
			solana.NewAccountMeta(mint, false, false),
			solana.NewAccountMeta(destination, false, true),
			solana.NewAccountMeta(owner, true, false),
		),
		data,
	)
}

// CloseAccountInstruction creates a close account instruction
func CloseAccountInstruction(
	account, destination, owner solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{CloseAccount}

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		solana.NewAccountMetaSlice(
			solana.NewAccountMeta(account, false, true),
			solana.NewAccountMeta(destination, false, true),
			solana.NewAccountMeta(owner, true, false),
		),
		data,
	)
}

// SyncNativeInstruction creates a sync native instruction (for WSOL)
func SyncNativeInstruction(account solana.PublicKey) *solana.GenericInstruction {
	data := []byte{SyncNative}

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		solana.NewAccountMetaSlice(
			solana.NewAccountMeta(account, false, true),
		),
		data,
	)
}

// InitializeAccountInstruction creates an initialize account instruction
func InitializeAccountInstruction(
	account, mint, owner solana.PublicKey,
) *solana.GenericInstruction {
	data := []byte{InitializeAccount}

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		solana.NewAccountMetaSlice(
			solana.NewAccountMeta(account, false, true),
			solana.NewAccountMeta(mint, false, false),
			solana.NewAccountMeta(owner, true, false),
			solana.NewAccountMeta(solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"), false, false),
		),
		data,
	)
}

// ApproveInstruction creates an approve instruction
func ApproveInstruction(
	source, delegate, owner solana.PublicKey,
	amount uint64,
	decimals uint8,
	checked bool,
) *solana.GenericInstruction {
	var data []byte
	var accounts []*solana.AccountMeta

	if checked {
		data = make([]byte, 10)
		data[0] = ApproveChecked
		binary.LittleEndian.PutUint64(data[1:9], amount)
		data[9] = decimals
		// Need mint for checked
		accounts = solana.NewAccountMetaSlice(
			solana.NewAccountMeta(source, false, true),
			solana.NewAccountMeta(delegate, false, false),
			solana.NewAccountMeta(owner, true, false),
		)
	} else {
		data = make([]byte, 9)
		data[0] = Approve
		binary.LittleEndian.PutUint64(data[1:9], amount)
		accounts = solana.NewAccountMetaSlice(
			solana.NewAccountMeta(source, false, true),
			solana.NewAccountMeta(delegate, false, false),
			solana.NewAccountMeta(owner, true, false),
		)
	}

	return solana.NewGenericInstruction(
		solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		accounts,
		data,
	)
}

// ===== Amount Helpers =====

// LamportsToSol converts lamports to SOL
func LamportsToSol(lamports uint64) float64 {
	return float64(lamports) / 1_000_000_000.0
}

// SolToLamports converts SOL to lamports
func SolToLamports(sol float64) uint64 {
	return uint64(sol * 1_000_000_000)
}

// TokensToUIAmount converts raw token amount to UI amount
func TokensToUIAmount(amount uint64, decimals uint8) float64 {
	return float64(amount) / float64(uint64(10)<<decimals)
}

// UIAmountToTokens converts UI amount to raw token amount
func UIAmountToTokens(uiAmount float64, decimals uint8) uint64 {
	return uint64(uiAmount * float64(uint64(10)<<decimals))
}

// ===== Error Definitions =====

var (
	ErrInvalidAccountData   = errors.New("invalid account data")
	ErrInvalidMintData      = errors.New("invalid mint data")
	ErrAccountNotInitialized = errors.New("account not initialized")
	ErrAccountFrozen        = errors.New("account frozen")
	ErrInsufficientFunds    = errors.New("insufficient funds")
)

// TokenError represents a token program error
type TokenError struct {
	Code    uint32
	Message string
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("token error %d: %s", e.Code, e.Message)
}

// Standard Token Program errors
var (
	ErrTokenInvalidInstruction = &TokenError{Code: 0, Message: "invalid instruction"}
	ErrTokenInvalidAccountIndex = &TokenError{Code: 1, Message: "invalid account index"}
	ErrTokenInvalidAmount = &TokenError{Code: 2, Message: "invalid amount"}
	ErrTokenInvalidDecimals = &TokenError{Code: 3, Message: "invalid decimals"}
)
