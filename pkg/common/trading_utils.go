package common

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

// ===== Trading Utilities - 100% port from Rust: src/trading/common/utils.rs =====

// TradingUtils provides async RPC utilities for trading operations
type TradingUtils struct {
	client *rpc.Client
}

// NewTradingUtils creates a new trading utils instance
func NewTradingUtils(client *rpc.Client) *TradingUtils {
	return &TradingUtils{client: client}
}

// GetMultiTokenBalances gets the balances of two tokens in the pool.
// 100% from Rust: src/trading/common/utils.rs get_multi_token_balances
func (t *TradingUtils) GetMultiTokenBalances(
	ctx context.Context,
	token0Vault solana.PublicKey,
	token1Vault solana.PublicKey,
) (uint64, uint64, error) {
	// Get token0 balance
	token0Result, err := t.client.GetTokenAccountBalance(ctx, token0Vault, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get token0 balance: %w", err)
	}
	token0Amount, err := strconv.ParseUint(token0Result.Value.Amount, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse token0 balance: %w", err)
	}

	// Get token1 balance
	token1Result, err := t.client.GetTokenAccountBalance(ctx, token1Vault, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get token1 balance: %w", err)
	}
	token1Amount, err := strconv.ParseUint(token1Result.Value.Amount, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse token1 balance: %w", err)
	}

	return token0Amount, token1Amount, nil
}

// GetTokenBalance gets token balance for a payer's ATA.
// 100% from Rust: src/trading/common/utils.rs get_token_balance
func (t *TradingUtils) GetTokenBalance(
	ctx context.Context,
	payer solana.PublicKey,
	mint solana.PublicKey,
) (uint64, error) {
	return t.GetTokenBalanceWithOptions(ctx, payer, mint, solana.MustPublicKeyFromBase58(TokenProgramID), false)
}

// GetTokenBalanceWithOptions gets token balance using consistent ATA derivation (optional seed).
// 100% from Rust: src/trading/common/utils.rs get_token_balance_with_options
func (t *TradingUtils) GetTokenBalanceWithOptions(
	ctx context.Context,
	payer solana.PublicKey,
	mint solana.PublicKey,
	tokenProgram solana.PublicKey,
	useSeed bool,
) (uint64, error) {
	var ata solana.PublicKey
	var err error

	if useSeed {
		ata, _, err = GetAssociatedTokenAddressUseSeed(payer, mint)
	} else {
		ata, _, err = GetAssociatedTokenAddress(payer, mint, false, tokenProgram)
	}
	if err != nil {
		return 0, err
	}

	result, err := t.client.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(result.Value.Amount, 10, 64)
}

// GetSolBalance gets SOL balance for an account.
// 100% from Rust: src/trading/common/utils.rs get_sol_balance
func (t *TradingUtils) GetSolBalance(
	ctx context.Context,
	account solana.PublicKey,
) (uint64, error) {
	result, err := t.client.GetBalance(ctx, account, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}
	return result.Value, nil
}

// TransferSOL transfers SOL from payer to receive_wallet.
// 100% from Rust: src/trading/common/utils.rs transfer_sol
func (t *TradingUtils) TransferSOL(
	ctx context.Context,
	payer solana.PrivateKey,
	receiveWallet solana.PublicKey,
	amount uint64,
) (solana.Signature, error) {
	if amount == 0 {
		return solana.Signature{}, errors.New("transfer_sol: Amount cannot be zero")
	}

	balance, err := t.GetSolBalance(ctx, payer.PublicKey())
	if err != nil {
		return solana.Signature{}, err
	}
	if balance < amount {
		return solana.Signature{}, errors.New("insufficient balance")
	}

	// Create transfer instruction
	transferIx := system.NewTransferInstructionBuilder().
		SetFundingAccount(payer.PublicKey()).
		SetRecipientAccount(receiveWallet).
		SetLamports(amount).
		Build()

	// Get recent blockhash
	recentBlockhash, err := t.client.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return solana.Signature{}, err
	}

	// Build transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return solana.Signature{}, err
	}

	// Sign transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		return nil
	})
	if err != nil {
		return solana.Signature{}, err
	}

	// Send transaction
	return t.client.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight: true,
	})
}

// CloseTokenAccount closes the associated token account for a specified token.
// 100% from Rust: src/trading/common/utils.rs close_token_account
func (t *TradingUtils) CloseTokenAccount(
	ctx context.Context,
	payer solana.PrivateKey,
	mint solana.PublicKey,
) (solana.Signature, error) {
	// Get associated token account address
	tokenProgram := solana.MustPublicKeyFromBase58(TokenProgramID)
	ata, _, err := GetAssociatedTokenAddress(payer.PublicKey(), mint, false, tokenProgram)
	if err != nil {
		return solana.Signature{}, err
	}

	// Check if account exists
	accountInfo, err := t.client.GetAccountInfo(ctx, ata)
	if err != nil || accountInfo == nil || accountInfo.Value == nil {
		// Account doesn't exist, return success
		return solana.Signature{}, nil
	}

	// Build close account instruction
	builder := NewTokenInstructionBuilder()
	closeIx := builder.BuildCloseAccount(ata, payer.PublicKey(), payer.PublicKey())

	// Get recent blockhash
	recentBlockhash, err := t.client.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return solana.Signature{}, err
	}

	// Build transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{closeIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return solana.Signature{}, err
	}

	// Sign transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		return nil
	})
	if err != nil {
		return solana.Signature{}, err
	}

	// Send transaction
	return t.client.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight: true,
	})
}

// ===== Global Functions (using default client) =====

// GetTokenBalance gets token balance using provided client
func GetTokenBalance(
	ctx context.Context,
	client *rpc.Client,
	payer solana.PublicKey,
	mint solana.PublicKey,
) (uint64, error) {
	utils := NewTradingUtils(client)
	return utils.GetTokenBalance(ctx, payer, mint)
}

// GetSolBalance gets SOL balance using provided client
func GetSolBalance(
	ctx context.Context,
	client *rpc.Client,
	account solana.PublicKey,
) (uint64, error) {
	utils := NewTradingUtils(client)
	return utils.GetSolBalance(ctx, account)
}

// GetMultiTokenBalances gets multiple token balances using provided client
func GetMultiTokenBalances(
	ctx context.Context,
	client *rpc.Client,
	token0Vault solana.PublicKey,
	token1Vault solana.PublicKey,
) (uint64, uint64, error) {
	utils := NewTradingUtils(client)
	return utils.GetMultiTokenBalances(ctx, token0Vault, token1Vault)
}

// TransferSOL transfers SOL using provided client
func TransferSOL(
	ctx context.Context,
	client *rpc.Client,
	payer solana.PrivateKey,
	receiveWallet solana.PublicKey,
	amount uint64,
) (solana.Signature, error) {
	utils := NewTradingUtils(client)
	return utils.TransferSOL(ctx, payer, receiveWallet, amount)
}

// CloseTokenAccount closes token account using provided client
func CloseTokenAccount(
	ctx context.Context,
	client *rpc.Client,
	payer solana.PrivateKey,
	mint solana.PublicKey,
) (solana.Signature, error) {
	utils := NewTradingUtils(client)
	return utils.CloseTokenAccount(ctx, payer, mint)
}
