package exampleutil

import (
	"context"
	"os"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	dexparams "github.com/0xfnzero/sol-trade-sdk-golang/pkg/params"
	"github.com/gagliardetto/solana-go"
)

func RunLive() bool {
	return os.Getenv("RUN_LIVE_EXAMPLES") == "1"
}

func RPCURL() string {
	if v := os.Getenv("RPC_URL"); v != "" {
		return v
	}
	return "https://api.mainnet-beta.solana.com"
}

func ExamplePublicKey(seed byte) solana.PublicKey {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return solana.PublicKeyFromBytes(b)
}

func ExampleHash(seed byte) solana.Hash {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return solana.HashFromBytes(b)
}

func transportPtr(v soltradesdk.SwqosTransport) *soltradesdk.SwqosTransport             { return &v }
func astralaneModePtr(v soltradesdk.AstralaneTransport) *soltradesdk.AstralaneTransport { return &v }
func boolPtr(v bool) *bool                                                              { return &v }

func DefaultSwqosConfigs() []soltradesdk.SwqosConfig {
	configs := []soltradesdk.SwqosConfig{{Type: soltradesdk.SwqosTypeDefault, Region: soltradesdk.SwqosRegionDefault}}
	if v := os.Getenv("JITO_UUID"); v != "" {
		configs = append(configs, soltradesdk.SwqosConfig{Type: soltradesdk.SwqosTypeJito, Region: soltradesdk.SwqosRegionFrankfurt, APIKey: v})
	}
	if v := os.Getenv("BLOXROUTE_AUTH_TOKEN"); v != "" {
		configs = append(configs, soltradesdk.SwqosConfig{Type: soltradesdk.SwqosTypeBloxroute, Region: soltradesdk.SwqosRegionFrankfurt, APIKey: v})
	}
	if v := os.Getenv("ASTRALANE_API_KEY"); v != "" {
		configs = append(configs, soltradesdk.SwqosConfig{
			Type:          soltradesdk.SwqosTypeAstralane,
			Region:        soltradesdk.SwqosRegionFrankfurt,
			APIKey:        v,
			MEVProtection: true,
			Transport:     transportPtr(soltradesdk.SwqosTransportQUIC),
			AstralaneMode: astralaneModePtr(soltradesdk.AstralaneTransportQUIC),
		})
	}
	if v := os.Getenv("HELIUS_API_KEY"); v != "" {
		configs = append(configs, soltradesdk.SwqosConfig{Type: soltradesdk.SwqosTypeHelius, Region: soltradesdk.SwqosRegionDefault, APIKey: v, SwqosOnly: boolPtr(true)})
	}
	return configs
}

func TradeConfig() *soltradesdk.TradeConfig {
	return soltradesdk.NewTradeConfigBuilder(RPCURL()).
		SwqosConfigs(DefaultSwqosConfigs()).
		UseSeedOptimize(true).
		SwqosCoresFromEnd(false).
		MaxSwqosSubmitConcurrency(8).
		Build()
}

func NewClient(ctx context.Context) (*soltradesdk.TradingClient, error) {
	wallet := solana.NewWallet()
	return soltradesdk.NewTradingClient(ctx, &wallet.PrivateKey, TradeConfig())
}

func LowLatencyGasStrategy() *soltradesdk.GasFeeStrategy {
	strategy := soltradesdk.NewGasFeeStrategy()
	strategy.SetGlobalFeeStrategy(500_000, 500_000, 180_000, 160_000, 1_000_000, 1_000_000)
	return strategy
}

func PumpFunParams() *dexparams.PumpFunParams {
	return &dexparams.PumpFunParams{
		BondingCurve: &dexparams.BondingCurveAccount{
			Account:              ExamplePublicKey(11),
			VirtualTokenReserves: 1_000_000_000,
			VirtualSolReserves:   30_000_000_000,
			RealTokenReserves:    800_000_000,
			RealSolReserves:      24_000_000_000,
			TokenTotalSupply:     1_000_000_000,
			Creator:              ExamplePublicKey(12),
			IsCashbackCoin:       true,
		},
		AssociatedBondingCurve: ExamplePublicKey(13),
		CreatorVault:           ExamplePublicKey(14),
		TokenProgram:           constants.TOKEN_PROGRAM,
		FeeRecipient:           ExamplePublicKey(15),
		QuoteMint:              constants.WSOL_TOKEN_ACCOUNT,
	}
}

func PumpSwapParams() *dexparams.PumpSwapParams {
	return dexparams.NewPumpSwapParams(
		ExamplePublicKey(21), ExamplePublicKey(22), constants.WSOL_TOKEN_ACCOUNT,
		ExamplePublicKey(23), ExamplePublicKey(24),
		2_000_000_000, 50_000_000_000,
		ExamplePublicKey(25), ExamplePublicKey(26),
		constants.TOKEN_PROGRAM, constants.TOKEN_PROGRAM,
		false, true,
	)
}

func BonkParams() *instruction.BonkParams {
	return &instruction.BonkParams{
		VirtualBase:               2_000_000_000,
		VirtualQuote:              50_000_000_000,
		RealBase:                  1_700_000_000,
		RealQuote:                 40_000_000_000,
		PoolState:                 ExamplePublicKey(31),
		BaseVault:                 ExamplePublicKey(32),
		QuoteVault:                ExamplePublicKey(33),
		MintTokenProgram:          constants.TOKEN_PROGRAM,
		PlatformConfig:            ExamplePublicKey(34),
		PlatformAssociatedAccount: ExamplePublicKey(35),
		CreatorAssociatedAccount:  ExamplePublicKey(36),
		GlobalConfig:              ExamplePublicKey(37),
	}
}

func RaydiumCpmmParams() *dexparams.RaydiumCpmmParams {
	return dexparams.NewRaydiumCpmmParams(
		ExamplePublicKey(41), ExamplePublicKey(42), ExamplePublicKey(43), constants.WSOL_TOKEN_ACCOUNT,
		ExamplePublicKey(44), ExamplePublicKey(45),
		2_000_000_000, 50_000_000_000,
		constants.TOKEN_PROGRAM, constants.TOKEN_PROGRAM, ExamplePublicKey(46),
	)
}

func RaydiumAmmV4Params() *dexparams.RaydiumAmmV4Params {
	return dexparams.NewRaydiumAmmV4Params(
		ExamplePublicKey(51), ExamplePublicKey(52), constants.WSOL_TOKEN_ACCOUNT,
		ExamplePublicKey(53), ExamplePublicKey(54),
		ExamplePublicKey(55), ExamplePublicKey(56), ExamplePublicKey(57), ExamplePublicKey(58),
		ExamplePublicKey(59), ExamplePublicKey(60), ExamplePublicKey(61),
		ExamplePublicKey(62), ExamplePublicKey(63), ExamplePublicKey(64),
		2_000_000_000, 50_000_000_000,
	)
}

func MeteoraDammV2Params() *dexparams.MeteoraDammV2Params {
	return dexparams.NewMeteoraDammV2Params(
		ExamplePublicKey(71), ExamplePublicKey(72), ExamplePublicKey(73), ExamplePublicKey(74), constants.WSOL_TOKEN_ACCOUNT,
		constants.TOKEN_PROGRAM, constants.TOKEN_PROGRAM,
	)
}

func ProtocolParams(dexType soltradesdk.DexType) interface{} {
	switch dexType {
	case soltradesdk.DexTypePumpFun:
		return PumpFunParams()
	case soltradesdk.DexTypePumpSwap:
		return PumpSwapParams()
	case soltradesdk.DexTypeBonk:
		return BonkParams()
	case soltradesdk.DexTypeRaydiumCpmm:
		return RaydiumCpmmParams()
	case soltradesdk.DexTypeRaydiumAmmV4:
		return RaydiumAmmV4Params()
	case soltradesdk.DexTypeMeteoraDammV2:
		return MeteoraDammV2Params()
	default:
		return nil
	}
}

func DefaultTradeMint(dexType soltradesdk.DexType) solana.PublicKey {
	switch dexType {
	case soltradesdk.DexTypePumpSwap:
		return PumpSwapParams().BaseMint
	case soltradesdk.DexTypeRaydiumCpmm:
		return RaydiumCpmmParams().BaseMint
	case soltradesdk.DexTypeRaydiumAmmV4:
		return RaydiumAmmV4Params().CoinMint
	case soltradesdk.DexTypeMeteoraDammV2:
		return MeteoraDammV2Params().TokenAMint
	default:
		return ExamplePublicKey(91)
	}
}

func ExampleBuyParams(dexType soltradesdk.DexType) soltradesdk.TradeBuyParams {
	blockhash := ExampleHash(99)
	grpcRecvUs := int64(0)
	inputTokenType := soltradesdk.TradeTokenTypeWSOL
	if dexType == soltradesdk.DexTypeBonk {
		inputTokenType = soltradesdk.TradeTokenTypeUSD1
	}
	params := soltradesdk.TradeBuyParams{
		DexType:             dexType,
		InputTokenType:      inputTokenType,
		Mint:                DefaultTradeMint(dexType),
		InputTokenAmount:    100_000,
		SlippageBasisPoints: 300,
		RecentBlockhash:     &blockhash,
		ExtensionParams:     ProtocolParams(dexType),
		WaitTxConfirmed:     true,
		CreateInputTokenATA: true,
		CloseInputTokenATA:  true,
		CreateMintATA:       true,
		GasFeeStrategy:      LowLatencyGasStrategy(),
		GrpcRecvUs:          &grpcRecvUs,
	}
	if dexType == soltradesdk.DexTypeMeteoraDammV2 {
		fixedOutput := uint64(90_000)
		params.FixedOutputTokenAmount = &fixedOutput
	}
	return params
}

func ExampleSellParams(dexType soltradesdk.DexType) soltradesdk.TradeSellParams {
	blockhash := ExampleHash(99)
	grpcRecvUs := int64(0)
	outputTokenType := soltradesdk.TradeTokenTypeWSOL
	if dexType == soltradesdk.DexTypeBonk {
		outputTokenType = soltradesdk.TradeTokenTypeUSD1
	}
	params := soltradesdk.TradeSellParams{
		DexType:              dexType,
		OutputTokenType:      outputTokenType,
		Mint:                 DefaultTradeMint(dexType),
		InputTokenAmount:     50_000,
		SlippageBasisPoints:  300,
		RecentBlockhash:      &blockhash,
		WithTip:              true,
		ExtensionParams:      ProtocolParams(dexType),
		WaitTxConfirmed:      true,
		CreateOutputTokenATA: true,
		CloseOutputTokenATA:  true,
		CloseMintTokenATA:    false,
		GasFeeStrategy:       LowLatencyGasStrategy(),
		GrpcRecvUs:           &grpcRecvUs,
	}
	if dexType == soltradesdk.DexTypeMeteoraDammV2 {
		fixedOutput := uint64(45_000)
		params.FixedOutputTokenAmount = &fixedOutput
	}
	return params
}

func DescribeDryRun(name string) string {
	return name + " prepared with current SDK types. Set RUN_LIVE_EXAMPLES=1 only after replacing placeholders with real RPC or decoded event data."
}
