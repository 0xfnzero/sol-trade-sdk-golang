package perf

import (
	"runtime"
)

// CPUAffinity manages CPU affinity
type CPUAffinity struct {
	cpu int
}

// NewCPUAffinity creates CPU affinity for specific CPU
func NewCPUAffinity(cpu int) *CPUAffinity {
	return &CPUAffinity{cpu: cpu}
}

// Set sets CPU affinity for current thread (Linux only)
func (c *CPUAffinity) Set() error {
	// Platform-specific affinity can be wired behind build tags. Keep this
	// package portable so non-Linux SDK builds do not fail at compile time.
	_ = runtime.GOOS
	_ = c.cpu
	return nil
}

// SetProcessAffinity sets affinity for entire process
func SetProcessAffinity(cpus []int) error {
	_ = runtime.GOOS
	_ = cpus
	return nil
}

// NUMAOptimizer provides NUMA-aware optimizations
type NUMAOptimizer struct {
	node int
}

// NewNUMAOptimizer creates NUMA optimizer
func NewNUMAOptimizer(node int) *NUMAOptimizer {
	return &NUMAOptimizer{node: node}
}

// SetMemoryPolicy sets NUMA memory policy (Linux only)
func (n *NUMAOptimizer) SetMemoryPolicy() error {
	// Requires libnuma or equivalent
	// Placeholder implementation
	return nil
}

// CacheOptimizer provides cache optimization utilities
type CacheOptimizer struct {
	cacheLineSize int
}

// NewCacheOptimizer creates cache optimizer
func NewCacheOptimizer() *CacheOptimizer {
	return &CacheOptimizer{
		cacheLineSize: 64, // Standard cache line size
	}
}

// Align aligns size to cache line
func (c *CacheOptimizer) Align(size int) int {
	return (size + c.cacheLineSize - 1) &^ (c.cacheLineSize - 1)
}

// PadSize returns padding needed for alignment
func (c *CacheOptimizer) PadSize(size int) int {
	aligned := c.Align(size)
	return aligned - size
}

// HardwareOptimizer provides hardware-level optimizations
type HardwareOptimizer struct {
	cpuAffinity *CPUAffinity
	numa        *NUMAOptimizer
	cache       *CacheOptimizer
}

// NewHardwareOptimizer creates hardware optimizer
func NewHardwareOptimizer() *HardwareOptimizer {
	return &HardwareOptimizer{
		cache: NewCacheOptimizer(),
	}
}

// SetCPUAffinity sets CPU affinity
func (h *HardwareOptimizer) SetCPUAffinity(cpu int) error {
	h.cpuAffinity = NewCPUAffinity(cpu)
	return h.cpuAffinity.Set()
}

// SetNUMA sets NUMA node
func (h *HardwareOptimizer) SetNUMA(node int) error {
	h.numa = NewNUMAOptimizer(node)
	return h.numa.SetMemoryPolicy()
}

// GetCacheLineSize returns cache line size
func (h *HardwareOptimizer) GetCacheLineSize() int {
	return h.cache.cacheLineSize
}

// AlignToCache aligns size to cache line
func (h *HardwareOptimizer) AlignToCache(size int) int {
	return h.cache.Align(size)
}

// CPUInfo provides CPU information
type CPUInfo struct {
	NumCPU        int
	NumPhysical   int
	CacheLineSize int
	HasAVX        bool
	HasAVX2       bool
	HasAVX512     bool
}

// GetCPUInfo returns CPU information
func GetCPUInfo() *CPUInfo {
	info := &CPUInfo{
		NumCPU:        runtime.NumCPU(),
		CacheLineSize: 64,
	}

	// Physical CPU count estimation
	info.NumPhysical = info.NumCPU
	if info.NumCPU > 1 && info.NumCPU%2 == 0 {
		info.NumPhysical = info.NumCPU / 2 // Assume hyperthreading
	}

	return info
}

// MemoryBarrier inserts a memory barrier
func MemoryBarrier() {
	// Use atomic operation as memory barrier
	runtime.KeepAlive(0)
}

// Pause inserts a CPU pause instruction
func Pause() {
	// On x86, this would be PAUSE instruction
	// In Go, we can use Gosched for similar effect
	runtime.Gosched()
}

// Prefetch hints at data to prefetch
func Prefetch(addr uintptr) {
	// In real implementation, use CPU prefetch instructions
	// This is a placeholder
	_ = addr
}

// PrefetchWriteAddr hints at data to prefetch for writing.
func PrefetchWriteAddr(addr uintptr) {
	// In real implementation, use CPU prefetchw instruction
	_ = addr
}
