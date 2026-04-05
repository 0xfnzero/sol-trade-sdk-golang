package seed

import (
	"crypto/sha256"
	"errors"

	"github.com/gagliardetto/solana-go"
)

// ===== PDA Derivation =====

// FindProgramAddress finds a program-derived address
func FindProgramAddress(seeds [][]byte, programID solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(seeds, programID)
}

// CreateProgramAddress creates a program-derived address without bump
func CreateProgramAddress(seeds [][]byte, programID solana.PublicKey) (solana.PublicKey, error) {
	return solana.CreateProgramAddress(seeds, programID)
}

// ===== PumpFun PDAs =====

// PumpFun Program ID
var PumpFunProgramID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKFJdMZzMMTrWr1Bv")

// GetBondingCurvePDA returns the bonding curve PDA for a mint
func GetBondingCurvePDA(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint[:],
	}
	return FindProgramAddress(seeds, PumpFunProgramID)
}

// GetGlobalAccountPDA returns the global account PDA
func GetGlobalAccountPDA() (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("global"),
	}
	return FindProgramAddress(seeds, PumpFunProgramID)
}

// GetFeeRecipientPDA returns the fee recipient PDA
func GetFeeRecipientPDA(isMayhemMode bool) (solana.PublicKey, uint8, error) {
	var seed []byte
	if isMayhemMode {
		seed = []byte("fee_recipient_mayhem")
	} else {
		seed = []byte("fee_recipient")
	}
	seeds := [][]byte{seed}
	return FindProgramAddress(seeds, PumpFunProgramID)
}

// GetEventAuthorityPDA returns the event authority PDA
func GetEventAuthorityPDA() (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("event"),
	}
	return FindProgramAddress(seeds, PumpFunProgramID)
}

// GetUserVolumeAccumulatorPDA returns the user volume accumulator PDA
func GetUserVolumeAccumulatorPDA(user solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("user_volume_accumulator"),
		user[:],
	}
	return FindProgramAddress(seeds, PumpFunProgramID)
}

// ===== PumpSwap PDAs =====

// PumpSwap Program ID
var PumpSwapProgramID = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKFRHoe4LvJhu5yQJtezhkEL5DHyJ")

// GetPumpSwapPoolPDA returns the pool PDA
func GetPumpSwapPoolPDA(baseMint, quoteMint solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("pool"),
		baseMint[:],
		quoteMint[:],
	}
	return FindProgramAddress(seeds, PumpSwapProgramID)
}

// ===== Raydium PDAs =====

// Raydium AMM V4 Program ID
var RaydiumAmmV4ProgramID = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")

// Raydium CPMM Program ID
var RaydiumCpmmProgramID = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")

// GetRaydiumAmmAuthorityPDA returns the AMM authority PDA
func GetRaydiumAmmAuthorityPDA() (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("amm authority"),
	}
	return FindProgramAddress(seeds, RaydiumAmmV4ProgramID)
}

// GetRaydiumCpmmPoolPDA returns the CPMM pool PDA
func GetRaydiumCpmmPoolPDA(ammConfig, baseMint, quoteMint solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("pool"),
		ammConfig[:],
		baseMint[:],
		quoteMint[:],
	}
	return FindProgramAddress(seeds, RaydiumCpmmProgramID)
}

// ===== Meteora PDAs =====

// Meteora DAMM V2 Program ID
var MeteoraDammV2ProgramID = solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")

// GetMeteoraPoolPDA returns the Meteora pool PDA
func GetMeteoraPoolPDA(tokenAMint, tokenBMint solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("pool"),
		tokenAMint[:],
		tokenBMint[:],
	}
	return FindProgramAddress(seeds, MeteoraDammV2ProgramID)
}

// ===== Associated Token Account =====

// AssociatedTokenProgramID is the Associated Token Account program ID
var AssociatedTokenProgramID = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

// TokenProgramID is the SPL Token program ID
var TokenProgramID = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

// Token2022ProgramID is the Token-2022 program ID
var Token2022ProgramID = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

// GetAssociatedTokenAddress returns the ATA for a wallet and mint
func GetAssociatedTokenAddress(wallet, mint solana.PublicKey, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		wallet[:],
		tokenProgram[:],
		mint[:],
	}
	addr, _, err := FindProgramAddress(seeds, AssociatedTokenProgramID)
	return addr, err
}

// ===== Seed-based ATA Creation =====

// SeedATA represents a seed-based ATA with pre-computed address
type SeedATA struct {
	Address    solana.PublicKey
	Mint       solana.PublicKey
	Owner      solana.PublicKey
	Bump       uint8
	Seed       []byte
	Exists     bool
	RentExempt uint64
}

// CreateSeedATA creates a seed-based ATA
func CreateSeedATA(owner, mint solana.PublicKey, seed []byte, tokenProgram solana.PublicKey) (*SeedATA, error) {
	seeds := [][]byte{
		owner[:],
		tokenProgram[:],
		mint[:],
		seed,
	}
	addr, bump, err := FindProgramAddress(seeds, AssociatedTokenProgramID)
	if err != nil {
		return nil, err
	}

	return &SeedATA{
		Address: addr,
		Mint:    mint,
		Owner:   owner,
		Bump:    bump,
		Seed:    seed,
	}, nil
}

// ===== Hash Utilities =====

// Hash256 computes SHA-256 hash
func Hash256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// Hash256Concat concatenates and hashes
func Hash256Concat(parts ...[]byte) [32]byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// ===== Error Definitions =====

var (
	ErrInvalidSeed       = errors.New("invalid seed")
	ErrInvalidBump       = errors.New("invalid bump")
	ErrPDANotFound       = errors.New("PDA not found")
	ErrInvalidProgramID  = errors.New("invalid program ID")
)
