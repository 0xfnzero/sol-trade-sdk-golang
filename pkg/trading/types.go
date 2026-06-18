package trading

import (
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

// Re-exports from the main package for convenience
type (
	DexType          = soltradesdk.DexType
	TradeType        = soltradesdk.TradeType
	TradeTokenType   = soltradesdk.TradeTokenType
	SwqosType        = soltradesdk.SwqosType
	SwqosRegion      = soltradesdk.SwqosRegion
	SwqosConfig      = soltradesdk.SwqosConfig
	GasFeeStrategy   = soltradesdk.GasFeeStrategy
	TradeConfig      = soltradesdk.TradeConfig
	AccountPolicy    = soltradesdk.AccountPolicy
	BuyAmountKind    = soltradesdk.BuyAmountKind
	BuyAmount        = soltradesdk.BuyAmount
	SellAmountKind   = soltradesdk.SellAmountKind
	SellAmount       = soltradesdk.SellAmount
	TradeBuyParams   = soltradesdk.TradeBuyParams
	TradeSellParams  = soltradesdk.TradeSellParams
	SimpleBuyParams  = soltradesdk.SimpleBuyParams
	SimpleSellParams = soltradesdk.SimpleSellParams
	TradeResult      = soltradesdk.TradeResult
	DurableNonceInfo = soltradesdk.DurableNonceInfo
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
	SwqosTypeSolami       = soltradesdk.SwqosTypeSolami
	SwqosTypeDefault      = soltradesdk.SwqosTypeDefault

	AccountPolicyAuto           = soltradesdk.AccountPolicyAuto
	AccountPolicyHotPathMinimal = soltradesdk.AccountPolicyHotPathMinimal
	AccountPolicyCreateMissing  = soltradesdk.AccountPolicyCreateMissing
	AccountPolicyAssumePrepared = soltradesdk.AccountPolicyAssumePrepared
	BuyAmountExactInput         = soltradesdk.BuyAmountExactInput
	BuyAmountExactOutput        = soltradesdk.BuyAmountExactOutput
	BuyAmountWithMaxInput       = soltradesdk.BuyAmountWithMaxInput
	SellAmountExactInput        = soltradesdk.SellAmountExactInput
	SellAmountExactOutput       = soltradesdk.SellAmountExactOutput

	SwqosRegionNewYork    = soltradesdk.SwqosRegionNewYork
	SwqosRegionFrankfurt  = soltradesdk.SwqosRegionFrankfurt
	SwqosRegionAmsterdam  = soltradesdk.SwqosRegionAmsterdam
	SwqosRegionDublin     = soltradesdk.SwqosRegionDublin
	SwqosRegionSLC        = soltradesdk.SwqosRegionSLC
	SwqosRegionTokyo      = soltradesdk.SwqosRegionTokyo
	SwqosRegionSingapore  = soltradesdk.SwqosRegionSingapore
	SwqosRegionLondon     = soltradesdk.SwqosRegionLondon
	SwqosRegionLosAngeles = soltradesdk.SwqosRegionLosAngeles
	SwqosRegionDefault    = soltradesdk.SwqosRegionDefault
)

var (
	BuyExactInput   = soltradesdk.BuyExactInput
	BuyExactOutput  = soltradesdk.BuyExactOutput
	BuyWithMaxInput = soltradesdk.BuyWithMaxInput
	SellExactInput  = soltradesdk.SellExactInput
	SellExactOutput = soltradesdk.SellExactOutput

	NewSimpleBuyParams                  = soltradesdk.NewSimpleBuyParams
	NewSimpleBuyParamsWithDurableNonce  = soltradesdk.NewSimpleBuyParamsWithDurableNonce
	NewSimpleSellParams                 = soltradesdk.NewSimpleSellParams
	NewSimpleSellParamsWithDurableNonce = soltradesdk.NewSimpleSellParamsWithDurableNonce
)
