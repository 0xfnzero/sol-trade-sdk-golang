package main

import (
	"context"
	"fmt"
	"os"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/addresslookup"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()
	altAddressText := os.Getenv("ALT_ADDRESS")
	fmt.Println("Address Lookup Table example prepared.")
	if altAddressText == "" {
		fmt.Println("Set ALT_ADDRESS to fetch a real lookup table.")
		return
	}

	altAddress := solana.MustPublicKeyFromBase58(altAddressText)
	alt, err := addresslookup.FetchAddressLookupTableAccount(ctx, rpc.New(exampleutil.RPCURL()), altAddress, rpc.CommitmentConfirmed)
	if err != nil {
		fmt.Println("Failed to fetch ALT:", err)
		return
	}
	fmt.Println("ALT size:", len(alt.Addresses))
}
