package trading

import (
	"context"
	"fmt"
	"math/big"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/gagliardetto/solana-go"
)

// TradeExecutorInterface defines the interface for trade executors
type TradeExecutorInterface interface {
	ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error)
	ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error)
}

// PumpFunExecutor handles PumpFun DEX trades
type PumpFunExecutor struct {
	*TradeExecutor
}

// NewPumpFunExecutor creates a new PumpFun executor
func NewPumpFunExecutor(base *TradeExecutor) *PumpFunExecutor {
	return &PumpFunExecutor{TradeExecutor: base}
}

// PumpFunBuyParams represents parameters for PumpFun buy
type PumpFunBuyParams struct {
	Payer                  solana.PublicKey
	OutputMint             solana.PublicKey
	InputAmount            uint64
	SlippageBasisPoints    uint16
	BondingCurve           solana.PublicKey
	CreatorVault           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
	TokenProgram           solana.PublicKey
	CreateOutputATA        bool
	UseExactSolAmount      bool
}

// PumpFunSellParams represents parameters for PumpFun sell
type PumpFunSellParams struct {
	Payer                  solana.PublicKey
	InputMint              solana.PublicKey
	TokenAmount            uint64
	SlippageBasisPoints    uint16
	BondingCurve           solana.PublicKey
	CreatorVault           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
	TokenProgram           solana.PublicKey
	CloseTokenAccount      bool
}

// ExecuteBuy executes a buy on PumpFun
func (e *PumpFunExecutor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	if _, ok := params.(*PumpFunBuyParams); !ok {
		return nil, fmt.Errorf("invalid PumpFun buy params: %T", params)
	}

	return nil, unavailableDexExecutorError(soltradesdk.DexTypePumpFun, "buy")
}

// ExecuteSell executes a sell on PumpFun
func (e *PumpFunExecutor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	if _, ok := params.(*PumpFunSellParams); !ok {
		return nil, fmt.Errorf("invalid PumpFun sell params: %T", params)
	}

	return nil, unavailableDexExecutorError(soltradesdk.DexTypePumpFun, "sell")
}

// PumpSwapExecutor handles PumpSwap DEX trades
type PumpSwapExecutor struct {
	*TradeExecutor
}

// NewPumpSwapExecutor creates a new PumpSwap executor
func NewPumpSwapExecutor(base *TradeExecutor) *PumpSwapExecutor {
	return &PumpSwapExecutor{TradeExecutor: base}
}

// PumpSwapBuyParams represents parameters for PumpSwap buy
type PumpSwapBuyParams struct {
	Payer                  solana.PublicKey
	Pool                   solana.PublicKey
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	InputAmount            uint64
	SlippageBasisPoints    uint16
	PoolBaseTokenAccount   solana.PublicKey
	PoolQuoteTokenAccount  solana.PublicKey
	PoolBaseTokenReserves  uint64
	PoolQuoteTokenReserves uint64
	VirtualQuoteReserves   *big.Int
	CoinCreatorVaultATA    solana.PublicKey
	CoinCreatorVaultAuth   solana.PublicKey
	BaseTokenProgram       solana.PublicKey
	QuoteTokenProgram      solana.PublicKey
	IsMayhemMode           bool
	IsCashbackCoin         bool
}

// PumpSwapSellParams represents parameters for PumpSwap sell
type PumpSwapSellParams struct {
	Payer                  solana.PublicKey
	Pool                   solana.PublicKey
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	TokenAmount            uint64
	SlippageBasisPoints    uint16
	PoolBaseTokenAccount   solana.PublicKey
	PoolQuoteTokenAccount  solana.PublicKey
	PoolBaseTokenReserves  uint64
	PoolQuoteTokenReserves uint64
	VirtualQuoteReserves   *big.Int
	CoinCreatorVaultATA    solana.PublicKey
	CoinCreatorVaultAuth   solana.PublicKey
	BaseTokenProgram       solana.PublicKey
	QuoteTokenProgram      solana.PublicKey
	IsMayhemMode           bool
	IsCashbackCoin         bool
}

// ExecuteBuy executes a buy on PumpSwap
func (e *PumpSwapExecutor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	if _, ok := params.(*PumpSwapBuyParams); !ok {
		return nil, fmt.Errorf("invalid PumpSwap buy params: %T", params)
	}

	return nil, unavailableDexExecutorError(soltradesdk.DexTypePumpSwap, "buy")
}

// ExecuteSell executes a sell on PumpSwap
func (e *PumpSwapExecutor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	if _, ok := params.(*PumpSwapSellParams); !ok {
		return nil, fmt.Errorf("invalid PumpSwap sell params: %T", params)
	}

	return nil, unavailableDexExecutorError(soltradesdk.DexTypePumpSwap, "sell")
}

// BonkExecutor handles Bonk DEX trades
type BonkExecutor struct {
	*TradeExecutor
}

// NewBonkExecutor creates a new Bonk executor
func NewBonkExecutor(base *TradeExecutor) *BonkExecutor {
	return &BonkExecutor{TradeExecutor: base}
}

// ExecuteBuy executes a buy on Bonk
func (e *BonkExecutor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeBonk, "buy")
}

// ExecuteSell executes a sell on Bonk
func (e *BonkExecutor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeBonk, "sell")
}

// RaydiumCpmmExecutor handles Raydium CPMM DEX trades
type RaydiumCpmmExecutor struct {
	*TradeExecutor
}

// NewRaydiumCpmmExecutor creates a new Raydium CPMM executor
func NewRaydiumCpmmExecutor(base *TradeExecutor) *RaydiumCpmmExecutor {
	return &RaydiumCpmmExecutor{TradeExecutor: base}
}

// ExecuteBuy executes a buy on Raydium CPMM
func (e *RaydiumCpmmExecutor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeRaydiumCpmm, "buy")
}

// ExecuteSell executes a sell on Raydium CPMM
func (e *RaydiumCpmmExecutor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeRaydiumCpmm, "sell")
}

// RaydiumAmmV4Executor handles Raydium AMM V4 DEX trades
type RaydiumAmmV4Executor struct {
	*TradeExecutor
}

// NewRaydiumAmmV4Executor creates a new Raydium AMM V4 executor
func NewRaydiumAmmV4Executor(base *TradeExecutor) *RaydiumAmmV4Executor {
	return &RaydiumAmmV4Executor{TradeExecutor: base}
}

// ExecuteBuy executes a buy on Raydium AMM V4
func (e *RaydiumAmmV4Executor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeRaydiumAmmV4, "buy")
}

// ExecuteSell executes a sell on Raydium AMM V4
func (e *RaydiumAmmV4Executor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeRaydiumAmmV4, "sell")
}

// MeteoraDammV2Executor handles Meteora DAMM V2 DEX trades
type MeteoraDammV2Executor struct {
	*TradeExecutor
}

// NewMeteoraDammV2Executor creates a new Meteora DAMM V2 executor
func NewMeteoraDammV2Executor(base *TradeExecutor) *MeteoraDammV2Executor {
	return &MeteoraDammV2Executor{TradeExecutor: base}
}

// ExecuteBuy executes a buy on Meteora DAMM V2
func (e *MeteoraDammV2Executor) ExecuteBuy(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeMeteoraDammV2, "buy")
}

// ExecuteSell executes a sell on Meteora DAMM V2
func (e *MeteoraDammV2Executor) ExecuteSell(ctx context.Context, params interface{}) (*ExecuteResult, error) {
	return nil, unavailableDexExecutorError(soltradesdk.DexTypeMeteoraDammV2, "sell")
}

func unavailableDexExecutorError(dexType soltradesdk.DexType, operation string) error {
	return soltradesdk.NewTradeError(
		2001,
		fmt.Sprintf("pkg/trading %s executor for %s does not build or submit protocol trades; build instructions with pkg/instruction and submit prebuilt transactions through TradeExecutor or pkg/trading/core", operation, dexType),
		soltradesdk.ErrTradingExecutionUnavailable,
	)
}

// TradeExecutorFactory creates trade executors for different DEX types
type TradeExecutorFactory struct {
	baseExecutor *TradeExecutor
	executors    map[soltradesdk.DexType]TradeExecutorInterface
}

// NewTradeExecutorFactory creates a new trade executor factory
func NewTradeExecutorFactory(baseExecutor *TradeExecutor) *TradeExecutorFactory {
	factory := &TradeExecutorFactory{
		baseExecutor: baseExecutor,
		executors:    make(map[soltradesdk.DexType]TradeExecutorInterface),
	}

	// Register default executors
	factory.RegisterExecutor(soltradesdk.DexTypePumpFun, NewPumpFunExecutor(baseExecutor))
	factory.RegisterExecutor(soltradesdk.DexTypePumpSwap, NewPumpSwapExecutor(baseExecutor))
	factory.RegisterExecutor(soltradesdk.DexTypeBonk, NewBonkExecutor(baseExecutor))
	factory.RegisterExecutor(soltradesdk.DexTypeRaydiumCpmm, NewRaydiumCpmmExecutor(baseExecutor))
	factory.RegisterExecutor(soltradesdk.DexTypeRaydiumAmmV4, NewRaydiumAmmV4Executor(baseExecutor))
	factory.RegisterExecutor(soltradesdk.DexTypeMeteoraDammV2, NewMeteoraDammV2Executor(baseExecutor))

	return factory
}

// RegisterExecutor registers a trade executor for a DEX type
func (f *TradeExecutorFactory) RegisterExecutor(dexType soltradesdk.DexType, executor TradeExecutorInterface) {
	f.executors[dexType] = executor
}

// GetExecutor returns the executor for a DEX type
func (f *TradeExecutorFactory) GetExecutor(dexType soltradesdk.DexType) (TradeExecutorInterface, error) {
	executor, ok := f.executors[dexType]
	if !ok {
		return nil, fmt.Errorf("no executor available for DEX type: %s", dexType)
	}
	return executor, nil
}

// TradingClient provides a high-level interface for trading
type TradingClient struct {
	factory *TradeExecutorFactory
}

// NewTradingClient creates a new trading client
func NewTradingClient(factory *TradeExecutorFactory) *TradingClient {
	return &TradingClient{
		factory: factory,
	}
}

// Buy executes a buy trade on the specified DEX
func (c *TradingClient) Buy(ctx context.Context, dexType soltradesdk.DexType, params interface{}) (*ExecuteResult, error) {
	executor, err := c.factory.GetExecutor(dexType)
	if err != nil {
		return nil, err
	}

	return executor.ExecuteBuy(ctx, params)
}

// Sell executes a sell trade on the specified DEX
func (c *TradingClient) Sell(ctx context.Context, dexType soltradesdk.DexType, params interface{}) (*ExecuteResult, error) {
	executor, err := c.factory.GetExecutor(dexType)
	if err != nil {
		return nil, err
	}

	return executor.ExecuteSell(ctx, params)
}
