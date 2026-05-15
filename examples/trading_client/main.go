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
	config := exampleutil.TradeConfig()

	fmt.Println("TradingClient created with current SDK constructor.")
	fmt.Println("Wallet:", client.GetPayer())
	fmt.Println("SWQoS providers:", len(config.SwqosConfigs))
	fmt.Println("Sender core order from end:", config.SwqosCoresFromEnd)
}
