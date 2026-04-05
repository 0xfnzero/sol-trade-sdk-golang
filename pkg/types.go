package soltradesdk

import (
	"context"

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
	SwqosRegionFrankfurt SwqosRegion = iota
	SwqosRegionNewYork
	SwqosRegionAmsterdam
	SwqosRegionTokyo
	SwqosRegionSingapore
)

// SwqosType represents the type of SWQOS service
type SwqosType int

const (
	SwqosTypeDefault SwqosType = iota
	SwqosTypeJito
	SwqosTypeBloxroute
	SwqosTypeZeroSlot
	SwqosTypeTemporal
	SwqosTypeFlashBlock
	SwqosTypeBlockRazor
	SwqosTypeNode1
	SwqosTypeAstralane
	SwqosTypeNextBlock
	SwqosTypeHelius
)

// SwqosConfig represents SWQOS service configuration
type SwqosConfig struct {
	Type      SwqosType
	Region    SwqosRegion
	APIKey    string
	CustomURL string
}

// GasFeeStrategy represents gas fee configuration
type GasFeeStrategy struct {
	BuyPriorityFee    uint64
	SellPriorityFee   uint64
	BuyComputeUnits   uint64
	SellComputeUnits  uint64
	BuyTipLamports    uint64
	SellTipLamports   uint64
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
	RPCUrl       string
	SwqosConfigs []SwqosConfig
	LogEnabled   bool
}

// NewTradeConfig creates a new TradeConfig
func NewTradeConfig(rpcUrl string, swqosConfigs []SwqosConfig) *TradeConfig {
	return &TradeConfig{
		RPCUrl:       rpcUrl,
		SwqosConfigs: swqosConfigs,
		LogEnabled:   true,
	}
}

// DurableNonceInfo represents durable nonce information
type DurableNonceInfo struct {
	NonceAccount      solana.PublicKey
	Authority         solana.PublicKey
	NonceHash         solana.Hash
	RecentBlockhash   solana.Hash
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
	AddressLookupTableAccount *solana.AddressLookupTableAccount
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
	AddressLookupTableAccount *solana.AddressLookupTableAccount
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
	payer         *solana.PrivateKey
	rpcClient     *rpc.Client
	tradeConfig   *TradeConfig
	middleware    MiddlewareManager
	logEnabled    bool
}

// NewTradingClient creates a new TradingClient
func NewTradingClient(ctx context.Context, payer *solana.PrivateKey, config *TradeConfig) (*TradingClient, error) {
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
	// Implementation will be added in executor module
	return nil, nil
}

// executeSell is the internal implementation for sell
func (c *TradingClient) executeSell(ctx context.Context, params TradeSellParams) (*TradeResult, error) {
	// Implementation will be added in executor module
	return nil, nil
}
