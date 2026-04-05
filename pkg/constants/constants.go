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
	TOKEN_PROGRAM       = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	TOKEN_PROGRAM_2022  = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
)

// Token mints
var (
	SOL_TOKEN_ACCOUNT   = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111111")
	WSOL_TOKEN_ACCOUNT  = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	USD1_TOKEN_ACCOUNT  = solana.MustPublicKeyFromBase58("USD1ttGY1N17NEEHLmELoaybftRBUSErhqYiQzvEmuB")
	USDC_TOKEN_ACCOUNT  = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
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
	PUMPFUN_PROGRAM_ID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKopJFfWcCzNfXt3D")
)

// PumpSwap (Pump AMM) program
var (
	PUMPSWAP_PROGRAM_ID = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwq52pCSbAhL")
)

// Bonk program
var (
	BONK_PROGRAM_ID = solana.MustPublicKeyFromBase58("bonk2zCzQaobPKMKsM5Rut46yHp3zQD1ntUk8Ld8ARq")
)

// Raydium CPMM program
var (
	RAYDIUM_CPMM_PROGRAM_ID = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTUBBfMTm5L2AhwDtLd6P4VeXvgQA2Po")
)

// Raydium AMM V4 program
var (
	RAYDIUM_AMM_V4_PROGRAM_ID = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
)

// Meteora DAMM v2 program
var (
	METEORA_DAMM_V2_PROGRAM_ID = solana.MustPublicKeyFromBase58("Eo7WjKq67rjJQSZxSbzmZ8p2UA3LJi5y6vZr3rP5L8k1")
)

// PumpFun constants
const (
	// Default slippage in basis points (5%)
	DEFAULT_SLIPPAGE = 500

	// PumpFun instruction discriminators
	BUY_DISCRIMINATOR            = [8]byte{102, 6, 141, 196, 242, 95, 28, 167}
	SELL_DISCRIMINATOR           = [8]byte{187, 75, 56, 100, 133, 176, 22, 141}
	BUY_EXACT_SOL_IN_DISCRIMINATOR = [8]byte{133, 104, 247, 38, 153, 106, 73, 253}
)

// Compute budget constants
const (
	DEFAULT_COMPUTE_UNITS = 200000
	DEFAULT_PRIORITY_FEE  = 100000
	DEFAULT_TIP_LAMPORTS  = 100000
)
