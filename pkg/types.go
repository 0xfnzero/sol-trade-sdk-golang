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
	SwqosTypeSolami
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

// IsSwqosTypeBlacklisted matches Rust v4.0.21 SWQOS_BLACKLIST.
func IsSwqosTypeBlacklisted(swqosType SwqosType) bool {
	return swqosType == SwqosTypeNextBlock
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
	SwqosCoresFromEnd         bool
	MaxSwqosSubmitConcurrency int
}

// NewTradeConfig creates a new TradeConfig
func NewTradeConfig(rpcUrl string, swqosConfigs []SwqosConfig) *TradeConfig {
	return &TradeConfig{
		RPCUrl:                 rpcUrl,
		SwqosConfigs:           NormalizeSwqosConfigs(rpcUrl, swqosConfigs),
		LogEnabled:             true,
		UseSeedOptimize:        true,
		CreateWsolAtaOnStartup: true,
		SwqosCoresFromEnd:      false,
	}
}

// NormalizeSwqosConfigs appends the default RPC route and filters Rust-blacklisted providers.
func NormalizeSwqosConfigs(rpcUrl string, configs []SwqosConfig) []SwqosConfig {
	out := make([]SwqosConfig, 0, len(configs)+1)
	hasDefault := false
	for _, cfg := range configs {
		if cfg.Type == SwqosTypeDefault {
			hasDefault = true
		}
		if IsSwqosTypeBlacklisted(cfg.Type) {
			continue
		}
		out = append(out, cfg)
	}
	if !hasDefault {
		out = append(out, SwqosConfig{Type: SwqosTypeDefault, Region: SwqosRegionDefault, CustomURL: rpcUrl})
	}
	return out
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
	swqosCoresFromEnd         bool
	maxSwqosSubmitConcurrency int
}

// NewTradeConfigBuilder creates a new TradeConfigBuilder
func NewTradeConfigBuilder(rpcUrl string) *TradeConfigBuilder {
	return &TradeConfigBuilder{
		rpcUrl:                 rpcUrl,
		swqosConfigs:           []SwqosConfig{},
		logEnabled:             true,
		mevProtection:          false,
		useSeedOptimize:        true,
		createWsolAtaOnStartup: true,
		swqosCoresFromEnd:      false,
	}
}

// SwqosConfigs sets the SWQOS configurations
func (b *TradeConfigBuilder) SwqosConfigs(configs []SwqosConfig) *TradeConfigBuilder {
	b.swqosConfigs = NormalizeSwqosConfigs(b.rpcUrl, configs)
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
		SwqosConfigs:              NormalizeSwqosConfigs(b.rpcUrl, b.swqosConfigs),
		LogEnabled:                b.logEnabled,
		MEVProtection:             b.mevProtection,
		CheckMinTip:               b.checkMinTip,
		UseSeedOptimize:           b.useSeedOptimize,
		CreateWsolAtaOnStartup:    b.createWsolAtaOnStartup,
		SwqosCoresFromEnd:         b.swqosCoresFromEnd,
		MaxSwqosSubmitConcurrency: b.maxSwqosSubmitConcurrency,
	}
}

// RecommendedSenderThreadCoreIndices returns Rust-parity SWQOS sender core indices.
// By default it returns the capped last-N core range, matching Rust's helper for
// TradeConfig.swqos_cores_from_end(true). Pass 0 as the optional second variadic
// argument to select the first capped core range.
func RecommendedSenderThreadCoreIndices(swqosCount int, args ...int) []int {
	cores := runtime.NumCPU()
	if len(args) > 0 {
		cores = args[0]
	}
	if swqosCount <= 0 || cores <= 0 {
		return nil
	}
	fromEnd := len(args) <= 1 || args[1] != 0
	count := swqosCount
	coreCap := cores * 2 / 3
	if coreCap < 1 {
		coreCap = 1
	}
	if count > coreCap {
		count = coreCap
	}
	if count > cores {
		count = cores
	}
	out := make([]int, 0, count)
	start := 0
	if fromEnd {
		start = cores - count
	}
	for i := start; i < start+count; i++ {
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

// AccountPolicy describes how high-level simple trade requests manage token accounts.
type AccountPolicy int

const (
	AccountPolicyAuto AccountPolicy = iota
	AccountPolicyHotPathMinimal
	AccountPolicyCreateMissing
	AccountPolicyAssumePrepared
)

// BuyAmountKind identifies the high-level buy sizing intent.
type BuyAmountKind int

const (
	BuyAmountExactInput BuyAmountKind = iota
	BuyAmountExactOutput
	BuyAmountWithMaxInput
)

// BuyAmount describes buy sizing intent before conversion to legacy params.
type BuyAmount struct {
	Kind           BuyAmountKind
	Amount         uint64
	OutputAmount   uint64
	MaxInputAmount uint64
}

// BuyExactInput spends exactly amount input tokens.
func BuyExactInput(amount uint64) BuyAmount {
	return BuyAmount{Kind: BuyAmountExactInput, Amount: amount}
}

// BuyExactOutput requests outputAmount tokens and caps input at maxInputAmount.
func BuyExactOutput(outputAmount, maxInputAmount uint64) BuyAmount {
	return BuyAmount{
		Kind:           BuyAmountExactOutput,
		Amount:         maxInputAmount,
		OutputAmount:   outputAmount,
		MaxInputAmount: maxInputAmount,
	}
}

// BuyWithMaxInput spends up to quoteAmount while allowing protocol-side output calculation.
func BuyWithMaxInput(quoteAmount uint64) BuyAmount {
	return BuyAmount{Kind: BuyAmountWithMaxInput, Amount: quoteAmount}
}

// SellAmountKind identifies the high-level sell sizing intent.
type SellAmountKind int

const (
	SellAmountExactInput SellAmountKind = iota
	SellAmountExactOutput
)

// SellAmount describes sell sizing intent before conversion to legacy params.
type SellAmount struct {
	Kind           SellAmountKind
	Amount         uint64
	OutputAmount   uint64
	MaxInputAmount uint64
}

// SellExactInput sells exactly amount base tokens.
func SellExactInput(amount uint64) SellAmount {
	return SellAmount{Kind: SellAmountExactInput, Amount: amount}
}

// SellExactOutput requests outputAmount quote tokens and caps input at maxInputAmount.
func SellExactOutput(outputAmount, maxInputAmount uint64) SellAmount {
	return SellAmount{
		Kind:           SellAmountExactOutput,
		Amount:         maxInputAmount,
		OutputAmount:   outputAmount,
		MaxInputAmount: maxInputAmount,
	}
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
	WaitForAllSubmits         bool
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
	WaitForAllSubmits         bool
	CreateOutputTokenATA      bool
	CloseOutputTokenATA       bool
	CloseMintTokenATA         bool
	DurableNonce              *DurableNonceInfo
	FixedOutputTokenAmount    *uint64
	GasFeeStrategy            *GasFeeStrategy
	Simulate                  bool
	GrpcRecvUs                *int64
}

// SimpleBuyParams is the high-level buy API that describes intent instead of low-level ATA flags.
type SimpleBuyParams struct {
	DexType                   DexType
	PayWith                   TradeTokenType
	Mint                      solana.PublicKey
	Amount                    BuyAmount
	ExtensionParams           interface{}
	RecentBlockhash           *solana.Hash
	GasFeeStrategy            *GasFeeStrategy
	SlippageBasisPoints       uint64
	AccountPolicy             AccountPolicy
	AddressLookupTableAccount *solana.PublicKey
	WaitTxConfirmed           bool
	WaitForAllSubmits         bool
	DurableNonce              *DurableNonceInfo
	Simulate                  bool
	GrpcRecvUs                *int64
}

// SimpleSellParams is the high-level sell API that describes intent instead of low-level ATA flags.
type SimpleSellParams struct {
	DexType                   DexType
	ReceiveAs                 TradeTokenType
	Mint                      solana.PublicKey
	Amount                    SellAmount
	ExtensionParams           interface{}
	RecentBlockhash           *solana.Hash
	GasFeeStrategy            *GasFeeStrategy
	SlippageBasisPoints       uint64
	AccountPolicy             AccountPolicy
	AddressLookupTableAccount *solana.PublicKey
	WaitTxConfirmed           bool
	WaitForAllSubmits         bool
	DurableNonce              *DurableNonceInfo
	Simulate                  bool
	WithTip                   *bool
	GrpcRecvUs                *int64
}

// NewSimpleBuyParams creates a simple buy request using a recent blockhash.
func NewSimpleBuyParams(
	dexType DexType,
	payWith TradeTokenType,
	mint solana.PublicKey,
	amount BuyAmount,
	extensionParams interface{},
	recentBlockhash solana.Hash,
	gasFeeStrategy *GasFeeStrategy,
) SimpleBuyParams {
	return SimpleBuyParams{
		DexType:           dexType,
		PayWith:           payWith,
		Mint:              mint,
		Amount:            amount,
		ExtensionParams:   extensionParams,
		RecentBlockhash:   &recentBlockhash,
		GasFeeStrategy:    gasFeeStrategy,
		AccountPolicy:     AccountPolicyAuto,
		WaitTxConfirmed:   false,
		WaitForAllSubmits: false,
		Simulate:          false,
	}
}

// NewSimpleBuyParamsWithDurableNonce creates a simple buy request using a durable nonce.
func NewSimpleBuyParamsWithDurableNonce(
	dexType DexType,
	payWith TradeTokenType,
	mint solana.PublicKey,
	amount BuyAmount,
	extensionParams interface{},
	durableNonce DurableNonceInfo,
	gasFeeStrategy *GasFeeStrategy,
) SimpleBuyParams {
	p := NewSimpleBuyParams(dexType, payWith, mint, amount, extensionParams, solana.Hash{}, gasFeeStrategy)
	return p.SetDurableNonce(durableNonce)
}

// SetSlippageBasisPoints returns a copy with slippage in basis points.
func (p SimpleBuyParams) SetSlippageBasisPoints(value uint64) SimpleBuyParams {
	p.SlippageBasisPoints = value
	return p
}

// SetAccountPolicy returns a copy with account lifecycle behavior.
func (p SimpleBuyParams) SetAccountPolicy(value AccountPolicy) SimpleBuyParams {
	p.AccountPolicy = value
	return p
}

// SetAddressLookupTableAccount returns a copy with an address lookup table.
func (p SimpleBuyParams) SetAddressLookupTableAccount(value solana.PublicKey) SimpleBuyParams {
	p.AddressLookupTableAccount = &value
	return p
}

// SetDurableNonce returns a copy using a durable nonce instead of a recent blockhash.
func (p SimpleBuyParams) SetDurableNonce(value DurableNonceInfo) SimpleBuyParams {
	p.DurableNonce = &value
	p.RecentBlockhash = nil
	return p
}

// SetWaitTxConfirmed returns a copy with confirmation waiting configured.
func (p SimpleBuyParams) SetWaitTxConfirmed(value bool) SimpleBuyParams {
	p.WaitTxConfirmed = value
	return p
}

// SetWaitForAllSubmits returns a copy with fast-submit response collection configured.
func (p SimpleBuyParams) SetWaitForAllSubmits(value bool) SimpleBuyParams {
	p.WaitForAllSubmits = value
	return p
}

// SetSimulate returns a copy configured for simulation instead of submission.
func (p SimpleBuyParams) SetSimulate(value bool) SimpleBuyParams {
	p.Simulate = value
	return p
}

// SetGrpcRecvUs returns a copy with upstream receive timestamp metadata.
func (p SimpleBuyParams) SetGrpcRecvUs(value int64) SimpleBuyParams {
	p.GrpcRecvUs = &value
	return p
}

// NewSimpleSellParams creates a simple sell request using a recent blockhash.
func NewSimpleSellParams(
	dexType DexType,
	receiveAs TradeTokenType,
	mint solana.PublicKey,
	amount SellAmount,
	extensionParams interface{},
	recentBlockhash solana.Hash,
	gasFeeStrategy *GasFeeStrategy,
) SimpleSellParams {
	return SimpleSellParams{
		DexType:           dexType,
		ReceiveAs:         receiveAs,
		Mint:              mint,
		Amount:            amount,
		ExtensionParams:   extensionParams,
		RecentBlockhash:   &recentBlockhash,
		GasFeeStrategy:    gasFeeStrategy,
		AccountPolicy:     AccountPolicyAuto,
		WaitTxConfirmed:   false,
		WaitForAllSubmits: false,
		Simulate:          false,
	}
}

// NewSimpleSellParamsWithDurableNonce creates a simple sell request using a durable nonce.
func NewSimpleSellParamsWithDurableNonce(
	dexType DexType,
	receiveAs TradeTokenType,
	mint solana.PublicKey,
	amount SellAmount,
	extensionParams interface{},
	durableNonce DurableNonceInfo,
	gasFeeStrategy *GasFeeStrategy,
) SimpleSellParams {
	p := NewSimpleSellParams(dexType, receiveAs, mint, amount, extensionParams, solana.Hash{}, gasFeeStrategy)
	return p.SetDurableNonce(durableNonce)
}

// SetSlippageBasisPoints returns a copy with slippage in basis points.
func (p SimpleSellParams) SetSlippageBasisPoints(value uint64) SimpleSellParams {
	p.SlippageBasisPoints = value
	return p
}

// SetAccountPolicy returns a copy with account lifecycle behavior.
func (p SimpleSellParams) SetAccountPolicy(value AccountPolicy) SimpleSellParams {
	p.AccountPolicy = value
	return p
}

// SetAddressLookupTableAccount returns a copy with an address lookup table.
func (p SimpleSellParams) SetAddressLookupTableAccount(value solana.PublicKey) SimpleSellParams {
	p.AddressLookupTableAccount = &value
	return p
}

// SetDurableNonce returns a copy using a durable nonce instead of a recent blockhash.
func (p SimpleSellParams) SetDurableNonce(value DurableNonceInfo) SimpleSellParams {
	p.DurableNonce = &value
	p.RecentBlockhash = nil
	return p
}

// SetWaitTxConfirmed returns a copy with confirmation waiting configured.
func (p SimpleSellParams) SetWaitTxConfirmed(value bool) SimpleSellParams {
	p.WaitTxConfirmed = value
	return p
}

// SetWaitForAllSubmits returns a copy with fast-submit response collection configured.
func (p SimpleSellParams) SetWaitForAllSubmits(value bool) SimpleSellParams {
	p.WaitForAllSubmits = value
	return p
}

// SetSimulate returns a copy configured for simulation instead of submission.
func (p SimpleSellParams) SetSimulate(value bool) SimpleSellParams {
	p.Simulate = value
	return p
}

// SetWithTip returns a copy with relay tips enabled or disabled.
func (p SimpleSellParams) SetWithTip(value bool) SimpleSellParams {
	p.WithTip = &value
	return p
}

// SetGrpcRecvUs returns a copy with upstream receive timestamp metadata.
func (p SimpleSellParams) SetGrpcRecvUs(value int64) SimpleSellParams {
	p.GrpcRecvUs = &value
	return p
}

func buyAccountFlags(policy AccountPolicy) (createInput, createMint, closeInput bool) {
	switch policy {
	case AccountPolicyHotPathMinimal, AccountPolicyAssumePrepared:
		return false, false, false
	case AccountPolicyCreateMissing:
		return true, true, false
	default:
		return false, true, false
	}
}

func sellAccountFlags(policy AccountPolicy, receiveAs TradeTokenType) (createOutput, closeOutput, closeMint bool) {
	switch policy {
	case AccountPolicyHotPathMinimal, AccountPolicyAssumePrepared:
		return false, false, false
	case AccountPolicyCreateMissing:
		return true, false, false
	default:
		return receiveAs != TradeTokenTypeSOL, false, false
	}
}

// ToTradeBuyParams converts a high-level buy request to the legacy parameter surface.
func (p SimpleBuyParams) ToTradeBuyParams() TradeBuyParams {
	inputAmount := p.Amount.Amount
	var fixedOutput *uint64
	useExactSolAmount := true

	switch p.Amount.Kind {
	case BuyAmountExactOutput:
		inputAmount = p.Amount.MaxInputAmount
		output := p.Amount.OutputAmount
		fixedOutput = &output
	case BuyAmountWithMaxInput:
		useExactSolAmount = false
	}

	createInput, createMint, closeInput := buyAccountFlags(p.AccountPolicy)
	recentBlockhash := p.RecentBlockhash
	if p.DurableNonce != nil {
		recentBlockhash = nil
	}

	return TradeBuyParams{
		DexType:                   p.DexType,
		InputTokenType:            p.PayWith,
		Mint:                      p.Mint,
		InputTokenAmount:          inputAmount,
		SlippageBasisPoints:       p.SlippageBasisPoints,
		RecentBlockhash:           recentBlockhash,
		ExtensionParams:           p.ExtensionParams,
		AddressLookupTableAccount: p.AddressLookupTableAccount,
		WaitTxConfirmed:           p.WaitTxConfirmed,
		WaitForAllSubmits:         p.WaitForAllSubmits,
		CreateInputTokenATA:       createInput,
		CloseInputTokenATA:        closeInput,
		CreateMintATA:             createMint,
		DurableNonce:              p.DurableNonce,
		FixedOutputTokenAmount:    fixedOutput,
		GasFeeStrategy:            p.GasFeeStrategy,
		Simulate:                  p.Simulate,
		UseExactSolAmount:         &useExactSolAmount,
		GrpcRecvUs:                p.GrpcRecvUs,
	}
}

// ToTradeSellParams converts a high-level sell request to the legacy parameter surface.
func (p SimpleSellParams) ToTradeSellParams() TradeSellParams {
	inputAmount := p.Amount.Amount
	var fixedOutput *uint64
	if p.Amount.Kind == SellAmountExactOutput {
		inputAmount = p.Amount.MaxInputAmount
		output := p.Amount.OutputAmount
		fixedOutput = &output
	}

	createOutput, closeOutput, closeMint := sellAccountFlags(p.AccountPolicy, p.ReceiveAs)
	recentBlockhash := p.RecentBlockhash
	if p.DurableNonce != nil {
		recentBlockhash = nil
	}

	withTip := true
	if p.WithTip != nil {
		withTip = *p.WithTip
	}

	return TradeSellParams{
		DexType:                   p.DexType,
		OutputTokenType:           p.ReceiveAs,
		Mint:                      p.Mint,
		InputTokenAmount:          inputAmount,
		SlippageBasisPoints:       p.SlippageBasisPoints,
		RecentBlockhash:           recentBlockhash,
		WithTip:                   withTip,
		ExtensionParams:           p.ExtensionParams,
		AddressLookupTableAccount: p.AddressLookupTableAccount,
		WaitTxConfirmed:           p.WaitTxConfirmed,
		WaitForAllSubmits:         p.WaitForAllSubmits,
		CreateOutputTokenATA:      createOutput,
		CloseOutputTokenATA:       closeOutput,
		CloseMintTokenATA:         closeMint,
		DurableNonce:              p.DurableNonce,
		FixedOutputTokenAmount:    fixedOutput,
		GasFeeStrategy:            p.GasFeeStrategy,
		Simulate:                  p.Simulate,
		GrpcRecvUs:                p.GrpcRecvUs,
	}
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
	if err := validateTradeBoundary(params.InputTokenAmount, params.SlippageBasisPoints, params.FixedOutputTokenAmount); err != nil {
		return nil, err
	}
	return c.executeTrade(ctx, TradeTypeBuy, params)
}

// Sell executes a sell order
func (c *TradingClient) Sell(ctx context.Context, params TradeSellParams) (*TradeResult, error) {
	if err := validateTradeBoundary(params.InputTokenAmount, params.SlippageBasisPoints, params.FixedOutputTokenAmount); err != nil {
		return nil, err
	}
	return c.executeSell(ctx, params)
}

func validateTradeBoundary(inputAmount, slippage uint64, fixedOutput *uint64) error {
	if inputAmount == 0 {
		return ErrInvalidAmount
	}
	if slippage >= 10_000 {
		return ErrInvalidSlippage
	}
	if fixedOutput != nil && *fixedOutput == 0 {
		return ErrInvalidAmount
	}
	return nil
}

// BuildBuyParamsFromSimple converts a high-level buy request to the legacy parameter surface.
func (c *TradingClient) BuildBuyParamsFromSimple(params SimpleBuyParams) TradeBuyParams {
	return params.ToTradeBuyParams()
}

// BuildSellParamsFromSimple converts a high-level sell request to the legacy parameter surface.
func (c *TradingClient) BuildSellParamsFromSimple(params SimpleSellParams) TradeSellParams {
	return params.ToTradeSellParams()
}

// BuySimple converts a high-level buy request and then follows the root Buy boundary.
// The root facade currently returns ErrTradingExecutionUnavailable for execution; use
// BuildBuyParamsFromSimple when only parameter conversion is needed.
func (c *TradingClient) BuySimple(ctx context.Context, params SimpleBuyParams) (*TradeResult, error) {
	return c.Buy(ctx, c.BuildBuyParamsFromSimple(params))
}

// SellSimple converts a high-level sell request and then follows the root Sell boundary.
// The root facade currently returns ErrTradingExecutionUnavailable for execution; use
// BuildSellParamsFromSimple when only parameter conversion is needed.
func (c *TradingClient) SellSimple(ctx context.Context, params SimpleSellParams) (*TradeResult, error) {
	return c.Sell(ctx, c.BuildSellParamsFromSimple(params))
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
