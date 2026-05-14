package common

import (
	"encoding/binary"
	"sync"
	"sync/atomic"

	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
)

type SwqosType = soltradesdk.SwqosType
type TradeType = soltradesdk.TradeType

const (
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

	TradeTypeBuy  = soltradesdk.TradeTypeBuy
	TradeTypeSell = soltradesdk.TradeTypeSell
)

func GetAllSwqosTypes() []SwqosType {
	return []SwqosType{
		SwqosTypeJito,
		SwqosTypeNextBlock,
		SwqosTypeZeroSlot,
		SwqosTypeTemporal,
		SwqosTypeBloxroute,
		SwqosTypeNode1,
		SwqosTypeFlashBlock,
		SwqosTypeBlockRazor,
		SwqosTypeAstralane,
		SwqosTypeStellium,
		SwqosTypeLightspeed,
		SwqosTypeSoyas,
		SwqosTypeSpeedlanding,
		SwqosTypeHelius,
		SwqosTypeDefault,
	}
}

// GasFeeStrategyType represents the type of gas fee strategy
type GasFeeStrategyType int

const (
	GasFeeStrategyTypeNormal GasFeeStrategyType = iota
	GasFeeStrategyTypeLowTipHighCuPrice
	GasFeeStrategyTypeHighTipLowCuPrice
)

func (g GasFeeStrategyType) String() string {
	return [...]string{"Normal", "LowTipHighCuPrice", "HighTipLowCuPrice"}[g]
}

// GasFeeStrategyValue represents the gas fee configuration values
type GasFeeStrategyValue struct {
	CuLimit uint32
	CuPrice uint64
	Tip     float64
}

// GasFeeStrategy manages gas fee configurations for different SWQOS types
type GasFeeStrategy struct {
	strategies sync.Map // map[StrategyKey]GasFeeStrategyValue
	mu         sync.RWMutex
}

// StrategyKey is the key for gas fee strategy map
type StrategyKey struct {
	SwqosType    SwqosType
	TradeType    TradeType
	StrategyType GasFeeStrategyType
}

// NewGasFeeStrategy creates a new GasFeeStrategy
func NewGasFeeStrategy() *GasFeeStrategy {
	return &GasFeeStrategy{
		strategies: sync.Map{},
	}
}

// SetGlobalFeeStrategy sets global fee strategy for all SWQOS types
func (g *GasFeeStrategy) SetGlobalFeeStrategy(
	buyCuLimit, sellCuLimit uint32,
	buyCuPrice, sellCuPrice uint64,
	buyTip, sellTip float64,
) {
	for _, swqosType := range GetAllSwqosTypes() {
		if swqosType == SwqosTypeDefault {
			continue
		}
		g.Set(swqosType, TradeTypeBuy, GasFeeStrategyTypeNormal, buyCuLimit, buyCuPrice, buyTip)
		g.Set(swqosType, TradeTypeSell, GasFeeStrategyTypeNormal, sellCuLimit, sellCuPrice, sellTip)
	}
	// Default (RPC) has no tip
	g.Set(SwqosTypeDefault, TradeTypeBuy, GasFeeStrategyTypeNormal, buyCuLimit, buyCuPrice, 0)
	g.Set(SwqosTypeDefault, TradeTypeSell, GasFeeStrategyTypeNormal, sellCuLimit, sellCuPrice, 0)
}

// Set sets a specific gas fee strategy
func (g *GasFeeStrategy) Set(
	swqosType SwqosType,
	tradeType TradeType,
	strategyType GasFeeStrategyType,
	cuLimit uint32,
	cuPrice uint64,
	tip float64,
) {
	key := StrategyKey{SwqosType: swqosType, TradeType: tradeType, StrategyType: strategyType}
	value := GasFeeStrategyValue{CuLimit: cuLimit, CuPrice: cuPrice, Tip: tip}

	// Remove conflicting strategies
	if strategyType == GasFeeStrategyTypeNormal {
		g.Delete(swqosType, tradeType, GasFeeStrategyTypeLowTipHighCuPrice)
		g.Delete(swqosType, tradeType, GasFeeStrategyTypeHighTipLowCuPrice)
	} else {
		g.Delete(swqosType, tradeType, GasFeeStrategyTypeNormal)
	}

	g.strategies.Store(key, value)
}

// Get gets a gas fee strategy
func (g *GasFeeStrategy) Get(
	swqosType SwqosType,
	tradeType TradeType,
	strategyType GasFeeStrategyType,
) (GasFeeStrategyValue, bool) {
	key := StrategyKey{SwqosType: swqosType, TradeType: tradeType, StrategyType: strategyType}
	if v, ok := g.strategies.Load(key); ok {
		return v.(GasFeeStrategyValue), true
	}
	return GasFeeStrategyValue{}, false
}

// Delete removes a specific gas fee strategy
func (g *GasFeeStrategy) Delete(
	swqosType SwqosType,
	tradeType TradeType,
	strategyType GasFeeStrategyType,
) {
	key := StrategyKey{SwqosType: swqosType, TradeType: tradeType, StrategyType: strategyType}
	g.strategies.Delete(key)
}

// DeleteAll removes all strategies for a SWQOS type and trade type
func (g *GasFeeStrategy) DeleteAll(swqosType SwqosType, tradeType TradeType) {
	g.Delete(swqosType, tradeType, GasFeeStrategyTypeNormal)
	g.Delete(swqosType, tradeType, GasFeeStrategyTypeLowTipHighCuPrice)
	g.Delete(swqosType, tradeType, GasFeeStrategyTypeHighTipLowCuPrice)
}

// GetStrategies gets all strategies for a trade type
func (g *GasFeeStrategy) GetStrategies(tradeType TradeType) []StrategyResult {
	var results []StrategyResult
	seenTypes := make(map[SwqosType]bool)

	g.strategies.Range(func(key, value interface{}) bool {
		k := key.(StrategyKey)
		if k.TradeType == tradeType {
			if !seenTypes[k.SwqosType] {
				seenTypes[k.SwqosType] = true
			}
			results = append(results, StrategyResult{
				SwqosType:    k.SwqosType,
				StrategyType: k.StrategyType,
				Value:        value.(GasFeeStrategyValue),
			})
		}
		return true
	})

	return results
}

func (g *GasFeeStrategy) defaultValue() GasFeeStrategyValue {
	if v, ok := g.Get(SwqosTypeDefault, TradeTypeBuy, GasFeeStrategyTypeNormal); ok {
		return v
	}
	if v, ok := g.Get(SwqosTypeJito, TradeTypeBuy, GasFeeStrategyTypeNormal); ok {
		return v
	}
	return GasFeeStrategyValue{}
}

// GetComputeUnitPrice returns the default buy compute-unit price.
func (g *GasFeeStrategy) GetComputeUnitPrice() uint64 {
	return g.defaultValue().CuPrice
}

// GetComputeUnitLimit returns the default buy compute-unit limit.
func (g *GasFeeStrategy) GetComputeUnitLimit() uint32 {
	return g.defaultValue().CuLimit
}

// GetPriorityFee returns the default buy tip converted from SOL to lamports.
func (g *GasFeeStrategy) GetPriorityFee() uint64 {
	return uint64(g.defaultValue().Tip * 1_000_000_000)
}

// SetComputeUnitPrice updates compute-unit price across all configured strategies.
func (g *GasFeeStrategy) SetComputeUnitPrice(price uint64) {
	g.strategies.Range(func(key, value interface{}) bool {
		v := value.(GasFeeStrategyValue)
		v.CuPrice = price
		g.strategies.Store(key, v)
		return true
	})
}

// SetPriorityFee updates tip across all configured strategies using lamports.
func (g *GasFeeStrategy) SetPriorityFee(lamports uint64) {
	tipSol := float64(lamports) / 1_000_000_000
	g.strategies.Range(func(key, value interface{}) bool {
		v := value.(GasFeeStrategyValue)
		v.Tip = tipSol
		g.strategies.Store(key, v)
		return true
	})
}

// StrategyResult represents a strategy search result
type StrategyResult struct {
	SwqosType    SwqosType
	StrategyType GasFeeStrategyType
	Value        GasFeeStrategyValue
}

// UpdateBuyTip updates buy tip for all strategies
func (g *GasFeeStrategy) UpdateBuyTip(buyTip float64) {
	g.strategies.Range(func(key, value interface{}) bool {
		k := key.(StrategyKey)
		if k.TradeType == TradeTypeBuy {
			v := value.(GasFeeStrategyValue)
			v.Tip = buyTip
			g.strategies.Store(key, v)
		}
		return true
	})
}

// UpdateSellTip updates sell tip for all strategies
func (g *GasFeeStrategy) UpdateSellTip(sellTip float64) {
	g.strategies.Range(func(key, value interface{}) bool {
		k := key.(StrategyKey)
		if k.TradeType == TradeTypeSell {
			v := value.(GasFeeStrategyValue)
			v.Tip = sellTip
			g.strategies.Store(key, v)
		}
		return true
	})
}

// Clear clears all strategies
func (g *GasFeeStrategy) Clear() {
	g.strategies = sync.Map{}
}

// ===== Bonding Curve =====

// BondingCurveAccount represents the bonding curve state
type BondingCurveAccount struct {
	Discriminator        uint64
	Account              [32]byte
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	RealSolReserves      uint64
	TokenTotalSupply     uint64
	Complete             bool
	Creator              [32]byte
	IsMayhemMode         bool
	IsCashbackCoin       bool
}

// Constants for bonding curve calculations
const (
	InitialVirtualTokenReserves uint64 = 1073000000000000
	InitialVirtualSolReserves   uint64 = 30000000000
	InitialRealTokenReserves    uint64 = 793000000000000
	TokenTotalSupply            uint64 = 1000000000000000
	FeeBasisPoints              uint64 = 100 // 1%
	CreatorFee                  uint64 = 50  // 0.5%
)

// GetBuyPrice calculates the amount of tokens received for a given SOL amount
func (b *BondingCurveAccount) GetBuyPrice(amount uint64) uint64 {
	if b.Complete || amount == 0 {
		return 0
	}

	n := uint128(uint64(b.VirtualSolReserves) * uint64(b.VirtualTokenReserves))
	i := uint128(uint64(b.VirtualSolReserves) + uint64(amount))
	r := n/i + 1
	s := uint128(uint64(b.VirtualTokenReserves)) - r

	if uint64(s) < b.RealTokenReserves {
		return uint64(s)
	}
	return b.RealTokenReserves
}

// GetSellPrice calculates the amount of SOL received for selling tokens
func (b *BondingCurveAccount) GetSellPrice(amount uint64, feeBasisPoints uint64) uint64 {
	if b.Complete || amount == 0 {
		return 0
	}

	n := (uint128(amount) * uint128(b.VirtualSolReserves)) / (uint128(b.VirtualTokenReserves) + uint128(amount))
	a := (n * uint128(feeBasisPoints)) / 10000

	return uint64(n - a)
}

// GetMarketCapSol calculates the current market cap in SOL
func (b *BondingCurveAccount) GetMarketCapSol() uint64 {
	if b.VirtualTokenReserves == 0 {
		return 0
	}
	return uint64((uint128(b.TokenTotalSupply) * uint128(b.VirtualSolReserves)) / uint128(b.VirtualTokenReserves))
}

// GetTokenPrice calculates the token price in SOL
func (b *BondingCurveAccount) GetTokenPrice() float64 {
	if b.VirtualTokenReserves == 0 {
		return 0
	}
	vSol := float64(b.VirtualSolReserves) / 100_000_000.0
	vTokens := float64(b.VirtualTokenReserves) / 100_000.0
	return vSol / vTokens
}

// GetFinalMarketCapSol calculates the final market cap in SOL after all tokens are sold.
// 100% from Rust: src/common/bonding_curve.rs get_final_market_cap_sol
func (b *BondingCurveAccount) GetFinalMarketCapSol(feeBasisPoints uint64) uint64 {
	totalSellValue := b.getBuyOutPriceInternal(b.RealTokenReserves, feeBasisPoints)
	totalVirtualValue := b.VirtualSolReserves + totalSellValue
	totalVirtualTokens := b.VirtualTokenReserves - b.RealTokenReserves

	if totalVirtualTokens == 0 {
		return 0
	}

	return (b.TokenTotalSupply * totalVirtualValue) / totalVirtualTokens
}

func (b *BondingCurveAccount) getBuyOutPriceInternal(amount uint64, feeBasisPoints uint64) uint64 {
	solTokens := amount
	if amount < b.RealSolReserves {
		solTokens = b.RealSolReserves
	}

	if b.VirtualTokenReserves <= solTokens {
		return 0
	}

	totalSellValue := (solTokens*b.VirtualSolReserves)/(b.VirtualTokenReserves-solTokens) + 1
	fee := (totalSellValue * feeBasisPoints) / 10000

	return totalSellValue + fee
}

// BondingCurveAccountSize is the size of bonding curve account data (after discriminator)
const BondingCurveAccountSize = 8 + 8 + 8 + 8 + 8 + 8 + 1 + 32 + 1 + 1 // 77 bytes

// DecodeBondingCurveAccount decodes a BondingCurveAccount from on-chain account data.
// 100% from Rust: src/common/bonding_curve.rs
func DecodeBondingCurveAccount(data []byte, account [32]byte) *BondingCurveAccount {
	if len(data) < BondingCurveAccountSize {
		return nil
	}

	offset := 0

	// Check if data starts with discriminator (8 bytes)
	if len(data) >= 8+BondingCurveAccountSize {
		// Skip discriminator
		offset = 8
	}

	curve := &BondingCurveAccount{
		Account: account,
	}

	// virtual_token_reserves: u64
	curve.VirtualTokenReserves = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// virtual_sol_reserves: u64
	curve.VirtualSolReserves = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// real_token_reserves: u64
	curve.RealTokenReserves = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// real_sol_reserves: u64
	curve.RealSolReserves = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// token_total_supply: u64
	curve.TokenTotalSupply = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// complete: bool
	curve.Complete = data[offset] == 1
	offset += 1

	// creator: Pubkey (32 bytes)
	copy(curve.Creator[:], data[offset:offset+32])
	offset += 32

	// is_mayhem_mode: bool
	curve.IsMayhemMode = data[offset] == 1
	offset += 1

	// is_cashback_coin: bool
	curve.IsCashbackCoin = data[offset] == 1

	return curve
}

// uint128 represents a 128-bit unsigned integer (simplified)
type uint128 = uint64 // Simplified for Go implementation

// ===== Clock =====

// Clock provides high-resolution timing
type Clock struct {
	startTime int64
}

var globalClock int64

// NowMicroseconds returns current time in microseconds
func NowMicroseconds() int64 {
	return atomic.LoadInt64(&globalClock)
}

// SetClockTime sets the global clock time (for testing)
func SetClockTime(t int64) {
	atomic.StoreInt64(&globalClock, t)
}

// ===== Nonce Cache =====

// DurableNonceInfo represents durable nonce information
type DurableNonceInfo struct {
	NonceAccount    [32]byte
	Authority       [32]byte
	NonceHash       [32]byte
	RecentBlockhash [32]byte
}

// NonceCache caches nonce information
type NonceCache struct {
	nonces sync.Map
}

// NewNonceCache creates a new nonce cache
func NewNonceCache() *NonceCache {
	return &NonceCache{}
}

// Set sets a nonce in the cache
func (n *NonceCache) Set(pubkey [32]byte, info DurableNonceInfo) {
	n.nonces.Store(pubkey, info)
}

// Get gets a nonce from the cache
func (n *NonceCache) Get(pubkey [32]byte) (DurableNonceInfo, bool) {
	if v, ok := n.nonces.Load(pubkey); ok {
		return v.(DurableNonceInfo), true
	}
	return DurableNonceInfo{}, false
}

// Delete removes a nonce from the cache
func (n *NonceCache) Delete(pubkey [32]byte) {
	n.nonces.Delete(pubkey)
}

// ===== Rent =====

var (
	splTokenRent     atomic.Uint64
	splToken2022Rent atomic.Uint64
	defaultTokenRent uint64 = 2_039_280 // ~0.00203928 SOL
)

// GetTokenAccountRent returns the rent for a token account
func GetTokenAccountRent(isToken2022 bool) uint64 {
	if isToken2022 {
		if v := splToken2022Rent.Load(); v != 0 {
			return v
		}
		return defaultTokenRent
	}
	if v := splTokenRent.Load(); v != 0 {
		return v
	}
	return defaultTokenRent
}

// SetTokenAccountRent sets the rent for token accounts
func SetTokenAccountRent(isToken2022 bool, rent uint64) {
	if isToken2022 {
		splToken2022Rent.Store(rent)
	} else {
		splTokenRent.Store(rent)
	}
}

// Instruction represents a Solana instruction (placeholder)
type Instruction struct {
	ProgramID [32]byte
	Accounts  []AccountMeta
	Data      []byte
}

// AccountMeta represents account metadata
type AccountMeta struct {
	Pubkey     [32]byte
	IsSigner   bool
	IsWritable bool
}
