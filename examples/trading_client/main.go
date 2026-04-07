// TradingClient Creation Example
//
// This example demonstrates two ways to create a TradingClient:
// 1. Simple method: NewTradeClient() - creates client with its own infrastructure
// 2. Shared method: NewTradeClientFromInfrastructure() - reuses existing infrastructure
//
// For multi-wallet scenarios, see the shared_infrastructure example.

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

	// Method 1: Simple - NewTradeClient() (recommended for single wallet)
	client1, err := createTradingClientSimple(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Method 1: Created TradingClient with NewTradeClient()\n")
	fmt.Printf("  Wallet: %s\n", client1.PayerPubkey())

	// Method 2: From infrastructure (recommended for multiple wallets)
	client2, err := createTradingClientFromInfrastructure(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nMethod 2: Created TradingClient with FromInfrastructure()\n")
	fmt.Printf("  Wallet: %s\n", client2.PayerPubkey())
}

// Method 1: Create TradingClient using TradeConfig (simple, self-contained)
//
// Use this when you have a single wallet or don't need to share infrastructure.
func createTradingClientSimple(ctx context.Context) (*trading.TradeClient, error) {
	// Use your keypair here
	payer := solana.NewWallet()

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, URL: rpcURL},
		{Type: soltradesdk.SwqosTypeJito, UUID: "your_uuid", Region: soltradesdk.SwqosRegionFrankfurt},
		{Type: soltradesdk.SwqosTypeBloxroute, APIToken: "your_api_token", Region: soltradesdk.SwqosRegionFrankfurt},
		{Type: soltradesdk.SwqosTypeZeroSlot, APIToken: "your_api_token", Region: soltradesdk.SwqosRegionFrankfurt},
		{Type: soltradesdk.SwqosTypeTemporal, APIToken: "your_api_token", Region: soltradesdk.SwqosRegionFrankfurt},
	}

	tradeConfig := soltradesdk.NewTradeConfigBuilder(rpcURL).
		SwqosConfigs(swqosConfigs).
		// MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	// Creates new infrastructure internally
	client, err := trading.NewTradeClient(ctx, payer, tradeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade client: %w", err)
	}

	return client, nil
}

// Method 2: Create TradingClient from shared infrastructure
//
// Use this when you have multiple wallets sharing the same configuration.
// The infrastructure (RPC client, SWQOS clients) is created once and shared.
func createTradingClientFromInfrastructure(ctx context.Context) (*trading.TradeClient, error) {
	// Use your keypair here
	payer := solana.NewWallet()

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	commitment := rpc.CommitmentConfirmed

	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, URL: rpcURL},
		{Type: soltradesdk.SwqosTypeJito, UUID: "your_uuid", Region: soltradesdk.SwqosRegionFrankfurt},
	}

	// Create infrastructure separately (can be shared across multiple wallets)
	infraConfig := &common.InfrastructureConfig{
		RPCURL:      rpcURL,
		SwqosConfigs: swqosConfigs,
		Commitment:  commitment,
	}
	infrastructure, err := trading.NewInfrastructure(ctx, infraConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create infrastructure: %w", err)
	}

	// Create client from existing infrastructure (fast, no async needed)
	client := trading.NewTradeClientFromInfrastructure(payer, infrastructure, true)

	return client, nil
}
