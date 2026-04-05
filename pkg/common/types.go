package common

import (
	"sync"
	"sync/atomic"
)

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
	FeeBasisPoints              uint64 = 100   // 1%
	CreatorFee                  uint64 = 50    // 0.5%
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
	splTokenRent      atomic.Uint64
	splToken2022Rent  atomic.Uint64
	defaultTokenRent  uint64 = 2_039_280 // ~0.00203928 SOL
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

// ===== WSOL Manager =====

// WSOLManager handles WSOL operations
type WSOLManager struct{}

// HandleWsol creates instructions to wrap SOL to WSOL
func (w *WSOLManager) HandleWsol(payer [32]byte, amount uint64) []Instruction {
	// Implementation creates ATA, transfers SOL, and syncs
	return nil // Placeholder - full implementation needed
}

// CloseWsol creates instructions to close WSOL account
func (w *WSOLManager) CloseWsol(payer [32]byte) []Instruction {
	return nil // Placeholder - full implementation needed
}

// CreateWsolATA creates instructions to create WSOL ATA
func (w *WSOLManager) CreateWsolATA(payer [32]byte) []Instruction {
	return nil // Placeholder - full implementation needed
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
