package main

import (
	"context"
	"fmt"
	"os"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func main() {
	ctx := context.Background()
	buyParams := exampleutil.ExampleBuyParams(soltradesdk.DexTypePumpFun)
	fmt.Println(exampleutil.DescribeDryRun("Durable nonce example for multi-SWQoS submission"))

	if nonceAccountText := os.Getenv("NONCE_ACCOUNT"); nonceAccountText != "" {
		nonceAccount := solana.MustPublicKeyFromBase58(nonceAccountText)
		nonceInfo, err := common.FetchNonceInfo(ctx, rpc.New(exampleutil.RPCURL()), nonceAccount)
		if err != nil {
			fmt.Println("Failed to fetch nonce:", err)
			return
		}
		buyParams.RecentBlockhash = nil
		buyParams.DurableNonce = nonceInfo
		fmt.Println("Fetched durable nonce:", nonceInfo.NonceHash)
	}
	fmt.Println("Durable nonce attached:", buyParams.DurableNonce != nil)
}
