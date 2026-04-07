// Raydium AMM V4 Trading Example
//
// This example demonstrates how to trade on Raydium AMM V4.
// Subscribe to swap events via gRPC and execute a buy + sell.

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
	fmt.Println("Testing Raydium AMM V4 trading...")

	// In a real scenario, subscribe to gRPC events and execute trade
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

func raydiumAmmV4Trade(
	ctx context.Context,
	client *trading.TradeClient,
	amm solana.PublicKey,
	coinMint solana.PublicKey,
	pcMint solana.PublicKey,
	tokenCoin solana.PublicKey,
	tokenPc solana.PublicKey,
	coinReserve uint64,
	pcReserve uint64,
) error {
	slippageBasisPoints := uint64(100)
	recentBlockhash, err := client.GetLatestBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	// Determine token type (WSOL or USDC)
	WSOLTokenAccount := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	USDCTokenAccount := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	isWSOL := pcMint.Equals(WSOLTokenAccount) || coinMint.Equals(WSOLTokenAccount)
	inputTokenType := soltradesdk.TradeTokenTypeWSOL
	if !isWSOL {
		inputTokenType = soltradesdk.TradeTokenTypeUSDC
	}

	// Determine mint to trade
	mintPubkey := coinMint
	if pcMint.Equals(WSOLTokenAccount) || pcMint.Equals(USDCTokenAccount) {
		mintPubkey = coinMint
	} else {
		mintPubkey = pcMint
	}

	// Build params
	params := &trading.RaydiumAmmV4Params{
		Amm:         amm,
		CoinMint:    coinMint,
		PcMint:      pcMint,
		TokenCoin:   tokenCoin,
		TokenPc:     tokenPc,
		CoinReserve: coinReserve,
		PcReserve:   pcReserve,
	}

	inputTokenAmount := uint64(100_000) // 0.0001 SOL or USDC

	buyParams := &trading.TradeBuyParams{
		DexType:              soltradesdk.DexTypeRaydiumAmmV4,
		InputTokenType:       inputTokenType,
		Mint:                 mintPubkey,
		InputTokenAmount:     inputTokenAmount,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateInputTokenATA:  isWSOL,
		CloseInputTokenATA:   isWSOL,
		CreateMintATA:        true,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams:      params,
	}

	// Execute buy
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)

	// Get token balance for sell
	tokenBalance, err := client.GetTokenBalance(ctx, mintPubkey)
	if err != nil {
		return fmt.Errorf("failed to get token balance: %w", err)
	}
	fmt.Printf("Token balance: %d\n", tokenBalance)

	sellParams := &trading.TradeSellParams{
		DexType:              soltradesdk.DexTypeRaydiumAmmV4,
		OutputTokenType:      inputTokenType,
		Mint:                 mintPubkey,
		InputTokenAmount:     tokenBalance,
		SlippageBasisPoints:  &slippageBasisPoints,
		RecentBlockhash:      &recentBlockhash,
		WaitConfirmed:        true,
		CreateOutputTokenATA: isWSOL,
		CloseOutputTokenATA:  isWSOL,
		CloseMintTokenATA:    false,
		GasFeeStrategy:       gasFeeStrategy,
		ExtensionParams:      params, // In real scenario, fetch fresh params via RPC
	}

	// Execute sell
	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		return fmt.Errorf("sell failed: %w", err)
	}
	fmt.Printf("Sell signature: %s\n", sellResult.Signature)
	fmt.Println("Raydium AMM V4 trade completed!")

	return nil
}
