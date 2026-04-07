// Nonce Cache Example
//
// This example demonstrates how to use durable nonce for transaction submission.
// Use durable nonce to implement transaction replay protection and optimize
// transaction processing when using multiple MEV services.

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

	// Nonce account must be created beforehand
	nonceAccount := solana.MustPublicKeyFromBase58("use_your_nonce_account_here")
	fmt.Printf("Using nonce account: %s\n", nonceAccount)

	// In a real scenario:
	// 1. Fetch nonce info
	// 2. Execute trade with durable_nonce
}

func createClient(ctx context.Context) (*trading.TradeClient, error) {
	payer := solana.NewWallet()
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, URL: rpcURL},
		{Type: soltradesdk.SwqosTypeJito, UUID: "your_uuid"},
		{Type: soltradesdk.SwqosTypeBloxroute, APIToken: "your_api_token"},
	}

	tradeConfig := soltradesdk.NewTradeConfigBuilder(rpcURL).
		SwqosConfigs(swqosConfigs).
		// MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	return trading.NewTradeClient(ctx, payer, tradeConfig)
}

func tradeWithNonce(
	ctx context.Context,
	client *trading.TradeClient,
	nonceAccount solana.PublicKey,
	mint solana.PublicKey,
) error {
	// Fetch nonce info
	durableNonce, err := common.FetchNonceInfo(ctx, client.RPC(), nonceAccount)
	if err != nil {
		return fmt.Errorf("failed to fetch nonce info: %w", err)
	}

	fmt.Printf("Nonce authority: %s\n", durableNonce.Authority.String())
	fmt.Printf("Nonce value: %s\n", durableNonce.Nonce.String())

	// Configure gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	buySOLAmount := uint64(100_000)

	buyParams := &trading.TradeBuyParams{
		DexType:              soltradesdk.DexTypePumpFun,
		InputTokenType:       soltradesdk.TradeTokenTypeSOL,
		Mint:                 mint,
		InputTokenAmount:     buySOLAmount,
		SlippageBasisPoints:  ptrUint64(100),
		RecentBlockhash:      nil, // Not used when durable_nonce is provided
		WaitConfirmed:        true,
		CreateInputTokenATA:  false,
		CloseInputTokenATA:   false,
		CreateMintATA:        true,
		DurableNonce:         durableNonce,
		GasFeeStrategy:       gasFeeStrategy,
		// ExtensionParams: pumpFunParams,
	}

	// Execute buy with nonce
	buyResult, err := client.Buy(ctx, buyParams)
	if err != nil {
		return fmt.Errorf("buy failed: %w", err)
	}
	fmt.Printf("Buy signature: %s\n", buyResult.Signature)
	fmt.Println("Trade with nonce completed!")

	return nil
}

func ptrUint64(v uint64) *uint64 {
	return &v
}
