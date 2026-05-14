// Gas Fee Strategy Example
//
// This example demonstrates how to configure gas fee strategy
// for optimal transaction landing.

package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
)

func main() {
	// Create gas fee strategy
	gasFeeStrategy := common.NewGasFeeStrategy()

	// Set global fee strategy
	// Parameters:
	// - computeUnitPrice: base compute unit price (micro-lamports)
	// - computeUnitLimit: maximum compute units
	// - priorityFee: priority fee in lamports
	// - rentExemptBalance: rent-exempt balance for accounts
	// - slippageBps: slippage in basis points
	// - tipBps: tip percentage in basis points
	gasFeeStrategy.SetGlobalFeeStrategy(150000, 150000, 500000, 500000, 0.001, 0.001)

	fmt.Println("Gas fee strategy configured:")
	fmt.Printf("  Compute unit price: %d\n", gasFeeStrategy.GetComputeUnitPrice())
	fmt.Printf("  Compute unit limit: %d\n", gasFeeStrategy.GetComputeUnitLimit())
	fmt.Printf("  Priority fee: %d\n", gasFeeStrategy.GetPriorityFee())

	// You can also set individual parameters
	gasFeeStrategy.SetComputeUnitPrice(200000)
	gasFeeStrategy.SetPriorityFee(600000)

	fmt.Println("\nUpdated gas fee strategy:")
	fmt.Printf("  Compute unit price: %d\n", gasFeeStrategy.GetComputeUnitPrice())
	fmt.Printf("  Priority fee: %d\n", gasFeeStrategy.GetPriorityFee())
}
