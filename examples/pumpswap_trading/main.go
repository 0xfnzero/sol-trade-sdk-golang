// PumpSwap Trading Example
//
// This example demonstrates how to trade on PumpSwap.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
	fmt.Println("Testing PumpSwap trading...")

	// Example pool and mint
	pool := solana.MustPublicKeyFromBase58("9qKxzRejsV6Bp2zkefXWCbGvg61c3hHei7ShXJ4FythA")
	mint := solana.MustPublicKeyFromBase58("2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv")

	// In a real scenario, fetch params via RPC and execute trade
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

func pumpSwapTrade(
	ctx context.Context,
	client *trading.TradeClient,
	pool solana.PublicKey,
	mint solana.PublicKey,
) error {
	slippageBasisPoints := uint64(100)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// In a real scenario, fetch params via RPC:
	// pumpSwapParams := trading.FetchPumpSwapParamsByRPC(ctx, client.RPC(), pool)

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
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)

	time.Sleep(4 * time.Second)

	// Get token balance for sell
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
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("PumpSwap trade completed!")

	return nil
}
