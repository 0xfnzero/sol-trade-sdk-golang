package main

import (
	"context"
	"fmt"
	"log"

	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
	"github.com/your-org/sol-trade-sdk-go/pkg/constants"
	"github.com/your-org/sol-trade-sdk-go/pkg/params"
	"github.com/gagliardetto/solana-go"
)

func main() {
	ctx := context.Background()

	// 1. Setup wallet and configuration
	privateKeyBase58 := "your_private_key_here"
	payer, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	// 2. Configure SWQOS services
	swqosConfigs := []soltradesdk.SwqosConfig{
		{Type: soltradesdk.SwqosTypeDefault, Region: soltradesdk.SwqosRegionFrankfurt},
		{Type: soltradesdk.SwqosTypeJito, Region: soltradesdk.SwqosRegionFrankfurt, APIKey: "your_jito_uuid"},
	}

	// 3. Create trade configuration
	rpcURL := "https://mainnet.helius-rpc.com/?api-key=your_api_key"
	tradeConfig := soltradesdk.NewTradeConfigBuilder(rpcURL).
		SwqosConfigs(swqosConfigs).
		// MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
		Build()

	// 4. Create trading client
	client, err := soltradesdk.NewTradingClient(ctx, &payer, tradeConfig)
	if err != nil {
		log.Fatalf("Failed to create trading client: %v", err)
	}

	fmt.Printf("Trading client created for wallet: %s\n", client.GetPayer())

	// 5. Example: Build PumpFun parameters
	mint := constants.MustPublicKeyFromBase58("your_token_mint_here")

	pumpFunParams := &params.PumpFunParams{
		BondingCurve: &params.BondingCurveAccount{
			Account:        constants.GetBondingCurvePDA(mint),
			VirtualTokenReserves: 1000000000,
			VirtualSolReserves:   30000000000,
			RealTokenReserves:    800000000,
			RealSolReserves:      24000000000,
			Creator:              constants.MustPublicKeyFromBase58("creator_address"),
			IsMayhemMode:         false,
			IsCashbackCoin:       false,
		},
		CreatorVault:   constants.GetCreatorVaultPDA(constants.MustPublicKeyFromBase58("creator_address")),
		TokenProgram:   constants.TOKEN_PROGRAM,
	}

	// 6. Get recent blockhash
	blockhash, err := client.GetRPC().GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		log.Fatalf("Failed to get blockhash: %v", err)
	}

	// 7. Build buy parameters
	buyParams := soltradesdk.TradeBuyParams{
		DexType:             soltradesdk.DexTypePumpFun,
		InputTokenType:      soltradesdk.TradeTokenTypeWSOL,
		Mint:                mint,
		InputTokenAmount:    10000000, // 0.01 SOL
		SlippageBasisPoints: 500,      // 5%
		RecentBlockhash:     &blockhash.Value.Blockhash,
		ExtensionParams:     pumpFunParams,
		WaitTxConfirmed:     true,
		CreateMintATA:       true,
		GasFeeStrategy:      soltradesdk.NewGasFeeStrategy(),
	}

	// 8. Execute buy
	result, err := client.Buy(ctx, buyParams)
	if err != nil {
		log.Fatalf("Buy failed: %v", err)
	}

	if result.Success {
		fmt.Printf("Buy successful! Signatures: %v\n", result.Signatures)
	} else {
		fmt.Printf("Buy failed: %v\n", result.Error)
	}

	// 9. Example: Sell tokens
	sellParams := soltradesdk.TradeSellParams{
		DexType:             soltradesdk.DexTypePumpFun,
		OutputTokenType:     soltradesdk.TradeTokenTypeWSOL,
		Mint:                mint,
		InputTokenAmount:    1000000, // Token amount to sell
		SlippageBasisPoints: 500,
		RecentBlockhash:     &blockhash.Value.Blockhash,
		ExtensionParams:     pumpFunParams,
		WithTip:             true,
		WaitTxConfirmed:     true,
		GasFeeStrategy:      soltradesdk.NewGasFeeStrategy(),
	}

	sellResult, err := client.Sell(ctx, sellParams)
	if err != nil {
		log.Fatalf("Sell failed: %v", err)
	}

	if sellResult.Success {
		fmt.Printf("Sell successful! Signatures: %v\n", sellResult.Signatures)
	} else {
		fmt.Printf("Sell failed: %v\n", sellResult.Error)
	}
}
