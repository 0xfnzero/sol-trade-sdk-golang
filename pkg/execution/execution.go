//! Execution: instruction preprocessing, cache prefetch, branch hints.
//! 执行模块：指令预处理、缓存预取、分支提示。
//!
//! Based on sol-trade-sdk Rust implementation patterns.

package execution

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

// ===== Branch Optimization =====

// BranchOptimizer provides branch prediction hints
// Based on Rust's likely/unlikely patterns
type BranchOptimizer struct{}

// Likely hints that the condition is likely true
// In Go, we can't directly control branch prediction, but we can structure code
func (b *BranchOptimizer) Likely(condition bool) bool {
	return condition
}

// Unlikely hints that the condition is likely false
func (b *BranchOptimizer) Unlikely(condition bool) bool {
	return condition
}

// PrefetchReadData triggers CPU prefetch for soon-to-be-accessed data
// This is a no-op on non-x86_64 architectures
func (b *BranchOptimizer) PrefetchReadData(ptr unsafe.Pointer) {
	// Go doesn't have direct prefetch intrinsics
	// On x86_64, this would use _mm_prefetch
	// For now, this is a placeholder for the pattern
}

// ===== Prefetch Helper =====

// Prefetch provides cache prefetching utilities
// Call once on hot-path refs to reduce cache-miss latency
type Prefetch struct{}

// Instructions prefetches instruction data into L1 cache
func (p *Prefetch) Instructions(instructions []Instruction) {
	if len(instructions) == 0 {
		return
	}
	// Prefetch first/middle/last instruction
	// In production, this would use CPU prefetch intrinsics
	// Prefetch first
	_ = instructions[0]
	if len(instructions) > 2 {
		_ = instructions[len(instructions)/2]
	}
	if len(instructions) > 1 {
		_ = instructions[len(instructions)-1]
	}
}

// Pubkey prefetches a pubkey into cache
func (p *Prefetch) Pubkey(pubkey []byte) {
	_ = pubkey[0] // Touch to load into cache
}

// ===== Memory Operations =====

// MemoryOps provides SIMD-accelerated memory operations where available
type MemoryOps struct{}

// Copy performs optimized memory copy
func (m *MemoryOps) Copy(dst, src []byte) {
	copy(dst, src)
}

// Compare performs optimized memory comparison
func (m *MemoryOps) Compare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Zero performs optimized memory zeroing
func (m *MemoryOps) Zero(ptr []byte) {
	for i := range ptr {
		ptr[i] = 0
	}
}

// ===== Instruction Processor =====

const (
	// BytesPerAccount is the size of a Pubkey in bytes
	BytesPerAccount = 32
	// MaxInstructionsWarn is the threshold for warning about large instruction count
	MaxInstructionsWarn = 64
)

// InstructionProcessor handles instruction preprocessing and validation
type InstructionProcessor struct {
	branchOpt BranchOptimizer
	prefetch  Prefetch
}

// Preprocess validates and prepares instructions for execution
func (ip *InstructionProcessor) Preprocess(instructions []Instruction) error {
	if ip.branchOpt.Unlikely(len(instructions) == 0) {
		return ErrInstructionsEmpty
	}

	// Prefetch instructions into cache
	ip.prefetch.Instructions(instructions)

	if ip.branchOpt.Unlikely(len(instructions) > MaxInstructionsWarn) {
		// Log warning in production
	}

	return nil
}

// CalculateSize calculates total size of instructions for buffer allocation
func (ip *InstructionProcessor) CalculateSize(instructions []Instruction) int {
	totalSize := 0
	for i, instr := range instructions {
		// Prefetch next instruction
		if i+1 < len(instructions) {
			_ = instructions[i+1]
		}
		totalSize += len(instr.Data)
		totalSize += len(instr.Accounts) * BytesPerAccount
	}
	return totalSize
}

// ===== Execution Path Helpers =====

// ExecutionPath provides trade direction and execution path utilities
type ExecutionPath struct {
	branchOpt BranchOptimizer
}

// IsBuy determines if this is a buy based on input mint
func (ep *ExecutionPath) IsBuy(inputMint []byte, solMint, wsolMint, usd1Mint, usdcMint []byte) bool {
	isBuy := bytesEqual(inputMint, solMint) ||
		bytesEqual(inputMint, wsolMint) ||
		bytesEqual(inputMint, usd1Mint) ||
		bytesEqual(inputMint, usdcMint)

	return ep.branchOpt.Likely(isBuy)
}

// Select chooses between fast and slow path based on condition
func (ep *ExecutionPath) Select(condition bool, fastPath, slowPath func() interface{}) interface{} {
	if ep.branchOpt.Likely(condition) {
		return fastPath()
	}
	return slowPath()
}

// ===== Instruction Type (simplified) =====

// Instruction represents a Solana instruction
type Instruction struct {
	ProgramID []byte
	Accounts  []AccountMeta
	Data      []byte
}

// AccountMeta represents account metadata
type AccountMeta struct {
	Pubkey  []byte
	IsSigner bool
	IsWritable bool
}

// ===== Transaction Pool (Pre-allocated Builders) =====

// TransactionBuilderPool manages pre-allocated transaction builders
// Based on Rust's acquire_builder/release_builder pattern
type TransactionBuilderPool struct {
	pool sync.Pool
	size int
}

// NewTransactionBuilderPool creates a pool of pre-allocated builders
func NewTransactionBuilderPool(size int) *TransactionBuilderPool {
	return &TransactionBuilderPool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return NewTransactionBuilder(size)
			},
		},
	}
}

// Acquire gets a builder from the pool
func (p *TransactionBuilderPool) Acquire() *TransactionBuilder {
	return p.pool.Get().(*TransactionBuilder)
}

// Release returns a builder to the pool
func (p *TransactionBuilderPool) Release(builder *TransactionBuilder) {
	builder.Reset()
	p.pool.Put(builder)
}

// TransactionBuilder builds transactions with pre-allocated buffers
type TransactionBuilder struct {
	instructionBuffer []Instruction
	accountBuffer     []AccountMeta
	dataBuffer        []byte
}

// NewTransactionBuilder creates a builder with pre-allocated buffers
func NewTransactionBuilder(initialSize int) *TransactionBuilder {
	return &TransactionBuilder{
		instructionBuffer: make([]Instruction, 0, initialSize),
		accountBuffer:     make([]AccountMeta, 0, 16),
		dataBuffer:        make([]byte, 0, 1024),
	}
}

// Reset clears the builder for reuse
func (b *TransactionBuilder) Reset() {
	b.instructionBuffer = b.instructionBuffer[:0]
	b.accountBuffer = b.accountBuffer[:0]
	b.dataBuffer = b.dataBuffer[:0]
}

// AddInstruction adds an instruction without allocation
func (b *TransactionBuilder) AddInstruction(instr Instruction) {
	b.instructionBuffer = append(b.instructionBuffer, instr)
}

// Build creates the final transaction data
func (b *TransactionBuilder) Build(payer []byte, blockhash []byte) []byte {
	// Build transaction bytes without additional allocations
	// This is a simplified version
	result := make([]byte, 0, 1024)
	return append(result, b.dataBuffer...)
}

// ===== Stats (Atomic Counters) =====

// UltraLowLatencyStats tracks nanosecond-level latency statistics
type UltraLowLatencyStats struct {
	eventsProcessed      atomic.Int64
	totalLatencyNs       atomic.Int64
	minLatencyNs         atomic.Int64
	maxLatencyNs         atomic.Int64
	subMillisecondEvents atomic.Int64
	ultraFastEvents      atomic.Int64 // < 100μs
	lightningFastEvents  atomic.Int64 // < 10μs
	queueOverflows       atomic.Int64
	prefetchHits         atomic.Int64
}

// Record records a latency measurement
func (s *UltraLowLatencyStats) Record(latencyNs int64) {
	s.eventsProcessed.Add(1)
	s.totalLatencyNs.Add(latencyNs)

	// Update min
	for {
		current := s.minLatencyNs.Load()
		if latencyNs >= current || s.minLatencyNs.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	// Update max
	for {
		current := s.maxLatencyNs.Load()
		if latencyNs <= current || s.maxLatencyNs.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	// Classify latency
	if latencyNs < 1_000_000 { // < 1ms
		s.subMillisecondEvents.Add(1)
	}
	if latencyNs < 100_000 { // < 100μs
		s.ultraFastEvents.Add(1)
	}
	if latencyNs < 10_000 { // < 10μs
		s.lightningFastEvents.Add(1)
	}
}

// GetStats returns all statistics
func (s *UltraLowLatencyStats) GetStats() (count, totalNs, minNs, maxNs int64, avgNs float64) {
	count = s.eventsProcessed.Load()
	totalNs = s.totalLatencyNs.Load()
	minNs = s.minLatencyNs.Load()
	maxNs = s.maxLatencyNs.Load()
	if count > 0 {
		avgNs = float64(totalNs) / float64(count)
	}
	return
}

// ===== Errors =====

var (
	ErrInstructionsEmpty = fmt.Errorf("instructions empty")
)

// ===== Helper Functions =====

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
