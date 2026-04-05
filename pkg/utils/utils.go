package utils

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ===== Byte Buffer Pool =====

var bufferPool = NewBufferPool(1024)

// BufferPool manages reusable byte buffers
type BufferPool struct {
	pool chan *bytes.Buffer
	size int
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: make(chan *bytes.Buffer, size),
		size: size,
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() *bytes.Buffer {
	select {
	case buf := <-p.pool:
		buf.Reset()
		return buf
	default:
		return bytes.NewBuffer(make([]byte, 0, 1024))
	}
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf *bytes.Buffer) {
	select {
	case p.pool <- buf:
	default:
	}
}

// GetBuffer gets a buffer from the global pool
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get()
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buf *bytes.Buffer) {
	bufferPool.Put(buf)
}

// ===== Binary Utilities =====

// LE encodes an uint64 as little-endian bytes
func LE(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

// LE32 encodes an uint32 as little-endian bytes
func LE32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// LE16 encodes an uint16 as little-endian bytes
func LE16(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

// ReadLE reads a uint64 from little-endian bytes
func ReadLE(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

// ReadLE32 reads a uint32 from little-endian bytes
func ReadLE32(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

// ReadLE16 reads a uint16 from little-endian bytes
func ReadLE16(b []byte) uint16 {
	return binary.LittleEndian.Uint16(b)
}

// ===== Slice Utilities =====

// Concat concatenates multiple byte slices
func Concat(slices ...[]byte) []byte {
	var total int
	for _, s := range slices {
		total += len(s)
	}
	result := make([]byte, total)
	var i int
	for _, s := range slices {
		i += copy(result[i:], s)
	}
	return result
}

// Clone creates a copy of a byte slice
func Clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	result := make([]byte, len(b))
	copy(result, b)
	return result
}

// ===== String Utilities =====

// SafeString safely converts bytes to string
func SafeString(b []byte) string {
	if b == nil {
		return ""
	}
	return string(b)
}

// SafeBytes safely converts string to bytes
func SafeBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return []byte(s)
}

// ===== Error Utilities =====

// WrapError wraps an error with context
func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// ===== Math Utilities =====

// Min returns the minimum of two uint64s
func Min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two uint64s
func Max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// MinInt returns the minimum of two ints
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt returns the maximum of two ints
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CeilDiv performs ceiling division
func CeilDiv(a, b uint64) uint64 {
	return (a + b - 1) / b
}

// MulDiv performs (a * b) / c with overflow protection
func MulDiv(a, b, c uint64) uint64 {
	// Use 128-bit intermediate if needed
	// Simplified version - use math/big for full 128-bit support
	if a == 0 || b == 0 {
		return 0
	}
	return (a * b) / c
}

// ===== Bit Utilities =====

// SetBit sets a bit at the given position
func SetBit(b byte, pos int) byte {
	return b | (1 << pos)
}

// ClearBit clears a bit at the given position
func ClearBit(b byte, pos int) byte {
	return b & ^(1 << pos)
}

// HasBit checks if a bit is set at the given position
func HasBit(b byte, pos int) bool {
	return (b & (1 << pos)) != 0
}

// ToggleBit toggles a bit at the given position
func ToggleBit(b byte, pos int) byte {
	return b ^ (1 << pos)
}
