package main

import (
	"context"
	"errors"
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
	template := exampleutil.ExampleBuyParams(soltradesdk.DexTypePumpFun)

	simple := soltradesdk.NewSimpleBuyParams(
		soltradesdk.DexTypePumpFun,
		soltradesdk.TradeTokenTypeWSOL,
		template.Mint,
		soltradesdk.BuyWithMaxInput(template.InputTokenAmount),
		template.ExtensionParams,
		*template.RecentBlockhash,
		template.GasFeeStrategy,
	).
		SetSlippageBasisPoints(template.SlippageBasisPoints).
		SetAccountPolicy(soltradesdk.AccountPolicyAuto).
		SetWaitTxConfirmed(false).
		SetWaitForAllSubmits(false)
	lowLevel := simple.ToTradeBuyParams()

	fmt.Println(exampleutil.DescribeDryRun("Simple buy intent API"))
	fmt.Println("Wallet:", client.GetPayer())
	fmt.Println("payWith:", simple.PayWith)
	fmt.Println("inputTokenAmount:", lowLevel.InputTokenAmount)
	fmt.Println("createInputTokenATA:", lowLevel.CreateInputTokenATA)
	fmt.Println("createMintATA:", lowLevel.CreateMintATA)

	if exampleutil.RunLive() {
		_, err := client.BuySimple(ctx, simple)
		if !errors.Is(err, soltradesdk.ErrTradingExecutionUnavailable) {
			log.Fatal(err)
		}
		fmt.Println("root TradingClient converted params, but execution is intentionally unavailable; use pkg/trading executors for live submission.")
	}
}
