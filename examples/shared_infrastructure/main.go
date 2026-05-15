package main

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/exampleutil"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
)

func main() {
	sharedConfig := exampleutil.TradeConfig()
	wallets := []*solana.Wallet{solana.NewWallet(), solana.NewWallet(), solana.NewWallet()}

	fmt.Println("Shared configuration prepared for multiple wallets.")
	for i, wallet := range wallets {
		fmt.Printf("Client %d: %s\n", i+1, wallet.PublicKey())
	}
	fmt.Println("Reuse one TradeConfig/gas strategy, while each client keeps its own signer.")
	_ = soltradesdk.RecommendedSenderThreadCoreIndices(len(sharedConfig.SwqosConfigs), 8)
}
