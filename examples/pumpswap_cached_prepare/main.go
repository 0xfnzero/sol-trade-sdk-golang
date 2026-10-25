// go run ./examples/pumpswap_cached_prepare [snapshot.json] [buy|sell]
// Historical cache replay: quotes and signs locally; never sends a transaction.
package main

import (
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/examples/internal/pumpswapreplay"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
)

func loadSnapshot(path string, payer solana.PublicKey, buy bool) (trading.PumpSwapRouteSearch, trading.CachedTradeRequest, error) {
	return pumpswapreplay.LoadSnapshot(path, payer, buy)
}

func main() {
	path := "examples/fixtures/pumpswap_ttwo_route_accounts.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	buy := len(os.Args) < 3 || os.Args[2] == "buy"
	if len(os.Args) > 2 && os.Args[2] != "buy" && os.Args[2] != "sell" {
		panic("direction must be buy or sell")
	}
	wallet := solana.NewWallet()
	search, request, err := loadSnapshot(path, wallet.PublicKey(), buy)
	if err != nil {
		panic(err)
	}
	selected, err := trading.PrepareBestPumpSwapTrade(search, request)
	if err != nil {
		panic(err)
	}
	wire, err := selected.Trade.BuildTransaction(request.Context, []solana.PrivateKey{wallet.PrivateKey})
	if err != nil {
		panic(err)
	}
	for _, leg := range selected.Trade.Steps() {
		fmt.Printf("pool=%s input=%s output=%s amount=%d protected=%d\n", leg.Pool, leg.InputMint, leg.OutputMint, leg.AmountIn, leg.MinimumNetAmountOut)
	}
	fmt.Printf("historical replay: routes=%d protected=%d signed_v1_bytes=%d (not submitted)\n", selected.RoutesEvaluated, selected.Trade.MinimumOutput(), len(wire))
}
