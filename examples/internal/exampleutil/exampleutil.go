package exampleutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	if RunLive() {
		payer, err := LoadPayerFromEnv("PRIVATE_KEY")
		if err != nil {
			return nil, err
		}
		return soltradesdk.NewTradingClient(ctx, &payer, TradeConfig())
	}
	wallet := solana.NewWallet()
	return soltradesdk.NewTradingClient(ctx, &wallet.PrivateKey, TradeConfig())
}

func LoadPayerFromEnv(name string) (solana.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv(name))
	if encoded == "" {
		return nil, fmt.Errorf("%s is required for live trading", name)
	}
	payer, err := solana.PrivateKeyFromBase58(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", name, err)
	}
	if len(payer) != 64 {
		return nil, fmt.Errorf("invalid %s: decoded private key has %d bytes, expected 64", name, len(payer))
	}
	return payer, nil
}

func IsEventFresh(receivedAt time.Time, maxAge time.Duration, now time.Time) bool {
	return maxAge > 0 && !receivedAt.After(now) && now.Sub(receivedAt) <= maxAge
}

func MatchesTarget(actual solana.PublicKey, expected *solana.PublicKey) bool {
	return expected == nil || actual.Equals(*expected)
}

func CheckedPositionDelta(before, after uint64) (uint64, error) {
	if after <= before {
		return 0, fmt.Errorf("buy produced no positive token balance delta: before=%d after=%d", before, after)
	}
	return after - before, nil
}

func ValidateTradeIntent(inputAmount, slippage uint64, fixedOutput *uint64) error {
	if inputAmount == 0 {
		return errors.New("input amount must be positive")
	}
	if slippage >= 10_000 {
		return errors.New("slippage must be less than 10000 basis points")
	}
	if fixedOutput != nil && *fixedOutput == 0 {
		return errors.New("fixed output amount must be positive when provided")
	}
	return nil
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
	return params
}

func DescribeDryRun(name string) string {
	return name + " prepared with current SDK types. Placeholder accounts are not submitted; see low_latency_bot for the guarded workflow."
}
