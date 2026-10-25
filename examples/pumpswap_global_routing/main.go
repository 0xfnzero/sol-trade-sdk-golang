// Offline all-default-pool buy/sell replay. No RPC or broadcast.
package main

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/examples/internal/pumpswapreplay"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/trading"
	"github.com/gagliardetto/solana-go"
)

func main() {
	config := trading.DefaultPumpSwapRoutingConfig()
	payer := solana.NewWallet().PublicKey()
	for _, buy := range []bool{true, false} {
		search, request, err := pumpswapreplay.LoadSnapshot("examples/fixtures/pumpswap_ttwo_route_accounts.json", payer, buy)
		if err != nil {
			panic(err)
		}
		if err = config.ExtendPools(search.ConversionPools); err != nil {
			panic(err)
		}
		result, err := config.Prepare(search.Pool, search.TokenMint, search.AssetMint, request)
		if err != nil {
			panic(err)
		}
		fmt.Println(result.Trade.MinimumOutput())
	}
}
