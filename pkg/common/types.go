package common

import (
	"encoding/binary"
	"fmt"
	"math/big"
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
	SwqosTypeSolami       = soltradesdk.SwqosTypeSolami
	SwqosTypeLunarLander  = soltradesdk.SwqosTypeLunarLander
	SwqosTypeGlaive       = soltradesdk.SwqosTypeGlaive
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
		SwqosTypeSolami,
		SwqosTypeLunarLander,
		SwqosTypeGlaive,
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
	QuoteMint            [32]byte
}

// Constants for bonding curve calculations
const (
	InitialVirtualTokenReserves uint64 = 1073000000000000
	InitialVirtualSolReserves   uint64 = 30000000000
	InitialRealTokenReserves    uint64 = 793100000000000
	TokenTotalSupply            uint64 = 1000000000000000
	FeeBasisPoints              uint64 = 95 // Pinned fallback; not current fee discovery.
	CreatorFee                  uint64 = 30
)

// Curve price math uses wide intermediates, as in the pinned Rust u128 methods.
func curveInt(n uint64) *big.Int { return new(big.Int).SetUint64(n) }

// Checked entries preserve Rust's error on completed or invalid curves.
func (b *BondingCurveAccount) GetBuyPriceChecked(amount uint64) (uint64, error) {
	if b.Complete {
		return 0, fmt.Errorf("Curve is complete")
	}
	if amount == 0 {
		return 0, nil
	}
	product := new(big.Int).Mul(curveInt(b.VirtualSolReserves), curveInt(b.VirtualTokenReserves))
	denominator := new(big.Int).Add(curveInt(b.VirtualSolReserves), curveInt(amount))
	reserve := new(big.Int).Add(new(big.Int).Quo(product, denominator), big.NewInt(1))
	if reserve.Cmp(curveInt(b.VirtualTokenReserves)) > 0 {
		return 0, fmt.Errorf("Invalid curve reserves")
	}
	out := new(big.Int).Sub(curveInt(b.VirtualTokenReserves), reserve).Uint64()
	if out > b.RealTokenReserves {
		out = b.RealTokenReserves
	}
	return out, nil
}
func (b *BondingCurveAccount) GetBuyPrice(amount uint64) (uint64, error) {
	return b.GetBuyPriceChecked(amount)
}
func (b *BondingCurveAccount) GetSellPriceChecked(amount, feeBasisPoints uint64) (uint64, error) {
	if b.Complete {
		return 0, fmt.Errorf("Curve is complete")
	}
	if amount == 0 {
		return 0, nil
	}
	gross := new(big.Int).Quo(new(big.Int).Mul(curveInt(amount), curveInt(b.VirtualSolReserves)), new(big.Int).Add(curveInt(b.VirtualTokenReserves), curveInt(amount)))
	fee := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).Set(gross), curveInt(feeBasisPoints)), big.NewInt(10000))
	if fee.Cmp(gross) > 0 {
		return 0, fmt.Errorf("Fee exceeds output")
	}
	return new(big.Int).Sub(gross, fee).Uint64(), nil
}
func (b *BondingCurveAccount) GetSellPrice(amount, feeBasisPoints uint64) (uint64, error) {
	return b.GetSellPriceChecked(amount, feeBasisPoints)
}
func (b *BondingCurveAccount) GetMarketCapSol() uint64 {
	if b.VirtualTokenReserves == 0 {
		return 0
	}
	return new(big.Int).Quo(new(big.Int).Mul(curveInt(b.TokenTotalSupply), curveInt(b.VirtualSolReserves)), curveInt(b.VirtualTokenReserves)).Uint64()
}
func (b *BondingCurveAccount) GetTokenPrice() float64 {
	return (float64(b.VirtualSolReserves) / 100_000_000.0) / (float64(b.VirtualTokenReserves) / 100_000.0)
}
func (b *BondingCurveAccount) GetBuyOutPriceChecked(amount, feeBasisPoints uint64) (uint64, error) {
	tokens := amount
	if tokens < b.RealSolReserves {
		tokens = b.RealSolReserves
	}
	if tokens >= b.VirtualTokenReserves {
		return 0, fmt.Errorf("Invalid buyout reserves")
	}
	value := new(big.Int).Add(new(big.Int).Quo(new(big.Int).Mul(curveInt(tokens), curveInt(b.VirtualSolReserves)), curveInt(b.VirtualTokenReserves-tokens)), big.NewInt(1))
	fee := new(big.Int).Quo(new(big.Int).Mul(new(big.Int).Set(value), curveInt(feeBasisPoints)), big.NewInt(10000))
	return new(big.Int).Add(value, fee).Uint64(), nil
}
func (b *BondingCurveAccount) GetBuyOutPrice(amount, feeBasisPoints uint64) (uint64, error) {
	return b.GetBuyOutPriceChecked(amount, feeBasisPoints)
}
func (b *BondingCurveAccount) GetFinalMarketCapSolChecked(feeBasisPoints uint64) (uint64, error) {
	value, err := b.GetBuyOutPriceChecked(b.RealTokenReserves, feeBasisPoints)
	if err != nil {
		return 0, err
	}
	if b.RealTokenReserves > b.VirtualTokenReserves {
		return 0, fmt.Errorf("Invalid curve reserves")
	}
	tokens := b.VirtualTokenReserves - b.RealTokenReserves
	if tokens == 0 {
		return 0, nil
	}
	virtualValue := new(big.Int).Add(curveInt(b.VirtualSolReserves), curveInt(value))
	return new(big.Int).Quo(new(big.Int).Mul(curveInt(b.TokenTotalSupply), virtualValue), curveInt(tokens)).Uint64(), nil
}
func (b *BondingCurveAccount) GetFinalMarketCapSol(feeBasisPoints uint64) (uint64, error) {
	return b.GetFinalMarketCapSolChecked(feeBasisPoints)
}

// BondingCurveAccountSize includes the discriminator and the V2 quote mint.
const BondingCurveAccountSize = 115

// DecodeBondingCurveAccount accepts discriminator-prefixed accounts and exact
// legacy/V2 Borsh bodies. Legacy bodies have no quote mint (zero means native).
// Reject partial quote keys and malformed Borsh booleans.
func DecodeBondingCurveAccount(data []byte, account [32]byte) *BondingCurveAccount {
	offset := 0
	if (len(data) == 75 || len(data) == 107) && string(data[:8]) != string([]byte{23, 183, 248, 55, 96, 216, 172, 96}) {
		// Explicit-size Borsh body.
	} else {
		if len(data) < 83 || (len(data) > 83 && len(data) < 115) || string(data[:8]) != string([]byte{23, 183, 248, 55, 96, 216, 172, 96}) {
			return nil
		}
		offset = 8
	}
	body := data[offset:]
	for _, index := range []int{40, 73, 74} {
		if body[index] > 1 {
			return nil
		}
	}
	curve := &BondingCurveAccount{Account: account}
	curve.VirtualTokenReserves = binary.LittleEndian.Uint64(body[0:8])
	curve.VirtualSolReserves = binary.LittleEndian.Uint64(body[8:16])
	curve.RealTokenReserves = binary.LittleEndian.Uint64(body[16:24])
	curve.RealSolReserves = binary.LittleEndian.Uint64(body[24:32])
	curve.TokenTotalSupply = binary.LittleEndian.Uint64(body[32:40])
	curve.Complete = body[40] == 1
	copy(curve.Creator[:], body[41:73])
	curve.IsMayhemMode = body[73] == 1
	curve.IsCashbackCoin = body[74] == 1
	if len(body) >= 107 {
		copy(curve.QuoteMint[:], body[75:107])
	}
	return curve
}

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
