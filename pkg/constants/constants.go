package constants

import (
	"github.com/gagliardetto/solana-go"
)

// System program
var (
	SYSTEM_PROGRAM = solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
)

// Token programs
var (
	TOKEN_PROGRAM      = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	TOKEN_PROGRAM_2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
)

// Token mints
var (
	SOL_TOKEN_ACCOUNT  = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111111")
	WSOL_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	USD1_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("USD1ttGY1N17NEEHLmELoaybftRBUSErhqYiQzvEmuB")
	USDC_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
)

// Associated token program
var (
	ASSOCIATED_TOKEN_PROGRAM_ID = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
)

// Rent sysvar
var (
	RENT = solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
)

// PumpFun program
var (
	PUMPFUN_PROGRAM_ID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
)

// PumpSwap (Pump AMM) program
var (
	PUMPSWAP_PROGRAM_ID = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
)

// Bonk program
var (
	BONK_PROGRAM_ID = solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
)

// Raydium CPMM program
var (
	RAYDIUM_CPMM_PROGRAM_ID = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")
)

// Raydium AMM V4 program
var (
	RAYDIUM_AMM_V4_PROGRAM_ID = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
)

// Meteora DAMM v2 program
var (
	METEORA_DAMM_V2_PROGRAM_ID = solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")
)

// PumpFun constants
const (
	// Default slippage in basis points (5%)
	DEFAULT_SLIPPAGE = 500
)

// PumpFun instruction discriminators
var (
	BUY_DISCRIMINATOR              = [8]byte{102, 6, 61, 18, 1, 218, 235, 234}
	SELL_DISCRIMINATOR             = [8]byte{51, 230, 133, 164, 1, 127, 131, 173}
	BUY_EXACT_SOL_IN_DISCRIMINATOR = [8]byte{56, 252, 116, 8, 158, 223, 205, 95}
)

// Compute budget constants
const (
	DEFAULT_COMPUTE_UNITS = 200000
	DEFAULT_PRIORITY_FEE  = 100000
	DEFAULT_TIP_LAMPORTS  = 100000
)
