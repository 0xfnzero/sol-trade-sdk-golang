// Package security provides secure key storage and input validation for Sol Trade SDK
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// ===== Errors =====

var (
	ErrKeyNotAvailable     = errors.New("key not available")
	ErrPasswordRequired    = errors.New("password required to unlock")
	ErrInvalidKey          = errors.New("invalid key")
	ErrValidationFailed    = errors.New("validation failed")
	ErrInvalidURL          = errors.New("invalid URL")
	ErrInvalidPubkey       = errors.New("invalid public key")
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrInvalidSlippage     = errors.New("invalid slippage")
	ErrTransactionTooLarge = errors.New("transaction too large")
)

// ===== Secure Key Storage =====

// KeyMetadata contains metadata about a stored key
type KeyMetadata struct {
	Pubkey       string
	CreatedAt    time.Time
	LastAccessed *time.Time
	AccessCount  int
}

// SecureKeyStorage provides secure storage for Solana private keys
type SecureKeyStorage struct {
	mu                sync.Mutex
	encryptedKey      []byte
	salt              []byte
	pubkey            string
	passwordProtected bool
	metadata          *KeyMetadata
}

// NewSecureKeyStorage creates a new secure key storage
func NewSecureKeyStorage() *SecureKeyStorage {
	return &SecureKeyStorage{}
}

// FromPrivateKey creates secure storage from a private key
func (s *SecureKeyStorage) FromPrivateKey(privateKey solana.PrivateKey, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pubkey = privateKey.PublicKey().String()

	// Get secret key bytes
	secretBytes := make([]byte, len(privateKey))
	copy(secretBytes, privateKey[:])

	defer s.secureZero(secretBytes)

	// Generate salt
	s.salt = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, s.salt); err != nil {
		return err
	}

	if password != "" {
		s.passwordProtected = true
		encrypted, err := s.encryptWithPassword(secretBytes, password, s.salt)
		if err != nil {
			return err
		}
		s.encryptedKey = encrypted
	} else {
		// Simple XOR encryption
		s.encryptedKey = s.xorEncrypt(secretBytes, s.salt)
	}

	s.metadata = &KeyMetadata{
		Pubkey:    s.pubkey,
		CreatedAt: time.Now(),
	}

	return nil
}

// FromSeed creates secure storage from a seed
func (s *SecureKeyStorage) FromSeed(seed []byte, password string) error {
	if len(seed) != 32 {
		return ErrInvalidKey
	}

	privateKey := solana.PrivateKey(seed)
	return s.FromPrivateKey(privateKey, password)
}

// Unlock temporarily unlocks the key and returns a callback to access it
func (s *SecureKeyStorage) Unlock(password string) (solana.PrivateKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encryptedKey == nil {
		return nil, ErrKeyNotAvailable
	}

	if s.passwordProtected && password == "" {
		return nil, ErrPasswordRequired
	}

	var decrypted []byte
	var err error

	if s.passwordProtected {
		decrypted, err = s.decryptWithPassword(s.encryptedKey, password, s.salt)
	} else {
		decrypted = s.xorEncrypt(s.encryptedKey, s.salt)
	}

	if err != nil {
		return nil, err
	}

	// Update metadata
	now := time.Now()
	if s.metadata != nil {
		s.metadata.LastAccessed = &now
		s.metadata.AccessCount++
	}

	privateKey := solana.PrivateKey(decrypted)
	return privateKey, nil
}

// SignMessage signs a message without exposing the keypair
func (s *SecureKeyStorage) SignMessage(message []byte, password string) (solana.Signature, error) {
	privateKey, err := s.Unlock(password)
	if err != nil {
		return solana.Signature{}, err
	}
	defer s.secureZero(privateKey)

	signature, err := privateKey.Sign(message)
	if err != nil {
		return solana.Signature{}, err
	}

	return signature, nil
}

// Pubkey returns the public key
func (s *SecureKeyStorage) Pubkey() string {
	return s.pubkey
}

// IsPasswordProtected returns true if storage requires password
func (s *SecureKeyStorage) IsPasswordProtected() bool {
	return s.passwordProtected
}

// Metadata returns key metadata
func (s *SecureKeyStorage) Metadata() *KeyMetadata {
	return s.metadata
}

// Clear permanently clears all key material
func (s *SecureKeyStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encryptedKey != nil {
		s.secureZero(s.encryptedKey)
		s.encryptedKey = nil
	}
	if s.salt != nil {
		s.secureZero(s.salt)
		s.salt = nil
	}
	s.pubkey = ""
	s.metadata = nil
}

func (s *SecureKeyStorage) encryptWithPassword(data []byte, password string, salt []byte) ([]byte, error) {
	// Derive key from password
	key := deriveKey(password, salt)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

func (s *SecureKeyStorage) decryptWithPassword(encrypted []byte, password string, salt []byte) ([]byte, error) {
	// Derive key from password
	key := deriveKey(password, salt)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("encrypted data too short")
	}

	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	// Decrypt
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

func (s *SecureKeyStorage) xorEncrypt(data []byte, key []byte) []byte {
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ key[i%len(key)]
	}
	return result
}

func (s *SecureKeyStorage) secureZero(data []byte) {
	for i := range data {
		data[i] = 0
	}
	for i := range data {
		data[i] = 0xFF
	}
	for i := range data {
		data[i] = 0
	}
}

func deriveKey(password string, salt []byte) []byte {
	// Simple key derivation (use PBKDF2 or Argon2 in production)
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	return h.Sum(nil)
}

// ===== Validators =====

// Known program IDs
var KnownProgramIDs = map[string][]string{
	"pumpfun": {
		"6EF8rrecthR5Dkzon8Nwu78hRvfCKopJFfWcCzNfXt3D",
	},
	"pumpswap": {
		"pAMMBay6oceH9fJKBRdGP4LmVn7LKwEqT7dPWn1oLKs",
	},
	"raydium": {
		"CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK",
		"675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8",
	},
	"meteora": {
		"MERLuDFBMmsHnsBPZw2sDQZHvXFM4sPkHePSuUZnPdK",
	},
	"system": {
		"11111111111111111111111111111111",
		"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
		"TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb",
		"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL",
	},
}

// ValidateRPCURL validates an RPC URL
func ValidateRPCURL(rawURL string, allowHTTP bool) (string, error) {
	if rawURL == "" {
		return "", ErrInvalidURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", ErrInvalidURL
	}

	// Check scheme
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", ErrInvalidURL
	}

	if parsed.Scheme == "http" && !allowHTTP {
		return "", errors.New("HTTP RPC URLs are insecure, use HTTPS or set allowHTTP=true")
	}

	// Check hostname
	if parsed.Hostname() == "" {
		return "", ErrInvalidURL
	}

	// Block private IPs
	hostname := strings.ToLower(parsed.Hostname())
	privatePatterns := []string{
		`^127\.`,
		`^10\.`,
		`^172\.(1[6-9]|2[0-9]|3[01])\.`,
		`^192\.168\.`,
		`^0\.`,
		`^localhost$`,
	}

	for _, pattern := range privatePatterns {
		matched, _ := regexp.MatchString(pattern, hostname)
		if matched {
			return "", errors.New("private IP/localhost RPC URLs not allowed for security")
		}
	}

	return parsed.String(), nil
}

// ValidatePubkey validates a Solana public key
func ValidatePubkey(pubkey string, name string) error {
	if pubkey == "" {
		return errors.New(name + " cannot be empty")
	}

	// Check base58 format
	base58Chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range pubkey {
		if !strings.ContainsRune(base58Chars, c) {
			return ErrInvalidPubkey
		}
	}

	// Check length
	if len(pubkey) < 32 || len(pubkey) > 48 {
		return ErrInvalidPubkey
	}

	return nil
}

// ValidateAmount validates an amount value
func ValidateAmount(amount uint64, name string, allowZero bool) error {
	if amount == 0 && !allowZero {
		return ErrInvalidAmount
	}

	// Check for overflow
	maxSafe := uint64(1<<63 - 1)
	if amount > maxSafe {
		return ErrInvalidAmount
	}

	return nil
}

// ValidateSlippage validates slippage in basis points
func ValidateSlippage(slippageBasisPoints uint64) error {
	if slippageBasisPoints >= 10000 {
		return ErrInvalidSlippage
	}
	return nil
}

// ValidateProgramID validates a program ID
func ValidateProgramID(programID string, expectedProgram string) error {
	if err := ValidatePubkey(programID, "program_id"); err != nil {
		return err
	}

	if expectedProgram != "" {
		expectedIDs, ok := KnownProgramIDs[strings.ToLower(expectedProgram)]
		if ok {
			found := false
			for _, id := range expectedIDs {
				if id == programID {
					found = true
					break
				}
			}
			if !found {
				return errors.New("program ID does not match expected program")
			}
		}
	}

	return nil
}

// ValidateMintPair validates a trading pair
func ValidateMintPair(inputMint, outputMint string) error {
	if err := ValidatePubkey(inputMint, "input_mint"); err != nil {
		return err
	}
	if err := ValidatePubkey(outputMint, "output_mint"); err != nil {
		return err
	}
	if inputMint == outputMint {
		return errors.New("input and output mint cannot be the same")
	}
	return nil
}

// ValidateTransactionSize validates transaction size
func ValidateTransactionSize(txBytes []byte, maxSize int) error {
	if maxSize == 0 {
		maxSize = 1232 // Default Solana max
	}
	if len(txBytes) > maxSize {
		return ErrTransactionTooLarge
	}
	return nil
}

// ===== Hex encoding helpers =====

// HexEncode encodes bytes to hex string
func HexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// HexDecode decodes hex string to bytes
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
