// Seed Trading Example
//
// This example demonstrates how to trade using seed optimization
// for faster ATA derivation and account operations.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()

	// Create client with seed optimization enabled
	client, err := createClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Client created: %s\n", client.PayerPubkey())
	fmt.Printf("Seed optimization enabled: %v\n", client.UseSeedOptimize())

	// Example pool and mint addresses
	pool := solana.MustPublicKeyFromBase58("9qKxzRejsV6Bp2zkefXWCbGvg61c3hHei7ShXJ4FythA")
	mint := solana.MustPublicKeyFromBase58("2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv")

	// In a real scenario, you would call seedTradingExample
	// with actual pool and mint addresses
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
		// UseSeedOptimize(true). // Enable seed optimization
		// MEVProtection(true).   // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	return trading.NewTradeClient(ctx, payer, tradeConfig)
}

func seedTradingExample(
	ctx context.Context,
	client *trading.TradeClient,
	pool solana.PublicKey,
	mint solana.PublicKey,
) error {
	fmt.Println("Testing PumpSwap trading with seed optimization...")

	slippageBasisPoints := uint64(100)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// In a real scenario, fetch params via RPC:
	// pumpSwapParams, err := trading.FetchPumpSwapParamsByRPC(ctx, client.RPC(), pool)

	buySOLAmount := uint64(100_000) // 0.0001 WSOL

	buyParams := &trading.TradeBuyParams{
		DexType:              soltradesdk.DexTypePumpSwap,
		InputTokenType:       soltradesdk.TradeTokenTypeWSOL,
		Mint:                 mint,
		InputTokenAmount:     buySOLAmount,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateInputTokenATA:  true,
		CloseInputTokenATA:   true,
		CreateMintATA:        true,
		GasFeeStrategy:       gasFeeStrategy,
		// ExtensionParams: pumpSwapParams,
	}

	// Execute buy
	fmt.Println("Buying tokens from PumpSwap...")
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)

	// Wait for confirmation
	time.Sleep(4 * time.Second)

	// Get token balance for sell (uses seed optimization internally)
	tokenBalance, err := client.GetTokenBalance(ctx, mint)
	if err != nil {
		return fmt.Errorf("failed to get token balance: %w", err)
	}
	fmt.Printf("Token balance: %d\n", tokenBalance)

	sellParams := &trading.TradeSellParams{
		DexType:              soltradesdk.DexTypePumpSwap,
		OutputTokenType:      soltradesdk.TradeTokenTypeWSOL,
		Mint:                 mint,
		InputTokenAmount:     tokenBalance,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateOutputTokenATA: true,
		CloseOutputTokenATA:  true,
		CloseMintTokenATA:    false,
		GasFeeStrategy:       gasFeeStrategy,
		// ExtensionParams: pumpSwapParams,
	}

	// Execute sell
	fmt.Println("Selling tokens...")
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("Seed trading example completed!")

	return nil
}
