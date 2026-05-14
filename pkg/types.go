package soltradesdk

import (
	"context"
	"runtime"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// DexType represents the DEX protocol type
type DexType int

const (
	DexTypePumpFun DexType = iota
	DexTypePumpSwap
	DexTypeBonk
	DexTypeRaydiumCpmm
	DexTypeRaydiumAmmV4
	DexTypeMeteoraDammV2
)

// String returns the string representation of DexType
func (d DexType) String() string {
	return [...]string{"PumpFun", "PumpSwap", "Bonk", "RaydiumCpmm", "RaydiumAmmV4", "MeteoraDammV2"}[d]
}

// TradeTokenType represents the type of token to trade
type TradeTokenType int

const (
	TradeTokenTypeSOL TradeTokenType = iota
	TradeTokenTypeWSOL
	TradeTokenTypeUSD1
	TradeTokenTypeUSDC
)

// String returns the string representation of TradeTokenType
func (t TradeTokenType) String() string {
	return [...]string{"SOL", "WSOL", "USD1", "USDC"}[t]
}

// TradeType represents buy or sell operation
type TradeType int

const (
	TradeTypeBuy TradeType = iota
	TradeTypeSell
)

// SwqosRegion represents the region for SWQOS service
type SwqosRegion int

const (
	SwqosRegionNewYork SwqosRegion = iota
	SwqosRegionFrankfurt
	SwqosRegionAmsterdam
	SwqosRegionDublin
	SwqosRegionSLC
	SwqosRegionTokyo
	SwqosRegionSingapore
	SwqosRegionLondon
	SwqosRegionLosAngeles
	SwqosRegionDefault
)

// SwqosTransport represents provider transport modes.
type SwqosTransport int

const (
	SwqosTransportHTTP SwqosTransport = iota
	SwqosTransportGRPC
	SwqosTransportQUIC
)

// AstralaneTransport represents Astralane-specific transport modes.
type AstralaneTransport int

const (
	AstralaneTransportBinary AstralaneTransport = iota
	AstralaneTransportPlain
	AstralaneTransportQUIC
)

// SwqosType represents the type of SWQOS service
type SwqosType int

const (
	SwqosTypeJito SwqosType = iota
	SwqosTypeNextBlock
	SwqosTypeZeroSlot
	SwqosTypeTemporal
	SwqosTypeBloxroute
	SwqosTypeNode1
	SwqosTypeFlashBlock
	SwqosTypeBlockRazor
	SwqosTypeAstralane
	SwqosTypeStellium
	SwqosTypeLightspeed
	SwqosTypeSoyas
	SwqosTypeSpeedlanding
	SwqosTypeHelius
	SwqosTypeDefault
)

// SwqosConfig represents SWQOS service configuration
type SwqosConfig struct {
	Type          SwqosType
	Region        SwqosRegion
	APIKey        string
	CustomURL     string
	MEVProtection bool
	Transport     *SwqosTransport
	AstralaneMode *AstralaneTransport
	SwqosOnly     *bool
}

// SwqosClient is the shared SWQOS sender contract used by trading executors.
type SwqosClient interface {
	SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error)
	SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error)
	GetTipAccount() string
	GetSwqosType() SwqosType
	MinTipSol() float64
}

// GasFeeStrategyType represents the strategy row used by Rust GasFeeStrategy.
type GasFeeStrategyType int

const (
	GasFeeStrategyTypeNormal GasFeeStrategyType = iota
	GasFeeStrategyTypeLowTipHighCuPrice
	GasFeeStrategyTypeHighTipLowCuPrice
)

// GasFeeStrategy represents gas fee configuration
type GasFeeStrategy struct {
	BuyPriorityFee   uint64
	SellPriorityFee  uint64
	BuyComputeUnits  uint64
	SellComputeUnits uint64
	BuyTipLamports   uint64
	SellTipLamports  uint64
}

// NewGasFeeStrategy creates a new GasFeeStrategy with default values
func NewGasFeeStrategy() *GasFeeStrategy {
	return &GasFeeStrategy{
		BuyPriorityFee:   100000,
		SellPriorityFee:  100000,
		BuyComputeUnits:  200000,
		SellComputeUnits: 200000,
		BuyTipLamports:   100000,
		SellTipLamports:  100000,
	}
}

// SetGlobalFeeStrategy sets the global fee strategy
func (g *GasFeeStrategy) SetGlobalFeeStrategy(buyPriority, sellPriority, buyCU, sellCU, buyTip, sellTip uint64) {
	g.BuyPriorityFee = buyPriority
	g.SellPriorityFee = sellPriority
	g.BuyComputeUnits = buyCU
	g.SellComputeUnits = sellCU
	g.BuyTipLamports = buyTip
	g.SellTipLamports = sellTip
}

// TradeConfig represents trading configuration
type TradeConfig struct {
	RPCUrl                    string
	SwqosConfigs              []SwqosConfig
	LogEnabled                bool
	MEVProtection             bool
	CheckMinTip               bool
	UseSeedOptimize           bool
	CreateWsolAtaOnStartup    bool
	UsePumpFunV2              bool
	SwqosCoresFromEnd         bool
	MaxSwqosSubmitConcurrency int
}

// NewTradeConfig creates a new TradeConfig
func NewTradeConfig(rpcUrl string, swqosConfigs []SwqosConfig) *TradeConfig {
	return &TradeConfig{
		RPCUrl:            rpcUrl,
		SwqosConfigs:      swqosConfigs,
		LogEnabled:        true,
		UseSeedOptimize:   true,
		SwqosCoresFromEnd: true,
	}
}

// TradeConfigBuilder provides a fluent API for building TradeConfig.
// All optional fields are discoverable via IDE autocomplete.
//
// Example:
//
//	config := NewTradeConfigBuilder(rpcURL).
//	    SwqosConfigs(swqosConfigs).
//	    // MEVProtection(true). // Enable MEV protection (BlockRazor: sandwichMitigation, Astralane: port 9000)
//	    Build()
type TradeConfigBuilder struct {
	rpcUrl                    string
	swqosConfigs              []SwqosConfig
	logEnabled                bool
	mevProtection             bool
	checkMinTip               bool
	useSeedOptimize           bool
	createWsolAtaOnStartup    bool
	usePumpFunV2              bool
	swqosCoresFromEnd         bool
	maxSwqosSubmitConcurrency int
}

// NewTradeConfigBuilder creates a new TradeConfigBuilder
func NewTradeConfigBuilder(rpcUrl string) *TradeConfigBuilder {
	return &TradeConfigBuilder{
		rpcUrl:            rpcUrl,
		swqosConfigs:      []SwqosConfig{},
		logEnabled:        true,
		mevProtection:     false,
		useSeedOptimize:   true,
		swqosCoresFromEnd: true,
	}
}

// SwqosConfigs sets the SWQOS configurations
func (b *TradeConfigBuilder) SwqosConfigs(configs []SwqosConfig) *TradeConfigBuilder {
	b.swqosConfigs = configs
	return b
}

// LogEnabled sets whether logging is enabled
func (b *TradeConfigBuilder) LogEnabled(enabled bool) *TradeConfigBuilder {
	b.logEnabled = enabled
	return b
}

// MEVProtection enables MEV protection.
// When enabled:
//   - BlockRazor uses mode=sandwichMitigation
//   - Astralane uses port 9000 MEV-protected QUIC endpoint
func (b *TradeConfigBuilder) MEVProtection(enabled bool) *TradeConfigBuilder {
	b.mevProtection = enabled
	return b
}

func (b *TradeConfigBuilder) CheckMinTip(enabled bool) *TradeConfigBuilder {
	b.checkMinTip = enabled
	return b
}

func (b *TradeConfigBuilder) UseSeedOptimize(enabled bool) *TradeConfigBuilder {
	b.useSeedOptimize = enabled
	return b
}

func (b *TradeConfigBuilder) CreateWsolAtaOnStartup(enabled bool) *TradeConfigBuilder {
	b.createWsolAtaOnStartup = enabled
	return b
}

func (b *TradeConfigBuilder) UsePumpFunV2(enabled bool) *TradeConfigBuilder {
	b.usePumpFunV2 = enabled
	return b
}

func (b *TradeConfigBuilder) SwqosCoresFromEnd(enabled bool) *TradeConfigBuilder {
	b.swqosCoresFromEnd = enabled
	return b
}

func (b *TradeConfigBuilder) MaxSwqosSubmitConcurrency(limit int) *TradeConfigBuilder {
	b.maxSwqosSubmitConcurrency = limit
	return b
}

// Build creates the TradeConfig
func (b *TradeConfigBuilder) Build() *TradeConfig {
	return &TradeConfig{
		RPCUrl:                    b.rpcUrl,
		SwqosConfigs:              b.swqosConfigs,
		LogEnabled:                b.logEnabled,
		MEVProtection:             b.mevProtection,
		CheckMinTip:               b.checkMinTip,
		UseSeedOptimize:           b.useSeedOptimize,
		CreateWsolAtaOnStartup:    b.createWsolAtaOnStartup,
		UsePumpFunV2:              b.usePumpFunV2,
		SwqosCoresFromEnd:         b.swqosCoresFromEnd,
		MaxSwqosSubmitConcurrency: b.maxSwqosSubmitConcurrency,
	}
}

// RecommendedSenderThreadCoreIndices returns Rust-parity SWQOS sender core indices.
func RecommendedSenderThreadCoreIndices(swqosCount int, availableCores ...int) []int {
	cores := runtime.NumCPU()
	if len(availableCores) > 0 {
		cores = availableCores[0]
	}
	if swqosCount <= 0 || cores <= 0 {
		return nil
	}
	count := swqosCount
	if count > cores {
		count = cores
	}
	out := make([]int, 0, count)
	for i := cores - count; i < cores; i++ {
		out = append(out, i)
	}
	return out
}

// DurableNonceInfo represents durable nonce information
type DurableNonceInfo struct {
	NonceAccount    solana.PublicKey
	Authority       solana.PublicKey
	NonceHash       solana.Hash
	RecentBlockhash solana.Hash
}

// TradeBuyParams represents parameters for buy operation
type TradeBuyParams struct {
	DexType                   DexType
	InputTokenType            TradeTokenType
	Mint                      solana.PublicKey
	InputTokenAmount          uint64
	SlippageBasisPoints       uint64
	RecentBlockhash           *solana.Hash
	ExtensionParams           interface{}
	AddressLookupTableAccount *solana.PublicKey
	WaitTxConfirmed           bool
	CreateInputTokenATA       bool
	CloseInputTokenATA        bool
	CreateMintATA             bool
	DurableNonce              *DurableNonceInfo
	FixedOutputTokenAmount    *uint64
	GasFeeStrategy            *GasFeeStrategy
	Simulate                  bool
	UseExactSolAmount         *bool
	GrpcRecvUs                *int64
}

// TradeSellParams represents parameters for sell operation
type TradeSellParams struct {
	DexType                   DexType
	OutputTokenType           TradeTokenType
	Mint                      solana.PublicKey
	InputTokenAmount          uint64
	SlippageBasisPoints       uint64
	RecentBlockhash           *solana.Hash
	WithTip                   bool
	ExtensionParams           interface{}
	AddressLookupTableAccount *solana.PublicKey
	WaitTxConfirmed           bool
	CreateOutputTokenATA      bool
	CloseOutputTokenATA       bool
	CloseMintTokenATA         bool
	DurableNonce              *DurableNonceInfo
	FixedOutputTokenAmount    *uint64
	GasFeeStrategy            *GasFeeStrategy
	Simulate                  bool
	GrpcRecvUs                *int64
}

// TradeResult represents the result of a trade operation
type TradeResult struct {
	Success    bool
	Signatures []solana.Signature
	Error      error
	Timings    []SwqosTiming
}

// SwqosTiming represents timing information for a SWQOS submission
type SwqosTiming struct {
	SwqosType SwqosType
	Duration  int64 // microseconds
}

// TradingClient is the main client for Solana trading operations
type TradingClient struct {
	payer       *solana.PrivateKey
	rpcClient   *rpc.Client
	tradeConfig *TradeConfig
	logEnabled  bool
}

// NewTradingClient creates a new TradingClient
func NewTradingClient(ctx context.Context, payer *solana.PrivateKey, config *TradeConfig) (*TradingClient, error) {
	if payer == nil {
		return nil, ErrInvalidPrivateKey
	}
	if config == nil {
		return nil, NewTradeError(1001, "trade config is required", nil)
	}
	rpcClient := rpc.New(config.RPCUrl)

	return &TradingClient{
		payer:       payer,
		rpcClient:   rpcClient,
		tradeConfig: config,
		logEnabled:  config.LogEnabled,
	}, nil
}

// GetRPC returns the RPC client
func (c *TradingClient) GetRPC() *rpc.Client {
	return c.rpcClient
}

// GetPayer returns the payer public key
func (c *TradingClient) GetPayer() solana.PublicKey {
	if c == nil || c.payer == nil {
		return solana.PublicKey{}
	}
	return c.payer.PublicKey()
}

// Buy executes a buy order
func (c *TradingClient) Buy(ctx context.Context, params TradeBuyParams) (*TradeResult, error) {
	return c.executeTrade(ctx, TradeTypeBuy, params)
}

// Sell executes a sell order
func (c *TradingClient) Sell(ctx context.Context, params TradeSellParams) (*TradeResult, error) {
	return c.executeSell(ctx, params)
}

// SellByPercent executes a sell order for a percentage of tokens
func (c *TradingClient) SellByPercent(ctx context.Context, params TradeSellParams, totalAmount, percent uint64) (*TradeResult, error) {
	if percent == 0 || percent > 100 {
		return nil, ErrInvalidPercentage
	}
	params.InputTokenAmount = totalAmount * percent / 100
	return c.Sell(ctx, params)
}

// executeTrade is the internal implementation for trading
func (c *TradingClient) executeTrade(ctx context.Context, tradeType TradeType, params TradeBuyParams) (*TradeResult, error) {
	return nil, NewTradeError(
		2001,
		"root TradingClient does not build or submit trades; use the pkg/trading executors or pkg/trading/core executor",
		ErrTradingExecutionUnavailable,
	)
}

// executeSell is the internal implementation for sell
func (c *TradingClient) executeSell(ctx context.Context, params TradeSellParams) (*TradeResult, error) {
	return nil, NewTradeError(
		2001,
		"root TradingClient does not build or submit trades; use the pkg/trading executors or pkg/trading/core executor",
		ErrTradingExecutionUnavailable,
	)
}
