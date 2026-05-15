package main

import (
	"context"
	"fmt"
	"log"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

func main() {
	ctx := context.Background()
	client, err := exampleutil.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	buyParams := exampleutil.ExampleBuyParams(soltradesdk.DexTypeBonk)
	sellParams := exampleutil.ExampleSellParams(soltradesdk.DexTypeBonk)

	fmt.Println(exampleutil.DescribeDryRun("Bonk sniper example using USD1/SOL token-type routing"))
	fmt.Println("Wallet:", client.GetPayer())
	fmt.Printf("Buy params: dex=%s amount=%d\n", buyParams.DexType, buyParams.InputTokenAmount)
	fmt.Printf("Sell params: dex=%s amount=%d\n", sellParams.DexType, sellParams.InputTokenAmount)

	if exampleutil.RunLive() {
		fmt.Println("Replace placeholder params with real on-chain/parser values before executing trades.")
	}
}
