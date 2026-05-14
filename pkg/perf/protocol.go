package perf

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"sort"
)

// TransactionConfig configuration for optimized transactions
type TransactionConfig struct {
	ComputeUnitLimit    uint32
	ComputeUnitPrice    uint64
	SkipPreflight       bool
	MaxRetries          uint8
	PreflightCommitment string
	Encoding            string
}

// DefaultTransactionConfig returns default config
func DefaultTransactionConfig() *TransactionConfig {
	return &TransactionConfig{
		ComputeUnitLimit:    1400000,
		ComputeUnitPrice:    0,
		SkipPreflight:       true,
		MaxRetries:          0,
		PreflightCommitment: "processed",
		Encoding:            "base64",
	}
}

// OptimizedInstruction represents an optimized instruction
type OptimizedInstruction struct {
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

// Serialize serializes instruction to bytes
func (i *OptimizedInstruction) Serialize() []byte {
	buf := make([]byte, 0, 1024)

	// Program ID
	buf = append(buf, i.ProgramID[:]...)

	// Account count
	buf = append(buf, byte(len(i.Accounts)))

	// Accounts
	for _, acc := range i.Accounts {
		buf = append(buf, acc.Pubkey[:]...)
		if acc.IsSigner {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
		if acc.IsWritable {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	}

	// Data length
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(i.Data)))

	// Data
	buf = append(buf, i.Data...)

	return buf
}

// OptimizedTransaction represents an optimized transaction
type OptimizedTransaction struct {
	Signatures      [][64]byte
	Message         []byte
	Instructions    []OptimizedInstruction
	RecentBlockhash [32]byte
	FeePayer        [32]byte
}

// Serialize serializes transaction
func (t *OptimizedTransaction) Serialize() []byte {
	buf := make([]byte, 0, 4096)

	// Signatures
	buf = append(buf, byte(len(t.Signatures)))
	for _, sig := range t.Signatures {
		buf = append(buf, byte(len(sig)))
		buf = append(buf, sig[:]...)
	}

	// Message
	buf = append(buf, t.Message...)

	return buf
}

// ToBase64 converts to base64
func (t *OptimizedTransaction) ToBase64() string {
	return base64.StdEncoding.EncodeToString(t.Serialize())
}

// TransactionBuilder builds optimized transactions
type TransactionBuilder struct {
	config       *TransactionConfig
	instructions []OptimizedInstruction
	signers      map[[32]byte][]byte
}

// NewTransactionBuilder creates a transaction builder
func NewTransactionBuilder(config *TransactionConfig) *TransactionBuilder {
	if config == nil {
		config = DefaultTransactionConfig()
	}
	return &TransactionBuilder{
		config:       config,
		instructions: make([]OptimizedInstruction, 0),
		signers:      make(map[[32]byte][]byte),
	}
}

// AddInstruction adds an instruction
func (b *TransactionBuilder) AddInstruction(inst OptimizedInstruction) {
	b.instructions = append(b.instructions, inst)
}

// AddSigner adds a signer
func (b *TransactionBuilder) AddSigner(pubkey [32]byte, secretKey []byte) {
	b.signers[pubkey] = secretKey
}

// Build builds the transaction
func (b *TransactionBuilder) Build(recentBlockhash [32]byte, feePayer *[32]byte) *OptimizedTransaction {
	payer := *feePayer
	if payer == [32]byte{} {
		// Use first signer as fee payer
		for pk := range b.signers {
			payer = pk
			break
		}
	}

	message := b.buildMessage(recentBlockhash, payer)
	signatures := b.signMessage(message)

	return &OptimizedTransaction{
		Signatures:      signatures,
		Message:         message,
		Instructions:    b.instructions,
		RecentBlockhash: recentBlockhash,
		FeePayer:        payer,
	}
}

func (b *TransactionBuilder) buildMessage(recentBlockhash [32]byte, feePayer [32]byte) []byte {
	buf := make([]byte, 0, 2048)

	// Version header (legacy)
	buf = append(buf, 0)

	// Header
	numSigners := len(b.signers)
	buf = append(buf, byte(numSigners))
	buf = append(buf, 0) // Readonly signed accounts
	buf = append(buf, 0) // Readonly unsigned accounts

	// Account keys
	accountKeys := [][32]byte{feePayer}
	keyIndex := make(map[[32]byte]int)
	keyIndex[feePayer] = 0

	for _, inst := range b.instructions {
		if _, ok := keyIndex[inst.ProgramID]; !ok {
			keyIndex[inst.ProgramID] = len(accountKeys)
			accountKeys = append(accountKeys, inst.ProgramID)
		}
		for _, acc := range inst.Accounts {
			if _, ok := keyIndex[acc.Pubkey]; !ok {
				keyIndex[acc.Pubkey] = len(accountKeys)
				accountKeys = append(accountKeys, acc.Pubkey)
			}
		}
	}

	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(accountKeys)))
	for _, key := range accountKeys {
		buf = append(buf, key[:]...)
	}

	// Recent blockhash
	buf = append(buf, recentBlockhash[:]...)

	// Instructions
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(b.instructions)))
	for _, inst := range b.instructions {
		programIDIndex := keyIndex[inst.ProgramID]
		buf = append(buf, byte(programIDIndex))

		accountIndices := make([]byte, len(inst.Accounts))
		for i, acc := range inst.Accounts {
			accountIndices[i] = byte(keyIndex[acc.Pubkey])
		}
		buf = append(buf, byte(len(accountIndices)))
		buf = append(buf, accountIndices...)

		buf = binary.LittleEndian.AppendUint16(buf, uint16(len(inst.Data)))
		buf = append(buf, inst.Data...)
	}

	return buf
}

func (b *TransactionBuilder) signMessage(message []byte) [][64]byte {
	keys := make([][32]byte, 0, len(b.signers))
	for pubkey := range b.signers {
		keys = append(keys, pubkey)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i][:], keys[j][:]) < 0
	})

	signatures := make([][64]byte, 0, len(keys))
	for _, pubkey := range keys {
		privateKey, ok := normalizeEd25519PrivateKey(b.signers[pubkey])
		if !ok {
			continue
		}
		sig := ed25519.Sign(privateKey, message)
		var fixed [64]byte
		copy(fixed[:], sig)
		signatures = append(signatures, fixed)
	}

	return signatures
}

func normalizeEd25519PrivateKey(secretKey []byte) (ed25519.PrivateKey, bool) {
	switch len(secretKey) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(secretKey), true
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(secretKey), true
	default:
		return nil, false
	}
}

// SerializationOptimizer provides optimized serialization
type SerializationOptimizer struct{}

// EncodeU64 encodes uint64 (little endian)
func (s *SerializationOptimizer) EncodeU64(v uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf
}

// EncodeU128 encodes uint128 (little endian)
func (s *SerializationOptimizer) EncodeU128(v [16]byte) []byte {
	return v[:]
}

// EncodeCompactU16 encodes compact u16
func (s *SerializationOptimizer) EncodeCompactU16(v uint16) []byte {
	if v < 0x80 {
		return []byte{byte(v)}
	} else if v < 0x4000 {
		return []byte{byte(v&0x7F | 0x80), byte(v >> 7)}
	} else {
		return []byte{byte(v&0x7F | 0x80), byte((v>>7)&0x7F | 0x80), byte(v >> 14)}
	}
}

// DecodeCompactU16 decodes compact u16
func (s *SerializationOptimizer) DecodeCompactU16(data []byte, offset int) (uint16, int) {
	var value uint16
	var shift uint
	bytesRead := 0

	for {
		if offset+bytesRead >= len(data) {
			return 0, 0
		}

		b := data[offset+bytesRead]
		bytesRead++

		value |= uint16(b&0x7F) << shift

		if b&0x80 == 0 {
			break
		}

		shift += 7
		if shift > 14 {
			return 0, 0
		}
	}

	return value, bytesRead
}

// ComputeBudgetOptimizer provides compute budget optimization
type ComputeBudgetOptimizer struct{}

// ComputeBudget program ID
var ComputeBudgetProgramID = [32]byte{
	0x06, 0xa1, 0xfc, 0xf1, 0x91, 0x9c, 0x05, 0xeb,
	0xba, 0x07, 0x9a, 0x22, 0xa6, 0x4e, 0x8e, 0x5f,
	0x6d, 0x97, 0x89, 0x0d, 0xec, 0x90, 0x91, 0x27,
	0xac, 0x8d, 0x47, 0x16, 0x42, 0xad, 0x04, 0x08,
}

// SetComputeUnitLimit creates compute unit limit instruction
func (c *ComputeBudgetOptimizer) SetComputeUnitLimit(units uint32) OptimizedInstruction {
	data := make([]byte, 5)
	data[0] = 2 // Instruction discriminator
	binary.LittleEndian.PutUint32(data[1:], units)

	return OptimizedInstruction{
		ProgramID: ComputeBudgetProgramID,
		Data:      data,
	}
}

// SetComputeUnitPrice creates compute unit price instruction
func (c *ComputeBudgetOptimizer) SetComputeUnitPrice(microLamports uint64) OptimizedInstruction {
	data := make([]byte, 9)
	data[0] = 3 // Instruction discriminator
	binary.LittleEndian.PutUint64(data[1:], microLamports)

	return OptimizedInstruction{
		ProgramID: ComputeBudgetProgramID,
		Data:      data,
	}
}
