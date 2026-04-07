// Address Lookup Table Example
//
// This example demonstrates how to use Address Lookup Tables (ALT)
// to optimize transaction size and reduce fees.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/your-org/sol-trade-sdk-go/pkg/addresslookup"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()
	fmt.Println("Address Lookup Table Example")

	// Create RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

	// Example ALT address
	altAddress := solana.MustPublicKeyFromBase58("your_alt_address_here")

	// Fetch ALT from chain
	alt, err := addresslookup.FetchAddressLookupTableAccount(
		ctx,
		rpcClient,
		altAddress,
	)
	if err != nil {
		log.Printf("Failed to fetch ALT: %v", err)
		fmt.Println("ALT example completed (no real ALT provided)")
		return
	}

	fmt.Printf("ALT contains %d addresses\n", len(alt.Addresses))

	// List addresses
	for i, addr := range alt.Addresses {
		fmt.Printf("  [%d] %s\n", i, addr.String())
	}

	fmt.Println("\nAddress Lookup Table example completed!")
}
