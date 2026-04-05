//! Compute Budget Manager - Caching compute budget instructions.
//! Based on sol-trade-sdk Rust implementation patterns.

package compute

import (
	"sync"
)

// ===== Constants =====

// ComputeBudgetProgram is the Solana compute budget program ID
var ComputeBudgetProgram = []byte{
	0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x42,
	0x75, 0x64, 0x67, 0x65, 0x74, 0x31, 0x31, 0x31,
	0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31,
	0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31,
}

// Instruction discriminators
var (
	// SetComputeUnitPrice discriminator (2)
	SetComputeUnitPriceDiscriminator = []byte{0x02}
	// SetComputeUnitLimit discriminator (0)
	SetComputeUnitLimitDiscriminator = []byte{0x00}
)

// ===== Cache Key =====

// ComputeBudgetCacheKey is the cache key for compute budget instructions
type ComputeBudgetCacheKey struct {
	UnitPrice uint64
	UnitLimit uint32
}

// ===== Cache =====

// ComputeBudgetCache stores compute budget instructions
// Uses RWMutex for high-performance concurrent access
type ComputeBudgetCache struct {
	mu     sync.RWMutex
	cache  map[ComputeBudgetCacheKey][][]byte
}

// Global cache instance
var globalCache = NewComputeBudgetCache()

// NewComputeBudgetCache creates a new cache
func NewComputeBudgetCache() *ComputeBudgetCache {
	return &ComputeBudgetCache{
		cache: make(map[ComputeBudgetCacheKey][][]byte),
	}
}

// ===== Instruction Builders =====

// SetComputeUnitPrice creates set compute unit price instruction
func SetComputeUnitPrice(price uint64) []byte {
	// Instruction: [discriminator (4 bytes) | price (8 bytes)]
	data := make([]byte, 12)
	copy(data[0:4], SetComputeUnitPriceDiscriminator)
	// Little-endian price
	data[4] = byte(price)
	data[5] = byte(price >> 8)
	data[6] = byte(price >> 16)
	data[7] = byte(price >> 24)
	data[8] = byte(price >> 32)
	data[9] = byte(price >> 40)
	data[10] = byte(price >> 48)
	data[11] = byte(price >> 56)
	return data
}

// SetComputeUnitLimit creates set compute unit limit instruction
func SetComputeUnitLimit(limit uint32) []byte {
	// Instruction: [discriminator (4 bytes) | limit (4 bytes)]
	data := make([]byte, 8)
	copy(data[0:4], SetComputeUnitLimitDiscriminator)
	// Little-endian limit
	data[4] = byte(limit)
	data[5] = byte(limit >> 8)
	data[6] = byte(limit >> 16)
	data[7] = byte(limit >> 24)
	return data
}

// ===== Cached Instruction Functions =====

// ExtendComputeBudgetInstructions extends instructions with compute budget instructions
// On cache hit, extends from cached slice (no allocation)
func ExtendComputeBudgetInstructions(
	instructions *[][]byte,
	unitPrice uint64,
	unitLimit uint32,
) {
	cacheKey := ComputeBudgetCacheKey{UnitPrice: unitPrice, UnitLimit: unitLimit}

	// Check cache
	globalCache.mu.RLock()
	if cached, ok := globalCache.cache[cacheKey]; ok {
		globalCache.mu.RUnlock()
		*instructions = append(*instructions, cached...)
		return
	}
	globalCache.mu.RUnlock()

	// Build new instructions
	var insts [][]byte
	if unitPrice > 0 {
		insts = append(insts, SetComputeUnitPrice(unitPrice))
	}
	if unitLimit > 0 {
		insts = append(insts, SetComputeUnitLimit(unitLimit))
	}

	// Store in cache
	globalCache.mu.Lock()
	globalCache.cache[cacheKey] = insts
	globalCache.mu.Unlock()

	*instructions = append(*instructions, insts...)
}

// ComputeBudgetInstructions returns compute budget instructions
// Note: prefer ExtendComputeBudgetInstructions on hot path
func ComputeBudgetInstructions(unitPrice uint64, unitLimit uint32) [][]byte {
	cacheKey := ComputeBudgetCacheKey{UnitPrice: unitPrice, UnitLimit: unitLimit}

	// Check cache
	globalCache.mu.RLock()
	if cached, ok := globalCache.cache[cacheKey]; ok {
		globalCache.mu.RUnlock()
		// Return copy
		result := make([][]byte, len(cached))
		copy(result, cached)
		return result
	}
	globalCache.mu.RUnlock()

	// Build new instructions
	var insts [][]byte
	if unitPrice > 0 {
		insts = append(insts, SetComputeUnitPrice(unitPrice))
	}
	if unitLimit > 0 {
		insts = append(insts, SetComputeUnitLimit(unitLimit))
	}

	// Store in cache
	globalCache.mu.Lock()
	// Store copy in cache
	cached := make([][]byte, len(insts))
	copy(cached, insts)
	globalCache.cache[cacheKey] = cached
	globalCache.mu.Unlock()

	return insts
}

// ===== Cache Statistics =====

// GetCacheStats returns cache statistics
func GetCacheStats() (size int) {
	globalCache.mu.RLock()
	defer globalCache.mu.RUnlock()
	return len(globalCache.cache)
}

// ClearCache clears the cache (for testing)
func ClearCache() {
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()
	globalCache.cache = make(map[ComputeBudgetCacheKey][][]byte)
}
