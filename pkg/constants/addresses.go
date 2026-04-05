package constants

import "github.com/gagliardetto/solana-go"

// Fee recipients
var (
	// Standard fee recipient for PumpFun
	FEE_RECIPIENT = MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4Cs9tM")

	// Mayhem fee recipients (random selection supported)
	MAYHEM_FEE_RECIPIENTS = []solana.PublicKey{
		MustPublicKeyFromBase58("7VtWHe8WJeU9Sy5j1XF5n8qPzDtJjWxMgYVtJ89AQrVj"),
		MustPublicKeyFromBase58("82jN8eGgPvMSW1KP9W6GdW4bQ3YbB7sGgC6BhZnLVQvR"),
	}
)

// PumpSwap fee recipients
var (
	PUMPSWAP_FEE_RECIPIENT = MustPublicKeyFromBase58("7VtWHe8WJeU9Sy5j1XF5n8qPzDtJjWxMgYVtJ89AQrVj")
)

// MustPublicKeyFromBase58 parses a base58 public key or panics
func MustPublicKeyFromBase58(s string) solana.PublicKey {
	pk, err := solana.PublicKeyFromBase58(s)
	if err != nil {
		panic(err)
	}
	return pk
}
