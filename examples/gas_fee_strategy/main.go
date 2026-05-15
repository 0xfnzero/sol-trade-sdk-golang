package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
)

func main() {
	strategy := exampleutil.LowLatencyGasStrategy()
	fmt.Println("Gas fee strategy configured:")
	fmt.Println("  Buy compute units:", strategy.BuyComputeUnits)
	fmt.Println("  Sell compute units:", strategy.SellComputeUnits)
	fmt.Println("  Buy priority fee:", strategy.BuyPriorityFee)
	fmt.Println("  Sell priority fee:", strategy.SellPriorityFee)
}
