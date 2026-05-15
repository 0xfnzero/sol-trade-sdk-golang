package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

func main() {
	config := exampleutil.TradeConfig()
	buyParams := exampleutil.ExampleBuyParams(soltradesdk.DexTypePumpSwap)
	sellParams := exampleutil.ExampleSellParams(soltradesdk.DexTypePumpSwap)

	fmt.Println(exampleutil.DescribeDryRun("Seed-optimized PumpSwap example"))
	fmt.Println("Seed optimization:", config.UseSeedOptimize)
	fmt.Println("Prepared params:", buyParams.DexType, sellParams.DexType)
}
