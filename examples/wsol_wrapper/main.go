package main

import (
	"context"
	"fmt"
	"log"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
)

func main() {
	ctx := context.Background()
	client, err := exampleutil.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	amountLamports := uint64(1_000_000)
	fmt.Println(exampleutil.DescribeDryRun("WSOL wrap and close example"))
	fmt.Println("Wallet:", client.GetPayer())
	fmt.Println("Wrap amount:", amountLamports)
	if exampleutil.RunLive() {
		fmt.Println("Use pkg/common WSOL instructions with a signed transaction before sending live funds.")
	}
}
