// PumpFun Sniper Trading Example
//
// This example demonstrates how to snipe new tokens on PumpFun.
// Listen for creator first buy events and execute a buy + sell.

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
	fmt.Println("Waiting for PumpFun events...")

	// In a real scenario, you would subscribe to gRPC events
	// and call pumpfunSniperTrade when a snipe opportunity is detected
}

func createClient(ctx context.Context) (*trading.TradeClient, error) {
	payer := solana.NewWallet() // Use your keypair here
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, URL: rpcURL},
		{Type: soltradesdk.SwqosTypeJito, APIKey: "your_uuid", Region: soltradesdk.SwqosRegionFrankfurt},
	}

	tradeConfig := soltradesdk.NewTradeConfigBuilder(rpcURL).
		SwqosConfigs(swqosConfigs).
		// MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	return trading.NewTradeClient(ctx, payer, tradeConfig)
}

func pumpfunSniperTrade(
	ctx context.Context,
	client *trading.TradeClient,
	mint solana.PublicKey,
	bondingCurve solana.PublicKey,
	associatedBondingCurve solana.PublicKey,
	creator solana.PublicKey,
	creatorVault solana.PublicKey,
	feeRecipient solana.PublicKey,
	virtualTokenReserves uint64,
	virtualSolReserves uint64,
	realTokenReserves uint64,
	realSolReserves uint64,
	isCashbackCoin bool,
) error {
	slippageBasisPoints := uint64(300)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// Buy parameters
	buySOLAmount := uint64(100_000) // 0.0001 SOL

	buyParams := &trading.TradeBuyParams{
		DexType:              soltradesdk.DexTypePumpFun,
		InputTokenType:       soltradesdk.TradeTokenTypeSOL,
		Mint:                 mint,
		InputTokenAmount:     buySOLAmount,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateInputTokenATA:  true,
		CloseInputTokenATA:   true,
		CreateMintATA:        true,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams: &trading.PumpFunParams{
			BondingCurve:         bondingCurve,
			AssociatedBondingCurve: associatedBondingCurve,
			Mint:                 mint,
			Creator:              creator,
			CreatorVault:         creatorVault,
			VirtualTokenReserves: virtualTokenReserves,
			VirtualSolReserves:   virtualSolReserves,
			RealTokenReserves:    realTokenReserves,
			RealSolReserves:      realSolReserves,
			HasCreator:           true,
			FeeRecipient:         feeRecipient,
			IsCashbackCoin:       isCashbackCoin,
		},
	}

	// Execute buy
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)

	// Get token balance for sell
	tokenBalance, err := client.GetTokenBalance(ctx, mint)
	if err != nil {
		return fmt.Errorf("failed to get token balance: %w", err)
	}
	fmt.Printf("Token balance: %d\n", tokenBalance)

	// Sell parameters
	sellParams := &trading.TradeSellParams{
		DexType:              soltradesdk.DexTypePumpFun,
		OutputTokenType:      soltradesdk.TradeTokenTypeSOL,
		Mint:                 mint,
		InputTokenAmount:     tokenBalance,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateOutputTokenATA: true,
		CloseOutputTokenATA:  true,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams: &trading.PumpFunParams{
			BondingCurve:         bondingCurve,
			AssociatedBondingCurve: associatedBondingCurve,
			Mint:                 mint,
			Creator:              creator,
			CreatorVault:         creatorVault,
			VirtualTokenReserves: virtualTokenReserves,
			VirtualSolReserves:   virtualSolReserves,
			RealTokenReserves:    realTokenReserves,
			RealSolReserves:      realSolReserves,
			HasCreator:           true,
			FeeRecipient:         feeRecipient,
			IsCashbackCoin:       isCashbackCoin,
		},
	}

	// Execute sell
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("Snipe buy + sell completed!")

	return nil
}
