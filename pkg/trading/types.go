package trading

import (
	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
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

	SwqosTypeJito         = soltradesdk.SwqosTypeJito
	SwqosTypeNextBlock    = soltradesdk.SwqosTypeNextBlock
	SwqosTypeZeroSlot     = soltradesdk.SwqosTypeZeroSlot
	SwqosTypeTemporal     = soltradesdk.SwqosTypeTemporal
	SwqosTypeBloxroute    = soltradesdk.SwqosTypeBloxroute
	SwqosTypeNode1        = soltradesdk.SwqosTypeNode1
	SwqosTypeFlashBlock   = soltradesdk.SwqosTypeFlashBlock
	SwqosTypeBlockRazor   = soltradesdk.SwqosTypeBlockRazor
	SwqosTypeAstralane    = soltradesdk.SwqosTypeAstralane
	SwqosTypeStellium     = soltradesdk.SwqosTypeStellium
	SwqosTypeLightspeed   = soltradesdk.SwqosTypeLightspeed
	SwqosTypeSoyas        = soltradesdk.SwqosTypeSoyas
	SwqosTypeSpeedlanding = soltradesdk.SwqosTypeSpeedlanding
	SwqosTypeHelius       = soltradesdk.SwqosTypeHelius
	SwqosTypeDefault      = soltradesdk.SwqosTypeDefault

	SwqosRegionNewYork    = soltradesdk.SwqosRegionNewYork
	SwqosRegionFrankfurt  = soltradesdk.SwqosRegionFrankfurt
	SwqosRegionAmsterdam  = soltradesdk.SwqosRegionAmsterdam
	SwqosRegionSLC        = soltradesdk.SwqosRegionSLC
	SwqosRegionTokyo      = soltradesdk.SwqosRegionTokyo
	SwqosRegionLondon     = soltradesdk.SwqosRegionLondon
	SwqosRegionLosAngeles = soltradesdk.SwqosRegionLosAngeles
	SwqosRegionDefault    = soltradesdk.SwqosRegionDefault
)
