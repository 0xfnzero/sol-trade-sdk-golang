// WSOL Wrapper Example
//
// This example demonstrates how to:
// 1. Wrap SOL to WSOL
// 2. Partially unwrap WSOL back to SOL using seed account
// 3. Close WSOL account and unwrap remaining balance

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()

	fmt.Println("WSOL Wrapper Example")
	fmt.Println("This example demonstrates:")
	fmt.Println("1. Wrapping SOL to WSOL")
	fmt.Println("2. Partial unwrapping WSOL back to SOL")
	fmt.Println("3. Closing WSOL account and unwrapping remaining balance")

	// Initialize client
	client, err := createClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nClient created: %s\n", client.PayerPubkey())

	// Example 1: Wrap SOL to WSOL
	fmt.Println("\n--- Example 1: Wrapping SOL to WSOL ---")
	wrapAmount := uint64(1_000_000) // 0.001 SOL in lamports
	fmt.Printf("Wrapping %d lamports (0.001 SOL) to WSOL...\n", wrapAmount)

	signature, err := client.WrapSOLToWSOL(ctx, wrapAmount)
	if err != nil {
		fmt.Printf("Failed to wrap SOL to WSOL: %v\n", err)
		return
	}
	fmt.Printf("Successfully wrapped SOL to WSOL!\n")
	fmt.Printf("Transaction signature: %s\n", signature)
	fmt.Printf("Explorer: https://solscan.io/tx/%s\n", signature)

	// Wait before partial unwrapping
	fmt.Println("\nWaiting 3 seconds before partial unwrapping...")
	time.Sleep(3 * time.Second)

	// Example 2: Unwrap half of the WSOL back to SOL using seed account
	fmt.Println("\n--- Example 2: Unwrapping half of WSOL back to SOL ---")
	unwrapAmount := wrapAmount / 2 // Half of the wrapped amount
	fmt.Printf("Unwrapping %d lamports (0.0005 SOL) back to SOL...\n", unwrapAmount)

	signature, err = client.WrapWSOLToSOL(ctx, unwrapAmount)
	if err != nil {
		fmt.Printf("Failed to unwrap WSOL to SOL: %v\n", err)
	} else {
		fmt.Printf("Successfully unwrapped half of WSOL back to SOL!\n")
		fmt.Printf("Transaction signature: %s\n", signature)
		fmt.Printf("Explorer: https://solscan.io/tx/%s\n", signature)
	}

	// Wait before final unwrapping
	fmt.Println("\nWaiting 3 seconds before final unwrapping...")
	time.Sleep(3 * time.Second)

	// Example 3: Close WSOL account and unwrap all remaining balance
	fmt.Println("\n--- Example 3: Closing WSOL account ---")
	fmt.Println("Closing WSOL account and unwrapping all remaining balance to SOL...")

	signature, err = client.CloseWSOL(ctx)
	if err != nil {
		fmt.Printf("Failed to close WSOL account: %v\n", err)
	} else {
		fmt.Printf("Successfully closed WSOL account and unwrapped remaining balance!\n")
		fmt.Printf("Transaction signature: %s\n", signature)
		fmt.Printf("Explorer: https://solscan.io/tx/%s\n", signature)
	}

	fmt.Println("\nWSOL Wrapper example completed!")
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
