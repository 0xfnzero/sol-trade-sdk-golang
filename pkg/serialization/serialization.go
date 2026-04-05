// Transaction serialization module.
// Based on sol-trade-sdk Rust implementation with buffer pooling.

package serialization

import (
	"encoding/base64"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
)

// ===== Errors =====

var (
	ErrUnsupportedEncoding = errors.New("unsupported encoding")
)

// ===== Constants =====

// SerializerPoolSize is the max number of reusable buffers in the queue
const SerializerPoolSize = 10000

// SerializerBufferSize is the per-buffer reserved capacity (bytes)
const SerializerBufferSize = 256 * 1024

// SerializerPrewarmBuffers is the cold-start prewarm count
const SerializerPrewarmBuffers = 64

// ===== Zero-Allocation Serializer =====

// ZeroAllocSerializer uses a buffer pool to avoid runtime allocation
type ZeroAllocSerializer struct {
	bufferPool *sync.Pool
	bufferSize int
	poolStats  poolStats
}

type poolStats struct {
	available atomic.Int64
	capacity  int64
}

// NewZeroAllocSerializer creates a new zero-allocation serializer
func NewZeroAllocSerializer(poolSize, bufferSize int) *ZeroAllocSerializer {
	return NewZeroAllocSerializerWithPrewarm(poolSize, bufferSize, SerializerPrewarmBuffers)
}

// NewZeroAllocSerializerWithPrewarm creates a serializer with custom prewarm count
func NewZeroAllocSerializerWithPrewarm(poolSize, bufferSize, prewarmBuffers int) *ZeroAllocSerializer {
	s := &ZeroAllocSerializer{
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, bufferSize)
			},
		},
		bufferSize: bufferSize,
	}
	s.poolStats.capacity = int64(poolSize)

	// Prewarm only a small hot set
	prewarmCount := prewarmBuffers
	if prewarmCount > poolSize {
		prewarmCount = poolSize
	}
	for i := 0; i < prewarmCount; i++ {
		buf := make([]byte, 0, bufferSize)
		s.bufferPool.Put(buf)
		s.poolStats.available.Add(1)
	}

	return s
}

// SerializeZeroAlloc serializes data using a pooled buffer
func (s *ZeroAllocSerializer) SerializeZeroAlloc(data []byte) []byte {
	// Get buffer from pool
	buf := s.bufferPool.Get().([]byte)
	s.poolStats.available.Add(-1)

	// Reset and copy data
	buf = buf[:0]
	buf = append(buf, data...)

	return buf
}

// ReturnBuffer returns a buffer to the pool
func (s *ZeroAllocSerializer) ReturnBuffer(buf []byte) {
	// Reset buffer
	buf = buf[:0]
	s.bufferPool.Put(buf)
	s.poolStats.available.Add(1)
}

// GetPoolStats returns pool statistics
func (s *ZeroAllocSerializer) GetPoolStats() (available, capacity int64) {
	return s.poolStats.available.Load(), s.poolStats.capacity
}

// ===== Global Serializer Instance =====

var globalSerializer = NewZeroAllocSerializer(SerializerPoolSize, SerializerBufferSize)

// ===== Base64 Encoder =====

// Base64Encoder provides optimized base64 encoding
type Base64Encoder struct{}

// Encode encodes data to base64
func (e *Base64Encoder) Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// EncodeFast encodes data using pooled buffer
func (e *Base64Encoder) EncodeFast(data []byte) string {
	// Estimate base64 size: ceil(n/3) * 4
	encodedLen := base64.StdEncoding.EncodedLen(len(data))
	buf := make([]byte, encodedLen)
	base64.StdEncoding.Encode(buf, data)
	return string(buf)
}

// ===== PooledTxBufferGuard =====

// PooledTxBufferGuard returns buffer to pool on release
type PooledTxBufferGuard struct {
	buffer     []byte
	serializer *ZeroAllocSerializer
}

// NewPooledTxBufferGuard creates a guarded buffer
func NewPooledTxBufferGuard(data []byte, serializer *ZeroAllocSerializer) *PooledTxBufferGuard {
	return &PooledTxBufferGuard{
		buffer:     serializer.SerializeZeroAlloc(data),
		serializer: serializer,
	}
}

// Bytes returns the underlying buffer
func (g *PooledTxBufferGuard) Bytes() []byte {
	return g.buffer
}

// Release returns the buffer to the pool
func (g *PooledTxBufferGuard) Release() {
	if g.buffer != nil {
		g.serializer.ReturnBuffer(g.buffer)
		g.buffer = nil
	}
}

// ===== Transaction Serialization =====

// TransactionEncoding represents encoding type
type TransactionEncoding int

const (
	EncodingBase58 TransactionEncoding = iota
	EncodingBase64
)

// SerializeTransactionSync serializes a transaction using buffer pool
func SerializeTransactionSync(transaction []byte, encoding TransactionEncoding) (string, error) {
	serialized := globalSerializer.SerializeZeroAlloc(transaction)
	defer globalSerializer.ReturnBuffer(serialized)

	switch encoding {
	case EncodingBase58:
		return encodeBase58(serialized), nil
	case EncodingBase64:
		return base64.StdEncoding.EncodeToString(serialized), nil
	default:
		return "", ErrUnsupportedEncoding
	}
}

// SerializeTransactionBatchSync serializes multiple transactions
func SerializeTransactionBatchSync(transactions [][]byte, encoding TransactionEncoding) ([]string, error) {
	results := make([]string, 0, len(transactions))
	for _, tx := range transactions {
		encoded, err := SerializeTransactionSync(tx, encoding)
		if err != nil {
			return nil, err
		}
		results = append(results, encoded)
	}
	return results, nil
}

// ===== Base58 Encoding =====

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// encodeBase58 encodes bytes to base58 string
func encodeBase58(input []byte) string {
	// Count leading zeros
	leadingZeros := 0
	for _, b := range input {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// Convert to base58
	num := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}

	// Add leading '1's for each leading zero byte
	for i := 0; i < leadingZeros; i++ {
		result = append(result, '1')
	}

	// Reverse result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// ===== Get Statistics =====

// GetSerializerStats returns global serializer statistics
func GetSerializerStats() (available, capacity int64) {
	return globalSerializer.GetPoolStats()
}
