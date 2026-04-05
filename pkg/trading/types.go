package trading

import (
	soltradesdk "github.com/your-org/sol-trade-sdk-go"
)

// Re-exports from the main package for convenience
type (
	DexType              = soltradesdk.DexType
	TradeType            = soltradesdk.TradeType
	TradeTokenType       = soltradesdk.TradeTokenType
	SwqosType            = soltradesdk.SwqosType
	SwqosRegion          = soltradesdk.SwqosRegion
	SwqosConfig          = soltradesdk.SwqosConfig
	GasFeeStrategy       = soltradesdk.GasFeeStrategy
	TradeConfig          = soltradesdk.TradeConfig
	TradeBuyParams       = soltradesdk.TradeBuyParams
	TradeSellParams      = soltradesdk.TradeSellParams
	TradeResult          = soltradesdk.TradeResult
	DurableNonceInfo     = soltradesdk.DurableNonceInfo
)

const (
	DexTypePumpFun       = soltradesdk.DexTypePumpFun
	DexTypePumpSwap      = soltradesdk.DexTypePumpSwap
	DexTypeBonk          = soltradesdk.DexTypeBonk
	DexTypeRaydiumCpmm   = soltradesdk.DexTypeRaydiumCpmm
	DexTypeRaydiumAmmV4  = soltradesdk.DexTypeRaydiumAmmV4
	DexTypeMeteoraDammV2 = soltradesdk.DexTypeMeteoraDammV2

	TradeTypeBuy  = soltradesdk.TradeTypeBuy
	TradeTypeSell = soltradesdk.TradeTypeSell

	SwqosTypeDefault   = soltradesdk.SwqosTypeDefault
	SwqosTypeJito      = soltradesdk.SwqosTypeJito
	SwqosTypeBloxroute = soltradesdk.SwqosTypeBloxroute
	SwqosTypeZeroSlot  = soltradesdk.SwqosTypeZeroSlot
	SwqosTypeTemporal  = soltradesdk.SwqosTypeTemporal
	SwqosTypeFlashBlock = soltradesdk.SwqosTypeFlashBlock
	SwqosTypeBlockRazor = soltradesdk.SwqosTypeBlockRazor
	SwqosTypeNode1      = soltradesdk.SwqosTypeNode1
	SwqosTypeAstralane  = soltradesdk.SwqosTypeAstralane
	SwqosTypeNextBlock  = soltradesdk.SwqosTypeNextBlock
	SwqosTypeHelius     = soltradesdk.SwqosTypeHelius
)
