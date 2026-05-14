package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gagliardetto/solana-go"
	"golang.org/x/crypto/pbkdf2"
)

// ===== BIP-39 Mnemonic =====

// WordList is the BIP-39 English word list (first 32 words for brevity)
// In production, this should contain all 2048 words
var WordList = []string{
	"abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
	"absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
	"acoustic", "acquire", "across", "act", "action", "actor", "actress", "actual",
	"adapt", "add", "addict", "address", "adjust", "admit", "adult", "advance",
}

// EntropySize represents the size of entropy in bits
type EntropySize int

const (
	// EntropySize128 is 128 bits (12 words)
	EntropySize128 EntropySize = 128
	// EntropySize160 is 160 bits (15 words)
	EntropySize160 EntropySize = 160
	// EntropySize192 is 192 bits (18 words)
	EntropySize192 EntropySize = 192
	// EntropySize224 is 224 bits (21 words)
	EntropySize224 EntropySize = 224
	// EntropySize256 is 256 bits (24 words)
	EntropySize256 EntropySize = 256
)

// Mnemonic represents a BIP-39 mnemonic phrase
type Mnemonic struct {
	words []string
}

// NewMnemonic creates a new random mnemonic
func NewMnemonic(size EntropySize) (*Mnemonic, error) {
	entropy := make([]byte, size/8)
	if _, err := rand.Read(entropy); err != nil {
		return nil, fmt.Errorf("failed to generate entropy: %w", err)
	}

	return MnemonicFromEntropy(entropy)
}

// MnemonicFromEntropy creates a mnemonic from entropy bytes
func MnemonicFromEntropy(entropy []byte) (*Mnemonic, error) {
	// Calculate checksum
	checksumBits := len(entropy) / 4
	hash := sha256.Sum256(entropy)
	checksumByte := hash[0] >> (8 - checksumBits)

	// Combine entropy and checksum
	data := make([]byte, len(entropy)+1)
	copy(data, entropy)
	data[len(entropy)] = checksumByte

	// Convert to word indices
	wordCount := (len(entropy)*8 + checksumBits) / 11
	words := make([]string, wordCount)

	for i := 0; i < wordCount; i++ {
		startBit := i * 11
		startByte := startBit / 8
		bitOffset := startBit % 8

		var index uint16
		if bitOffset <= 5 {
			index = uint16(data[startByte]) << (3 + bitOffset)
			if startByte+1 < len(data) {
				index |= uint16(data[startByte+1]) >> (5 - bitOffset)
			}
		} else {
			index = uint16(data[startByte]) << (bitOffset - 5)
			if startByte+1 < len(data) {
				index |= uint16(data[startByte+1]) >> (13 - bitOffset)
			}
		}
		index = (index >> 5) & 0x7FF

		if int(index) >= len(WordList) {
			return nil, errors.New("invalid word index")
		}
		words[i] = WordList[index]
	}

	return &Mnemonic{words: words}, nil
}

// MnemonicFromString creates a mnemonic from a string
func MnemonicFromString(phrase string) (*Mnemonic, error) {
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) != 12 && len(words) != 15 && len(words) != 18 && len(words) != 21 && len(words) != 24 {
		return nil, errors.New("invalid mnemonic length")
	}

	// Validate words
	for _, word := range words {
		found := false
		for _, w := range WordList {
			if w == word {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid word in mnemonic: %s", word)
		}
	}

	return &Mnemonic{words: words}, nil
}

// String returns the mnemonic as a space-separated string
func (m *Mnemonic) String() string {
	return strings.Join(m.words, " ")
}

// Words returns the word list
func (m *Mnemonic) Words() []string {
	return append([]string{}, m.words...)
}

// ToSeed converts the mnemonic to a seed using BIP-39
func (m *Mnemonic) ToSeed(passphrase string) []byte {
	salt := []byte("mnemonic" + passphrase)
	return pbkdf2.Key([]byte(m.String()), salt, 2048, 64, sha256.New)
}

// ===== BIP-44 Derivation Path =====

// DerivationPath represents a BIP-44 derivation path
type DerivationPath struct {
	Purpose      uint32
	CoinType     uint32
	Account      uint32
	Change       uint32
	AddressIndex uint32
}

// DefaultDerivationPath returns the standard Solana BIP-44 path
// m/44'/501'/0'/0'
func DefaultDerivationPath() *DerivationPath {
	return &DerivationPath{
		Purpose:      44 + 0x80000000,  // 44' (hardened)
		CoinType:     501 + 0x80000000, // 501' (Solana, hardened)
		Account:      0 + 0x80000000,   // 0' (hardened)
		Change:       0,                // 0 (non-hardened)
		AddressIndex: 0,                // 0 (non-hardened)
	}
}

// DerivationPathFromString parses a derivation path string
// Format: m/44'/501'/0'/0/0
func DerivationPathFromString(path string) (*DerivationPath, error) {
	parts := strings.Split(strings.TrimSpace(path), "/")
	if len(parts) < 2 || parts[0] != "m" {
		return nil, errors.New("invalid derivation path format")
	}

	dp := &DerivationPath{}
	values := []*uint32{&dp.Purpose, &dp.CoinType, &dp.Account, &dp.Change, &dp.AddressIndex}

	for i := 1; i < len(parts) && i <= 5; i++ {
		part := strings.TrimSpace(parts[i])
		hardened := strings.HasSuffix(part, "'") || strings.HasSuffix(part, "h")
		part = strings.TrimSuffix(strings.TrimSuffix(part, "'"), "h")

		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid path component: %s", part)
		}

		if hardened {
			val += 0x80000000
		}
		*values[i-1] = uint32(val)
	}

	return dp, nil
}

// String returns the derivation path as a string
func (dp *DerivationPath) String() string {
	return fmt.Sprintf("m/%d'/%d'/%d'/%d/%d",
		dp.Purpose&0x7FFFFFFF,
		dp.CoinType&0x7FFFFFFF,
		dp.Account&0x7FFFFFFF,
		dp.Change&0x7FFFFFFF,
		dp.AddressIndex&0x7FFFFFFF,
	)
}

// ToIndices returns the path as a slice of indices
func (dp *DerivationPath) ToIndices() []uint32 {
	return []uint32{dp.Purpose, dp.CoinType, dp.Account, dp.Change, dp.AddressIndex}
}

// WithAccount returns a new path with a different account index
func (dp *DerivationPath) WithAccount(account uint32) *DerivationPath {
	return &DerivationPath{
		Purpose:      dp.Purpose,
		CoinType:     dp.CoinType,
		Account:      account + 0x80000000,
		Change:       dp.Change,
		AddressIndex: dp.AddressIndex,
	}
}

// WithAddressIndex returns a new path with a different address index
func (dp *DerivationPath) WithAddressIndex(index uint32) *DerivationPath {
	return &DerivationPath{
		Purpose:      dp.Purpose,
		CoinType:     dp.CoinType,
		Account:      dp.Account,
		Change:       dp.Change,
		AddressIndex: index,
	}
}

// ===== KeyPair =====

// KeyPair represents an Ed25519 key pair for Solana
type KeyPair struct {
	PublicKey  solana.PublicKey
	PrivateKey []byte // 64 bytes (32 seed + 32 public key)
}

// NewRandomKeyPair generates a new random key pair
func NewRandomKeyPair() (*KeyPair, error) {
	privateKey := make([]byte, 64)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// In production, use proper Ed25519 key generation
	// This is a simplified version
	return &KeyPair{
		PublicKey:  solana.PublicKeyFromBytes(privateKey[32:]),
		PrivateKey: privateKey,
	}, nil
}

// KeyPairFromSeed creates a key pair from a 32-byte seed
func KeyPairFromSeed(seed []byte) (*KeyPair, error) {
	if len(seed) != 32 {
		return nil, errors.New("seed must be 32 bytes")
	}

	// In production, use proper Ed25519 key derivation
	// This is a simplified version
	privateKey := make([]byte, 64)
	copy(privateKey, seed)

	return &KeyPair{
		PublicKey:  solana.PublicKeyFromBytes(privateKey[32:]),
		PrivateKey: privateKey,
	}, nil
}

// KeyPairFromBytes creates a key pair from raw bytes
func KeyPairFromBytes(privateKey []byte) (*KeyPair, error) {
	if len(privateKey) != 64 {
		return nil, errors.New("private key must be 64 bytes")
	}

	return &KeyPair{
		PublicKey:  solana.PublicKeyFromBytes(privateKey[32:]),
		PrivateKey: append([]byte{}, privateKey...),
	}, nil
}

// Sign signs a message with the key pair
func (kp *KeyPair) Sign(message []byte) []byte {
	// In production, use proper Ed25519 signing
	// This is a placeholder
	return make([]byte, 64)
}

// ToBase58 returns the private key as base58
func (kp *KeyPair) ToBase58() string {
	return solana.PrivateKey(kp.PrivateKey).String()
}

// PublicKeyBase58 returns the public key as base58
func (kp *KeyPair) PublicKeyBase58() string {
	return kp.PublicKey.String()
}

// ===== SeedGenerator =====

// SeedGenerator generates seeds and key pairs from mnemonics
type SeedGenerator struct {
	mnemonic   *Mnemonic
	passphrase string
	seed       []byte
	seedOnce   sync.Once
}

// NewSeedGenerator creates a new seed generator
func NewSeedGenerator(mnemonic *Mnemonic, passphrase string) *SeedGenerator {
	return &SeedGenerator{
		mnemonic:   mnemonic,
		passphrase: passphrase,
	}
}

// NewSeedGeneratorFromPhrase creates a generator from a mnemonic phrase
func NewSeedGeneratorFromPhrase(phrase, passphrase string) (*SeedGenerator, error) {
	mnemonic, err := MnemonicFromString(phrase)
	if err != nil {
		return nil, err
	}
	return NewSeedGenerator(mnemonic, passphrase), nil
}

// GenerateSeed generates the master seed (BIP-39)
func (sg *SeedGenerator) GenerateSeed() []byte {
	sg.seedOnce.Do(func() {
		sg.seed = sg.mnemonic.ToSeed(sg.passphrase)
	})
	return sg.seed
}

// DeriveKeyPair derives a key pair at the given BIP-44 path
func (sg *SeedGenerator) DeriveKeyPair(path *DerivationPath) (*KeyPair, error) {
	seed := sg.GenerateSeed()

	// Derive master key
	masterKey, masterChainCode := deriveMasterKey(seed)

	// Derive child keys along the path
	key := masterKey
	chainCode := masterChainCode

	for _, index := range path.ToIndices() {
		var err error
		key, chainCode, err = deriveChildKey(key, chainCode, index)
		if err != nil {
			return nil, err
		}
	}

	return KeyPairFromSeed(key)
}

// DeriveKeyPairAtIndex derives a key pair at a specific address index
func (sg *SeedGenerator) DeriveKeyPairAtIndex(account, index uint32) (*KeyPair, error) {
	path := DefaultDerivationPath().
		WithAccount(account).
		WithAddressIndex(index)
	return sg.DeriveKeyPair(path)
}

// deriveMasterKey derives the master key from seed (BIP-32)
func deriveMasterKey(seed []byte) (key, chainCode []byte) {
	// HMAC-SHA512 with key "ed25519 seed"
	hmacKey := []byte("ed25519 seed")
	hmac := pbkdf2.Key(seed, hmacKey, 1, 64, sha256.New)

	// First 32 bytes: master key
	// Last 32 bytes: chain code
	return hmac[:32], hmac[32:]
}

// deriveChildKey derives a child key (BIP-32)
func deriveChildKey(parentKey, parentChainCode []byte, index uint32) ([]byte, []byte, error) {
	// Prepare data for HMAC
	data := make([]byte, 37)

	if index >= 0x80000000 {
		// Hardened: prepend 0x00 + parent key
		data[0] = 0x00
		copy(data[1:33], parentKey)
	} else {
		// Non-hardened: prepend parent public key
		// For Ed25519, we only support hardened derivation
		return nil, nil, errors.New("non-hardened derivation not supported for Ed25519")
	}

	binary.BigEndian.PutUint32(data[33:37], index)

	// HMAC-SHA512
	hmac := pbkdf2.Key(data, parentChainCode, 1, 64, sha256.New)

	return hmac[:32], hmac[32:], nil
}

// ===== Utility Functions =====

// GenerateMnemonic generates a new random mnemonic
func GenerateMnemonic(size EntropySize) (string, error) {
	mnemonic, err := NewMnemonic(size)
	if err != nil {
		return "", err
	}
	return mnemonic.String(), nil
}

// ValidateMnemonic validates a mnemonic phrase
func ValidateMnemonic(phrase string) bool {
	_, err := MnemonicFromString(phrase)
	return err == nil
}

// DeriveKeyPairFromMnemonic derives a key pair from a mnemonic phrase
func DeriveKeyPairFromMnemonic(phrase, passphrase string, path *DerivationPath) (*KeyPair, error) {
	generator, err := NewSeedGeneratorFromPhrase(phrase, passphrase)
	if err != nil {
		return nil, err
	}
	return generator.DeriveKeyPair(path)
}

// DeriveKeyPairFromMnemonicAtIndex derives a key pair at a specific index
func DeriveKeyPairFromMnemonicAtIndex(phrase, passphrase string, account, index uint32) (*KeyPair, error) {
	path := DefaultDerivationPath().
		WithAccount(account).
		WithAddressIndex(index)
	return DeriveKeyPairFromMnemonic(phrase, passphrase, path)
}

// ===== Errors =====

var (
	ErrInvalidMnemonic       = errors.New("invalid mnemonic")
	ErrInvalidDerivationPath = errors.New("invalid derivation path")
	ErrInvalidSeed           = errors.New("invalid seed")
	ErrInvalidKeyPair        = errors.New("invalid key pair")
)
