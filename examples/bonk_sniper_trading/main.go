// Bonk Sniper Trading Example
//
// This example demonstrates how to snipe new tokens on Bonk.
// Listen for developer token creation events and execute a buy + sell.
// Uses ShredStream for ultra-low latency.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()

	client, err := createClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Client created: %s\n", client.PayerPubkey())
	fmt.Println("Waiting for Bonk events...")

	// In a real scenario, subscribe to ShredStream for Bonk events
	// and call bonkSniperTrade when is_dev_create_token_trade is detected
}

func createClient(ctx context.Context) (*trading.TradeClient, error) {
	payer := solana.NewWallet()
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, URL: rpcURL},
	}

	tradeConfig := soltradesdk.NewTradeConfigBuilder(rpcURL).
		SwqosConfigs(swqosConfigs).
		// MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	return trading.NewTradeClient(ctx, payer, tradeConfig)
}

func bonkSniperTrade(
	ctx context.Context,
	client *trading.TradeClient,
	baseTokenMint solana.PublicKey,
	quoteTokenMint solana.PublicKey,
	poolState solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	baseTokenProgram solana.PublicKey,
	platformConfig solana.PublicKey,
	platformAssociatedAccount solana.PublicKey,
	creatorAssociatedAccount solana.PublicKey,
	globalConfig solana.PublicKey,
) error {
	slippageBasisPoints := uint64(300)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// Determine token type
	USD1TokenAccount := solana.MustPublicKeyFromBase58("B9C6PQJqM9vLZHMvPMJUfzHvPrPxYT4rL5hXhgS3nYVr")
	tokenType := soltradesdk.TradeTokenTypeSOL
	if quoteTokenMint.Equals(USD1TokenAccount) {
		tokenType = soltradesdk.TradeTokenTypeUSD1
	}

	buySOLAmount := uint64(100_000) // 0.0001 SOL or USD1

	// Build params from dev trade
	bonkParams := &trading.BonkParams{
		PoolState:                poolState,
		BaseVault:                baseVault,
		QuoteVault:               quoteVault,
		BaseTokenProgram:         baseTokenProgram,
		PlatformConfig:           platformConfig,
		PlatformAssociatedAccount: platformAssociatedAccount,
		CreatorAssociatedAccount: creatorAssociatedAccount,
		GlobalConfig:             globalConfig,
	}

	buyParams := &trading.TradeBuyParams{
		DexType:              soltradesdk.DexTypeBonk,
		InputTokenType:       tokenType,
		Mint:                 baseTokenMint,
		InputTokenAmount:     buySOLAmount,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateInputTokenATA:  true,
		CloseInputTokenATA:   true,
		CreateMintATA:        true,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams:      bonkParams,
	}

	// Execute buy
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)

	// Get token balance for sell
	tokenBalance, err := client.GetTokenBalance(ctx, baseTokenMint)
	if err != nil {
		return fmt.Errorf("failed to get token balance: %w", err)
	}
	fmt.Printf("Token balance: %d\n", tokenBalance)

	// Sell with immediate sell params
	sellParams := &trading.TradeSellParams{
		DexType:              soltradesdk.DexTypeBonk,
		OutputTokenType:      tokenType,
		Mint:                 baseTokenMint,
		InputTokenAmount:     tokenBalance,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateOutputTokenATA: true,
		CloseOutputTokenATA:  true,
		CloseMintTokenATA:    false,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams:      bonkParams.ImmediateSell(),
	}

	// Execute sell
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("Bonk snipe buy + sell completed!")

	return nil
}
