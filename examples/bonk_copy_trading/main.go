// Bonk Copy Trading Example
//
// This example demonstrates how to copy trade on Bonk.
// Subscribe to Bonk buy/sell events via gRPC and execute a buy + sell.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	soltradesdk "github.com/your-org/sol-trade-sdk-go"
	"github.com/your-org/sol-trade-sdk-go/pkg/common"
	"github.com/your-org/sol-trade-sdk-go/pkg/trading"
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

	// In a real scenario, subscribe to gRPC events for Bonk trades
	// and call bonkCopyTrade when a trade event is received
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

func bonkCopyTrade(
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
	virtualBase uint64,
	virtualQuote uint64,
	realBaseAfter uint64,
	realQuoteAfter uint64,
) error {
	slippageBasisPoints := uint64(100)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// Determine token type
	USD1TokenAccount := solana.MustPublicKeyFromBase58("B9C6PQJqM9vLZHMvPMJUfzHvPrPxYT4rL5hXhgS3nYVr")
	inputTokenType := soltradesdk.TradeTokenTypeSOL
	if quoteTokenMint.Equals(USD1TokenAccount) {
		inputTokenType = soltradesdk.TradeTokenTypeUSD1
	}

	buySOLAmount := uint64(100_000) // 0.0001 SOL or USD1

	// Build params from trade
	bonkParams := &trading.BonkParams{
		VirtualBase:              virtualBase,
		VirtualQuote:             virtualQuote,
		RealBaseAfter:            realBaseAfter,
		RealQuoteAfter:           realQuoteAfter,
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
		InputTokenType:       inputTokenType,
		Mint:                 baseTokenMint,
		InputTokenAmount:     buySOLAmount,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateInputTokenATA:  true,
		CloseInputTokenATA:   false,
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

	sellParams := &trading.TradeSellParams{
		DexType:              soltradesdk.DexTypeBonk,
		OutputTokenType:      inputTokenType,
		Mint:                 baseTokenMint,
		InputTokenAmount:     tokenBalance,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateOutputTokenATA: false,
		CloseOutputTokenATA:  false,
		CloseMintTokenATA:    false,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams:      bonkParams,
	}

	// Execute sell
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("Bonk copy trade completed!")

	return nil
}
