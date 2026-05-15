package main

import (
	"context"
	"fmt"
	"log"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
)

func main() {
	ctx := context.Background()
	client, err := exampleutil.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	buyParams := exampleutil.ExampleBuyParams(soltradesdk.DexTypePumpFun)

	fmt.Println(exampleutil.DescribeDryRun("Complete PumpFun buy flow"))
	fmt.Println("Wallet:", client.GetPayer())
	fmt.Println("PumpFun v2 enabled:", exampleutil.TradeConfig().UsePumpFunV2)
	if pp, ok := buyParams.ExtensionParams.(*params.PumpFunParams); ok {
		fmt.Println("Cashback flag:", pp.BondingCurve.IsCashbackCoin)
	}
}
