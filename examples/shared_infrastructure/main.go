// Shared Infrastructure Example
//
// This example demonstrates how to share infrastructure across multiple wallets.
// The infrastructure (RPC client, SWQOS clients) is created once and shared.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/your-org/sol-trade-sdk-go/pkg/common"
	"github.com/your-org/sol-trade-sdk-go/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}

	// Configure SWQoS services
	swqosConfigs := []common.SwqosConfig{
		{Type: common.SwqosTypeDefault, URL: rpcURL},
		{Type: common.SwqosTypeJito, APIKey: "your_uuid", Region: common.SwqosRegionFrankfurt},
		{Type: common.SwqosTypeBloxroute, APIKey: "your_api_token", Region: common.SwqosRegionFrankfurt},
	}

	// Create infrastructure once (expensive operation)
	infraConfig := &common.InfrastructureConfig{
		RPCURL:      rpcURL,
		SwqosConfigs: swqosConfigs,
		Commitment:  rpc.CommitmentConfirmed,
	}
	infrastructure, err := trading.NewInfrastructure(ctx, infraConfig)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Infrastructure created successfully!")

	// Create multiple clients sharing the same infrastructure (fast)
	payer1 := solana.NewWallet()
	payer2 := solana.NewWallet()
	payer3 := solana.NewWallet()

	client1 := trading.NewTradeClientFromInfrastructure(payer1, infrastructure, true)
	client2 := trading.NewTradeClientFromInfrastructure(payer2, infrastructure, true)
	client3 := trading.NewTradeClientFromInfrastructure(payer3, infrastructure, true)

	fmt.Printf("Client 1: %s\n", client1.PayerPubkey())
	fmt.Printf("Client 2: %s\n", client2.PayerPubkey())
	fmt.Printf("Client 3: %s\n", client3.PayerPubkey())

	// All clients share the same RPC and SWQoS connections
	fmt.Println("All clients share the same infrastructure!")
}
