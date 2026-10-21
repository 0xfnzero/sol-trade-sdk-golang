package main

import (
	"context"
	"fmt"
	"log"

	"github.com/0xfnzero/sol-trade-sdk-golang/v9/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v9/pkg/params"
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
	if pp, ok := buyParams.ExtensionParams.(*params.PumpFunParams); ok {
		fmt.Println("PumpFun quote mint:", pp.QuoteMint)
		fmt.Println("Cashback flag:", pp.BondingCurve.IsCashbackCoin)
	}
}
